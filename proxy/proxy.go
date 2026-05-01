package proxy

import (
	"fmt"
	"net/http"
	"strconv"

	"benchmark-splice/backend"
)

type Handler struct {
	upstreamURL string
	backend     backend.Handler
	chunkSize   int64
	chunks      int
	client      *http.Client
}

type Option func(*Handler)

func New(upstreamURL string, h backend.Handler, opts ...Option) *Handler {
	handler := &Handler{
		upstreamURL: upstreamURL,
		backend:     h,
		chunkSize:   backend.DefaultChunkSize,
		chunks:      backend.DefaultChunks,
		client:      backend.DefaultHTTPClient(),
	}
	for _, opt := range opts {
		opt(handler)
	}
	return handler
}

func WithChunkSize(size int64) Option {
	return func(h *Handler) {
		h.chunkSize = size
	}
}

func WithChunks(chunks int) Option {
	return func(h *Handler) {
		h.chunks = chunks
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(h *Handler) {
		h.client = client
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/download" {
		http.NotFound(w, r)
		return
	}
	if h.backend == nil {
		http.Error(w, "backend is required", http.StatusInternalServerError)
		return
	}

	spec, err := h.spec(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.backend.Serve(r.Context(), w, spec); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
}

func (h *Handler) spec(r *http.Request) (backend.Spec, error) {
	chunkSize, err := int64Param(r, "chunk_size", h.chunkSize)
	if err != nil {
		return backend.Spec{}, err
	}
	chunks64, err := int64Param(r, "chunks", int64(h.chunks))
	if err != nil {
		return backend.Spec{}, err
	}
	if chunks64 <= 0 {
		return backend.Spec{}, fmt.Errorf("chunks must be positive")
	}
	totalBytes := chunks64 * chunkSize
	if raw := r.URL.Query().Get("bytes"); raw != "" {
		totalBytes, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return backend.Spec{}, fmt.Errorf("invalid bytes %q", raw)
		}
		if totalBytes < 0 {
			return backend.Spec{}, fmt.Errorf("bytes must not be negative")
		}
		chunks64 = (totalBytes + chunkSize - 1) / chunkSize
	}
	if chunks64 > int64(int(chunks64)) {
		return backend.Spec{}, fmt.Errorf("chunks is too large")
	}
	return backend.Spec{
		UpstreamURL: h.upstreamURL,
		ChunkSize:   chunkSize,
		Chunks:      int(chunks64),
		TotalBytes:  totalBytes,
		Client:      h.client,
	}.WithDefaults(), nil
}

func int64Param(r *http.Request, name string, fallback int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", name, raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return value, nil
}
