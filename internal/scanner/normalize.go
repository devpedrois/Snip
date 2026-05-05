package scanner

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/devpedrois/snip/internal/domain"
)

// NormalizeURL canonicalizes a URL for consistent storage and comparison.
// It lowercases scheme and host, removes the fragment, strips default ports,
// and removes a trailing slash when the path is exactly "/".
func NormalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("normalize: parse: %w", domain.ErrInvalidURL)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	host := u.Hostname()
	port := u.Port()
	switch {
	case u.Scheme == "http" && port == "80":
		u.Host = host
	case u.Scheme == "https" && port == "443":
		u.Host = host
	}

	if u.Path == "/" {
		u.Path = ""
	}

	return u.String(), nil
}
