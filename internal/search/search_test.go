package search_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/base/log"
	"github.com/moov-io/rail-msg-sql/internal/search"
	"github.com/moov-io/rail-msg-sql/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestSearch_ACH(t *testing.T) {
	logger := log.NewTestLogger()

	fileStorage, err := storage.NewRepositories(storage.Config{
		Filesystem: &storage.FilesystemConfig{
			Directories: []string{
				filepath.Join("..", "..", "testdata", "ach"),
			},
			AchValidateOpts: &ach.ValidateOpts{
				AllowMissingFileHeader:  true,
				AllowMissingFileControl: true,
			},
		},
	})
	require.NoError(t, err)

	svc, err := search.NewService(logger, fileStorage)
	require.NoError(t, err)

	cases := []struct {
		name          string
		query         string
		expected      search.Results
		expectedError error
	}{
		{
			name:  "basic WHERE",
			query: `SELECT trace_number, return_code FROM ach_files WHERE amount > 25;`,
			expected: search.Results{
				Headers: search.Row{
					Columns: []interface{}{"trace_number", "return_code"},
				},
				Rows: []search.Row{
					{Columns: []interface{}{"", "R14"}},
					{Columns: []interface{}{nil, nil}},
					{Columns: []interface{}{"273976361273620", "R02"}},
					{Columns: []interface{}{"121042880000001", nil}},
					{Columns: []interface{}{"231380100000001", nil}},
					{Columns: []interface{}{"121042880000001", nil}},
					{Columns: []interface{}{"121042880000001", nil}},
					{Columns: []interface{}{"121042880000001", nil}},
					{Columns: []interface{}{"088888880123459", nil}},
					{Columns: []interface{}{"091000017611242", "R01"}},
					{Columns: []interface{}{"081000030000000", nil}},
				},
			},
		},
		{
			name:  "basic SUM MIN MAX",
			query: `select sum(amount), min(amount), max(amount) from ach_files where amount > 12.34;`,
			expected: search.Results{
				Headers: search.Row{
					Columns: []interface{}{"SUM(amount)", "MIN(amount)", "MAX(amount)"},
				},
				Rows: []search.Row{
					{Columns: []interface{}{12.34, 0.01, 0.01}},
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

					msg := fmt.Sprintf("results[%d], column[%d] - %#v  vs  %#v", idx, c, expected, got)
					require.Equal(t, expected, got, msg)
				}
			}
		})
	}
}
