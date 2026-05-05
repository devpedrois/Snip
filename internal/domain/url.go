package domain

import "time"

type URL struct {
	ID             uint64
	Hash           string
	OriginalURL    string
	CreatedAt      time.Time
	LastAccessedAt *time.Time
	ExpiresAt      *time.Time

	VTStatus    string     `db:"vt_status"`
	VTScannedAt *time.Time `db:"vt_scanned_at"`
	VTPositives *int       `db:"vt_positives"`
	VTPermalink *string    `db:"vt_permalink"`
}
