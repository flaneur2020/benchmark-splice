package prefetch

import (
	"context"
	"net/http"

	"benchmark-splice/backend"
)

type Backend struct {
	ReadAhead int
}

func New(readAhead int) Backend {
	if readAhead < 0 {
		readAhead = 0
	}
	return Backend{ReadAhead: readAhead}
}

func (b Backend) Name() string {
	return "prefetch"
}

func (b Backend) Serve(ctx context.Context, w http.ResponseWriter, spec backend.Spec) error {
	spec = spec.WithDefaults()
	if err := spec.Validate(); err != nil {
		return err
	}
	readAhead := b.ReadAhead
	if readAhead == 0 {
		readAhead = backend.DefaultReadAhead
	}

	backend.WriteDownloadHeaders(w, spec.TotalBytes)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	requests := spec.Requests()
	scheduled := make([]bool, requests)
	futures := make(map[int]*future, min(requests, readAhead))

	for i := 0; i < requests; i++ {
		chunk, err := waitOrFetch(ctx, spec, i, futures)
		if err != nil {
			return err
		}

		for ahead := 1; ahead <= readAhead; ahead++ {
			idx := i + ahead
			if idx >= requests || scheduled[idx] {
				continue
			}
			scheduled[idx] = true
			futures[idx] = startFetch(ctx, spec, idx)
		}

		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

type future struct {
	done chan result
}

type result struct {
	buf []byte
	err error
}

func waitOrFetch(ctx context.Context, spec backend.Spec, index int, futures map[int]*future) ([]byte, error) {
	if f, ok := futures[index]; ok {
		delete(futures, index)
		res := <-f.done
		return res.buf, res.err
	}
	return fetch(ctx, spec, index)
}

func startFetch(ctx context.Context, spec backend.Spec, index int) *future {
	f := &future{done: make(chan result, 1)}
	go func() {
		buf, err := fetch(ctx, spec, index)
		f.done <- result{buf: buf, err: err}
	}()
	return f
}

func fetch(ctx context.Context, spec backend.Spec, index int) ([]byte, error) {
	return backend.ReadN(ctx, spec, spec.BytesForChunk(index))
}
