package rules

import (
	"fmt"
	"log/slog"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/processor"
)

// Whitespace matches one or more whitespace characters.
// Example: ` `, ` \t`, `\n\r\n`, `\t\t `
func Whitespace(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return newCharacterClassMatcher(
		isCommonWhitespace,
		1,  // Must have at least one whitespace character.
		-1, // No upper limit on the number of whitespace characters.
	)(s)
}

// UnsignedInteger matches one or more digits.
// Example: `123`, `0`, `987654`
func UnsignedInteger(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return newCharacterClassMatcher(
		isASCIIDigit,
		1,  // Must have at least one digit.
		-1, // No upper limit on the number of digits.
	)(s)
}

// Word matches one or more Unicode letters.
// Example: `hello`, `GutenTag`, `세계`
func Word(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return newCharacterClassMatcher(
		isLetter,
		1,  // Must have at least one letter.
		-1, // No upper limit.
	)(s)
}

// ASCIIWord matches one or more ASCII-only letters (a-z, A-Z).
// Example: `hello`, `WORLD`
func ASCIIWord(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return newCharacterClassMatcher(
		isASCIILetter,
		1,  // Must have at least one ASCII letter.
		-1, // No upper limit.
	)(s)
}

// Identifier matches identifiers starting with a letter or underscore, followed
// by letters, digits, or underscores.
func Identifier() textlexer.Rule {
	return newStartPartMatcher(
		isIdentifierStart,
		isIdentifierPart,
	)
}

func UntilEOF() textlexer.Rule {
	var rule textlexer.Rule

	rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateAccept
		}

		return rule, textlexer.StateContinue
	}

	return rule
}

func Except(runes ...rune) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		for _, r := range runes {
			if s.Rune() == r {
				return nil, textlexer.StateReject
			}
		}

		return nil, textlexer.StateAccept
	}
}

func UntilEOL() textlexer.Rule {
	return newCharacterClassMatcher(
		func(r rune) bool {
			return r != '\n' && r != '\r'
		},
		0,
		-1,
	)
}

// Literal creates a rule that matches an exact string literal. This is ideal
// for matching keywords (e.g., "if", "for") or multi-character operators
// (e.g., "==", "=>").
//
// Panics if literal is an empty string.
func Literal(literal string) textlexer.Rule {
	if literal == "" {
		panic("literal string cannot be empty")
	}

	runes := []rune(literal)

	var ruleFor func(sub []rune) textlexer.Rule

	ruleFor = func(sub []rune) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() != sub[0] {
				return nil, textlexer.StateReject
			}

			if len(sub) == 1 {
				return nil, textlexer.StateAccept
			}

			return ruleFor(sub[1:]), textlexer.StateContinue
		}
	}

	return ruleFor(runes)
}

// Delimited creates a rule that matches a sequence of characters enclosed by
// start and end delimiters. It can also handle a specified escape character.
// This is ideal for matching quoted strings or block comments.
// If escapeRune is 0, no escaping is performed.
//
// Panics if startDelim or endDelim are empty strings.
func Delimited(startDelim, endDelim string, escapeRune rune) textlexer.Rule {
	if startDelim == "" || endDelim == "" {
		panic("start and end delimiters cannot be empty")
	}

	startRunes := []rune(startDelim)
	endRunes := []rune(endDelim)

	var matchStart, maybeMatchEnd func(sub []rune) textlexer.Rule
	var inDelimited textlexer.Rule

	maybeMatchEnd = func(sub []rune) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == sub[0] {
				if len(sub) == 1 {
					return nil, textlexer.StateAccept
				}

				return maybeMatchEnd(sub[1:]), textlexer.StateContinue
			}

			if s.IsEOF() {
				return nil, textlexer.StateReject
			}

			return inDelimited, textlexer.StateContinue
		}
	}

	inDelimited = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()

		if escapeRune != 0 && r == escapeRune {
			return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.IsEOF() {
					return nil, textlexer.StateReject
				}
				return inDelimited, textlexer.StateContinue
			}, textlexer.StateContinue
		}

		if r == endRunes[0] {
			return maybeMatchEnd(endRunes)(s)
		}

		return inDelimited, textlexer.StateContinue
	}

	matchStart = func(sub []rune) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == sub[0] {
				if len(sub) == 1 {
					return inDelimited, textlexer.StateContinue
				}

				return matchStart(sub[1:]), textlexer.StateContinue
			}

			return nil, textlexer.StateReject
		}
	}

	return matchStart(startRunes)
}

// SignedInteger matches an integer that may optionally be prefixed with a '+' or '-' sign.
// A sign character must be followed by at least one digit.
// Example: `-12`, `+42`, `100`, `0`
func SignedInteger() textlexer.Rule {
	var loopDigits, afterSign textlexer.Rule

	loopDigits = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(s.Rune()) {
			return loopDigits, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}

	afterSign = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(s.Rune()) {
			return loopDigits, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}

	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()

		if r == '+' || r == '-' {
			return afterSign, textlexer.StateContinue
		}

		if isASCIIDigit(r) {
			return loopDigits, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}
}

// UnsignedFloat matches an unsigned floating-point number.
// It requires a decimal point and at least one digit on either side.
// Example: `.1`, `0.0`, `0.`, `12.`, `12.12`
func UnsignedFloat() textlexer.Rule {
	var integerPart, afterInitialRadix, afterIntegerRadix, fractionalPart textlexer.Rule

	fractionalPart = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(s.Rune()) {
			return fractionalPart, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}

	afterIntegerRadix = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(s.Rune()) {
			return fractionalPart, textlexer.StateAccept
		}

		return PushBackCurrentAndAccept(s)
	}

	afterInitialRadix = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(s.Rune()) {
			return fractionalPart, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}

	integerPart = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(s.Rune()) {
			return integerPart, textlexer.StateContinue
		}

		if s.Rune() == '.' {
			return afterIntegerRadix, textlexer.StateAccept
		}

		return nil, textlexer.StateReject
	}

	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if isASCIIDigit(r) {
			return integerPart, textlexer.StateContinue
		}

		if r == '.' {
			return afterInitialRadix, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}
}

// SignedFloat matches a floating-point number that may optionally be prefixed
// with a '+' or '-' sign. It reuses the UnsignedFloat rule logic.
// Example: `-0.`, `+12.22`, `.5`
func SignedFloat() textlexer.Rule {
	var afterSign textlexer.Rule
	unsignedFloatRule := UnsignedFloat() // Get the entry point to the unsigned float machine.

	afterSign = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		nextRule, nextState := unsignedFloatRule(s)

		if nextState == textlexer.StateReject {
			return nil, textlexer.StateReject
		}

		return nextRule, textlexer.StateAccept
	}

	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if r == '+' || r == '-' {
			return afterSign, textlexer.StateContinue
		}

		return unsignedFloatRule(s)
	}
}

// Sequence creates a rule that matches a sequence of sub-rules in order.
//
// Panics if no rules are provided.
func Sequence(rules ...textlexer.Rule) textlexer.Rule {
	var buildMatcher func(index int) textlexer.Rule
	var ruleMatcher func(processor.StateProcessor, textlexer.Rule, int) textlexer.Rule

	if len(rules) == 0 {
		panic("Sequence requires at least one rule")
	}

	ruleMatcher = func(sp processor.StateProcessor, currentRule textlexer.Rule, index int) textlexer.Rule {
		isLastRule := index == len(rules)-1
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			nextRule, state := currentRule(s)
			err := sp.Execute(state)
			if err != nil {
				return nil, textlexer.StateReject
			}

			if state == textlexer.StateAccept {
				if nextRule == nil {
					if isLastRule {
						return nil, textlexer.StateAccept
					}
					return buildMatcher(index + 1), textlexer.StateContinue
				}
				if isLastRule && s.IsEOF() {
					return nil, textlexer.StateAccept
				}
				return ruleMatcher(sp, nextRule, index), textlexer.StateContinue
			}

			if state == textlexer.StateReject {
				accepted, offset := sp.Position()
				if accepted > 0 {
					// Rejected, but we had some matches before rejection.
					if isLastRule {
						return Backtrack(int(offset)+1, textlexer.StateAccept)(s)
					}
					return BacktrackAndContinue(
						int(offset)+1,
						buildMatcher(index+1),
					)(s)
				}
			}

			if nextRule == nil {
				return nil, state
			}

			return ruleMatcher(sp, nextRule, index), state
		}
	}

	buildMatcher = func(index int) textlexer.Rule {
		sp := processor.NewStateProcessor()
		initialRule := rules[index]
		return ruleMatcher(sp, initialRule, index)
	}

	return buildMatcher(0)
}

// Choice creates a rule that tries multiple sub-rules and matches if any one
// of them matches.
//
// Panics if no choices are provided.
func Choice(choices ...textlexer.Rule) textlexer.Rule {
	var buildChoice func(choices []textlexer.Rule) textlexer.Rule

	if len(choices) == 0 {
		panic("Choice requires at least one rule")
	}

	buildChoice = func(choiceRules []textlexer.Rule) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			activeChoices := make([]textlexer.Rule, 0, len(choiceRules))
			finalState := textlexer.StateReject

			for _, choiceRule := range choiceRules {
				nextRule, state := choiceRule(s)

				if state == textlexer.StateReject {
					continue // This choice is no longer viable.
				}

				switch state {
				case textlexer.StateAccept, textlexer.StateContinue:
					if nextRule != nil {
						activeChoices = append(activeChoices, nextRule)
					}
					if finalState != textlexer.StateAccept {
						finalState = state
					}
				default:
					panic("unhandled state in Choice") // TODO: implement PushBack handling
				}
			}

			if len(activeChoices) == 0 {
				// All choices have been eliminated.
				return nil, finalState
			}

			return buildChoice(activeChoices), finalState
		}
	}

	return buildChoice(choices)
}

// HexIntegerBody matches one or more hexadecimal digits (0-9, a-f, A-F).
// This is useful as a component for a full hexadecimal number rule.
// Example: `FFF`, `1A2B`, `deadbeef`
func HexIntegerBody() textlexer.Rule {
	return newCharacterClassMatcher(
		isHexDigit,
		1,
		-1,
	)
}

// Hexadecimal matches a C-style hexadecimal number, like `0xABC` or `0X123`.
// It uses a Choice combinator to handle both '0x' and '0X' prefixes.
func Hexadecimal() textlexer.Rule {
	return Sequence(
		Choice(
			Literal("0x"),
			Literal("0X"),
		),
		HexIntegerBody(),
	)
}

func pushbackAndContinue(n int, nextRule textlexer.Rule) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if n > 0 {
			return pushbackAndContinue(n-1, nextRule), textlexer.StatePushBack
		}

		return nextRule(s)
	}
}

// Lookahead creates a rule that matches the first rule only if it is
// followed by the second rule. The second rule's symbols are not consumed.
func Lookahead(
	rule textlexer.Rule,
	lookaheadRule textlexer.Rule,
) textlexer.Rule {

	var ruleMatcher func(processor.StateProcessor, textlexer.Rule) textlexer.Rule
	var lookaheadRuleMatcher func(processor.StateProcessor, textlexer.Rule) textlexer.Rule

	lookaheadRuleMatcher = func(sp processor.StateProcessor, currentLookaheadRule textlexer.Rule) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			nextRule, state := currentLookaheadRule(s)

			switch state {
			case textlexer.StateReject:
				return nil, textlexer.StateReject

			case textlexer.StateAccept:
				err := sp.Execute(textlexer.StateAccept)
				if err != nil {
					return nil, textlexer.StateReject
				}

				start, offset := sp.Position()
				backtrackAmount := int(start + offset)
				return Backtrack(
					backtrackAmount,
					textlexer.StateAccept,
				)(s)

			case textlexer.StateContinue:
				err := sp.Execute(textlexer.StateContinue)
				if err != nil {
					return nil, textlexer.StateReject
				}
				if nextRule == nil {
					return nil, textlexer.StateReject
				}
				return lookaheadRuleMatcher(sp, nextRule), textlexer.StateContinue

			case textlexer.StatePushBack:
				// A sub-rule (like a nested Lookahead's Backtrack) needs to push back.
				// We must execute this on our internal processor to keep the final
				// backtrack count correct, and then propagate the state and the
				// sub-rule's next state upwards.
				err := sp.Execute(textlexer.StatePushBack)
				if err != nil {
					return nil, textlexer.StateReject
				}
				// We continue our own state machine, but with the next rule provided by the sub-rule.
				return lookaheadRuleMatcher(sp, nextRule), textlexer.StatePushBack
			}
			// This panic should now be truly unreachable.
			panic("unreachable: unhandled state in lookaheadRuleMatcher")
		}
	}

	ruleMatcher = func(sp processor.StateProcessor, currentMainRule textlexer.Rule) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			nextRule, state := currentMainRule(s)

			if state == textlexer.StateReject {
				start, _ := sp.Position()
				if start > 0 {
					// "Completion Rejection": Main rule matched and is now done.
					// Start the lookahead check with the current symbol.
					return lookaheadRuleMatcher(processor.NewStateProcessor(), lookaheadRule)(s)
				}
				// "Failure Rejection": Main rule never matched.
				return nil, textlexer.StateReject
			}

			err := sp.Execute(state)
			if err != nil {
				return nil, textlexer.StateReject
			}

			if nextRule == nil {
				// Main rule fully matched (e.g., Literal). Transition to lookahead.
				return lookaheadRuleMatcher(processor.NewStateProcessor(), lookaheadRule), textlexer.StateContinue
			}

			// Main rule needs more symbols.
			start, offset := sp.Position()
			if s.IsEOF() {
				if start > 0 {
					return BacktrackAndContinue(
						int(offset)+1,
						lookaheadRuleMatcher(
							processor.NewStateProcessor(),
							lookaheadRule,
						),
					)(s)

				}
			}
			return ruleMatcher(sp, nextRule), textlexer.StateContinue
		}
	}

	return ruleMatcher(
		processor.NewStateProcessor(),
		rule,
	)
}

// Repeat creates a rule that matches the given rule between min and max times.
// Use max = -1 for unlimited repetitions.
//
// Examples:
//   - Repeat(rule, 0, 1) matches 0 or 1 times (same as Optional)
//   - Repeat(rule, 1, -1) matches 1 or more times (same as OneOrMore)
//   - Repeat(rule, 3, 5) matches between 3 and 5 times
func Repeat(rule textlexer.Rule, min, max int) textlexer.Rule {
	// Input validation
	if min < 0 {
		panic("Repeat: min cannot be negative")
	}
	if max != -1 && max < min {
		panic("Repeat: max cannot be less than min (use -1 for unlimited)")
	}
	if max == 0 {
		panic("Repeat: max cannot be 0 (use Optional or omit the rule)")
	}
	if min == 0 && max == 0 {
		panic("Repeat: both min and max cannot be 0")
	}

	type repeatState struct {
		count int
		sp    processor.StateProcessor

		currentRule textlexer.Rule
	}

	var buildRepeatMatcher func(*repeatState) textlexer.Rule

	buildRepeatMatcher = func(state *repeatState) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if state.currentRule == nil {
				state.currentRule = rule
			}

			slog.Info(
				"Repeat: processing symbol.",
				"symbol", string(s.Rune()),
				"count", state.count,
				"min", min,
				"max", max,
				"currentRule", state.currentRule != nil,
			)

			nextRule, matchState := state.currentRule(s)

			err := state.sp.Execute(matchState)
			if err != nil {
				panic(fmt.Sprintf("processor: %v", err))
			}

			slog.Info(
				"Repeat: sub-rule executed.",
				"state", matchState,
				"nextRule", nextRule != nil,
			)

			state.currentRule = nextRule

			if matchState == textlexer.StateAccept || matchState == textlexer.StateMatch {
				state.count++ // Increment count on successful match
			}

			hasMinimum := state.count >= min
			hasMaximum := max != -1 && state.count >= max

			isAccepted := hasMinimum && (max == -1 || hasMaximum)

			slog.Info(
				"Repeat: evaluating state.",
				"symbol", string(s.Rune()),
				"count", state.count,
				"matchState", matchState,
				"hasMinimum", hasMinimum,
				"hasMaximum", hasMaximum,
				"isAccepted", isAccepted,
				"nextRule", nextRule != nil,
				"sp", state.sp,
			)

			switch matchState {
			case textlexer.StateAccept:
				slog.Info("Repeat: ==> matched")
				if isAccepted {
					return nil, textlexer.StateAccept
				}
				return buildRepeatMatcher(state), textlexer.StateContinue
			case textlexer.StateContinue:
				slog.Info("Repeat: ==> continuing")
				return buildRepeatMatcher(state), textlexer.StateContinue
			case textlexer.StateReject:
				slog.Info("Repeat: ==> rejected")
				if hasMinimum {
					// We have enough matches, accept what we have
					slog.Info("Repeat: match zero")
					return nil, textlexer.StateMatch
				}
				panic("Repeat: not enough matches, rejecting")
			default:
				panic(fmt.Sprintf("Repeat: unhandled state %v", matchState))
			}

			panic(fmt.Sprintf("Repeat: should not reach here, state=%v", matchState))
		}
	}

	return buildRepeatMatcher(&repeatState{
		sp: processor.NewStateProcessor(),
	})
}

// Optional creates a rule that matches the given rule zero or one time.
// Equivalent to the regex ? quantifier.
//
// Example: Optional(Literal("-")) matches an optional minus sign.
func Optional(rule textlexer.Rule) textlexer.Rule {
	return Repeat(rule, 0, 1)
}

// ZeroOrMore creates a rule that matches the given rule zero or more times.
// Equivalent to the regex * quantifier.
//
// Example: ZeroOrMore(Whitespace) matches any amount of whitespace, including none.
func ZeroOrMore(rule textlexer.Rule) textlexer.Rule {
	return Repeat(rule, 0, -1)
}

// OneOrMore creates a rule that matches the given rule one or more times.
// Equivalent to the regex + quantifier.
//
// Example: OneOrMore(Digit) matches one or more digits.
func OneOrMore(rule textlexer.Rule) textlexer.Rule {
	return Repeat(rule, 1, -1)
}

// Exactly creates a rule that matches the given rule exactly n times.
//
// Example: Exactly(Digit, 4) matches exactly four digits (like a PIN code).
func Exactly(rule textlexer.Rule, n int) textlexer.Rule {
	if n < 0 {
		panic("Exactly: n cannot be negative")
	}
	return Repeat(rule, n, n)
}

// AtLeast creates a rule that matches the given rule at least n times.
//
// Example: AtLeast(Digit, 3) matches three or more digits.
func AtLeast(rule textlexer.Rule, n int) textlexer.Rule {
	if n < 0 {
		panic("AtLeast: n cannot be negative")
	}
	return Repeat(rule, n, -1)
}

// AtMost creates a rule that matches the given rule at most n times (including zero).
//
// Example: AtMost(Digit, 3) matches zero to three digits.
func AtMost(rule textlexer.Rule, n int) textlexer.Rule {
	if n < 0 {
		panic("AtMost: n cannot be negative")
	}
	return Repeat(rule, 0, n)
}
