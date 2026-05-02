package domain

import "time"

type Click struct {
	ID         uint64
	URLID      uint64
	AccessedAt time.Time
	UserAgent  string
	IP         string
}
