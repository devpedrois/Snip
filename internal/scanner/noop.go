package scanner

import (
	"context"
	"time"
)

type NoopScanner struct{}

func NewNoopScanner() *NoopScanner {
	return &NoopScanner{}
}

func (n *NoopScanner) Scan(_ context.Context, _ string) (ScanResult, error) {
	return ScanResult{Status: ScanClean, ScannedAt: time.Now()}, nil
}
