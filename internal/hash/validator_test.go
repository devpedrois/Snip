package hash_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/domain"
	"github.com/devpedrois/snip/internal/hash"
)

const testBaseURL = "http://localhost:8080"

func TestValidateURL_Valid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"https simple", "https://example.com"},
		{"http with path and query", "http://x.com/path?q=1#frag"},
		{"https with non-default port", "https://x.com:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hash.ValidateURL(tt.url, testBaseURL)
			assert.NoError(t, err)
		})
	}
}

func TestValidateURL_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr error
	}{
		{"empty", "", domain.ErrInvalidURL},
		{"ftp scheme", "ftp://x.com", domain.ErrInvalidURL},
		{"no scheme", "://noscheme", domain.ErrInvalidURL},
		{"http no host", "http://", domain.ErrInvalidURL},
		{"too long", strings.Repeat("a", 2049), domain.ErrInvalidURL},
		{"javascript scheme", "javascript:alert(1)", domain.ErrInvalidURL},
		{"data scheme", "data:text/html,xss", domain.ErrInvalidURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hash.ValidateURL(tt.url, testBaseURL)
			require.Error(t, err, "ValidateURL(%q) should fail", tt.url)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestValidateURL_Credentials(t *testing.T) {
	err := hash.ValidateURL("http://admin:pass@evil.com", testBaseURL)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrURLHasCredentials)
}

func TestValidateURL_PrivateIPs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"192.168 range", "http://192.168.1.1"},
		{"10.x range", "http://10.0.0.1/admin"},
		{"localhost", "http://127.0.0.1"},
		{"link-local / cloud metadata", "http://169.254.169.254/metadata"},
		{"ipv6 loopback", "http://[::1]/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hash.ValidateURL(tt.url, testBaseURL)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrURLPrivateIP)
		})
	}
}

func TestValidateURL_SelfReference(t *testing.T) {
	err := hash.ValidateURL("http://localhost:8080/x", "http://localhost:8080")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrURLSelfRef)
}

func TestValidateURL_Blocked(t *testing.T) {
	err := hash.ValidateURL("https://bit.ly/something", testBaseURL)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrURLBlocked)
}

func TestValidateURL_HomographCyrillic(t *testing.T) {
	// "а" is Cyrillic U+0430, visually identical to Latin "a"
	err := hash.ValidateURL("http://exаmple.com/path", testBaseURL)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrURLHomograph)
}

func TestValidateURL_EmptyBaseURL_NoSelfRefError(t *testing.T) {
	err := hash.ValidateURL("https://example.com", "")
	assert.NoError(t, err)
}

func TestValidateURL_SelfReference_DifferentHost(t *testing.T) {
	err := hash.ValidateURL("https://example.com", "http://localhost:8080")
	assert.NoError(t, err)
}
