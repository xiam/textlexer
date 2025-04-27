package rules

import (
	"github.com/xiam/textlexer"
)

// IsEOF checks if the given rune is the end of file (EOF) marker.
func IsEOF(r rune) bool {
	return r == textlexer.RuneEOF
}

// IsBOF checks if the given rune is the beginning of file (BOF) marker.
func IsBOF(r rune) bool {
	return r == textlexer.RuneBOF
}
