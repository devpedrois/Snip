package hash_test

import (
	"strings"
	"testing"

	"github.com/devpedrois/snip/internal/hash"
	"github.com/stretchr/testify/assert"
)

func TestValidateURL_Valid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"https simple", "https://example.com"},
		{"http with path and query", "http://x.com/path?q=1#frag"},
		{"https with port", "https://x.com:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hash.ValidateURL(tt.url)
			assert.NoError(t, err)
		})
	}
}

func TestValidateURL_Invalid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"ftp scheme", "ftp://x.com"},
		{"no scheme", "://noscheme"},
		{"http no host", "http://"},
		{"too long", strings.Repeat("a", 2049)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hash.ValidateURL(tt.url)
			assert.Error(t, err, "ValidateURL(%q) should fail", tt.url)
		})
	}
}
