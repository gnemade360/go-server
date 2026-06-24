package conn

import "unicode/utf8"

// ValidUTF8 reports whether p is a valid UTF-8 sequence. The read pump
// applies this to every text frame; RFC 6455 §8.1 requires it.
func ValidUTF8(p []byte) bool {
	return utf8.Valid(p)
}
