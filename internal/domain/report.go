package domain

import "time"

const (
	ReasonPhishing = "phishing"
	ReasonMalware  = "malware"
	ReasonSpam     = "spam"
	ReasonIllegal  = "illegal"
	ReasonOther    = "other"
)

var validReasons = map[string]bool{
	ReasonPhishing: true,
	ReasonMalware:  true,
	ReasonSpam:     true,
	ReasonIllegal:  true,
	ReasonOther:    true,
}

// Report represents an abuse report submitted for a shortened URL.
type Report struct {
	ID         uint64
	URLID      uint64
	ReporterIP string
	Reason     string
	CreatedAt  time.Time
}

// IsValidReason reports whether r is an accepted report reason.
func IsValidReason(r string) bool {
	return validReasons[r]
}
