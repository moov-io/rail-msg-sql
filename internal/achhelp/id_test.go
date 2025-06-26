package achhelp_test

import (
	"path/filepath"
	"testing"

	"github.com/moov-io/ach"
	"github.com/moov-io/rail-msg-sql/internal/achhelp"

	"github.com/stretchr/testify/require"
)

func TestPopulatedIDs(t *testing.T) {
	file, err := ach.ReadFile(filepath.Join("..", "search", "testdata", "ach", "ppd-debit.ach"))
	require.NoError(t, err)

	file = achhelp.PopulateIDs(file)
	require.NotNil(t, file)

	// File ID
	require.Equal(t, "5256dc55cab0a28d9ee8d2ccddec55d97477372573caff4de56ce85bee0f0a7b", file.ID)

	// Batch ID
	require.Equal(t, "7cf510873dc46d322fd93777487472d1efed4d4e3bdbccb0e8068ff108eddd7b", file.Batches[0].ID())

	// Entry ID
	entries := file.Batches[0].GetEntries()
	require.Equal(t, "7d47b497f775a8957ae3a9a58e94bc5fa1acdacfa17b644019c3f715671c3f4e", entries[0].ID)
}
