package scanner

import "strings"

var blockedDomains = map[string]bool{
	"bit.ly":      true,
	"tinyurl.com": true,
	"t.co":        true,
	"goo.gl":      true,
	"ow.ly":       true,
	"is.gd":       true,
	"buff.ly":     true,
	"adf.ly":      true,
	"rb.gy":       true,
	"shorturl.at": true,
	"cutt.ly":     true,
	"tiny.cc":     true,
	"clck.ru":     true,
	"soo.gd":      true,
}

// IsBlocked reports whether host is a known URL shortener domain.
func IsBlocked(host string) bool {
	return blockedDomains[normalizeHost(host)]
}

func normalizeHost(host string) string {
	h := strings.ToLower(host)
	h = strings.TrimPrefix(h, "www.")
	return h
}
