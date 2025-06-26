package storage

import (
	"time"
)

type FilterParams struct {
	StartDate time.Time
	EndDate   time.Time
	Pattern   string
}
