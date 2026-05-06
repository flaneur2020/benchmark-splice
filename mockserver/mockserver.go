package mockserver

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"benchmark-splice/backend"
)

type Handler struct {
	chunk       []byte
	bytesServed atomic.Uint64
}

func New(size int64) (*Handler, error) {
	if size <= 0 {
		size = backend.DefaultChunkSize
	}
	chunk := make([]byte, size)
	if _, err := rand.Read(chunk); err != nil {
		return nil, fmt.Errorf("generate random chunk: %w", err)
	}
	return &Handler{chunk: chunk}, nil
}

func NewWithChunk(chunk []byte) *Handler {
	return &Handler{chunk: append([]byte(nil), chunk...)}
}

func (h *Handler) BytesServed() uint64 {
	return h.bytesServed.Load()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/chunk" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(h.chunk)))
	w.WriteHeader(http.StatusOK)
	n, _ := w.Write(h.chunk)
	h.bytesServed.Add(uint64(n))
}
