package iocopy

import (
	"context"
	"net/http"

	"benchmark-splice/backend"
)

type Backend struct{}

func New() Backend {
	return Backend{}
}

func (Backend) Name() string {
	return "io_copy"
}

func (Backend) Serve(ctx context.Context, w http.ResponseWriter, spec backend.Spec) error {
	spec = spec.WithDefaults()
	if err := spec.Validate(); err != nil {
		return err
	}

	backend.WriteDownloadHeaders(w, spec.TotalBytes)
	for i := 0; i < spec.Requests(); i++ {
		if err := backend.CopyLimited(ctx, w, spec, spec.BytesForChunk(i)); err != nil {
			return err
		}
	}
	return nil
}
