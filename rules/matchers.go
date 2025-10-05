package rules

import (
	"unicode"

	"github.com/xiam/textlexer"
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

// isBinaryDigit checks if a rune is a binary digit (0 or 1).
func isBinaryDigit(r rune) bool {
	return r == '0' || r == '1'
}

/*
func IsCommonWhitespace(r rune) bool {
	return isCommonWhitespace(r)
}

func IsWhitespace(r rune) bool {
	return isCommonWhitespace(r) || unicode.IsSpace(r)
}
*/

// newCharacterClassMatcher creates a rule that matches a sequence of characters
// belonging to a specified character class, with defined minimum and maximum lengths.
func newCharacterClassMatcher(
	characterClass func(rune) bool,
	min int,
	max int,
	// NOTE: The nextRule parameter is no longer needed, as the terminal
	//       state is now handled internally with a pushback.
) textlexer.Rule {
	if min < 0 {
		panic("min must be non-negative")
	}
	if max >= 0 && max < min {
		panic("max must be greater than or equal to min, or -1 for unlimited")
	}

	// 1. 'count' is declared *outside* the returned rules.
	//    This is the closure's state and will persist across calls.
	var count int

	// 2. We declare the 'loop' rule so it can refer to itself recursively.
	var loop textlexer.Rule

	// This is the main loop for matching subsequent characters.
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if characterClass(s.Rune()) {
			count++
			if max != -1 && count >= max {
				// We've hit the maximum allowed characters. Accept and stop.
				// We don't push back because this character is part of the match.
				return nil, textlexer.StateAccept
			}
			// The character is valid, continue the loop.
			return loop, textlexer.StateContinue
		}

		// The character does NOT match the class. The token has ended.
		if count < min {
			// We didn't find the minimum required characters.
			return nil, textlexer.StateReject
		}

		// 3. We found a valid token. Push back the current non-matching
		//    symbol and accept the match up to this point.
		return PushBackCurrentAndAccept(s)
	}

	// This is the entry point for the rule.
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		// Reset the count for a new potential token.
		count = 0

		if characterClass(s.Rune()) {
			count++
			// If min is 1 and max is 1, we can accept immediately.
			if min <= 1 && max == 1 {
				return nil, textlexer.StateAccept
			}
			// Otherwise, transition to the loop.
			return loop, textlexer.StateContinue
		}

		// First character didn't match.
		if min == 0 {
			// A zero-length match is valid, push back and accept.
			return PushBackCurrentAndAccept(s)
		}

		return nil, textlexer.StateReject
	}
}
