package storage_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/moov-io/ach"
	achwebviewer "github.com/moov-io/ach-web-viewer/pkg/service"
	"github.com/moov-io/rail-msg-sql/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestRepository(t *testing.T) {
	repo, err := storage.NewRepository(storage.Config{
		ACH: achwebviewer.Sources{
			{
				Filesystem: &achwebviewer.FilesystemConfig{
					Paths: []string{
						filepath.Join("testdata", "ach"),
					},
				},
			},
		},
		ACHValidateOpts: &ach.ValidateOpts{
			AllowMissingFileHeader:  true,
			AllowMissingFileControl: true,
		},
	})
	require.NoError(t, err)

	// Change dir so filelist package lets us read the dir
	t.Chdir(filepath.Join("..", "search"))

	ctx := context.Background()
	params := storage.FilterParams{
		StartDate: time.Date(2018, time.September, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	files, err := repo.ListAchFiles(ctx, params)
	require.NoError(t, err)
	require.Greater(t, len(files), 5)
}
