package hash

import (
	"errors"
	"strings"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// offset = 62^6 = 56_800_235_584. Adding it before encoding guarantees the
// result is always at least 7 Base62 digits, even for id=0.
const offset = uint64(56_800_235_584)

// Encode converts id to a Base62 string of at least 7 characters.
func Encode(id uint64) string {
	n := id + offset
	if n == 0 {
		return string(alphabet[0])
	}
	var buf [16]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = alphabet[n%62]
		n /= 62
	}
	return string(buf[pos:])
}

// Decode reverses Encode, validating each character and subtracting the offset.
func Decode(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("hash: empty string")
	}
	var n uint64
	for _, ch := range s {
		idx := strings.IndexRune(alphabet, ch)
		if idx < 0 {
			return 0, errors.New("hash: invalid character in hash")
		}
		n = n*62 + uint64(idx)
	}
	if n < offset {
		return 0, errors.New("hash: value underflows offset")
	}
	return n - offset, nil
}
