package storage

import (
	"github.com/moov-io/ach"
)

type File struct {
	Filename string
	File     *ach.File
}
