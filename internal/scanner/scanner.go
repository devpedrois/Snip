package scanner

import (
	"context"
	"time"
)

type ScanStatus string

const (
	ScanClean      ScanStatus = "clean"
	ScanMalicious  ScanStatus = "malicious"
	ScanUnverified ScanStatus = "unverified"
)

type ScanResult struct {
	Status    ScanStatus
	Positives int
	Total     int
	ScannedAt time.Time
	Permalink string
}

type URLScanner interface {
	Scan(ctx context.Context, url string) (ScanResult, error)
}
