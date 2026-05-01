package proxy_test

import (
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

func TestDownloadBackends(t *testing.T) {
	chunk := make([]byte, 1024)
	for i := range chunk {
		chunk[i] = byte(i)
	}
	mockServer := httptest.NewServer(mockserver.NewWithChunk(chunk))
	defer mockServer.Close()

	cases := []backend.Handler{
		prefetch.New(2),
		iocopy.New(),
		splice.New(),
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name(), func(t *testing.T) {
			if tc.Name() == "splice" && !splice.Supported() {
				t.Skip("splice backend is only supported on linux")
			}
			proxyServer := httptest.NewServer(proxy.New(
				mockServer.URL+"/chunk",
				tc,
				proxy.WithChunkSize(int64(len(chunk))),
				proxy.WithChunks(4),
			))
			defer proxyServer.Close()

			resp, err := http.Get(proxyServer.URL + "/download?bytes=2500")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status %d", resp.StatusCode)
			}
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 2500 {
				t.Fatalf("len=%d, want 2500", len(got))
			}
			for i, b := range got {
				if b != chunk[i%len(chunk)] {
					t.Fatalf("byte %d=%d, want %d", i, b, chunk[i%len(chunk)])
				}
			}
		})
	}
}
