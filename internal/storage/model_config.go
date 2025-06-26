package storage

import (
	"github.com/moov-io/ach"
	achwebviewer "github.com/moov-io/ach-web-viewer/pkg/service"
)

type Config struct {
	ACH achwebviewer.Sources

	ACHValidateOpts *ach.ValidateOpts
}
