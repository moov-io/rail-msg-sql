package search

import (
	"time"

	"github.com/moov-io/ach/cmd/achcli/describe/mask"
)

type Config struct {
	Sqlite *SqliteConfig

	BackgroundPrepare bool

	AchMasking mask.Options
}

type SqliteConfig struct {
	Directory   string
	BusyTimeout time.Duration

	MaxOpenConns int
	MaxIdleConns int
}
