package rules

import (
	"github.com/xiam/textlexer"
)

func AcceptCurrentAndStop(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

func RejectCurrent(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateReject
}

func MatchAnyCharacter(r rune) (textlexer.Rule, textlexer.State) {
	return MatchAnyCharacter, textlexer.StateContinue
}

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

func MatchUntilWhitespaceOrEOF(r rune) (textlexer.Rule, textlexer.State) {
	if isCommonWhitespace(r) || textlexer.IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nil, textlexer.StateReject
}

func MatchSignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	var skipCommonWhitespace textlexer.Rule

	skipCommonWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) {
			return skipCommonWhitespace, textlexer.StateReject
		}

		return MatchUnsignedInteger(r)
	}

	// a signed integer may start with a minus or plus sign
	if r == '-' || r == '+' {
		// discard whitespace after the sign
		return skipCommonWhitespace, textlexer.StateContinue
	}

	return MatchUnsignedInteger(r)
}

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

	return radixPoint(r)
}

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

	return expectInteger(r)
}

func MatchSignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	var anyWhitespace, expectInteger, scanInteger, expectDecimal, scanDecimal textlexer.Rule

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

	anyWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) {
			return anyWhitespace, textlexer.StateContinue
		}

		return expectInteger(r)
	}

	if r == '-' || r == '+' {
		return anyWhitespace, textlexer.StateContinue
	}

	return expectInteger(r)
}

func MatchSignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var skipCommonWhitespace textlexer.Rule

	skipCommonWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) {
			return skipCommonWhitespace, textlexer.StateReject
		}

		return MatchUnsignedFloat(r)
	}

	if r == '-' || r == '+' {
		return skipCommonWhitespace, textlexer.StateContinue
	}

	return MatchUnsignedFloat(r)
}

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

func MatchDoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '"' {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		return nextChar, textlexer.StateContinue
	}

	if r == '"' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func MatchSingleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\'' {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		return nextChar, textlexer.StateContinue
	}

	if r == '\'' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func MatchEscapedDoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		if r == '"' {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		if r == '\\' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if isCommonWhitespace(r) || textlexer.IsEOF(r) {
					return nil, textlexer.StateReject
				}

				return nextChar, textlexer.StateContinue
			}, textlexer.StateContinue
		}

		return nextChar, textlexer.StateContinue
	}

	if r == '"' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func MatchInlineComment(r rune) (textlexer.Rule, textlexer.State) {
	return NewMatchStartingWithString("//", MatchUntilEOL)(r)
}

func MatchUntilEOF(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return MatchUntilEOF, textlexer.StateContinue
	}(r)
}

func MatchUntilEOL(r rune) (textlexer.Rule, textlexer.State) {
	var untilNewLine textlexer.Rule

	untilNewLine = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\n' || textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return untilNewLine, textlexer.StateContinue
	}

	return untilNewLine(r)
}

func MatchBasicMathOperator(r rune) (textlexer.Rule, textlexer.State) {
	if r == '+' || r == '-' || r == '*' || r == '/' {
		return AcceptCurrentAndStop, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func MatchSlashStarComment(r rune) (textlexer.Rule, textlexer.State) {
	return NewMatchStartingWithString(
		"/*",
		NewMatchUntilString(
			"*/",
			AcceptCurrentAndStop,
		),
	)(r)
}

func AcceptAnyParen(r rune) (textlexer.Rule, textlexer.State) {
	if r == '(' || r == ')' {
		return AcceptCurrentAndStop, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func AcceptLParen(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('(')(r)
}

func AcceptRParen(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(')')(r)
}

func AcceptLBrace(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('{')(r)
}

func AcceptRBrace(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('}')(r)
}

func AcceptLBracket(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('[')(r)
}

func AcceptRBracket(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(']')(r)
}

func AcceptLAngle(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('<')(r)
}

func AcceptRAngle(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('>')(r)
}

func AcceptComma(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(',')(r)
}

func AcceptColon(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(':')(r)
}

func AcceptSemicolon(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter(';')(r)
}

func AcceptPeriod(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('.')(r)
}

func AcceptPlus(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('+')(r)
}

func AcceptMinus(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('-')(r)
}

func AcceptStar(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('*')(r)
}

func AcceptSlash(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('/')(r)
}

func AcceptPercent(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('%')(r)
}

func AcceptEqual(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('=')(r)
}

func AcceptExclamation(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('!')(r)
}

func AcceptPipe(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('|')(r)
}

func AcceptAmpersand(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('&')(r)
}

func AcceptQuestionMark(r rune) (textlexer.Rule, textlexer.State) {
	return NewAcceptSingleCharacter('?')(r)
}
