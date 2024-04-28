package rules

import (
	"github.com/xiam/textlexer"
)

func Accept(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

func Reject(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateReject
}

func AlwaysReject(r rune) (textlexer.Rule, textlexer.State) {
	return AlwaysReject, textlexer.StateReject
}

func AlwaysAccept(r rune) (textlexer.Rule, textlexer.State) {
	return AlwaysAccept, textlexer.StateAccept
}

func AlwaysContinue(r rune) (textlexer.Rule, textlexer.State) {
	return AlwaysContinue, textlexer.StateContinue
}

func UnsignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	var nextDigit textlexer.Rule

	nextDigit = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more digits
		if isNumeric(r) {
			return nextDigit, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	// starts with a digit
	if isNumeric(r) {
		return nextDigit, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func WhitespaceDelimiter(r rune) (textlexer.Rule, textlexer.State) {
	if isSpace(r) || textlexer.IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nil, textlexer.StateReject
}

func SignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	var skipWhitespace textlexer.Rule

	skipWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isSpace(r) {
			return skipWhitespace, textlexer.StateReject
		}

		return UnsignedInteger(r)
	}

	// a signed integer may start with a minus or plus sign
	if r == '-' || r == '+' {
		// discard whitespace after the sign
		return skipWhitespace, textlexer.StateContinue
	}

	return UnsignedInteger(r)
}

func UnsignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var integerPart, radixPoint, fractionalPart textlexer.Rule

	integerPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return integerPart, textlexer.StateContinue
		}

		// expects a point after the integer part
		return radixPoint(r)
	}

	radixPoint = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '.' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				// expects a digit immediately after the radix point
				if isNumeric(r) {
					return fractionalPart, textlexer.StateContinue
				}

				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	fractionalPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return fractionalPart, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	if isNumeric(r) {
		return integerPart, textlexer.StateContinue
	}

	return radixPoint(r)
}

func NewSingleMatch(match rune) func(r rune) (textlexer.Rule, textlexer.State) {
	anyChar := func(r rune) (textlexer.Rule, textlexer.State) {
		return nil, textlexer.StateAccept
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		if r == match {
			return anyChar, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

func NewChainAnyUntilLiteralMatch(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule
		var offset int

		if match == "" {
			return nil, textlexer.StateReject
		}

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(match) {
				return next(r)
			}

			if r == rune(match[offset]) {
				offset++
				return nextChar, textlexer.StateContinue
			}

			if textlexer.IsEOF(r) {
				return nil, textlexer.StateReject
			}

			return nextChar, textlexer.StateContinue
		}

		return nextChar(r)
	}
}

func NewChainAnyAfterLiteralMatch(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule
		var offset int

		if len(match) == 0 {
			return nil, textlexer.StateReject
		}

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(match) {
				return next(r)
			}

			if r == rune(match[offset]) {
				offset++
				return nextChar, textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}

		return nextChar(r)
	}
}

func NewLiteralMatch(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return Accept
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {

		var nextChar textlexer.Rule
		var offset int

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(match) {
				return nil, textlexer.StateAccept
			}

			if r == rune(match[offset]) {
				offset++
				return nextChar, textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}

		return nextChar(r)
	}
}

func NewCaseInsensitiveLiteralMatch(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return Accept
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule

		offset := 0

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(match) {
				return nil, textlexer.StateAccept
			}

			if toLower(r) == toLower(rune(match[offset])) {
				offset++
				return nextChar, textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}

		return nextChar(r)
	}
}

func UnsignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	var expectInteger, scanInteger, expectDecimal, scanDecimal textlexer.Rule

	scanDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	scanInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanInteger, textlexer.StateContinue
		}

		if r == '.' {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	expectDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	expectInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanInteger, textlexer.StateContinue
		}

		if r == '.' {
			return expectDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	return expectInteger(r)
}

func Numeric(r rune) (textlexer.Rule, textlexer.State) {
	var anyWhitespace, expectInteger, scanInteger, expectDecimal, scanDecimal textlexer.Rule

	scanDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	scanInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanInteger, textlexer.StateContinue
		}

		if r == '.' {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	expectDecimal = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	expectInteger = func(r rune) (textlexer.Rule, textlexer.State) {
		if isNumeric(r) {
			return scanInteger, textlexer.StateContinue
		}

		if r == '.' {
			return expectDecimal, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	anyWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isSpace(r) {
			return anyWhitespace, textlexer.StateContinue
		}

		return expectInteger(r)
	}

	if r == '-' || r == '+' {
		return anyWhitespace, textlexer.StateContinue
	}

	return expectInteger(r)
}

func SignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var skipWhitespace textlexer.Rule

	skipWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isSpace(r) {
			return skipWhitespace, textlexer.StateReject
		}

		return UnsignedFloat(r)
	}

	if r == '-' || r == '+' {
		return skipWhitespace, textlexer.StateContinue
	}

	return UnsignedFloat(r)
}

func Whitespace(r rune) (textlexer.Rule, textlexer.State) {
	var nextSpace textlexer.Rule

	nextSpace = func(r rune) (textlexer.Rule, textlexer.State) {
		if isSpace(r) {
			return nextSpace, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	if isSpace(r) {
		return nextSpace, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func Word(r rune) (next textlexer.Rule, state textlexer.State) {
	var nextLetter textlexer.Rule

	nextLetter = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more letters or digits
		if isLetter(r) || isNumeric(r) {
			return nextLetter, textlexer.StateContinue
		}

		// ends with any other character
		return nil, textlexer.StateAccept
	}

	// starts with a letter
	if isLetter(r) {
		return nextLetter, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func DoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '"' {
			return Accept, textlexer.StateContinue
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

func SingleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\'' {
			return Accept, textlexer.StateContinue
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

func DoubleQuotedFormattedString(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		if r == '"' {
			return Accept, textlexer.StateContinue
		}

		if r == '\\' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if isSpace(r) || textlexer.IsEOF(r) {
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

func InlineComment(r rune) (textlexer.Rule, textlexer.State) {
	return NewChainAnyAfterLiteralMatch("//", UntilEOL)(r)
}

func UntilEOF(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return UntilEOF, textlexer.StateContinue
	}(r)
}

func UntilEOL(r rune) (textlexer.Rule, textlexer.State) {
	var untilNewLine textlexer.Rule

	untilNewLine = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\n' || textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return untilNewLine, textlexer.StateContinue
	}

	return untilNewLine(r)
}

func BasicMathOperator(r rune) (textlexer.Rule, textlexer.State) {
	if r == '+' || r == '-' || r == '*' || r == '/' {
		return Accept, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func Invert(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	var contRejected func(textlexer.Rule) func(rune) (textlexer.Rule, textlexer.State)
	var contContinued func(textlexer.Rule) func(rune) (textlexer.Rule, textlexer.State)

	contRejected = func(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			next, state := rule(r)
			if state == textlexer.StateReject {
				if textlexer.IsEOF(r) {
					return nil, textlexer.StateAccept
				}
				if next == nil {
					return contRejected(rule), textlexer.StateContinue
				}
				return contRejected(next), textlexer.StateContinue
			}

			if state == textlexer.StateContinue {
				return nil, textlexer.StateAccept
			}

			panic("unexpected state")
		}
	}

	contContinued = func(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			next, state := rule(r)

			if state == textlexer.StateReject {
				if next != nil {
					return contContinued(next), textlexer.StateContinue
				}
				return nil, textlexer.StateAccept
			}

			if state == textlexer.StateContinue {
				if next != nil {
					return contContinued(next), textlexer.StateContinue
				}
				return contContinued(rule), textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		next, state := rule(r)

		if state == textlexer.StateReject {
			return contRejected(rule), textlexer.StateContinue
		}

		if state == textlexer.StateContinue {
			if next != nil {
				return contContinued(next), textlexer.StateContinue
			}
			return contContinued(rule), textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}
}

func Compose(rules ...func(r rune) (textlexer.Rule, textlexer.State)) func(r rune) (textlexer.Rule, textlexer.State) {
	var match func(int, textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State)

	match = func(offset int, rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {

			if offset >= len(rules) {
				return nil, textlexer.StateAccept
			}

			if rule == nil {
				rule = rules[offset]
			}

			next, state := rule(r)

			if state == textlexer.StateReject {
				return nil, textlexer.StateReject
			}

			if state == textlexer.StateAccept {
				return match(offset+1, nil)(r)
			}

			return match(offset, next), state
		}
	}

	return match(0, nil)
}

func SlashStarComment(r rune) (textlexer.Rule, textlexer.State) {
	return NewChainAnyAfterLiteralMatch(
		"/*",
		NewChainAnyUntilLiteralMatch(
			"*/",
			Accept,
		),
	)(r)
}

func NewMatchAnyOf(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	var matchAnyOf func([]textlexer.Rule) textlexer.Rule

	matchAnyOf = func(rules []textlexer.Rule) textlexer.Rule {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			var next textlexer.Rule
			var state textlexer.State

			matched := []textlexer.Rule{}

			for i := range rules {
				next, state = rules[i](r)
				if state == textlexer.StateAccept {
					return nil, textlexer.StateAccept
				}

				if state == textlexer.StateContinue {
					if next != nil {
						matched = append(matched, next)
					} else {
						matched = append(matched, rules[i])
					}
				}
			}

			if len(matched) > 0 {
				return matchAnyOf(matched), textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}
	}

	return matchAnyOf(rules)
}

func Paren(r rune) (textlexer.Rule, textlexer.State) {
	if r == '(' || r == ')' {
		return Accept, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func LParen(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('(')(r)
}

func RParen(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch(')')(r)
}

func LBrace(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('{')(r)
}

func RBrace(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('}')(r)
}

func LBracket(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('[')(r)
}

func RBracket(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch(']')(r)
}

func LAngle(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('<')(r)
}

func RAngle(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('>')(r)
}

func Comma(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch(',')(r)
}

func Colon(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch(':')(r)
}

func Semicolon(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch(';')(r)
}

func Period(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('.')(r)
}

func Plus(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('+')(r)
}

func Minus(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('-')(r)
}

func Star(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('*')(r)
}

func Slash(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('/')(r)
}

func Percent(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('%')(r)
}

func Equal(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('=')(r)
}

func Exclamation(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('!')(r)
}

func Pipe(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('|')(r)
}

func Ampersand(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('&')(r)
}

func QuestionMark(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatch('?')(r)
}
