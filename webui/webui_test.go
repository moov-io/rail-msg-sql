package webui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebRoot(t *testing.T) {
	bs, err := WebRoot.ReadFile("index.html.tmpl")
	require.NoError(t, err)
	require.NotEmpty(t, bs)
}
