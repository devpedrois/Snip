package hash

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/devpedrois/snip/internal/domain"
)

func ValidateURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("url is empty: %w", domain.ErrInvalidURL)
	}
	if len(raw) > 2048 {
		return fmt.Errorf("url exceeds 2048 characters: %w", domain.ErrInvalidURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("url parse error: %w", domain.ErrInvalidURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url scheme %q not allowed: %w", u.Scheme, domain.ErrInvalidURL)
	}
	if u.Host == "" {
		return fmt.Errorf("url host is empty: %w", domain.ErrInvalidURL)
	}
	return nil
}
