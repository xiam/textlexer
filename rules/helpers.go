package rules

import (
	"unicode"
)

var (
	asciiLetters     = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	asciiDigits      = []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	asciiWhitespace  = []rune{' ', '\t', '\r', '\n', '\f'}
	hexDigits        = []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F'}
	octalDigits      = []rune{'0', '1', '2', '3', '4', '5', '6', '7'}
	binaryDigits     = []rune{'0', '1'}
	punctuationMarks = []rune{'.', ',', ';', ':', '!', '?', '"', '\'', '(', ')', '[', ']', '{', '}', '-', '—', '…'}
)

// isCommonWhitespace returns true if r is a common whitespace
func isCommonWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n', '\f':
		return true
	}
	return false
}

// isASCIILetter returns true if r is an ASCII letter (a-z, A-Z).
func isASCIILetter(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	return false
}

// isASCIIDigit returns true if r is an ASCII digit (0-9)
func isASCIIDigit(r rune) bool {
	if r >= '0' && r <= '9' {
		return true
	}
	return false
}

// isHexDigit checks if a rune is a hexadecimal digit (0-9, a-f, A-F).
func isHexDigit(r rune) bool {
	return isASCIIDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// isOctalDigit checks if a rune is an octal digit (0-7).
func isOctalDigit(r rune) bool {
	return r >= '0' && r <= '7'
}

// isASCIILetterOrDigit checks if a rune is an ASCII letter or digit.
func isASCIILetterOrDigit(r rune) bool {
	return isASCIILetter(r) || isASCIIDigit(r)
}

// isLetter checks if a rune is a letter (including non-ASCII Unicode letters).
func isLetter(r rune) bool {
	return isASCIILetter(r) || (r > 127 && unicode.IsLetter(r))
}

// isPunctuation checks if a rune is a punctuation mark.
func isPunctuation(r rune) bool {
	switch r {
	case '.', ',', ';', ':', '!', '?', '"', '\'', '(', ')', '[', ']', '{', '}', '-', '—', '…':
		return true
	default:
		return false
	}
}

// isBinaryDigit checks if a rune is a binary digit (0 or 1).
func isBinaryDigit(r rune) bool {
	return r == '0' || r == '1'
}

func IsCommonWhitespace(r rune) bool {
	return isCommonWhitespace(r)
}

func IsWhitespace(r rune) bool {
	return isCommonWhitespace(r) || unicode.IsSpace(r)
}
