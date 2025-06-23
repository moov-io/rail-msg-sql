package storage

import (
	"context"
	"fmt"

	"github.com/moov-io/ach"
)

type MultiRepository struct {
	repos []Repository
}

var _ Repository = (&MultiRepository{})

func NewRepositories(config Config) (Repository, error) {
	var repos []Repository

	if config.Filesystem != nil {
		repos = append(repos, &filesystemRepository{
			config: *config.Filesystem,
		})
	}

	return &MultiRepository{
		repos: repos,
	}, nil
}

func (r *MultiRepository) ListFiles(ctx context.Context, params FilterParams) ([]*ach.File, error) {
	var out []*ach.File

	for idx := range r.repos {
		files, err := r.repos[idx].ListFiles(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("listing files from %T failed: %w", r.repos[idx], err)
		}
		out = append(out, files...)
	}

	return out, nil
}
