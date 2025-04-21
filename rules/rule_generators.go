package rules

import (
	"unicode"

	"github.com/xiam/textlexer"
)

func NewAcceptSingleCharacter(match rune) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if r == match {
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}
}

func NewMatchUntilString(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewMatchStartingWithString(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewMatchString(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptCurrentAndStop
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

func NewMatchStringIgnoreCase(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptCurrentAndStop
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

func NewMatchInvertedRule(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewMatchRuleSequence(rules ...func(r rune) (textlexer.Rule, textlexer.State)) func(r rune) (textlexer.Rule, textlexer.State) {
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

func NewMatchAnyRule(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
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
