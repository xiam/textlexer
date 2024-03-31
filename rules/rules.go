package rules

import (
	"unicode"

	"github.com/xiam/textlexer"
)

func AlwaysReject(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateReject
}

func AlwaysAccept(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

func AlwaysContinue(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateContinue
}

func UnsignedIntegerTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextDigit textlexer.Rule

	nextDigit = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more digits
		if unicode.IsDigit(r) {
			return nextDigit, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	// starts with a digit
	if unicode.IsDigit(r) {
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

func SignedIntegerTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var discardWhitespace textlexer.Rule

	discardWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsSpace(r) {
			return discardWhitespace, textlexer.StateReject
		}

		return UnsignedIntegerTokenRule(r)
	}

	// a signed integer may start with a minus or plus sign
	if r == '-' || r == '+' {
		// discard whitespace after the sign
		return discardWhitespace, textlexer.StateContinue
	}

	return UnsignedIntegerTokenRule(r)
}

func UnsignedFloatTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var integerPart, radixPoint, fractionalPart textlexer.Rule

	integerPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsDigit(r) {
			return integerPart, textlexer.StateContinue
		}

		// expects a point after the integer part
		return radixPoint(r)
	}

	radixPoint = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '.' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				// expects a digit immediately after the radix point
				if unicode.IsDigit(r) {
					return fractionalPart, textlexer.StateContinue
				}

				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	fractionalPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsDigit(r) {
			return fractionalPart, textlexer.StateContinue
		}

		return nil, textlexer.StateAccept
	}

	if unicode.IsDigit(r) {
		return integerPart, textlexer.StateContinue
	}

	return radixPoint(r)
}

func NewSingleMatchTokenRule(match rune) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewLiteralMatchTokenRule(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptTokenRule
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

func NewCaseInsensitiveLiteralMatchTokenRule(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptTokenRule
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

func SignedFloatTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var discardWhitespace textlexer.Rule

	discardWhitespace = func(r rune) (textlexer.Rule, textlexer.State) {
		if unicode.IsSpace(r) {
			return discardWhitespace, textlexer.StateReject
		}

		return UnsignedFloatTokenRule(r)
	}

	if r == '-' || r == '+' {
		return discardWhitespace, textlexer.StateContinue
	}

	return UnsignedFloatTokenRule(r)
}

func WhitespaceTokenRule(r rune) (textlexer.Rule, textlexer.State) {
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

func WordTokenRule(r rune) (next textlexer.Rule, state textlexer.State) {
	var nextLetter textlexer.Rule

	nextLetter = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more letters or digits
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
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

func DoubleQuotedStringTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '"' {
			return AcceptTokenRule, textlexer.StateContinue
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

func SingleQuotedStringTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\'' {
			return AcceptTokenRule, textlexer.StateContinue
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

func DoubleQuotedFormattedStringTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateReject
		}

		if r == '"' {
			return AcceptTokenRule, textlexer.StateContinue
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

func InlineCommentTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewChainAnyAfterLiteralMatchRule("//", UntilEOLTokenRule)(r)
}

func UntilEOFTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return UntilEOFTokenRule, textlexer.StateContinue
	}(r)
}

func UntilEOLTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	var untilNewLine textlexer.Rule

	untilNewLine = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\n' || textlexer.IsEOF(r) {
			return nil, textlexer.StateAccept
		}

		return untilNewLine, textlexer.StateContinue
	}

	return untilNewLine(r)
}

func AcceptTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

func BasicMathOperatorTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	if r == '+' || r == '-' || r == '*' || r == '/' {
		return AcceptTokenRule, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

func InvertTokenRule(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func ComposeTokenRules(rules ...func(r rune) (textlexer.Rule, textlexer.State)) func(r rune) (textlexer.Rule, textlexer.State) {
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

func SlashStarCommentTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewChainAnyAfterLiteralMatchRule(
		"/*",
		NewChainAnyUntilLiteralMatchRule(
			"*/",
			AcceptTokenRule,
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

func LParenTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('(')(r)
}

func RParenTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule(')')(r)
}

func LBraceTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('{')(r)
}

func RBraceTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('}')(r)
}

func LBracketTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('[')(r)
}

func RBracketTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule(']')(r)
}

func LAngleTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('<')(r)
}

func RAngleTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('>')(r)
}

func CommaTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule(',')(r)
}

func ColonTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule(':')(r)
}

func SemicolonTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule(';')(r)
}

func PeriodTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('.')(r)
}

func PlusTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('+')(r)
}

func MinusTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('-')(r)
}

func StarTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('*')(r)
}

func SlashTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('/')(r)
}

func PercentTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('%')(r)
}

func EqualTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('=')(r)
}

func ExclamationTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('!')(r)
}

func PipeTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('|')(r)
}

func AmpersandTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('&')(r)
}

func QuestionMarkTokenRule(r rune) (textlexer.Rule, textlexer.State) {
	return NewSingleMatchTokenRule('?')(r)
}
