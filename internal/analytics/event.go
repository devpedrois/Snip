package analytics

import "time"

type ClickEvent struct {
	URLID      uint64
	AccessedAt time.Time
	UserAgent  string
	IP         string
}
