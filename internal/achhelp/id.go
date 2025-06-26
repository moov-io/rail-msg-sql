package achhelp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/moov-io/ach"
)

func PopulateIDs(file *ach.File) *ach.File {
	// Set overall FileID
	if file.ID == "" {
		var buf bytes.Buffer
		ach.NewWriter(&buf).Write(file)
		file.ID = hash(buf.Bytes())
	}

	// Set each Batch's ID (by the FileID + BatchHeader)
	for idx := range file.Batches {
		if file.Batches[idx].ID() == "" {
			bh := file.Batches[idx].GetHeader().String()
			file.Batches[idx].SetID(hash([]byte(file.ID + bh)))
		}

		entries := file.Batches[idx].GetEntries()
		for e := range entries {
			if entries[e].ID == "" {
				entries[e].ID = hash([]byte(file.Batches[idx].ID() + entries[e].String()))
			}
		}
	}

	return file
}

func hash(data []byte) string {
	ss := sha256.New()
	ss.Write(data)
	return hex.EncodeToString(ss.Sum(nil))
}
