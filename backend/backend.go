package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const (
	DefaultChunkSize = 10 * 1024 * 1024
	DefaultChunks    = 512
	DefaultReadAhead = 4
)

type Handler interface {
	Name() string
	Serve(ctx context.Context, w http.ResponseWriter, spec Spec) error
}

type Spec struct {
	UpstreamURL string
	ChunkSize   int64
	Chunks      int
	TotalBytes  int64
	Client      *http.Client
}

func (s Spec) WithDefaults() Spec {
	if s.ChunkSize <= 0 {
		s.ChunkSize = DefaultChunkSize
	}
	if s.Chunks <= 0 {
		s.Chunks = DefaultChunks
	}
	if s.TotalBytes <= 0 {
		s.TotalBytes = int64(s.Chunks) * s.ChunkSize
	}
	if s.Client == nil {
		s.Client = DefaultHTTPClient()
	}
	return s
}

func (s Spec) Validate() error {
	if s.UpstreamURL == "" {
		return fmt.Errorf("upstream URL is required")
	}
	if s.ChunkSize <= 0 {
		return fmt.Errorf("chunk size must be positive")
	}
	if s.TotalBytes < 0 {
		return fmt.Errorf("total bytes must not be negative")
	}
	return nil
}

func (s Spec) Requests() int {
	if s.TotalBytes <= 0 || s.ChunkSize <= 0 {
		return 0
	}
	return int((s.TotalBytes + s.ChunkSize - 1) / s.ChunkSize)
}

func (s Spec) BytesForChunk(index int) int64 {
	start := int64(index) * s.ChunkSize
	remaining := s.TotalBytes - start
	if remaining <= 0 {
		return 0
	}
	if remaining > s.ChunkSize {
		return s.ChunkSize
	}
	return remaining
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}

func WriteDownloadHeaders(w http.ResponseWriter, totalBytes int64) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(totalBytes, 10))
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusOK)
}

func OpenChunk(ctx context.Context, client *http.Client, upstreamURL string) (*http.Response, error) {
	if client == nil {
		client = DefaultHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	return resp, nil
}

func CopyLimited(ctx context.Context, dst io.Writer, spec Spec, n int64) error {
	if n <= 0 {
		return nil
	}
	resp, err := OpenChunk(ctx, spec.Client, spec.UpstreamURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	written, err := io.Copy(dst, io.LimitReader(resp.Body, n))
	if err != nil {
		return fmt.Errorf("copy chunk: wrote %d of %d bytes: %w", written, n, err)
	}
	if written != n {
		return fmt.Errorf("copy chunk: wrote %d bytes, want %d", written, n)
	}
	return nil
}

func ReadN(ctx context.Context, spec Spec, n int64) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	resp, err := OpenChunk(ctx, spec.Client, spec.UpstreamURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, n)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) != n {
		return nil, fmt.Errorf("read chunk: got %d bytes, want %d", len(buf), n)
	}
	return buf, nil
}
