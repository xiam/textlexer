package rules

import (
	"fmt"
	stdlog "log"
	"unicode"

	"github.com/xiam/textlexer"
)

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

		ruleStates := map[int]*ruleState{}

		for i, matcher := range rules {
			ruleStates[i] = &ruleState{
				rule:  matcher,
				state: textlexer.StateContinue,
			}
		}

		anyOfMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
			matched := false

			for i := range ruleStates {
				ruleState := ruleStates[i]

				if ruleState.state == textlexer.StateContinue {
					next, state := ruleState.rule(r)

					ruleState.state = state
					ruleState.rule = next
				}

				if ruleState.state == textlexer.StateContinue {
					matched = true
					continue
				}

				if ruleState.state == textlexer.StateAccept {
					return nil, textlexer.StateAccept
				}

				if ruleState.state == textlexer.StateReject {
					delete(ruleStates, i)
					continue
				}

				panic("unexpected state from inner rule " + ruleState.state.String())
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

				stdlog.Printf("matched sequence %q\n", sequence[:offset])
				return nil, textlexer.StateAccept
			}

			return nil, textlexer.StateReject
		}

		return matcher(r)
	}
}

func newLiteralSequenceMatcher(sequence []rune) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		offset := 0

		var matcher textlexer.Rule

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset >= len(sequence) {
				// already ran through all characters
				return nil, textlexer.StateAccept
			}

			if r == sequence[offset] {
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
			stdlog.Printf("idx: %d, start: %q\n", idx, r)

			if next == nil {
				next = rule
			}

			next, state = next(r)
			stdlog.Printf("next: %v, state: %s\n", next, state)

			if state == textlexer.StatePushBack {
				if next == nil {
					next = rule
				}
				next, state = next(r) // consume the pushback
				stdlog.Printf("AFTER PUSHBACK: next: %v, state: %s\n", next, state)
				if state == textlexer.StateAccept {
					if idx+1 < len(rules) {
						return matcher(idx + 1)(r)
					}
					return PushBackCurrentAndAccept(r)
				}
			}

			if state == textlexer.StateAccept {
				if idx+1 < len(rules) {
					stdlog.Printf("idx+1: %d, len(rules): %d\n", idx+1, len(rules))
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

	/*
		return func(r rune) (textlexer.Rule, textlexer.State) {
			var matcher textlexer.Rule

			var next textlexer.Rule

			offset := 0

			matcher = func(r rune) (textlexer.Rule, textlexer.State) {
				var state textlexer.State

				if offset >= len(rules) {
					stdlog.Printf("matcher: already ran through all rules")
					// already ran through all rules
					return nil, textlexer.StateReject
				}

				if next == nil {
					stdlog.Printf("rule was reset, offset: %d\n", offset)
					next = rules[offset]
				}

				next, state = next(r)
				stdlog.Printf("next: %v, state: %s\n", next, state)

				pushedBack := false
				if state == textlexer.StatePushBack {
					stdlog.Printf("pushed back after reading %q\n", r)

					return matcher, textlexer.StatePushBack

					// consume the pushback
					next, state = next(r)
					if state == textlexer.StateReject {
						return nil, textlexer.StateReject
					}

					stdlog.Printf("AFTER PUSHBACK: next: %v, state: %s\n", next, state)
					if state == textlexer.StateAccept {
						return PushBackCurrentAndAccept(r)
					}

					pushedBack = true
				}

				if state == textlexer.StateContinue {
					if IsEOF(r) {
						return nil, textlexer.StateReject
					}

					return matcher, textlexer.StateContinue
				}

				if state == textlexer.StateAccept {
					offset++

					if offset < len(rules) {
						stdlog.Printf("advanced to next rule: %d\n", offset)
						// there are more rules to process
						next = nil
						return matcher, textlexer.StateContinue
					}

					if pushedBack {
						stdlog.Printf("pushed back, accepting")
						return PushBackCurrentAndAccept(r)
					}

					stdlog.Printf("processed all rules, accepting")
					return nil, textlexer.StateAccept
				}

				if state == textlexer.StateReject {
					return nil, textlexer.StateReject
				}

				panic("unexpected state from inner rule " + state.String())
			}

			return matcher(r)
		}
	*/
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

/*
// NewMatchWithLookahead creates a rule that matches if the main rule matches
// and is followed by a pattern that matches the lookahead rule. The lookahead
// pattern is not consumed.
//
// Cases:
// +-----------+-----------------+-------------------------+
// | mainRule  | lookaheadRule   | final                   |
// +-----------+-----------------+-------------------------+
// | Accept    | Accept          | Accept -> Reject        |
// | Accept    | Reject          | Accept -> Backtrack     |
// | Reject    | -               | Reject                  |
// | Continue  | -               | Continue                |
// | Backtrack | -               | Backtrack               |
// +-----------+-----------------+-------------------------+
func NewMatchWithLookahead(mainRule, lookaheadRule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	debug := func(msg string, args ...interface{}) {
		stdlog.Printf("##### NewMatchWithLookahead: "+msg, args...)
	}

	newBacktrackMatcher := func(next textlexer.Rule, offset int) func(r rune) (textlexer.Rule, textlexer.State) {
		var matcher textlexer.Rule

		debug("newBacktrackMatcher: offset: %d\n", offset)

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			if offset <= 0 {
				return nil, textlexer.StateAccept
			}

			offset--

			return matcher, textlexer.StateContinue
		}

		return matcher
	}

	newLookaheadMatcher := func(next textlexer.Rule, offset int) func(r rune) (textlexer.Rule, textlexer.State) {
		var state textlexer.State
		var matcher textlexer.Rule

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			debug("lookaheadMatcher: r: %q\n", r)

			if next == nil {
				panic("lookaheadMatcher: next is nil")
			}

			next, state = next(r)

			if state == textlexer.StateContinue {
				return matcher, textlexer.StateContinue
			}

			for next != nil && state == textlexer.StateAccept {
				next, state = next(r)
			}

			for next != nil && state == textlexer.StateReject {
				panic("lookaheadMatcher: reject panik")
				next, state = next(r)
			}

			if state == textlexer.StateAccept {
				debug("!!!!!! lookaheadMatcher: accepted")
				return newBacktrackMatcher(mainRule, offset), textlexer.StateBacktrack
			}

			if next == nil {
				debug("lookaheadMatcher: final state, next is nil\n")
				return nil, state
			}

			return matcher, state
		}

		return matcher
	}

	newMainRuleMatcher := func(next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		var offset int

		var state textlexer.State
		var matcher textlexer.Rule

		matcher = func(r rune) (textlexer.Rule, textlexer.State) {
			debug("mainRuleMatcher: r: %q\n", r)

			if next == nil {
				panic("mainRule: next is nil")
			}

			next, state = next(r)

			if state == textlexer.StateContinue {
				offset++

				return matcher, textlexer.StateContinue
			}

			if state == textlexer.StateBacktrack {
				offset = 0 // discard the offset
				return matcher, textlexer.StateBacktrack
			}

			for next != nil && state == textlexer.StateReject {
				next, state = next(r)
			}

			if state == textlexer.StateReject {
				return nil, textlexer.StateReject
			}

			for next != nil && state == textlexer.StateAccept {
				next, state = next(r)
			}

			if state == textlexer.StateAccept {
				debug("mainRuleMatcher: accepted, transitioning to lookaheadMatcher\n")
				return newLookaheadMatcher(lookaheadRule, offset)(r)
			}

			if next == nil {
				return nil, state
			}

			panic("mainRuleMatcher: unexpected state from inner rule " + state.String())
		}

		return matcher
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		return newMainRuleMatcher(mainRule)(r)
	}
}
*/
