package domain

import "time"

type URL struct {
	ID             uint64
	Hash           string
	OriginalURL    string
	CreatedAt      time.Time
	LastAccessedAt *time.Time
	ExpiresAt      *time.Time
}
