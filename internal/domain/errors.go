package domain

import "errors"

var (
	ErrURLNotFound  = errors.New("url not found")
	ErrURLExpired   = errors.New("url expired")
	ErrHashConflict = errors.New("hash conflict")
	ErrInvalidURL   = errors.New("invalid url")

	ErrRateLimitExceed = errors.New("rate limit exceeded")
	ErrBodyTooLarge    = errors.New("request body too large")
	ErrMediaType       = errors.New("unsupported media type")
	ErrUnknownFields   = errors.New("unknown fields in request body")

	ErrURLSelfRef        = errors.New("url cannot reference snip itself")
	ErrURLPrivateIP      = errors.New("url cannot point to private ip address")
	ErrURLBlocked        = errors.New("url domain is blocked")
	ErrURLHasCredentials = errors.New("url cannot contain userinfo credentials")
	ErrURLHomograph      = errors.New("url contains suspicious unicode characters")

	ErrDuplicateReport = errors.New("url already reported by this ip")
	ErrInvalidReason   = errors.New("invalid report reason")

	ErrURLMalicious = errors.New("url flagged as malicious")
)
