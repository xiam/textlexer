package rules

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
