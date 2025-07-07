package search

import (
	"github.com/moov-io/ach/cmd/achcli/describe/mask"
)

type Config struct {
	SqliteFilepath string

	BackgroundPrepare bool

	AchMasking mask.Options
}
