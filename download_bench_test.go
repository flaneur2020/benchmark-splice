package benchmarksplice_test

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"benchmark-splice/backend"
	"benchmark-splice/backend/iocopy"
	"benchmark-splice/backend/prefetch"
	"benchmark-splice/backend/splice"
	"benchmark-splice/mockserver"
	"benchmark-splice/proxy"
)

const benchmarkBytes = backend.DefaultChunkSize * backend.DefaultChunks

var clientConcurrency = flag.Int("client-concurrency", 1, "number of concurrent benchmark client downloads")

func BenchmarkDownload512Chunks(b *testing.B) {
	concurrency := *clientConcurrency
	if concurrency <= 0 {
		b.Fatalf("-client-concurrency must be positive, got %d", concurrency)
	}

	mock, err := mockserver.New(backend.DefaultChunkSize)
	if err != nil {
		b.Fatal(err)
	}
	mockServer := httptest.NewServer(mock)
	defer mockServer.Close()

	cases := []backend.Handler{
		prefetch.New(backend.DefaultReadAhead),
		iocopy.New(),
		splice.New(),
	}

	for _, tc := range cases {
		tc := tc
		name := tc.Name()
		if concurrency > 1 {
			name = fmt.Sprintf("%s/client_concurrency_%d", name, concurrency)
		}
		b.Run(name, func(b *testing.B) {
			if tc.Name() == "splice" && !splice.Supported() {
				b.Skip("splice backend is only supported on linux")
			}

			proxyServer := httptest.NewServer(proxy.New(mockServer.URL+"/chunk", tc))
			defer proxyServer.Close()

			client := &http.Client{
				Transport: &http.Transport{DisableKeepAlives: true},
				Timeout:   0,
			}
			url := fmt.Sprintf("%s/download", proxyServer.URL)

			b.ReportAllocs()
			b.SetBytes(benchmarkBytes * int64(concurrency))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := downloadConcurrently(client, url, concurrency); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func downloadConcurrently(client *http.Client, url string, concurrency int) error {
	start := make(chan struct{})
	errs := make(chan error, concurrency)

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			<-start
			if err := downloadOnce(client, url); err != nil {
				errs <- err
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func downloadOnce(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	n, copyErr := io.Copy(io.Discard, resp.Body)
	closeErr := resp.Body.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if n != benchmarkBytes {
		return fmt.Errorf("downloaded %d bytes, want %d", n, benchmarkBytes)
	}
	return nil
}
