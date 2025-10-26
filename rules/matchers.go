package rules

import (
	"unicode"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/processor"
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

// isIdentifierStart returns true if a rune is a valid starting character for an identifier.
func isIdentifierStart(r rune) bool {
	return isASCIILetter(r) || r == '_'
}

// isIdentifierPart returns true if a rune is a valid non-starting character for an identifier.
func isIdentifierPart(r rune) bool {
	return isIdentifierStart(r) || isASCIIDigit(r)
}

// newCharacterClassMatcher creates a rule that matches a sequence of
// characters belonging to a specified character class, with defined minimum
// and maximum lengths.
//
// Panics if minLen is negative or if maxLen is not -1 and less than minLen.
func newCharacterClassMatcher(
	characterClass func(rune) bool,
	minLen int,
	maxLen int,
) textlexer.Rule {
	// check min and max, panic if invalid
	if minLen < 0 {
		panic("minLen must be non-negative")
	}
	if maxLen != -1 && maxLen < minLen {
		panic("maxLen must be -1 (unlimited) or >= minLen")
	}

	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		var loop textlexer.Rule
		sp := processor.NewStateProcessor()

		loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			start, offset := sp.Position()
			currentLen := int(start + offset)

			// Check if character belongs to the class
			if !characterClass(s.Rune()) {
				// Character doesn't match
				if currentLen < minLen {
					// Haven't reached minimum length yet
					return nil, textlexer.StateReject
				}
				// We've matched enough characters, push back this one
				return nil, textlexer.StateReject
			}

			// Character matches the class
			if err := sp.Execute(textlexer.StateContinue); err != nil {
				// This indicates a logical error in the state processor itself,
				// but we shouldn't panic. Rejecting is the safest option.
				return nil, textlexer.StateReject
			}

			start, offset = sp.Position()
			currentLen = int(start + offset)

			// Check if we've reached maximum length
			if maxLen != -1 && currentLen >= maxLen {
				// Reached max length, accept but don't continue
				return nil, textlexer.StateAccept
			}

			// Check if we've reached minimum length
			if currentLen >= minLen {
				// At or above minimum, accept and continue
				return loop, textlexer.StateAccept
			}

			// Below minimum, continue without accepting
			return loop, textlexer.StateContinue
		}

		return loop(s)
	}
}

// newStartPartMatcher creates a rule that matches a sequence of characters
// defined by two character classes: one for the starting character and one for
// all subsequent characters.
func newStartPartMatcher(
	isStart func(rune) bool,
	isPart func(rune) bool,
) textlexer.Rule {
	var maybePart textlexer.Rule

	maybePart = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()

		if isPart(r) || isStart(r) {
			return maybePart, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}

	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isStart(s.Rune()) {
			return maybePart, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}
}
