package storage

import (
	"context"
	"fmt"
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

func (r *MultiRepository) ListAchFiles(ctx context.Context, params FilterParams) ([]File, error) {
	var out []File

	for idx := range r.repos {
		files, err := r.repos[idx].ListAchFiles(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("listing ACH files from %T failed: %w", r.repos[idx], err)
		}
		out = append(out, files...)
	}

	return out, nil
}
