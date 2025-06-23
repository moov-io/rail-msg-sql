package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moov-io/ach"
)

type filesystemRepository struct {
	config FilesystemConfig
}

var _ Repository = (&filesystemRepository{})

func (r *filesystemRepository) ListAchFiles(ctx context.Context, params FilterParams) ([]*ach.File, error) {
	var out []*ach.File

	for _, dir := range r.config.Directories {
		files, err := r.readFiles(ctx, dir)
		if err != nil {
			return nil, fmt.Errorf("reading %s failed: %w", dir, err)
		}

		out = append(out, files...)
	}

	return out, nil
}

func (r *filesystemRepository) readFiles(ctx context.Context, dir string) ([]*ach.File, error) {
	var out []*ach.File

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("accessing %s failed: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".ach", ".txt":
			fd, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("opening %s failed: %w", path, err)
			}
			defer fd.Close()

			rdr := ach.NewReader(fd)
			rdr.SetValidation(r.config.AchValidateOpts)

			file, err := rdr.Read()
			if err != nil {
				return fmt.Errorf("reading %s (as Nacha) failed: %w", path, err)
			}
			out = append(out, &file)

		case ".json":
			file, err := ach.ReadJSONFileWith(path, r.config.AchValidateOpts)
			if err != nil {
				return fmt.Errorf("reading %s (as JSON) failed: %w", path, err)
			}
			out = append(out, file)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s failed: %w", dir, err)
	}

	return out, nil
}
