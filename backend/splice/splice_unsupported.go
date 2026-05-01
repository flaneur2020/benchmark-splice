//go:build !linux

package splice

import (
	"context"
	"fmt"
	"net/http"

	"benchmark-splice/backend"
)

func Supported() bool {
	return false
}

func serve(context.Context, http.ResponseWriter, backend.Spec) error {
	return fmt.Errorf("splice backend is only supported on linux")
}
