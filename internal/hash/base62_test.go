package hash_test

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devpedrois/snip/internal/hash"
)

func TestEncode_MinLength(t *testing.T) {
	ids := []uint64{0, 1, 100, 1000, math.MaxUint32, 1_000_000_000}
	for _, id := range ids {
		result := hash.Encode(id)
		assert.GreaterOrEqual(t, len(result), 7, "Encode(%d) length = %d, want >= 7", id, len(result))
	}
}

func TestEncode_OnlyAlphabetChars(t *testing.T) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ids := []uint64{0, 1, 42, 999, math.MaxUint32}
	for _, id := range ids {
		result := hash.Encode(id)
		for _, ch := range result {
			assert.True(t, strings.ContainsRune(alphabet, ch), "char %q not in alphabet", ch)
		}
	}
}

func TestDecodeEncode_RoundTrip(t *testing.T) {
	for i := 0; i < 1000; i++ {
		id := rand.Uint64() % (math.MaxUint32 * 100) //nolint:gosec
		encoded := hash.Encode(id)
		decoded, err := hash.Decode(encoded)
		require.NoError(t, err)
		assert.Equal(t, id, decoded, "round-trip failed for id=%d", id)
	}
}

func TestDecode_RejectsInvalidChars(t *testing.T) {
	invalid := []string{"abc!123", "abc-def", "abc def", "ÄÖÜ"}
	for _, s := range invalid {
		_, err := hash.Decode(s)
		assert.Error(t, err, "Decode(%q) should fail", s)
	}
}

func TestDecode_RejectsEmptyString(t *testing.T) {
	_, err := hash.Decode("")
	assert.Error(t, err)
}

func TestDecode_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"maxUint32", math.MaxUint32},
		{"large", 9_999_999_999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := hash.Encode(tt.id)
			decoded, err := hash.Decode(encoded)
			require.NoError(t, err)
			assert.Equal(t, tt.id, decoded)
		})
	}
}
