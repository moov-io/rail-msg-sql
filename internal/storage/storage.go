package storage

import (
	"context"

	"github.com/moov-io/ach"
)

type Repository interface {
	ListFiles(ctx context.Context, params FilterParams) ([]*ach.File, error)
}
