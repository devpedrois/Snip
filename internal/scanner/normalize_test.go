package scanner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/scanner"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "uppercase scheme and host with trailing slash",
			input: "HTTP://EXAMPLE.COM/",
			want:  "http://example.com",
		},
		{
			name:  "default http port removed with path",
			input: "http://example.com:80/path",
			want:  "http://example.com/path",
		},
		{
			name:  "default https port removed with trailing slash",
			input: "https://example.com:443/",
			want:  "https://example.com",
		},
		{
			name:  "fragment removed",
			input: "http://example.com/page#frag",
			want:  "http://example.com/page",
		},
		{
			name:  "non-default port preserved",
			input: "http://example.com:8080/path",
			want:  "http://example.com:8080/path",
		},
		{
			name:  "query string preserved",
			input: "https://example.com/search?q=test",
			want:  "https://example.com/search?q=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scanner.NormalizeURL(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
