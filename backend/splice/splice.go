package splice

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
	return "splice"
}

func (Backend) Serve(ctx context.Context, w http.ResponseWriter, spec backend.Spec) error {
	spec = spec.WithDefaults()
	if err := spec.Validate(); err != nil {
		return err
	}
	return serve(ctx, w, spec)
}
