package scanner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/devpedrois/snip/internal/scanner"
)

func TestIsBlocked(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		blocked bool
	}{
		{"bit.ly blocked", "bit.ly", true},
		{"www.bit.ly blocked", "www.bit.ly", true},
		{"uppercase BIT.LY blocked", "BIT.LY", true},
		{"tinyurl.com blocked", "tinyurl.com", true},
		{"example.com not blocked", "example.com", false},
		{"github.com not blocked", "github.com", false},
		{"t.co blocked", "t.co", true},
		{"cutt.ly blocked", "cutt.ly", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.blocked, scanner.IsBlocked(tt.host))
		})
	}
}
