package analytics_test

import (
	"testing"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/stretchr/testify/assert"
)

func TestAnonymizeIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{name: "ipv4 last octet zeroed", ip: "192.168.1.45", want: "192.168.1.0"},
		{name: "ipv4 already zero", ip: "10.0.0.1", want: "10.0.0.0"},
		{name: "ipv6 last 8 bytes zeroed", ip: "2001:db8:85a3::8a2e:370:7334", want: "2001:db8:85a3::"},
		{name: "empty string", ip: "", want: ""},
		{name: "invalid ip", ip: "invalid", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.AnonymizeIP(tt.ip)
			assert.Equal(t, tt.want, got)
		})
	}
}
