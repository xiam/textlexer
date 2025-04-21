package rules

import (
	"unicode"

	"github.com/xiam/textlexer"
)

// NewAcceptSingleCharacter creates a rule that matches only the specified
// rune.
func NewAcceptSingleCharacter(match rune) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if r == match {
			// Use AcceptCurrentAndStop from the other file (implicitly available in the package)
			return AcceptCurrentAndStop, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}
}

// NewMatchUntilString creates a rule that consumes characters until a specific
// string is found, then transitions to the 'next' rule. The target string
// itself is matched.
func NewMatchUntilString(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule
		var matchRunes []rune
		var currentOffset int // Tracks how much of the target string we've matched *so far*

		if match == "" {
			// Cannot match an empty string this way
			return nil, textlexer.StateReject
		}
		matchRunes = []rune(match)

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			if textlexer.IsEOF(r) {
				// Reached EOF before finding the target string
				return nil, textlexer.StateReject
			}

			// Check if the current rune matches the expected rune in the target string
			if r == matchRunes[currentOffset] {
				currentOffset++
				// If we've matched the entire target string
				if currentOffset == len(matchRunes) {
					// Transition to the next rule provided
					return next, textlexer.StateContinue
				}
				// Continue matching the rest of the target string
				return nextChar, textlexer.StateContinue
			}

			// If the current rune doesn't match the expected target rune
			if currentOffset > 0 {
				// We were partially matching the target string, but failed.
				// We need to reset the match offset and re-evaluate the current rune
				// against the beginning of the target string.
				currentOffset = 0
				if r == matchRunes[0] {
					currentOffset = 1
					if currentOffset == len(matchRunes) { // Handle single-char target match on reset
						return next, textlexer.StateContinue
					}
					// Continue matching target from the start
					return nextChar, textlexer.StateContinue
				}
			}

			// Current rune is not part of the target string (or a failed partial match)
			// Continue consuming, looking for the start of the target string
			return nextChar, textlexer.StateContinue
		}

		// Start the process by checking the first rune
		return nextChar(r)
	}
}

// NewMatchStartingWithString creates a rule that requires the input to start
// exactly with the specified string, then transitions to the 'next' rule.
func NewMatchStartingWithString(match string, next textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule
		var offset int

		if len(match) == 0 {
			// Cannot start with an empty string
			return nil, textlexer.StateReject
		}

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			// Check if we have successfully matched the entire prefix string
			if offset >= len(match) {
				// Prefix matched, transition to the next rule with the current rune
				return next(r)
			}

			// Check if the current rune matches the expected character in the prefix
			if r == rune(match[offset]) {
				offset++
				// Continue matching the next character of the prefix
				return nextChar, textlexer.StateContinue
			}

			// Mismatch found in the prefix
			return nil, textlexer.StateReject
		}

		// Start matching the first character
		return nextChar(r)
	}
}

// NewMatchString creates a rule that matches an exact string.
func NewMatchString(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		return AcceptCurrentAndStop
	}

	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule
		var offset int

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			// Check if we have successfully matched the entire string
			if offset >= len(match) {
				// Entire string matched, accept. The rune that triggered this acceptance
				// belongs to the *next* token, so we don't consume it here.
				return nil, textlexer.StateAccept
			}

			// Check if the current rune matches the expected character in the string
			if r == rune(match[offset]) {
				offset++
				// Continue matching the next character
				return nextChar, textlexer.StateContinue
			}

			// Mismatch found
			return nil, textlexer.StateReject
		}

		// Start matching the first character
		return nextChar(r)
	}
}

// NewMatchStringIgnoreCase creates a rule that matches an exact string,
// ignoring case.
func NewMatchStringIgnoreCase(match string) func(r rune) (textlexer.Rule, textlexer.State) {
	if match == "" {
		// See note in NewMatchString regarding empty matches.
		return AcceptCurrentAndStop
	}

	matchRunes := []rune(match) // Work with runes for proper Unicode case folding

	return func(r rune) (textlexer.Rule, textlexer.State) {
		var nextChar textlexer.Rule
		offset := 0

		nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
			// Check if we have successfully matched the entire string
			if offset >= len(matchRunes) {
				// Entire string matched, accept.
				return nil, textlexer.StateAccept
			}

			// Compare runes case-insensitively
			if unicode.ToLower(r) == unicode.ToLower(matchRunes[offset]) {
				offset++
				// Continue matching the next character
				return nextChar, textlexer.StateContinue
			}

			// Mismatch found
			return nil, textlexer.StateReject
		}

		// Start matching the first character
		return nextChar(r)
	}
}

// NewMatchInvertedRule creates a rule that matches sequences *not* matched by
// the provided rule.  It consumes characters as long as the original rule
// would reject or continue internally, and accepts when the original rule
// would finally accept or reject definitively after continuing.
func NewMatchInvertedRule(rule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	// This function's logic is complex and relies heavily on the state transitions
	// of the inner rule. Documenting its exact behavior concisely is difficult.
	// The core idea is to consume input while the inner 'rule' is *not* finding a complete match.

	var contRejected func(textlexer.Rule) func(rune) (textlexer.Rule, textlexer.State)
	var contContinued func(textlexer.Rule) func(rune) (textlexer.Rule, textlexer.State)

	// State for when the inner rule has just rejected a rune.
	// We continue consuming as long as it keeps rejecting.
	contRejected = func(currentRule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			next, state := currentRule(r) // Test the rune with the inner rule

			if state == textlexer.StateReject {
				// Inner rule still rejects. If EOF, we accept the inverted match.
				if textlexer.IsEOF(r) {
					return nil, textlexer.StateAccept
				}
				// Continue consuming with the appropriate next state of the inner rule (or the original if nil).
				nextRule := next
				if nextRule == nil {
					nextRule = currentRule // Reset if inner rule didn't provide a new one on reject
				}
				return contRejected(nextRule), textlexer.StateContinue
			}

			// If the inner rule accepts or continues, our inverted match ends here.
			// We accept the sequence matched *before* this rune.
			if state == textlexer.StateContinue || state == textlexer.StateAccept {
				return nil, textlexer.StateAccept
			}

			panic("unexpected state from inner rule") // Should not happen
		}
	}

	// State for when the inner rule is in a 'continue' state.
	// We continue consuming, following the inner rule's transitions.
	// If the inner rule eventually rejects, we might continue consuming (via contRejected).
	// If the inner rule eventually accepts, our inverted match ends.
	contContinued = func(currentRule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			next, state := currentRule(r) // Test the rune with the inner rule

			if state == textlexer.StateReject {
				// Inner rule rejected after continuing. This means the potential match failed.
				// For the inverted rule, this might mean we continue consuming.
				// We accept the sequence matched *before* this rune.
				return nil, textlexer.StateAccept
			}

			if state == textlexer.StateContinue {
				// Inner rule continues. We continue the inverted match.
				nextRule := next
				if nextRule == nil {
					nextRule = currentRule // Keep same rule if inner rule didn't provide a new one
				}
				return contContinued(nextRule), textlexer.StateContinue
			}

			if state == textlexer.StateAccept {
				// Inner rule accepted. This means our inverted match must stop *before* this acceptance.
				// We reject the inverted match attempt because the inner rule succeeded.
				return nil, textlexer.StateReject
			}

			panic("unexpected state from inner rule") // Should not happen
		}
	}

	// Initial state: test the first rune with the inner rule.
	return func(r rune) (textlexer.Rule, textlexer.State) {
		next, state := rule(r)

		if state == textlexer.StateReject {
			// Inner rule rejected the first rune. Start consuming in the 'rejected' state.
			nextRule := next
			if nextRule == nil {
				nextRule = rule
			}
			return contRejected(nextRule), textlexer.StateContinue
		}

		if state == textlexer.StateContinue {
			// Inner rule continued on the first rune. Start consuming in the 'continued' state.
			nextRule := next
			if nextRule == nil {
				nextRule = rule
			}
			return contContinued(nextRule), textlexer.StateContinue
		}

		if state == textlexer.StateAccept {
			// Inner rule accepted the very first rune. Inverted match fails immediately.
			return nil, textlexer.StateReject
		}

		panic("unexpected state from inner rule") // Should not happen
	}
}

// NewMatchRuleSequence creates a rule that matches a sequence of other rules
// in order.
func NewMatchRuleSequence(rules ...func(r rune) (textlexer.Rule, textlexer.State)) func(r rune) (textlexer.Rule, textlexer.State) {
	var match func(int, textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State)

	// match processes the rule at index 'offset', potentially using an intermediate 'currentSubRule'.
	match = func(offset int, currentSubRule textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {

			// Check if we have successfully matched all rules in the sequence
			if offset >= len(rules) {
				// Sequence complete, accept. The current rune 'r' belongs to the next token.
				return nil, textlexer.StateAccept
			}

			// Determine the rule to apply for the current rune
			ruleToApply := currentSubRule
			if ruleToApply == nil {
				// If no sub-rule is active, start the next rule in the main sequence
				ruleToApply = rules[offset]
			}

			// Apply the determined rule to the current rune
			nextSubRule, state := ruleToApply(r)

			switch state {
			case textlexer.StateReject:
				// If any rule in the sequence rejects, the whole sequence fails
				return nil, textlexer.StateReject

			case textlexer.StateAccept:
				// The current rule accepted. Move to the *next* rule in the sequence.
				// We need to re-evaluate the *same rune* 'r' with the next rule.
				// This handles cases where rules match zero-width patterns or where
				// the accepting rune is the start of the next required pattern.
				return match(offset+1, nil)(r) // Pass nil to start the next rule from scratch

			case textlexer.StateContinue:
				// The current rule is continuing. Stay on the same rule in the sequence (offset doesn't change),
				// but update the sub-rule state for the next rune.
				return match(offset, nextSubRule), textlexer.StateContinue

			default:
				panic("unexpected state from sequence rule") // Should not happen
			}
		}
	}

	// Start matching with the first rule (index 0) and no active sub-rule
	return match(0, nil)
}

// NewMatchAnyRule creates a rule that matches if *any* of the provided rules
// match. The first rule that accepts will cause the overall match to accept.
func NewMatchAnyRule(rules ...textlexer.Rule) func(r rune) (textlexer.Rule, textlexer.State) {
	var matchAnyOf func([]textlexer.Rule) textlexer.Rule

	matchAnyOf = func(currentRules []textlexer.Rule) textlexer.Rule {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			var next textlexer.Rule
			var state textlexer.State

			// Store rules that are still potential matches after processing rune 'r'
			nextPotentialRules := []textlexer.Rule{}

			for _, rule := range currentRules {
				next, state = rule(r)

				if state == textlexer.StateAccept {
					// If any rule accepts, the overall match accepts immediately.
					return nil, textlexer.StateAccept
				}

				if state == textlexer.StateContinue {
					// This rule is still a potential match. Add its next state (or itself)
					// to the list for the next rune.
					if next != nil {
						nextPotentialRules = append(nextPotentialRules, next)
					} else {
						// If rule returned nil next state, it means continue with the same rule logic.
						nextPotentialRules = append(nextPotentialRules, rule)
					}
				}
			}

			// If there are still rules that could potentially match on subsequent runes...
			if len(nextPotentialRules) > 0 {
				// ...continue matching with the reduced set of potential rules.
				return matchAnyOf(nextPotentialRules), textlexer.StateContinue
			}

			// No rules accepted, and no rules are continuing. The overall match fails.
			return nil, textlexer.StateReject
		}
	}

	// Start the process with the initial list of rules.
	return matchAnyOf(rules)
}
