package middleware

import "strings"

// SanitizeLogValue removes ASCII control characters (0-31 and 127) from s,
// preventing log injection via malicious headers.
func SanitizeLogValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}
