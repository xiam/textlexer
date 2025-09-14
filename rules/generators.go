package rules

import (
	"fmt"
	"unicode"

	"github.com/xiam/textlexer"
)

// newRuleConsumer creates a rule consumer that can accept or reject a match
// based on the provided rules. It allows for custom acceptance and rejection
// functions to be specified.
func newRuleConsumer(rule textlexer.Rule, onAccept textlexer.Rule, onReject textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	if onAccept == nil {
		onAccept = func(r rune) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateAccept
		}
	}

	if onReject == nil {
		onReject = func(r rune) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateReject
		}
	}

	var matcher func(next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State)

	matcher = func(next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			var state textlexer.State

			if next == nil {
				next = rule
			}

			next, state = next(r)

			if state == textlexer.StateAccept {
				return onAccept(r)
			}

			if state == textlexer.StateReject {
				return onReject(r)
			}

			if state == textlexer.StateContinue || state == textlexer.StatePushBack {
				return matcher(next), state
			}

			panic("newRuleConsumer: unexpected state from inner rule " + state.String())
		}
	}

	return matcher(rule)
}

func newRuleWrapper(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if rule == nil {
			return nil, textlexer.StateReject
		}

		next, state := rule(r)

		if state == textlexer.StatePushBack {
			next, state = next(r) // consume the pushback
		}

		return next, state
	}
}

func newAnyOfCharactersMatcher(accept []rune) func(r rune) (textlexer.Rule, textlexer.State) {
	acceptMap := make(map[rune]struct{}, len(accept))
	for _, a := range accept {
		acceptMap[a] = struct{}{}
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		if _, ok := acceptMap[r]; ok {
			return nil, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}
}

func newAnyOfRulesMatcher(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var anyOfMatcher textlexer.Rule

		matching := make([]textlexer.Rule, len(rules))
		copy(matching, rules)

		anyOfMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
			var matched bool
			var state textlexer.State

			for i := range matching {
				rule := matching[i]
				if rule == nil {
					rule = rules[i]
				}

				matching[i], state = rule(r)

				if state == textlexer.StateContinue {
					matched = true
					continue
				}

				if state == textlexer.StateAccept {
					return nil, textlexer.StateAccept
				}

				if state == textlexer.StateReject {
					continue
				}

				panic("unexpected state from inner rule " + state.String())
			}

			if !matched {
				return nil, textlexer.StateReject
			}

			return anyOfMatcher, textlexer.StateContinue
		}

		return anyOfMatcher(r)
	}
}

func newSequenceIgnoreCaseMatcher(sequence []rune) func(r rune) (textlexer.Rule, textlexer.State) {
	for i := 0; i < len(sequence); i++ {
		sequence[i] = unicode.ToLower(sequence[i])
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		offset := 0

		var matcher textlexer.Rule

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(sequence) {
				// already ran through all characters
				return nil, textlexer.StateAccept
			}

			if unicode.ToLower(r) == sequence[offset] {
				offset++

				if offset < len(sequence) {
					return matcher, textlexer.StateContinue
				}

				return nil, textlexer.StateAccept
			}

			return nil, textlexer.StateReject
		}

		return matcher(r)
	}
}

func newLiteralSequenceMatcher(sequence []rune) func(r rune) (textlexer.Rule, textlexer.State) {
	var matcher func(int) func(rune) (textlexer.Rule, textlexer.State)

	matcher = func(offset int) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {

			if offset >= len(sequence) {
				// already ran through all characters
				return nil, textlexer.StateAccept
			}

			if r == sequence[offset] {
				if offset+1 < len(sequence) {
					return matcher(offset + 1), textlexer.StateContinue
				}

				return nil, textlexer.StateAccept
			}

			return nil, textlexer.StateReject
		}
	}

	return matcher(0)
}

func newChainedRuleMatcher(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	var matcher func(idx int) func(r rune) (textlexer.Rule, textlexer.State)

	matcher = func(idx int) func(r rune) (textlexer.Rule, textlexer.State) {
		var start textlexer.Rule

		if idx >= len(rules) {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				panic("matcher: already ran through all rules")
			}
		}

		rule := rules[idx]

		var next textlexer.Rule

		start = func(r rune) (textlexer.Rule, textlexer.State) {
			var state textlexer.State

			if next == nil {
				next = rule
			}

			next, state = next(r)

			if state == textlexer.StatePushBack {
				if next == nil {
					next = rule
				}
				next, state = next(r) // consume the pushback
				if state == textlexer.StateAccept {
					if idx+1 < len(rules) {
						return matcher(idx + 1)(r)
					}
					return PushBackCurrentAndAccept(r)
				}
			}

			if state == textlexer.StateAccept {
				if idx+1 < len(rules) {
					return matcher(idx + 1), textlexer.StateContinue
				}
				return nil, textlexer.StateAccept
			}

			if state == textlexer.StateContinue {
				if IsEOF(r) {
					return nil, textlexer.StateReject
				}
				return start, textlexer.StateContinue
			}

			if state == textlexer.StateReject {
				return nil, textlexer.StateReject
			}

			panic("matcher: not implemented yet " + state.String())
		}

		return start
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		return matcher(0)(r)
	}
}

// newReverseStateMatcher creates a matcher that reverses the state of the
// provided rule. It will accept if the rule rejects, and vice versa.
func newReverseStateMatcher(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var matcher textlexer.Rule

		var next textlexer.Rule
		var state textlexer.State

		var matching bool

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			if next == nil {
				next = rule
			}

			next, state = next(r)

			if state == textlexer.StatePushBack {
				_, state = next(r) // consume the pushback
				next = nil
			}

			if state == textlexer.StateAccept {
				return nil, textlexer.StateReject
			}

			if state == textlexer.StateReject {
				if IsEOF(r) {
					return PushBackCurrentAndAccept(r)
				}

				next = nil
				matching = true
				return matcher, textlexer.StateContinue
			}

			if state == textlexer.StateContinue {
				if matching {
					return PushBackCurrentAndAccept(r)
				}

				if IsEOF(r) {
					return nil, textlexer.StateReject
				}

				return matcher, textlexer.StateContinue
			}

			panic("unexpected state from inner rule " + state.String())
		}

		return matcher(r)
	}
}

func newWhitespaceConsumer(continueWith textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	var matcher textlexer.Rule
	matcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) {
			return matcher, textlexer.StateContinue
		}

		return continueWith(r)
	}

	return matcher
}

type ruleState struct {
	rule  textlexer.Rule
	state textlexer.State
}

func NewSwitchMatcher(matchers ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var switchMatcher textlexer.Rule

		ruleStates := map[int]*ruleState{}

		for i, matcher := range matchers {
			ruleStates[i] = &ruleState{
				rule:  matcher,
				state: textlexer.StateContinue,
			}
		}

		furthestAccepted := -1
		furthestPushBack := -1

		switchMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
			matched := false

			for i := range ruleStates {
				ruleState := ruleStates[i]

				if ruleState.state == textlexer.StateAccept || ruleState.state == textlexer.StatePushBack {
					continue
				}

				if ruleState.state == textlexer.StateContinue {
					next, rule := ruleState.rule(r)

					ruleState.state = rule
					ruleState.rule = next
				}

				if ruleState.state == textlexer.StateContinue {
					matched = true
					continue
				}

				if ruleState.state == textlexer.StateAccept {
					furthestAccepted = i
					continue
				}

				if ruleState.state == textlexer.StatePushBack {
					furthestPushBack = i
					continue
				}

				if ruleState.state == textlexer.StateReject {
					// If the rule rejects, we can remove it from the list of active rules
					delete(ruleStates, i)
					continue
				}

				panic(fmt.Sprintf("unexpected state from inner rule %s", ruleState.state.String()))
			}

			if !matched {
				if furthestAccepted >= 0 {
					return ruleStates[furthestAccepted].rule, textlexer.StateAccept
				}

				if furthestPushBack >= 0 {
					return ruleStates[furthestPushBack].rule, textlexer.StatePushBack
				}

				return nil, textlexer.StateReject
			}

			return switchMatcher, textlexer.StateContinue
		}

		return switchMatcher(r)
	}
}

// NewGreedyFunctionMatcher creates a rule that matches a sequence of
// characters based on a custom filter function. The filter function should
// return true for runes that should be accepted and false otherwise.
func NewGreedyFunctionMatcher(filter func(r rune) bool) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var matcher textlexer.Rule

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			// Check if the current rune is accepted by the filter function
			if filter(r) {
				// If it is, continue matching
				return matcher, textlexer.StateContinue
			}

			// If the current rune is not accepted, push back and accept
			return PushBackCurrentAndAccept(r)
		}

		if filter(r) {
			// if the first rune is accepted, we can start matching
			return matcher, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}
}

// NewGreedyCharacterMatcher creates a rule that matches a sequence of
// characters based on a given list. The rule will consume characters until it
// encounters a character not in the list. The rule will accept the sequence of
// characters matched so far.
func NewGreedyCharacterMatcher(accept []rune) func(r rune) (textlexer.Rule, textlexer.State) {
	acceptMap := make(map[rune]struct{}, len(accept))
	for _, a := range accept {
		acceptMap[a] = struct{}{}
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		var matcher textlexer.Rule

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			// Check if the current rune is in the list of accepted runes
			if _, ok := acceptMap[r]; ok {
				// If it is, continue matching
				return matcher, textlexer.StateContinue
			}

			// If the current rune is not in the list, accept the matched sequence
			// and stop matching
			return PushBackCurrentAndAccept(r)
		}

		if _, ok := acceptMap[r]; ok {
			// if the first rune is accepted, we can start matching
			return matcher, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}
}

// NewFunctionSingleMatcher creates a rule that matches a rune based on a custom
// filter function. The filter function should return true for runes that
// should be accepted and false otherwise.
func NewFunctionSingleMatcher(filter func(r rune) bool) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if filter(r) {
			return nil, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

// NewCharacterMatcher creates a rule that matches only the specified
// rune.
func NewCharacterMatcher(match rune) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if r == match {
			return nil, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}
}

// NewMatchExceptString creates a rule that consumes characters until a
// specific string is found, then transitions to the 'next' rule. The target
// string itself is matched.
func NewMatchExceptString(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	runes := []rune(match)

	var matcher func(i int) textlexer.Rule

	matcher = func(i int) textlexer.Rule {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			if i >= len(runes) {
				return nil, textlexer.StateAccept
			}

			if r == runes[i] {
				if i+1 >= len(runes) {
					return nil, textlexer.StateAccept
				}
				return matcher(i + 1), textlexer.StateContinue
			}

			if IsEOF(r) {
				return nil, textlexer.StateReject
			}

			return matcher(0), textlexer.StateContinue
		}
	}

	return matcher(0)
}

// NewMatchStartingWithString creates a rule that requires the input to start
// exactly with the specified string, then transitions to the 'next' rule.
func NewMatchStartingWithString(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	start := newLiteralSequenceMatcher([]rune(match))
	return newChainedRuleMatcher(
		start,
		next,
	)
}

// NewMatchString creates a rule that matches an exact string.
func NewMatchString(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	return newLiteralSequenceMatcher([]rune(match))
}

// NewMatchStringIgnoreCase creates a rule that matches an exact string,
// ignoring case.
func NewMatchStringIgnoreCase(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	return newSequenceIgnoreCaseMatcher([]rune(match))
}

func NewMatchInvertedRule(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	matcher := newReverseStateMatcher(rule)

	return matcher
}

// NewMatchRuleSequence creates a rule that matches a sequence of other rules
// in order.
func NewMatchRuleSequence(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return newChainedRuleMatcher(rules...)
}

// NewMatchAnyRule creates a rule that matches if *any* of the provided rules
// match. The first rule that accepts will cause the overall match to accept.
func NewMatchAnyRule(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return newAnyOfRulesMatcher(rules...)
}

// NewLookaheadMatcher creates a rule that matches if the main rule matches and
// is followed by a pattern that matches the lookahead rule. The lookahead
// pattern is not consumed.
func NewLookaheadMatcher(mainRule, lookaheadRule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var lookaheadWrapper func(textlexer.Rule) func(rune) (textlexer.Rule, textlexer.State)

		var offset int

		lookaheadWrapper = func(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				next, state := rule(r)
				if state == textlexer.StatePushBack {
					offset--
				}
				if state == textlexer.StateContinue || state == textlexer.StateAccept {
					offset++
				}
				return lookaheadWrapper(next), state
			}
		}

		lookaheadConsumer := newRuleConsumer(
			lookaheadWrapper(lookaheadRule),
			// on accept, backtrack to the main rule and accept
			func(r rune) (textlexer.Rule, textlexer.State) {
				return Backtrack(offset, textlexer.StateAccept)(r)
			},
			nil,
		)

		mainConsumer := newRuleConsumer(
			mainRule,
			// on accept, continue with the lookahead consumer
			func(r rune) (textlexer.Rule, textlexer.State) {
				return lookaheadConsumer, textlexer.StateContinue
			},
			nil,
		)

		return mainConsumer(r)
	}
}
