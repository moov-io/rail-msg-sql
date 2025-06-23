package storage

import (
	"github.com/moov-io/ach"
)

type Config struct {
	Filesystem *FilesystemConfig
}

type FilesystemConfig struct {
	Directories  []string
	ValidateOpts *ach.ValidateOpts
}
