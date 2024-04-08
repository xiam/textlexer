package rules

import (
	"unicode"

	"github.com/xiam/textlexer"
)

func AlwaysReject(r rune) (textlexer.Rule, textlexer.State) {
	return AlwaysReject, textlexer.StateReject
}

func AlwaysAccept(r rune) (textlexer.Rule, textlexer.State) {
	return AlwaysAccept, textlexer.StateAccept
}

func AlwaysContinue(r rune) (textlexer.Rule, textlexer.State) {
	return AlwaysContinue, textlexer.StateContinue
}

func UnsignedIntegerLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
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

func WhitespaceDelimiterRule(r rune) (textlexer.Rule, textlexer.State) {
	if unicode.IsSpace(r) || textlexer.IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nil, textlexer.StateReject
}

func SignedIntegerLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var discardWhitespace textlexer.Rule

	discardWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsSpace(r) {
			return discardWhitespace, textlexer.StateReject
		}

		return UnsignedIntegerLexemeRule(r)
	}

	// a signed integer may start with a minus or plus sign
	if r == '-' || r == '+' {
		// discard whitespace after the sign
		return discardWhitespace, textlexer.StateContinue
	}

	return UnsignedIntegerLexemeRule(r)
}

func UnsignedFloatLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
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

func NewSingleMatchLexemeRule(match rune) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewChainAnyUntilLiteralMatchRule(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewChainAnyAfterLiteralMatchRule(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewLiteralMatchLexemeRule(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptLexemeRule
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

func NewCaseInsensitiveLiteralMatchLexemeRule(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptLexemeRule
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule

		offset := 0

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(match) {
				return nil, textlexer.StateAccept
			}

			if unicode.ToLower(r) == unicode.ToLower(rune(match[offset])) {
				offset++
				return nextChar, textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}

		return nextChar(r)
	}
}

func NumericLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var discardWhitespace, expectInteger, scanInteger, expectDecimal, scanDecimal textlexer.Rule

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

	discardWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsSpace(r) {
			return discardWhitespace, textlexer.StateReject
		}

		return expectInteger(r)
	}

	if r == '-' || r == '+' {
		return discardWhitespace, textlexer.StateContinue
	}

	return expectInteger(r)
}

func SignedFloatLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var discardWhitespace textlexer.Rule

	discardWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsSpace(r) {
			return discardWhitespace, textlexer.StateReject
		}

		return UnsignedFloatLexemeRule(r)
	}

	if r == '-' || r == '+' {
		return discardWhitespace, textlexer.StateContinue
	}

	return UnsignedFloatLexemeRule(r)
}

func WhitespaceLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextSpace textlexer.Rule

	nextSpace = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsSpace(r) {
			return nextSpace, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	if unicode.IsSpace(r) {
		return nextSpace, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func WordLexemeRule(r rune) (next textlexer.Rule, state textlexer.State) {
	var nextLetter textlexer.Rule

	nextLetter = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more letters or digits
		if unicode.IsLetter(r) || isNumeric(r) {
			return nextLetter, textlexer.StateContinue
		}

		// ends with any other character
		return nil, textlexer.StateAccept
	}

	// starts with a letter
	if unicode.IsLetter(r) {
		return nextLetter, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func DoubleQuotedStringLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '"' {
			return AcceptLexemeRule, textlexer.StateContinue
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

func SingleQuotedStringLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\'' {
			return AcceptLexemeRule, textlexer.StateContinue
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

func DoubleQuotedFormattedStringLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		if r == '"' {
			return AcceptLexemeRule, textlexer.StateContinue
		}

		if r == '\\' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if unicode.IsSpace(r) || textlexer.IsEOF(r) {
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

func InlineCommentLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewChainAnyAfterLiteralMatchRule("//", UntilEOLLexemeRule)(r)
}

func UntilEOFLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return UntilEOFLexemeRule, textlexer.StateContinue
	}(r)
}

func UntilEOLLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	var untilNewLine textlexer.Rule

	untilNewLine = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\n' || textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return untilNewLine, textlexer.StateContinue
	}

	return untilNewLine(r)
}

func AcceptLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

func BasicMathOperatorLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	if r == '+' || r == '-' || r == '*' || r == '/' {
		return AcceptLexemeRule, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func InvertLexemeRule(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func ComposeLexemeRules(rules ...func(r rune) (textlexer.Rule, textlexer.State)) func(r rune) (textlexer.Rule, textlexer.State) {
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

func SlashStarCommentLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewChainAnyAfterLiteralMatchRule(
		"/*",
		NewChainAnyUntilLiteralMatchRule(
			"*/",
			AcceptLexemeRule,
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

func LParenLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('(')(r)
}

func RParenLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule(')')(r)
}

func LBraceLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('{')(r)
}

func RBraceLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('}')(r)
}

func LBracketLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('[')(r)
}

func RBracketLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule(']')(r)
}

func LAngleLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('<')(r)
}

func RAngleLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('>')(r)
}

func CommaLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule(',')(r)
}

func ColonLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule(':')(r)
}

func SemicolonLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule(';')(r)
}

func PeriodLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('.')(r)
}

func PlusLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('+')(r)
}

func MinusLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('-')(r)
}

func StarLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('*')(r)
}

func SlashLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('/')(r)
}

func PercentLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('%')(r)
}

func EqualLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('=')(r)
}

func ExclamationLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('!')(r)
}

func PipeLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('|')(r)
}

func AmpersandLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('&')(r)
}

func QuestionMarkLexemeRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchLexemeRule('?')(r)
}

func isNumeric(r rune) bool {
	if r >= '0' && r <= '9' {
		return true
	}
	return false
}
