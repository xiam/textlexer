package rules

import (
	"unicode"
)

func isSpace(r rune) bool {
	switch byte(r) {
	case ' ', '\t', '\r', '\n', '\f':
		return true
	}
	return false
}

func toLower(r rune) rune {
	return unicode.ToLower(r)
}

func isLetter(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	return false
}

func isNumeric(r rune) bool {
	if r >= '0' && r <= '9' {
		return true
	}
	return false
}
