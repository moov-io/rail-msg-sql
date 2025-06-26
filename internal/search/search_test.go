package search_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/moov-io/ach"
	achwebviewer "github.com/moov-io/ach-web-viewer/pkg/service"
	"github.com/moov-io/base/log"
	"github.com/moov-io/rail-msg-sql/internal/search"
	"github.com/moov-io/rail-msg-sql/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestSearch_ACH(t *testing.T) {
	logger := log.NewTestLogger()

	fileStorage, err := storage.NewRepository(storage.Config{
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

	conf := search.Config{
		SqliteFilepath: filepath.Join(t.TempDir(), "ach.db"),
	}

	svc, err := search.NewService(logger, conf, fileStorage)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, svc.Close())
	})

	ctx := context.Background()
	params := storage.FilterParams{
		StartDate: time.Date(2018, time.September, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC),
	}

	// Ingest multiple times (to skip duplicates)
	err = svc.IngestACHFiles(ctx, params)
	require.NoError(t, err)

	err = svc.IngestACHFiles(ctx, params)
	require.NoError(t, err)

	cases := []struct {
		name          string
		query         string
		expected      search.Results
		expectedError error
	}{
		{
			name: "basic WHERE",
			query: `
SELECT ach_entries.trace_number, return_code
FROM ach_entries
INNER JOIN ach_addendas
WHERE amount > 12500 AND return_code = 'R03'
GROUP BY ach_entries.trace_number
ORDER BY ach_entries.id ASC`,
			expected: search.Results{
				Headers: search.Row{
					Columns: []interface{}{"trace_number", "return_code"},
				},
				Rows: []search.Row{
					{Columns: []interface{}{"121042880000001", "R03"}},
					{Columns: []interface{}{"088888880123459", "R03"}},
					{Columns: []interface{}{"088888880123460", "R03"}},
					{Columns: []interface{}{"081000030000004", "R03"}},
					{Columns: []interface{}{"081000030000005", "R03"}},
				},
			},
		},
		{
			name:  "basic SUM MIN MAX",
			query: `select sum(amount), min(amount), max(amount) from ach_entries where amount > 12.34;`,
			expected: search.Results{
				Headers: search.Row{
					Columns: []interface{}{"sum(amount)", "min(amount)", "max(amount)"},
				},
				Rows: []search.Row{
					{Columns: []interface{}{int64(300358904), int64(19), int64(100000000)}},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			var params storage.FilterParams

			results, err := svc.Search(ctx, tc.query, params)
			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			require.NotNil(t, results)

			require.Equal(t, len(tc.expected.Headers.Columns), len(results.Headers.Columns))
			require.Equal(t, len(tc.expected.Rows), len(results.Rows))

			for idx := range results.Headers.Columns {
				require.Equal(t, tc.expected.Headers.Columns[idx], results.Headers.Columns[idx])
			}
			for idx := range results.Rows {
				require.Equal(t, len(tc.expected.Rows[idx].Columns), len(results.Rows[idx].Columns))

				for c := range results.Rows[idx].Columns {
					expected := tc.expected.Rows[idx].Columns[c]
					got := results.Rows[idx].Columns[c]

					msg := fmt.Sprintf("results[%d], column[%d] - %q  vs  %q", idx, c, expected, got)
					require.Equal(t, expected, got, msg)
				}
			}
		})
	}
}
