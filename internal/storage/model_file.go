package storage

import (
	"github.com/moov-io/ach"
)

type FileListing struct {
	Name        string
	StoragePath string
	SourceID    string
}

type File struct {
	Filename string
	Contents *ach.File
}
