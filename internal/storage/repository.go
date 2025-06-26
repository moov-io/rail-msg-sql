package storage

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/moov-io/ach"
	"github.com/moov-io/ach-web-viewer/pkg/filelist"
)

type Repository struct {
	ach filelist.Listers

	achValidateOpts *ach.ValidateOpts
}

func NewRepository(config Config) (*Repository, error) {
	out := &Repository{}

	if len(config.ACH) > 0 {
		ls, err := filelist.NewListers(config.ACH)
		if err != nil {
			return nil, fmt.Errorf("creating ach filelist: %w", err)
		}

		out.ach = ls
		out.achValidateOpts = config.ACHValidateOpts
	}

	return out, nil
}

func (r *Repository) ListAchFiles(ctx context.Context, params FilterParams) ([]File, error) {
	resp, err := r.ach.GetFiles(filelist.ListOpts{
		StartDate: params.StartDate,
		EndDate:   params.EndDate,
		Pattern:   params.Pattern,
	})
	if err != nil {
		return nil, fmt.Errorf("problem getting ACH files: %w", err)
	}

	var out []File
	for _, fs := range resp {
		for idx := range fs.Files {
			// Grab the File
			path := filepath.Join(fs.Files[idx].StoragePath, fs.Files[idx].Name)

			file, err := r.ach.GetFile(fs.SourceID, path)
			if err != nil {
				return nil, fmt.Errorf("opening %s failed: %w", path, err)
			}

			out = append(out, File{
				Filename: file.Name,
				File:     file.Contents,
			})
		}
	}
	return out, nil
}
