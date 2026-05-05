package hash

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/scanner"
)

var privateCIDRs []*net.IPNet

func init() {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"0.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fd00::/8",
		"fe80::/10",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("validator: invalid cidr %q: %v", cidr, err))
		}
		privateCIDRs = append(privateCIDRs, network)
	}
}

// ValidateURL validates rawURL against security and format rules.
// baseURL is the application's own base URL, used to detect self-references.
func ValidateURL(raw string, baseURL string) error {
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

	if u.User != nil {
		return fmt.Errorf("url contains credentials: %w", domain.ErrURLHasCredentials)
	}

	if err := checkSelfRef(u, baseURL); err != nil {
		return err
	}

	if err := checkPrivateIP(u.Host); err != nil {
		return err
	}

	if err := checkHomograph(u.Hostname()); err != nil {
		return err
	}

	if scanner.IsBlocked(u.Hostname()) {
		return fmt.Errorf("url domain %q is blocked: %w", u.Hostname(), domain.ErrURLBlocked)
	}

	return nil
}

func checkSelfRef(u *url.URL, baseURL string) error {
	if baseURL == "" {
		return nil
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	if strings.EqualFold(u.Hostname(), base.Hostname()) {
		return fmt.Errorf("url references snip itself: %w", domain.ErrURLSelfRef)
	}
	return nil
}

func checkPrivateIP(rawHost string) error {
	host, _, err := net.SplitHostPort(rawHost)
	if err != nil {
		host = rawHost
	}

	// Strip brackets from IPv6 literals like [::1]
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")

	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}

	for _, network := range privateCIDRs {
		if network.Contains(ip) {
			return fmt.Errorf("url points to private ip %s: %w", ip, domain.ErrURLPrivateIP)
		}
	}
	return nil
}

func checkHomograph(hostname string) error {
	if hostname == "" {
		return nil
	}
	asciiHost, err := idna.Lookup.ToASCII(hostname)
	if err != nil {
		return nil
	}
	if asciiHost != hostname {
		return fmt.Errorf("url contains suspicious unicode in host %q: %w", hostname, domain.ErrURLHomograph)
	}
	return nil
}
