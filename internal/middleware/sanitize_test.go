package middleware_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/devpedrois/snip/internal/middleware"
)

func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "normal string unchanged", input: "normal string", want: "normal string"},
		{name: "crlf injection removed", input: "evil\r\ninjection", want: "evilinjection"},
		{name: "tab removed", input: "tab\there", want: "tabhere"},
		{name: "null byte removed", input: "\x00null", want: "null"},
		{name: "del character removed", input: "del\x7fchar", want: "delchar"},
		{name: "empty string", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := middleware.SanitizeLogValue(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
