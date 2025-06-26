package storage

import (
	"context"
)

type Repository interface {
	ListAchFiles(ctx context.Context, params FilterParams) ([]File, error)
}
