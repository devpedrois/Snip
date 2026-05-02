package domain

import "errors"

var (
	ErrURLNotFound  = errors.New("url not found")
	ErrURLExpired   = errors.New("url expired")
	ErrHashConflict = errors.New("hash conflict")
	ErrInvalidURL   = errors.New("invalid url")
)
