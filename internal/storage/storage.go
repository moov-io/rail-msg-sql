package storage

import (
	"context"

	"github.com/moov-io/ach"
)

type Repository interface {
	ListAchFiles(ctx context.Context, params FilterParams) ([]*ach.File, error)
}
