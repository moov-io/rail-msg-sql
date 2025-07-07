package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/moov-io/ach"
	"github.com/moov-io/ach-web-viewer/pkg/filelist"
	"github.com/moov-io/base/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

func (r *Repository) ListAchFiles(ctx context.Context, params FilterParams) ([]FileListing, error) {
	ctx, span := telemetry.StartSpan(ctx, "storage-list-ach-files", trace.WithAttributes(
		attribute.String("filter.start", params.StartDate.Format(time.RFC3339)),
		attribute.String("filter.end", params.EndDate.Format(time.RFC3339)),
		attribute.String("filter.pattern", params.Pattern),
	))
	defer span.End()

	resp, err := r.ach.GetFiles(ctx, filelist.ListOpts{
		StartDate: params.StartDate,
		EndDate:   params.EndDate,
		Pattern:   params.Pattern,
	})
	if err != nil {
		return nil, fmt.Errorf("problem getting ACH files: %w", err)
	}

	var out []FileListing
	for _, fs := range resp {
		for idx := range fs.Files {
			out = append(out, FileListing{
				Name:        fs.Files[idx].Name,
				StoragePath: fs.Files[idx].StoragePath,
				SourceID:    fs.SourceID,
			})
		}
	}
	return out, nil
}

func (r *Repository) GetAchFile(ctx context.Context, listing FileListing) (*File, error) {
	ctx, span := telemetry.StartSpan(ctx, "storage-get-ach-file", trace.WithAttributes(
		attribute.String("listing.name", listing.Name),
		attribute.String("listing.storage_path", listing.StoragePath),
		attribute.String("listing.source_id", listing.SourceID),
	))
	defer span.End()

	// Grab the File
	path := filepath.Join(listing.StoragePath, listing.Name)

	file, err := r.ach.GetFile(ctx, listing.SourceID, path)
	if err != nil {
		return nil, fmt.Errorf("opening %s failed: %w", path, err)
	}

	return &File{
		Filename: file.Name,
		Contents: file.Contents,
	}, nil
}
