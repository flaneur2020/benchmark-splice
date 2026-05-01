package benchmarksplice_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"benchmark-splice/backend"
	"benchmark-splice/backend/iocopy"
	"benchmark-splice/backend/prefetch"
	"benchmark-splice/backend/splice"
	"benchmark-splice/mockserver"
	"benchmark-splice/proxy"
)

const benchmarkBytes = backend.DefaultChunkSize * backend.DefaultChunks

func BenchmarkDownload512Chunks(b *testing.B) {
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
		b.Run(tc.Name(), func(b *testing.B) {
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
			b.SetBytes(benchmarkBytes)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				resp, err := client.Get(url)
				if err != nil {
					b.Fatal(err)
				}
				n, copyErr := io.Copy(io.Discard, resp.Body)
				closeErr := resp.Body.Close()
				if copyErr != nil {
					b.Fatal(copyErr)
				}
				if closeErr != nil {
					b.Fatal(closeErr)
				}
				if resp.StatusCode != http.StatusOK {
					b.Fatalf("status %d", resp.StatusCode)
				}
				if n != benchmarkBytes {
					b.Fatalf("downloaded %d bytes, want %d", n, benchmarkBytes)
				}
			}
		})
	}
}
