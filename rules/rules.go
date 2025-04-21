package rules

import (
	"github.com/xiam/textlexer"
)

// AcceptCurrentAndStop accepts the current rune and stops processing.
func AcceptCurrentAndStop(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

// RejectCurrent rejects the current rune and stops processing.
func RejectCurrent(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateReject
}

// MatchAnyCharacter matches any single rune and continues.
func MatchAnyCharacter(r rune) (textlexer.Rule, textlexer.State) {
	return MatchAnyCharacter, textlexer.StateContinue
}

// MatchUnsignedInteger matches one or more digits (0-9).
// Example Matches: `123`, `0`, `9876543210`
func MatchUnsignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	var nextDigit textlexer.Rule

	nextDigit = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more digits
		if isASCIIDigit(r) {
			return nextDigit, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	// starts with a digit
	if isASCIIDigit(r) {
		return nextDigit, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchUntilWhitespaceOrEOF matches content before whitespace or EOF.
func MatchUntilWhitespaceOrEOF(r rune) (textlexer.Rule, textlexer.State) {
	if isCommonWhitespace(r) || textlexer.IsEOF(r) {
		return nil, textlexer.StateAccept
	}
	// Note: This rule actually rejects the first character if it's not whitespace/EOF.
	// It's intended to be used *after* an initial match, like with NewMatchStartingWithString.
	// A more typical "match until" would continue on non-whitespace and accept on whitespace/EOF.
	// However, adhering to the "do not change code" rule, the documentation reflects its behavior.
	// If used directly, it only accepts an empty string right before whitespace/EOF.
	return nil, textlexer.StateReject
}

// MatchSignedInteger matches an optional sign (+/-) and digits. It allows
// whitespace between the sign and the number.
// Example Matches: `123`, `+45`, `-0`, `+ 99`, `- 1`
func MatchSignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	var anyWhitespace textlexer.Rule

	anyWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		// Allows whitespace between sign and number
		if isCommonWhitespace(r) {
			return anyWhitespace, textlexer.StateContinue
		}

		// After whitespace (or if none), expect the numeric part
		return MatchUnsignedInteger(r)
	}

	// a signed integer may start with a minus or plus sign
	if r == '-' || r == '+' {
		// discard whitespace after the sign
		return anyWhitespace, textlexer.StateContinue
	}

	return MatchUnsignedInteger(r)
}

// MatchUnsignedFloat matches a floating-point number without sign.
// Example Matches: `123.45`, `0.1`, `.56`
func MatchUnsignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var integerPart, radixPoint, fractionalPart textlexer.Rule

	integerPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return integerPart, textlexer.StateContinue
		}

		// expects a point after the integer part
		return radixPoint(r)
	}

	radixPoint = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '.' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				// expects a digit immediately after the radix point
				if isASCIIDigit(r) {
					return fractionalPart, textlexer.StateContinue
				}

				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	fractionalPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return fractionalPart, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	if isASCIIDigit(r) {
		return integerPart, textlexer.StateContinue
	}

	// Allows starting with a decimal point if followed by a digit.
	if r == '.' {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			// expects a digit immediately after the radix point
			if isASCIIDigit(r) {
				return fractionalPart, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchUnsignedNumeric matches integers or floating-point numbers.
// Example Matches: `123`, `0`, `123.45`, `0.5`, `.5`, `5.`
func MatchUnsignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	var expectInteger, scanInteger, expectDecimal, scanDecimal textlexer.Rule

	scanDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	scanInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return scanInteger, textlexer.StateContinue
		}

		if r == '.' {
			// Found a decimal point after digits, scan fractional part (if any)
			return scanDecimal, textlexer.StateContinue
		}

		// End of integer part without a decimal point
		return nil, textlexer.StateAccept
	}

	expectDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		// Must have a digit after the initial '.'
		if isASCIIDigit(r) {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	expectInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			// Starts with a digit, scan the integer part
			return scanInteger, textlexer.StateContinue
		}

		if r == '.' {
			// Starts with a decimal point, expect fractional part
			return expectDecimal, textlexer.StateContinue
		}

		// Does not start with digit or '.'
		return nil, textlexer.StateReject
	}

	return expectInteger(r)
}

// MatchSignedNumeric matches numbers with optional sign (+/-). Whitespace is
// allowed after the sign and before the number.
// Example Matches: `123`, `+45`, `-0.5`, `+ 99`, `- .5`, `+ 1.`, `- 123.45`
func MatchSignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	var anyWhitespace, expectInteger, scanInteger, expectDecimal, scanDecimal textlexer.Rule

	// Reusing logic from MatchUnsignedNumeric
	scanDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return scanDecimal, textlexer.StateContinue
		}
		return nil, textlexer.StateAccept
	}

	scanInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return scanInteger, textlexer.StateContinue
		}
		if r == '.' {
			return scanDecimal, textlexer.StateContinue
		}
		return nil, textlexer.StateAccept
	}

	expectDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return scanDecimal, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	expectInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return scanInteger, textlexer.StateContinue
		}
		if r == '.' {
			return expectDecimal, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	// End of reused logic

	anyWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		// Allows whitespace between sign and number
		if isCommonWhitespace(r) {
			return anyWhitespace, textlexer.StateContinue
		}

		// After whitespace (or if none), expect the numeric part
		return expectInteger(r)
	}

	// Starts with a sign
	if r == '-' || r == '+' {
		return anyWhitespace, textlexer.StateContinue
	}

	// No sign, directly expect the numeric part
	return expectInteger(r)
}

// MatchSignedFloat matches a float with optional sign (+/-). Whitespace is
// allowed after the sign and before the number.
// Example Matches: `123.45`, `+0.1`, `-.56`, `+ 123.45`, `- .5`
func MatchSignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var anyWhitespace textlexer.Rule

	anyWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		// Allows whitespace between sign and number
		if isCommonWhitespace(r) {
			return anyWhitespace, textlexer.StateContinue
		}

		// After whitespace (or if none), expect the numeric part
		return MatchUnsignedFloat(r)
	}

	if r == '-' || r == '+' {
		return anyWhitespace, textlexer.StateContinue
	}

	return MatchUnsignedFloat(r)
}

// MatchWhitespace matches one or more whitespace characters.
// Example Matches: ` `, ` \t`, `\n\r\n`, `\t\t `
func MatchWhitespace(r rune) (textlexer.Rule, textlexer.State) {
	var nextSpace textlexer.Rule

	nextSpace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) {
			return nextSpace, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	if isCommonWhitespace(r) {
		return nextSpace, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchIdentifier matches programming identifiers (letter followed by letters/digits).
// Example Matches: `variable`, `count1`, `isValid`, `i`, `MyClass`
func MatchIdentifier(r rune) (next textlexer.Rule, state textlexer.State) {
	var nextLetter textlexer.Rule

	nextLetter = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more letters or digits
		if isASCIILetter(r) || isASCIIDigit(r) {
			return nextLetter, textlexer.StateContinue
		}

		// ends with any other character
		return nil, textlexer.StateAccept
	}

	// starts with a letter
	if isASCIILetter(r) {
		return nextLetter, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchDoubleQuotedString matches text in double quotes.
// Example Matches: `"hello world"`, `""`, `"a"`, `"quote"`
func MatchDoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		// Match ends when the closing quote is found
		if r == '"' {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		// Reject if EOF is reached before the closing quote
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		// Continue consuming characters inside the quotes
		return nextChar, textlexer.StateContinue
	}

	// String must start with a double quote
	if r == '"' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchSingleQuotedString matches text in single quotes.
// Example Matches: `'hello world'`, `"`, `'a'`, `'quote'`
func MatchSingleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		// Match ends when the closing quote is found
		if r == '\'' {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		// Reject if EOF is reached before the closing quote
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		// Continue consuming characters inside the quotes
		return nextChar, textlexer.StateContinue
	}

	// String must start with a single quote
	if r == '\'' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchEscapedDoubleQuotedString matches text in double quotes with escape
// sequences.
// Example Matches: `"hello \"world\""`, `"a\\b"`, `""`, `"escaped"`
func MatchEscapedDoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		// Reject if EOF is reached before the closing quote
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		// Match ends when an unescaped closing quote is found
		if r == '"' {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		// Handle escape character
		if r == '\\' {
			// Consume the next character after backslash
			return func(r rune) (textlexer.Rule, textlexer.State) {
				// Reject if escaped character is whitespace or EOF
				if isCommonWhitespace(r) || textlexer.IsEOF(r) {
					return nil, textlexer.StateReject
				}
				// After consuming the escaped character, continue scanning
				return nextChar, textlexer.StateContinue
			}, textlexer.StateContinue
		}

		// Continue consuming normal characters inside the quotes
		return nextChar, textlexer.StateContinue
	}

	// String must start with a double quote
	if r == '"' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchInlineComment matches single-line comments starting with //.
// Example Matches: `// This is a comment\n`, `// Another comment<EOF>`
func MatchInlineComment(r rune) (textlexer.Rule, textlexer.State) {
	// Uses a helper (NewMatchStartingWithString) to first match "//"
	// then uses MatchUntilEOL to consume the rest of the line.
	return NewMatchStartingWithString("//", MatchUntilEOL)(r)
}

// MatchUntilEOF consumes all characters until EOF.
func MatchUntilEOF(r rune) (textlexer.Rule, textlexer.State) {
	// Define the recursive part inside
	var matchUntilEOFRecursive textlexer.Rule
	matchUntilEOFRecursive = func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			// Accept when EOF is encountered (EOF itself isn't included)
			return nil, textlexer.StateAccept
		}
		// Continue consuming any other character
		return matchUntilEOFRecursive, textlexer.StateContinue
	}
	// Start the recursive matching
	return matchUntilEOFRecursive(r)
}

// MatchUntilEOL consumes characters until newline or EOF.
func MatchUntilEOL(r rune) (textlexer.Rule, textlexer.State) {
	var untilNewLine textlexer.Rule

	untilNewLine = func(r rune) (textlexer.Rule, textlexer.State) {
		// Stop and accept when newline or EOF is found
		if r == '\n' || r == '\r' || textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		// Continue consuming other characters
		return untilNewLine, textlexer.StateContinue
	}

	// Start the matching process
	return untilNewLine(r)
}

// MatchBasicMathOperator matches +, -, *, or /.
func MatchBasicMathOperator(r rune) (textlexer.Rule, textlexer.State) {
	if r == '+' || r == '-' || r == '*' || r == '/' {
		// Accept the operator and stop
		return AcceptCurrentAndStop, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchSlashStarComment matches C-style /* */ comments.
// Example Matches: `/* comment */`, `/* multi\nline */`, `/**/`
func MatchSlashStarComment(r rune) (textlexer.Rule, textlexer.State) {
	// Uses helpers:
	// 1. NewMatchStartingWithString to match "/*"
	// 2. NewMatchUntilString to consume content until "*/"
	// 3. AcceptCurrentAndStop to finalize after "*/" is matched.
	return NewMatchStartingWithString(
		"/*",
		NewMatchUntilString(
			"*/",
			AcceptCurrentAndStop, // Final state after matching "*/"
		),
	)(r)
}

// AcceptAnyParen matches either ( or ).
// Example Matches: `(`, `)`
func AcceptAnyParen(r rune) (textlexer.Rule, textlexer.State) {
	if r == '(' || r == ')' {
		return AcceptCurrentAndStop, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// AcceptLParen matches an opening parenthesis (.
func AcceptLParen(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('(')(r)
}

// AcceptRParen matches a closing parenthesis ).
func AcceptRParen(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(')')(r)
}

// AcceptLBrace matches an opening brace {.
func AcceptLBrace(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('{')(r)
}

// AcceptRBrace matches a closing brace }.
func AcceptRBrace(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('}')(r)
}

// AcceptLBracket matches an opening bracket [.
func AcceptLBracket(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('[')(r)
}

// AcceptRBracket matches a closing bracket ].
func AcceptRBracket(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(']')(r)
}

// AcceptLAngle matches a left angle bracket <.
func AcceptLAngle(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('<')(r)
}

// AcceptRAngle matches a right angle bracket >.
func AcceptRAngle(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('>')(r)
}

// AcceptComma matches a comma.
func AcceptComma(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(',')(r)
}

// AcceptColon matches a colon.
func AcceptColon(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(':')(r)
}

// AcceptSemicolon matches a semicolon.
func AcceptSemicolon(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(';')(r)
}

// AcceptPeriod matches a period.
func AcceptPeriod(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('.')(r)
}

// AcceptPlus matches a plus sign.
func AcceptPlus(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('+')(r)
}

// AcceptMinus matches a minus sign.
func AcceptMinus(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('-')(r)
}

// AcceptStar matches an asterisk.
func AcceptStar(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('*')(r)
}

// AcceptSlash matches a forward slash.
func AcceptSlash(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('/')(r)
}

// AcceptPercent matches a percent sign.
func AcceptPercent(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('%')(r)
}

// AcceptEqual matches an equals sign.
func AcceptEqual(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('=')(r)
}

// AcceptExclamation matches an exclamation mark.
func AcceptExclamation(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('!')(r)
}

// AcceptPipe matches a pipe symbol.
func AcceptPipe(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('|')(r)
}

// AcceptAmpersand matches an ampersand.
func AcceptAmpersand(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('&')(r)
}

// AcceptQuestionMark matches a question mark.
func AcceptQuestionMark(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('?')(r)
}
