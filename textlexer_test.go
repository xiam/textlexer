// LLM Programming Directive:
//
// Objective:
// This file provides a comprehensive, self-contained, and robust test suite for the `textlexer` package.
// Its primary purpose is to validate the correctness and resilience of the lexer's state machine.
//
// Core Abstractions for This File:
// - `Rule` (textlexer.Rule): A function `func(s textlexer.Symbol) (Rule, State)` representing a state in a state machine.
// - `Symbol` (textlexer.Symbol): A rune enriched with contextual flags (e.g., IsBOL, IsEOF).
// - `State` (textlexer.State): An enum (`StateAccept`, `StateContinue`, `StateReject`, `StatePushBack`)
//   that signals the lexer's next action.
// - `LexemeType`: A string identifier for token types (e.g., "INT", "FLOAT", "KEYWORD").
// - `Lexeme`: A token with Type, Text, Offset, and Length.
//
// Key Test Philosophies:
// 1.  Self-Contained: All lexical rules are implemented locally as test helpers (e.g., `matchString`,
//     `newUnsignedFloatRule`) to eliminate external dependencies and keep tests readable and focused.
// 2.  Adversarial Focus: A significant portion of the tests are designed to be adversarial, probing
//     pathological behaviors and edge cases (e.g., zero-length matches, infinite loops, deep
//     backtracking, rule panics) to ensure the core processing loop is exceptionally robust.
// 3.  Deterministic Verification: Most tests use exact lexeme sequence matching to catch regressions.
//
// Test Categories Covered:
// - Basic Functionality: Longest-match semantics, rule precedence, handling of unknown tokens.
// - Context-Aware Rules: Verification of rules using flags like `IsBOL` (beginning-of-line).
// - Resource Management: Dynamic buffer growth and memory compaction.
// - Concurrency: Goroutine safety of `Next()` and `AddRule()` (verified with the -race detector).
// - Error Handling: Propagation of reader errors and recovery from panics in user-provided rules.
// - Pathological Rules: A dedicated suite (`TestLexerPathologicalRules`) for complex, malicious,
//   or badly-behaved rules designed to stress the state machine's logic.
//
// Common Patterns and Helpers:
// - `matchString(s)`: Creates a rule that matches an exact string.
// - `matchAnyOf(ss...)`: Creates a composite rule that matches any of the provided strings.
// - `excludeLast(n, state)`: The verdict that excludes the last n symbols with n pushbacks,
//   then ends the chain with the given state.
// - Table-driven tests in `TestLexerProcessor` for basic functionality.
// - Adversarial sub-tests in `TestLexerPathologicalRules`.
//
// Known Constraints:
// - Each `LexemeType` can only have one rule associated with it.
// - A rule that is still active at EOF is retired; only its recorded accepts count.
// - The lexer uses longest-match semantics with rule priority as a tiebreaker.
//
// Guideline for Adding New Tests:
// 1. For basic functionality: Add a case to the `testCases` slice in `TestLexerProcessor`.
// 2. For adversarial testing: Add a `t.Run()` sub-test to `TestLexerPathologicalRules`.
// 3. For new rule types: Create a helper function following the `new*Rule()` pattern.
// 4. Always verify both positive cases (expected tokens) and negative cases (error conditions).
// 5. Include comments explaining the specific edge case or behavior being tested.

package textlexer_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
)

// excludeLast returns the verdict of a rule that excludes the last n symbols
// (n >= 1) from its match and then ends the pushback chain with (nil, s). It
// issues n StatePushBack verdicts in total: the returned one, and n-1 more from
// the Rule it returns. The first pushback removes the symbol that was just read
// (at end of input, the EOF symbol). See "Rule states and pushback" in
// docs/specs/core-contract.md.
func excludeLast(n int, s textlexer.State) (textlexer.Rule, textlexer.State) {
	if n < 1 {
		panic("excludeLast: n must be at least 1")
	}
	return pushBackChain(n-1, s), textlexer.StatePushBack
}

// pushBackChain returns a Rule that issues n more StatePushBack verdicts and
// then returns (nil, s). Entered through one StatePushBack, it excludes n+1
// symbols; excludeLast states the total count directly.
func pushBackChain(n int, s textlexer.State) textlexer.Rule {
	return func(sym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if n > 0 {
			return pushBackChain(n-1, s), textlexer.StatePushBack
		}
		return nil, s
	}
}

// boundedRule wraps r so that the test fails once r, and every Rule reached
// from it, has been invoked more than budget times in total. It bounds a test
// by work instead of by wall-clock time: a lexer that stops terminating keeps
// invoking its rules, and the invocation that exceeds the budget fails the test.
//
// The wrapped rule runs inside Next, so the test must call Next on the test
// goroutine; t.Fatalf is then legal there. callRule recovers panics but not
// runtime.Goexit, so the failure stops the test instead of retiring the rule.
// A defect that loops without invoking any rule is not caught here; go test
// -timeout remains the backstop for it.
func boundedRule(t testing.TB, budget int, r textlexer.Rule) textlexer.Rule {
	calls := 0
	var wrap func(textlexer.Rule) textlexer.Rule
	wrap = func(r textlexer.Rule) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			calls++
			if calls > budget {
				t.Fatalf("rule invoked more than %d times: the lexer does not terminate", budget)
			}
			next, state := r(s)
			if next != nil {
				next = wrap(next)
			}
			return next, state
		}
	}
	return wrap(r)
}

// matchString is a test helper that creates a simple Rule to match an exact string.
func matchString(s string) textlexer.Rule {
	var ruleFor func(sub string) textlexer.Rule
	ruleFor = func(sub string) textlexer.Rule {
		return func(sym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			runes := []rune(sub)
			if sym.Rune() != runes[0] {
				return nil, textlexer.StateReject
			}
			if len(runes) == 1 {
				return nil, textlexer.StateAccept
			}
			return ruleFor(string(runes[1:])), textlexer.StateContinue
		}
	}
	if s == "" {
		return func(sym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateAccept
		}
	}
	return ruleFor(s)
}

// matchAnyOf creates a single composite rule that matches any of the provided strings.
func matchAnyOf(ss ...string) textlexer.Rule {
	type scanner struct {
		rule textlexer.Rule
	}

	return func(sym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		var activeScanners []*scanner
		var overallState textlexer.State = textlexer.StateReject

		for _, s := range ss {
			if len(s) > 0 && rune(s[0]) == sym.Rune() {
				nextRule := matchString(s[1:])
				activeScanners = append(activeScanners, &scanner{rule: nextRule})

				if len(s) == 1 {
					overallState = textlexer.StateAccept
				} else {
					if overallState != textlexer.StateAccept {
						overallState = textlexer.StateContinue
					}
				}
			}
		}

		if len(activeScanners) == 0 {
			return nil, textlexer.StateReject
		}

		var nextCompositeRule textlexer.Rule
		nextCompositeRule = func(nextSym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			var nextActiveScanners []*scanner
			var nextOverallState textlexer.State = textlexer.StateReject

			for _, sc := range activeScanners {
				// Defensive check: A completed sub-match might have a nil rule.
				if sc.rule == nil {
					continue
				}

				nextRule, state := sc.rule(nextSym)
				if state != textlexer.StateReject {
					// *** FIX: Only add a scanner to the next state if its rule is not nil. ***
					// This prevents a nil dereference on the next symbol.
					if nextRule != nil {
						nextActiveScanners = append(nextActiveScanners, &scanner{rule: nextRule})
					}

					if state == textlexer.StateAccept {
						nextOverallState = textlexer.StateAccept
					} else if nextOverallState != textlexer.StateAccept {
						nextOverallState = textlexer.StateContinue
					}
				}
			}

			if len(nextActiveScanners) == 0 {
				// If no scanners can continue, the composite rule is done.
				// Return nil for the next rule. The overall state (Accept/Reject)
				// will be determined by what was last seen.
				return nil, nextOverallState
			}
			activeScanners = nextActiveScanners
			return nextCompositeRule, nextOverallState
		}

		return nextCompositeRule, overallState
	}
}

func isWhitespace(s textlexer.Symbol) bool {
	r := s.Rune()
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isDigit(s textlexer.Symbol) bool {
	r := s.Rune()
	return r >= '0' && r <= '9'
}

func isLetter(s textlexer.Symbol) bool {
	r := s.Rune()
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func newWhitespaceRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isWhitespace(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isWhitespace(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newUnsignedIntegerRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newIdentifierRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if isLetter(s) || isDigit(s) || r == '_' {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if isLetter(s) || r == '_' {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

// newUnsignedFloatRule matches "1.5", "1." and ".5". It decides by lookahead: it
// returns StateContinue until it reads the first symbol that cannot extend the
// float, then excludes that one symbol (the EOF symbol at end of input) and
// accepts. It never accepts before that, because a rule's recorded match only
// grows within a token, so a pushback could not shorten an earlier accept.
func newUnsignedFloatRule() textlexer.Rule {
	var start, integerPart, afterInitialRadix, afterIntegerRadix, fractionalPart textlexer.Rule

	// Exclude the symbol just read, which ends the float, and accept the rest.
	pushBackAndAccept := func(_ textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return excludeLast(1, textlexer.StateAccept)
	}

	// State for matching digits after a decimal point.
	fractionalPart = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return fractionalPart, textlexer.StateContinue
		}
		return pushBackAndAccept(s)
	}

	// State after a radix point that followed an integer part (e.g., after "123.").
	afterIntegerRadix = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return fractionalPart, textlexer.StateContinue
		}
		if s.Rune() == '.' {
			// A second radix point: "1.." starts a range, so "1." is not a float.
			return nil, textlexer.StateReject
		}
		return pushBackAndAccept(s)
	}

	// State after a radix point was the *first* character. A fractional part is required.
	afterInitialRadix = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return fractionalPart, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	// State for matching the initial integer part. This is NOT a float yet, and
	// neither is the integer part followed by '.': afterIntegerRadix decides.
	integerPart = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return integerPart, textlexer.StateContinue
		}
		if s.Rune() == '.' {
			return afterIntegerRadix, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return integerPart, textlexer.StateContinue
		}
		if s.Rune() == '.' {
			return afterInitialRadix, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	return start
}

func newSignedIntegerRule() textlexer.Rule {
	var start, loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isDigit(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if r == '+' || r == '-' {
			// A sign must be followed by at least one digit.
			return loop, textlexer.StateContinue
		}
		if isDigit(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return start
}

func newSymbolRule() textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		switch s.Rune() {
		case '+', '-', '*', '/', '=', ';', '(', ')':
			return nil, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newSingleQuotedStringRule() textlexer.Rule {
	var loop, afterQuote textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == '\'' {
			return afterQuote, textlexer.StateAccept
		}
		// Note: This simple version doesn't handle escaped quotes.
		return loop, textlexer.StateContinue
	}
	afterQuote = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return nil, textlexer.StateReject
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == '\'' {
			return loop, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

func newSlashStarCommentRule() textlexer.Rule {
	var inComment, afterStar textlexer.Rule
	inComment = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == '*' {
			return afterStar, textlexer.StateContinue
		}
		return inComment, textlexer.StateContinue
	}
	afterStar = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == '/' {
			return nil, textlexer.StateAccept
		}
		return inComment, textlexer.StateContinue
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == '/' {
			return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == '*' {
					return inComment, textlexer.StateContinue
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

// A mock reader that fails N times before succeeding.
type recoverableReader struct {
	reader    io.RuneReader
	failCount int
	maxFails  int
}

func (rr *recoverableReader) ReadRune() (rune, int, error) {
	if rr.failCount < rr.maxFails {
		rr.failCount++
		return 0, 0, errors.New("temporary reader error")
	}
	return rr.reader.ReadRune()
}

var (
	whitespaceRule      = newWhitespaceRule()
	unsignedIntegerRule = newUnsignedIntegerRule()
	unsignedFloatRule   = newUnsignedFloatRule()
	identifierRule      = newIdentifierRule()
)

func TestLexerProcessor(t *testing.T) {
	const (
		lexTypeInteger    = textlexer.LexemeType("INT")
		lexTypeFloat      = textlexer.LexemeType("FLOAT")
		lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
		lexTypeIdentifier = textlexer.LexemeType("IDENTIFIER")
		lexTypeKeywordIF  = textlexer.LexemeType("IF")
		lexTypeLoop       = textlexer.LexemeType("LOOP")
		lexTypeComment    = textlexer.LexemeType("COMMENT")
		lexTypeHash       = textlexer.LexemeType("HASH")
	)

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
		expectedError   string
	}{
		{
			name:  "Numeric and Whitespace",
			input: "  12.3  \t   4 5.6 \t 7\n 8 9.0",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeWhitespace, whitespaceRule)
				lx.MustAddRule(lexTypeFloat, unsignedFloatRule)
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  ", 0),
				textlexer.NewLexemeFromString(lexTypeFloat, "12.3", 2),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  \t   ", 6),
				textlexer.NewLexemeFromString(lexTypeInteger, "4", 12),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 13),
				textlexer.NewLexemeFromString(lexTypeFloat, "5.6", 14),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " \t ", 17),
				textlexer.NewLexemeFromString(lexTypeInteger, "7", 20),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "\n ", 21),
				textlexer.NewLexemeFromString(lexTypeInteger, "8", 23),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 24),
				textlexer.NewLexemeFromString(lexTypeFloat, "9.0", 25),
			},
		},
		{
			name:  "Context-Aware Rules (BOL)",
			input: "  # not a comment\n# a comment",
			setupRules: func(lx *textlexer.TextLexer) {
				commentRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					// This comment rule only matches '#' if it's at the beginning of a line.
					if !s.IsBOL() || s.Rune() != '#' {
						return nil, textlexer.StateReject
					}

					// This rule is returned after we see a terminator. It ignores its input
					// and just signals acceptance. This prevents a recursive pushback.
					acceptorRule := func(_ textlexer.Symbol) (textlexer.Rule, textlexer.State) {
						return nil, textlexer.StateAccept
					}

					var loop textlexer.Rule
					loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
						// Consume until EOL or EOF, then pushback to not include the terminator.
						if s.IsEOL() || s.IsEOF() {
							return acceptorRule, textlexer.StatePushBack
						}
						return loop, textlexer.StateContinue
					}
					// Accept the '#' itself.
					return loop, textlexer.StateAccept
				}
				lx.MustAddRule(lexTypeComment, commentRule)
				lx.MustAddRule(lexTypeWhitespace, whitespaceRule)
				lx.MustAddRule(lexTypeHash, matchString("#")) // Fallback for '#' not at BOL
				lx.MustAddRule(lexTypeIdentifier, newIdentifierRule())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  ", 0),
				textlexer.NewLexemeFromString(lexTypeHash, "#", 2),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 3),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "not", 4),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 7),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "a", 8),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 9),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "comment", 10),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "\n", 17),
				textlexer.NewLexemeFromString(lexTypeComment, "# a comment", 18),
			},
		},
		{
			name:  "Empty Input",
			input: "",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeWhitespace, whitespaceRule)
			},
			expectedLexemes: []*textlexer.Lexeme{},
		},
		{
			name:  "No Matching Rules (produces UNKNOWN)",
			input: "abc",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "c", 2),
			},
		},
		{
			name:  "Longest Match (Float vs Integer)",
			input: "12.345",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
				lx.MustAddRule(lexTypeFloat, unsignedFloatRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "12.345", 0),
			},
		},
		{
			name:  "Float with trailing decimal at EOF",
			input: "42.",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, unsignedFloatRule)
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "42.", 0),
			},
		},
		{
			name:  "Fallback from potential Float to Integer",
			input: "42a",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, unsignedFloatRule)
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "42", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 2),
			},
		},
		{
			name:  "Tokens immediately at EOF",
			input: "123",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "123", 0),
			},
		},
		{
			name:  "Rule Priority (Keyword vs Identifier)",
			input: "if identifier",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeywordIF, matchString("if"))
				lx.MustAddRule(lexTypeIdentifier, identifierRule)
				lx.MustAddRule(lexTypeWhitespace, whitespaceRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeKeywordIF, "if", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 2),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "identifier", 3),
			},
		},
		{
			name:  "Rule Priority Reversed (Identifier wins)",
			input: "if",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, identifierRule)
				lx.MustAddRule(lexTypeKeywordIF, matchString("if"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "if", 0),
			},
		},
		{
			name:  "Unicode Characters",
			input: "123\n世界456",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "123", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "\n", 3),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "世", 4),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "界", 5),
				textlexer.NewLexemeFromString(lexTypeInteger, "456", 6),
			},
		},
		{
			name:  "Adjacent Floats (No Whitespace)",
			input: "1.2.3",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, unsignedFloatRule)
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "1.2", 0),
				textlexer.NewLexemeFromString(lexTypeFloat, ".3", 3),
			},
		},
		{
			name:  "Signed Numerics and Math Operators",
			input: "-12+-3++4-5",
			setupRules: func(lx *textlexer.TextLexer) {
				// Rule order matters: SignedInteger must come before a general Symbol rule.
				lx.MustAddRule(textlexer.LexemeType("SIGNED_INT"), newSignedIntegerRule())
				lx.MustAddRule(textlexer.LexemeType("SYMBOL"), newSymbolRule())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeType("SIGNED_INT"), "-12", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SYMBOL"), "+", 3),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SIGNED_INT"), "-3", 4),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SYMBOL"), "+", 6),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SIGNED_INT"), "+4", 7),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SIGNED_INT"), "-5", 9),
			},
		},
		{
			name:  "SQL-like Statement with Comments and Strings",
			input: "SELECT * FROM users WHERE name = 'John Doe' /* a comment */;",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(textlexer.LexemeType("COMMENT"), newSlashStarCommentRule())
				lx.MustAddRule(textlexer.LexemeType("STRING"), newSingleQuotedStringRule())
				lx.MustAddRule(textlexer.LexemeType("SELECT"), matchString("SELECT"))
				lx.MustAddRule(textlexer.LexemeType("FROM"), matchString("FROM"))
				lx.MustAddRule(textlexer.LexemeType("WHERE"), matchString("WHERE"))
				lx.MustAddRule(textlexer.LexemeType("IDENTIFIER"), newIdentifierRule())
				lx.MustAddRule(textlexer.LexemeType("SYMBOL"), newSymbolRule()) // Catches *, =, ;
				lx.MustAddRule(textlexer.LexemeType("WHITESPACE"), newWhitespaceRule())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeType("SELECT"), "SELECT", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 6),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SYMBOL"), "*", 7),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 8),
				textlexer.NewLexemeFromString(textlexer.LexemeType("FROM"), "FROM", 9),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 13),
				textlexer.NewLexemeFromString(textlexer.LexemeType("IDENTIFIER"), "users", 14),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 19),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHERE"), "WHERE", 20),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 25),
				textlexer.NewLexemeFromString(textlexer.LexemeType("IDENTIFIER"), "name", 26),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 30),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SYMBOL"), "=", 31),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 32),
				textlexer.NewLexemeFromString(textlexer.LexemeType("STRING"), "'John Doe'", 33),
				textlexer.NewLexemeFromString(textlexer.LexemeType("WHITESPACE"), " ", 43),
				textlexer.NewLexemeFromString(textlexer.LexemeType("COMMENT"), "/* a comment */", 44),
				textlexer.NewLexemeFromString(textlexer.LexemeType("SYMBOL"), ";", 59),
			},
		},
		{
			name:          "No Rules Configured",
			input:         "abc",
			setupRules:    func(lx *textlexer.TextLexer) {},
			expectedError: "rules",
		},
		{
			name:  "Rule Never Settles (UNKNOWN per rune)",
			input: "aaaa",
			setupRules: func(lx *textlexer.TextLexer) {
				// This rule always continues and never accepts, so it is still
				// active at end of input. The VM retires it after the EOF
				// symbol; with no recorded accept, each token is one UNKNOWN
				// rune. Each token reads to the end of input first.
				var brokenRule textlexer.Rule
				brokenRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return brokenRule, textlexer.StateContinue
				}
				lx.MustAddRule(lexTypeLoop, brokenRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 1),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 2),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 3),
			},
		},
		{
			name:  "Excessive Pushback (Index Out of Bounds)",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				// On 'a' the stack holds one symbol, and the rule tries to exclude
				// three. The second pushback passes the bottom of the stack, so the
				// VM retires the rule (processor.go:110) and no rule matches.
				pushbackRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return excludeLast(3, textlexer.StateAccept)
				}
				lx.MustAddRule(lexTypeLoop, pushbackRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
			},
		},
		{
			name:  "Very large input string with no matches",
			input: strings.Repeat("€", 5000),
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			// This tests for performance issues or excessive memory allocation.
			expectedLexemes: func() []*textlexer.Lexeme {
				lexemes := make([]*textlexer.Lexeme, 5000)
				for i := 0; i < 5000; i++ {
					lexemes[i] = textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "€", i)
				}
				return lexemes
			}(),
		},
		{
			name:  "Zero-Length Match Causes Infinite Loop",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				// A rule that ACCEPTS a match of length 0 must be handled by the lexer
				// to prevent an infinite loop. It should advance by 1 rune as an UNKNOWN token.
				// One pushback on the first symbol excludes it, so the accept has
				// length 0. TestProcessZeroLengthMatch proves that the VM takes the
				// zero-length branch (textlexer.go:482-485), not the no-match path.
				zeroLengthRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return excludeLast(1, textlexer.StateAccept)
				}
				lx.MustAddRule(lexTypeLoop, zeroLengthRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
			},
		},
		{
			name:  "Pushback Infinite Loop",
			input: "abc",
			setupRules: func(lx *textlexer.TextLexer) {
				// This rule creates a loop by always pushing back and returning to itself.
				var pushbackLoopRule textlexer.Rule
				pushbackLoopRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return pushbackLoopRule, textlexer.StatePushBack
				}
				lx.MustAddRule(lexTypeLoop, pushbackLoopRule)
			},
			// The processor's pushback limit should prevent an infinite loop.
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "c", 2),
			},
		},
		{
			name:  "Rule returns nil, StateContinue",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				// The processor must handle an invalid (nil, StateContinue) return
				// by deactivating the scanner to prevent undefined behavior.
				badRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return nil, textlexer.StateContinue
				}
				lx.MustAddRule(lexTypeLoop, badRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
			},
		},
		{
			name:  "Rule in StateContinue at EOF",
			input: "ab",
			setupRules: func(lx *textlexer.TextLexer) {
				// Rule that continues without accepting, then hits EOF.
				ruleAtEOF := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					if s.Rune() == 'a' {
						return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
							if s.Rune() == 'b' {
								// Continue without accepting - will hit EOF.
								return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
									return nil, textlexer.StateReject
								}, textlexer.StateContinue
							}
							return nil, textlexer.StateReject
						}, textlexer.StateContinue
					}
					return nil, textlexer.StateReject
				}
				lx.MustAddRule("EOF_CONTINUE", ruleAtEOF)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
			},
		},
		{
			name:  "Pushback at EOF",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				// The rule continues at 'a' without accepting, so its only accept
				// comes from the pushback: at end of input the first pushback
				// removes the EOF symbol, which leaves "a" as the match.
				pushbackAtEOF := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					if s.Rune() == 'a' {
						return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
							return excludeLast(1, textlexer.StateAccept)
						}, textlexer.StateContinue
					}
					return nil, textlexer.StateReject
				}
				lx.MustAddRule("PUSHBACK_EOF", pushbackAtEOF)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeType("PUSHBACK_EOF"), "a", 0),
			},
		},
		{
			name:  "BOL and EOL Flags with Multiple Newlines",
			input: "a\n\nb",
			setupRules: func(lx *textlexer.TextLexer) {
				// This test requires capturing the flags of each symbol as it is
				// processed. The rule itself will perform the assertions.
				var processedSymbols []textlexer.Symbol
				var mu sync.Mutex

				captureRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					if s.IsEOF() {
						return nil, textlexer.StateReject
					}
					mu.Lock()
					processedSymbols = append(processedSymbols, s)
					mu.Unlock()
					return nil, textlexer.StateAccept // Accept one symbol at a time.
				}
				lx.MustAddRule("CAPTURE", captureRule)

				// This anonymous function will be called after the lexer runs
				// to perform the final assertions on the captured symbols.
				t.Cleanup(func() {
					require.Len(t, processedSymbols, 4, "Should have processed 4 symbols")

					// Symbol 'a' at offset 0
					symA := processedSymbols[0]
					assert.Equal(t, 'a', symA.Rune())
					assert.True(t, symA.IsBOF(), "'a' should be at beginning of file")
					assert.True(t, symA.IsBOL(), "'a' should be at beginning of line")
					assert.False(t, symA.IsEOL(), "'a' should not be at end of line")

					// First '\n' at offset 1
					symNL1 := processedSymbols[1]
					assert.Equal(t, '\n', symNL1.Rune())
					assert.False(t, symNL1.IsBOF(), "first '\\n' is not BOF")
					assert.False(t, symNL1.IsBOL(), "first '\\n' is not BOL")
					assert.True(t, symNL1.IsEOL(), "first '\\n' should be EOL")

					// Second '\n' at offset 2
					symNL2 := processedSymbols[2]
					assert.Equal(t, '\n', symNL2.Rune())
					assert.False(t, symNL2.IsBOF(), "second '\\n' is not BOF")
					assert.True(t, symNL2.IsBOL(), "second '\\n' should be BOL")
					assert.True(t, symNL2.IsEOL(), "second '\\n' should be EOL")

					// Symbol 'b' at offset 3
					symB := processedSymbols[3]
					assert.Equal(t, 'b', symB.Rune())
					assert.False(t, symB.IsBOF(), "'b' is not BOF")
					assert.True(t, symB.IsBOL(), "'b' should be BOL")
					assert.False(t, symB.IsEOL(), "'b' should not be EOL")
				})
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString("CAPTURE", "a", 0),
				textlexer.NewLexemeFromString("CAPTURE", "\n", 1),
				textlexer.NewLexemeFromString("CAPTURE", "\n", 2),
				textlexer.NewLexemeFromString("CAPTURE", "b", 3),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			if tc.setupRules != nil {
				tc.setupRules(lx)
			}

			foundLexemes := make([]*textlexer.Lexeme, 0, len(tc.expectedLexemes))

			for {
				lex, err := lx.Next()
				if tc.expectedError != "" {
					require.Error(t, err, "Expected an error but got nil")
					assert.Contains(t, err.Error(), tc.expectedError, "Error message does not match")
					return
				}

				if err == io.EOF {
					break
				}
				require.NoError(t, err)

				foundLexemes = append(foundLexemes, lex)
			}

			ok := assert.Equal(t, lexemeKeys(tc.expectedLexemes), lexemeKeys(foundLexemes), "The stream of lexemes did not match the expected output.")
			if !ok {
				for i, lex := range foundLexemes {
					t.Logf("\tFound[%d]\tText=%q, Type=%q, Offset=%d", i, lex.Text(), lex.Type(), lex.Offset())
				}
			}

			_, err := lx.Next()
			require.Equal(t, io.EOF, err, "Expected EOF after consuming all tokens")
		})
	}
}

// TestLexerTokenLargerThanInitialBuffer verifies the internal buffer can grow
// to accommodate a token larger than its initial capacity.
func TestLexerTokenLargerThanInitialBuffer(t *testing.T) {
	const (
		lexTypeInteger    = textlexer.LexemeType("INT")
		lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
		initialBufferSize = 4096
		largeTokenSize    = initialBufferSize * 2
	)

	largeToken := strings.Repeat("1", largeTokenSize)
	input := largeToken + " "

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
	lx.MustAddRule(lexTypeWhitespace, whitespaceRule)

	lex, err := lx.Next()
	require.NoError(t, err)
	require.NotNil(t, lex)
	assert.Equal(t, lexTypeInteger, lex.Type())
	assert.Equal(t, largeToken, lex.Text())
	assert.Equal(t, largeTokenSize, lex.Len())
	assert.Equal(t, 0, lex.Offset())

	lex, err = lx.Next()
	require.NoError(t, err)
	require.NotNil(t, lex)
	assert.Equal(t, lexTypeWhitespace, lex.Type())
	assert.Equal(t, " ", lex.Text())

	_, err = lx.Next()
	require.Equal(t, io.EOF, err)
}

// TestLexerWithLargeInputOfManySmallTokens checks for resource exhaustion issues,
// such as a buffer that grows indefinitely without compaction.
func TestLexerWithLargeInputOfManySmallTokens(t *testing.T) {
	const (
		lexTypeIdentifier = textlexer.LexemeType("ID")
		lexTypeWhitespace = textlexer.LexemeType("WS")
		numTokens         = 100_000
	)

	var b strings.Builder
	b.Grow(numTokens * 2)
	for i := 0; i < numTokens; i++ {
		b.WriteString("a ")
	}
	input := b.String()

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeIdentifier, identifierRule)
	lx.MustAddRule(lexTypeWhitespace, whitespaceRule)

	tokenCount := 0
	for {
		_, err := lx.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		tokenCount++
	}

	assert.Equal(t, numTokens*2, tokenCount, "Did not process the expected number of tokens")
}

// TestLexerConcurrentAccess ensures that the lexer is goroutine-safe. Run with -race.
func TestLexerConcurrentAccess(t *testing.T) {
	const (
		lexTypeWord = textlexer.LexemeType("WORD")
		numWords    = 5000
	)

	input := strings.Repeat("word ", numWords)
	lx := textlexer.New(strings.NewReader(input))

	wordOrSpaceRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == 'w' {
			return matchString("ord "), textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	lx.MustAddRule(lexTypeWord, wordOrSpaceRule)

	var wg sync.WaitGroup
	numGoroutines := 4
	wg.Add(numGoroutines)

	resultsChan := make(chan *textlexer.Lexeme, numWords)
	errorsChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					return
				}
				if err != nil {
					errorsChan <- err
					return
				}
				resultsChan <- lex
			}
		}()
	}

	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	for err := range errorsChan {
		t.Errorf("Received unexpected error from Next(): %v", err)
	}

	tokenCount := 0
	for range resultsChan {
		tokenCount++
	}

	assert.Equal(t, numWords, tokenCount, "The total number of tokens from all goroutines is incorrect")
}

// TestLexerConcurrentDeterminism verifies that concurrent access produces deterministic results.
func TestLexerConcurrentDeterminism(t *testing.T) {
	const (
		lexTypeID = textlexer.LexemeType("ID")
		numTokens = 100
	)

	var b strings.Builder
	for i := 0; i < numTokens; i++ {
		b.WriteString(fmt.Sprintf("id%d ", i))
	}
	input := b.String()

	for run := 0; run < 3; run++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeID, identifierRule)
		lx.MustAddRule("WS", whitespaceRule)

		var mu sync.Mutex
		tokens := make([]*textlexer.Lexeme, 0, numTokens*2)

		var wg sync.WaitGroup
		numGoroutines := 4
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for {
					lex, err := lx.Next()
					if err == io.EOF {
						return
					}
					require.NoError(t, err)

					mu.Lock()
					tokens = append(tokens, lex)
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// The tokens were collected concurrently, so their order in the slice is not
		// guaranteed. We must sort them by their offset in the original input string
		// to restore the deterministic order before verification.
		sort.Slice(tokens, func(i, j int) bool {
			return tokens[i].Offset() < tokens[j].Offset()
		})

		assert.Equal(t, numTokens*2, len(tokens), "Run %d: incorrect token count", run)

		for i := 0; i < numTokens; i++ {
			idToken := tokens[i*2]
			wsToken := tokens[i*2+1]

			expectedID := fmt.Sprintf("id%d", i)
			assert.Equal(t, lexTypeID, idToken.Type(), "Run %d, Token %d: wrong type", run, i)
			assert.Equal(t, expectedID, idToken.Text(), "Run %d, Token %d: wrong text", run, i)
			assert.Equal(t, textlexer.LexemeType("WS"), wsToken.Type(), "Run %d, Token %d: wrong whitespace type", run, i)
		}
	}
}

// faultyReader simulates an io.Reader that returns an error after N reads.
type faultyReader struct {
	reader     io.RuneReader
	failAfterN int
	readCount  int
}

func (fr *faultyReader) ReadRune() (r rune, size int, err error) {
	if fr.readCount >= fr.failAfterN {
		return 0, 0, errors.New("simulated underlying reader error")
	}
	fr.readCount++
	return fr.reader.ReadRune()
}

// TestLexerHandlesReaderError verifies that the lexer gracefully propagates
// an error from the underlying reader.
func TestLexerHandlesReaderError(t *testing.T) {
	const lexTypeID = textlexer.LexemeType("ID")
	input := "abc def"
	reader := &faultyReader{
		reader:     strings.NewReader(input),
		failAfterN: 5, // Fails when trying to read 'e'
	}

	lx := textlexer.New(reader)
	lx.MustAddRule(lexTypeID, identifierRule)
	lx.MustAddRule("WS", whitespaceRule)

	_, err := lx.Next()
	require.NoError(t, err)

	_, err = lx.Next()
	require.NoError(t, err)

	_, err = lx.Next()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated underlying reader error")
}

// TestLexerPanickingRuleIsRecoveredAndRetired: the only rule panics on every
// symbol. The VM recovers each panic and retires the rule as for StateReject,
// so no rule matches and each rune is one UNKNOWN lexeme, then io.EOF. The
// panic never reaches the caller. See KD-2 in docs/specs/core-contract.md.
func TestLexerPanickingRuleIsRecoveredAndRetired(t *testing.T) {
	const lexTypePanic = textlexer.LexemeType("PANIC")
	input := "crash"

	panickingRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		panic("user rule panicked")
	}

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypePanic, panickingRule)

	assertRetireStream(t, lx, unknownLexemes(input, 0))
}

// TestLexerPanicRetiresRuleAndLexingContinues: a rule panics once, on the
// first symbol of the first token. The VM recovers the panic and retires that
// rule for the token; the other rules' matches survive, and later tokens are
// lexed with every rule, the panicking one included.
func TestLexerPanicRetiresRuleAndLexingContinues(t *testing.T) {
	const (
		lexTypePanic  = textlexer.LexemeType("PANIC")
		lexTypeNormal = textlexer.LexemeType("NORMAL")
	)
	input := "panic normal"

	panicCount := 0
	recoveringPanicRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if r == 'p' && panicCount == 0 {
			panicCount++
			panic("recoverable panic")
		}
		if r >= 'a' && r <= 'z' {
			return nil, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypePanic, recoveringPanicRule)
	lx.MustAddRule(lexTypeNormal, identifierRule)
	lx.MustAddRule("WS", whitespaceRule)

	// In the second token PANIC runs again and accepts "n", but NORMAL's
	// longer "normal" wins.
	assertRetireStream(t, lx, []*textlexer.Lexeme{
		textlexer.NewLexemeFromString(lexTypeNormal, "panic", 0),
		textlexer.NewLexemeFromString("WS", " ", 5),
		textlexer.NewLexemeFromString(lexTypeNormal, "normal", 6),
	})
	assert.Equal(t, 1, panicCount)
}

// TestLexerBufferCompaction verifies that the internal buffer compacts after
// processing large tokens, preventing unbounded memory growth.
func TestLexerBufferCompaction(t *testing.T) {
	const (
		lexTypeID         = textlexer.LexemeType("ID")
		lexTypeWhitespace = textlexer.LexemeType("WS")
		longTokenSize     = 8192
		numShortTokens    = 50_000
	)

	var b strings.Builder
	b.WriteString(strings.Repeat("a", longTokenSize))
	b.WriteString(" ")
	b.WriteString(strings.Repeat("b ", numShortTokens))
	input := b.String()

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeID, identifierRule)
	lx.MustAddRule(lexTypeWhitespace, whitespaceRule)

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	totalTokensProcessed := 0

	lex, err := lx.Next()
	require.NoError(t, err)
	assert.Equal(t, longTokenSize, lex.Len())
	totalTokensProcessed++

	_, err = lx.Next() // whitespace
	require.NoError(t, err)
	totalTokensProcessed++

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	memAfterLargeToken := m2.Alloc

	for i := 0; i < numShortTokens; i++ {
		_, err := lx.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		totalTokensProcessed++
	}

	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)
	memAfterSmallTokens := m3.Alloc

	// The memory usage should not grow significantly after processing small tokens
	// if the buffer was properly compacted.
	memGrowth := int64(0)
	if memAfterSmallTokens > memAfterLargeToken {
		memGrowth = int64(memAfterSmallTokens - memAfterLargeToken)
	}

	// Allow some growth but it should be much less than the large token size.
	maxAllowedGrowth := int64(longTokenSize / 2)
	assert.LessOrEqual(t, memGrowth, maxAllowedGrowth,
		"Memory grew by %d bytes after processing small tokens, suggesting buffer wasn't compacted", memGrowth)

	for {
		_, err := lx.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		totalTokensProcessed++
	}

	expectedTokenCount := 1 + 1 + (numShortTokens * 2)
	assert.Equal(t, expectedTokenCount, totalTokensProcessed)
}

// TestLexerConcurrentAddRuleAndNext checks for race conditions when AddRule
// is called while another goroutine is calling Next. Run with -race.
func TestLexerConcurrentAddRuleAndNext(t *testing.T) {
	const (
		lexTypeA = textlexer.LexemeType("A")
		lexTypeB = textlexer.LexemeType("B")
		lexTypeC = textlexer.LexemeType("C")
	)

	input := strings.Repeat("abc", 1000)
	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeA, matchString("a"))

	lexingStarted := make(chan struct{})
	addRuleDone := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	var raceError error
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		// Signal after the first Next has returned, so that AddRule starts
		// only once the processor exists. Whether AddRule then overlaps a
		// later Next is up to the scheduler; -race reports a race either way.
		signal := sync.OnceFunc(func() { close(lexingStarted) })
		defer signal()
		for {
			_, err := lx.Next()
			signal()
			if err == io.EOF {
				return
			}
			if err != nil {
				mu.Lock()
				if raceError == nil {
					raceError = err
				}
				mu.Unlock()
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		defer close(addRuleDone)
		<-lexingStarted

		err := lx.AddRule(lexTypeB, matchString("b"))
		if err != nil {
			mu.Lock()
			if raceError == nil {
				raceError = err
			}
			mu.Unlock()
		}

		err = lx.AddRule(lexTypeC, matchString("c"))
		if err != nil {
			mu.Lock()
			if raceError == nil {
				raceError = err
			}
			mu.Unlock()
		}
	}()

	wg.Wait()
	assert.NoError(t, raceError, "Encountered error during concurrent operations")
}

// TestLexerPathologicalRuleWithDeepBacktracking pins the only-grows rule with a
// deep pushback chain. The rule accepts every 'a', continues through 'b', and at
// end of input runs a chain of backtrackDepth+1 pushbacks. On n 'a's followed by
// 'b' the stack holds n+2 symbols (EOF included), so the chain's accept has
// length 1. That is shorter than the n symbols the rule already accepted, so the
// VM ignores it (processor.go:125): a pushback chain, however deep, cannot
// shorten or withdraw a match the rule already recorded.
func TestLexerPathologicalRuleWithDeepBacktracking(t *testing.T) {
	const lexTypePathological = textlexer.LexemeType("PATHOLOGICAL")
	const lexTypeUnknown = textlexer.LexemeTypeUnknown

	testCases := []struct {
		name           string
		input          string
		backtrackDepth int
		expected       []*textlexer.Lexeme
	}{
		{
			name:           "Shallow backtrack",
			input:          "aaab",
			backtrackDepth: 3,
			expected: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypePathological, "aaa", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "b", 3),
			},
		},
		{
			name:           "Medium backtrack",
			input:          "aaaaaaaab",
			backtrackDepth: 8,
			expected: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypePathological, "aaaaaaaa", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "b", 8),
			},
		},
		{
			name:           "Deep backtrack",
			input:          strings.Repeat("a", 100) + "b",
			backtrackDepth: 100,
			expected: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypePathological, strings.Repeat("a", 100), 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "b", 100),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pathologicalRule, finalState textlexer.Rule

			// Excludes the EOF symbol, 'b', and backtrackDepth-1 of the 'a's.
			finalState = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return excludeLast(tc.backtrackDepth+1, textlexer.StateAccept)
			}

			// This rule accepts every 'a' but continues matching. When it sees a 'b',
			// it continues into the final state, whose deep chain reports a match of
			// length 1. The earlier accept of all the 'a's stays.
			pathologicalRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				r := s.Rune()
				if r == 'a' {
					return pathologicalRule, textlexer.StateAccept
				}
				if r == 'b' {
					return finalState, textlexer.StateContinue
				}
				return nil, textlexer.StateReject
			}

			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypePathological, pathologicalRule)

			var found []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				found = append(found, lex)
			}

			assert.Equal(t, lexemeKeys(tc.expected), lexemeKeys(found), "Backtrack depth %d failed", tc.backtrackDepth)
		})
	}
}

// TestLexerWithInputContainingRuneEOF verifies the lexer does not prematurely
// terminate when the input stream itself contains the special RuneEOF value.
func TestLexerWithInputContainingRuneEOF(t *testing.T) {
	const lexTypeID = textlexer.LexemeType("ID")
	inputWithEOF := "hello" + string(textlexer.RuneEOF) + "world"

	lx := textlexer.New(strings.NewReader(inputWithEOF))
	lx.MustAddRule(lexTypeID, identifierRule)

	expected := []*textlexer.Lexeme{
		textlexer.NewLexemeFromString(lexTypeID, "hello", 0),
		textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, string(textlexer.RuneEOF), 5),
		textlexer.NewLexemeFromString(lexTypeID, "world", 6),
	}

	var found []*textlexer.Lexeme
	for {
		lex, err := lx.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		found = append(found, lex)
	}

	assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
}

// TestLexerWithManyRules is a stress test for a large number of rules.
func TestLexerWithManyRules(t *testing.T) {
	const numRules = 2000
	targetLexType := textlexer.LexemeType(fmt.Sprintf("RULE_%d", numRules-1))
	targetText := fmt.Sprintf("id%d", numRules-1)
	input := fmt.Sprintf("   %s   ", targetText)

	lx := textlexer.New(strings.NewReader(input))

	for i := 0; i < numRules; i++ {
		ruleText := fmt.Sprintf("id%d", i)
		ruleType := textlexer.LexemeType(fmt.Sprintf("RULE_%d", i))
		lx.MustAddRule(ruleType, matchString(ruleText))
	}
	lx.MustAddRule("WS", whitespaceRule)

	lex, err := lx.Next()
	require.NoError(t, err)
	assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())

	lex, err = lx.Next()
	require.NoError(t, err)
	assert.Equal(t, targetLexType, lex.Type())
	assert.Equal(t, targetText, lex.Text())

	lex, err = lx.Next()
	require.NoError(t, err)
	assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())

	_, err = lx.Next()
	require.Equal(t, io.EOF, err)
}

// TestLexerMaximumLimits tests boundary conditions for various lexer limits.
func TestLexerMaximumLimits(t *testing.T) {
	t.Run("Maximum buffer size", func(t *testing.T) {
		const maxBufferSize = 1024 * 1024 // 1MB
		largeToken := strings.Repeat("a", maxBufferSize)
		input := largeToken + " end"

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("ID", identifierRule)
		lx.MustAddRule("WS", whitespaceRule)

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, maxBufferSize, lex.Len())

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, "end", lex.Text())
	})

	t.Run("Maximum pushback depth", func(t *testing.T) {
		const maxPushback = 1000
		input := strings.Repeat("a", maxPushback+10)

		// deepPushbackRule continues over every 'a' and, at end of input,
		// excludes the last `exclude` symbols of the stack. The stack then holds
		// maxPushback+10 'a's plus the EOF symbol.
		deepPushbackRule := func(exclude int) textlexer.Rule {
			var loop textlexer.Rule
			loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == 'a' {
					return loop, textlexer.StateContinue
				}
				if s.IsEOF() {
					return excludeLast(exclude, textlexer.StateAccept)
				}
				return nil, textlexer.StateReject
			}
			return loop
		}

		// A chain of maxPushback+1 pushbacks (the EOF symbol and 1000 'a's)
		// runs to its accept and leaves the first 10 'a's.
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("DEEP_PUSHBACK", deepPushbackRule(maxPushback+1))

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("DEEP_PUSHBACK"), lex.Type())
		assert.Equal(t, strings.Repeat("a", 10), lex.Text())
		assert.Equal(t, 0, lex.Offset())

		// One pushback more than the stack holds passes its bottom, so the VM
		// retires the rule (processor.go:110) and no rule matches the first 'a'.
		lx = textlexer.New(strings.NewReader(input))
		lx.MustAddRule("DEEP_PUSHBACK", deepPushbackRule(maxPushback+12))

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type())
		assert.Equal(t, "a", lex.Text())
		assert.Equal(t, 0, lex.Offset())
	})

	t.Run("Maximum number of concurrent scanners", func(t *testing.T) {
		const numRules = 10000
		input := "test"

		lx := textlexer.New(strings.NewReader(input))

		// Add many rules that all start matching 't'.
		for i := 0; i < numRules; i++ {
			ruleText := fmt.Sprintf("t%d", i)
			ruleType := textlexer.LexemeType(fmt.Sprintf("RULE_%d", i))
			lx.MustAddRule(ruleType, matchString(ruleText))
		}
		lx.MustAddRule("TEST", matchString("test"))

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("TEST"), lex.Type())
		assert.Equal(t, "test", lex.Text())
	})
}

// TestLexerZeroLengthMatchPrevention verifies that zero-length matches are properly
// prevented from causing infinite loops.
func TestLexerZeroLengthMatchPrevention(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int
	}{
		{"Single character", "a", 1},
		{"Multiple characters", "abc", 3},
		{"Long input", strings.Repeat("x", 100), 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This rule always produces a zero-length match: one pushback on the
			// first symbol excludes it, and the accept has length 0. The VM
			// commits no empty token and emits one UNKNOWN rune instead
			// (textlexer.go:482-485); TestProcessZeroLengthMatch proves the
			// branch.
			zeroLengthRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return excludeLast(1, textlexer.StateAccept)
			}

			// Each Next reads one symbol and the rule takes two invocations
			// on it (the read and the pushback), so a run over the input and
			// the EOF symbol takes 2*(len+1). The budget doubles that; an
			// infinite loop inside one Next exceeds it and fails the test.
			budget := 4 * (len(tc.input) + 1)

			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule("ZERO", boundedRule(t, budget, zeroLengthRule))

			tokenCount := 0
			iterations := 0
			maxIterations := tc.expected*2 + 10

			for iterations < maxIterations {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				require.NotNil(t, lex)

				// Should produce UNKNOWN tokens, one per character.
				assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type())
				assert.Equal(t, 1, lex.Len(), "Zero-length match should result in single-char UNKNOWN token")

				tokenCount++
				iterations++
			}

			assert.Equal(t, tc.expected, tokenCount, "Should produce one token per input character")
			assert.Less(t, iterations, maxIterations, "Too many iterations - possible infinite loop")
		})
	}
}

// TestLexerPathologicalRules tests adversarial or badly-behaved rules to
// ensure the state machine is robust.
func TestLexerPathologicalRules(t *testing.T) {
	t.Run("Interleaving Longest Match", func(t *testing.T) {
		// Ensures the processor tracks the longest match even when the winning rule alternates.
		// Rule A matches odd lengths, Rule B matches even. The absolute longest must win.
		const (
			lexTypeOdd  = textlexer.LexemeType("ODD")
			lexTypeEven = textlexer.LexemeType("EVEN")
		)
		input := "aaaaa"

		var ruleA, ruleA_continue textlexer.Rule
		ruleA = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return ruleA_continue, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		ruleA_continue = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return ruleA, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}

		var ruleB, ruleB_accept textlexer.Rule
		ruleB = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return ruleB_accept, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}
		ruleB_accept = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return ruleB, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeOdd, ruleA)
		lx.MustAddRule(lexTypeEven, ruleB)

		// Expected: Rule A's lastAccept is 5, Rule B's is 4. The processor must pick the longest.
		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeOdd, "aaaaa", 0),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Bait and Switch Rule", func(t *testing.T) {
		// A malicious rule ("Bait") finds a long match, but then backtracks to report a shorter one.
		// The lexer must not be fooled and should select the longest match recorded by any rule.
		// This test pins the only-grows rule: within a token a rule's recorded match only
		// grows (processor.go:125), so a pushback chain cannot shorten an earlier accept.
		const (
			lexTypeBait   = textlexer.LexemeType("BAIT")
			lexTypeHonest = textlexer.LexemeType("HONEST")
		)
		input := "token;"

		var baitRule textlexer.Rule
		baitFinalizer := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == ';' {
				// Exclude five of the six symbols ("oken;"): a match of length 1.
				return excludeLast(5, textlexer.StateAccept)
			}
			return nil, textlexer.StateReject
		}
		match_n := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'n' {
				return baitFinalizer, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_e := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'e' {
				return match_n, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_k := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'k' {
				return match_e, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_o := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'o' {
				return match_k, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		baitRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 't' {
				return match_o, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeBait, baitRule)
		lx.MustAddRule(lexTypeHonest, matchString("token"))

		// Expected: Both rules achieve a maximal match of length 5. The Bait rule's
		// attempt to backtrack to a shorter match is ignored. Since "Bait" was added
		// first, it wins the tie for the longest match.
		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeBait, "token", 0),
			textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, ";", 5),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Rule Rejects After Full Buffer Consumption (Veto Rule)", func(t *testing.T) {
		// A rule consumes a valid token, but upon seeing a terminator, rejects its own future
		// participation. The lexer should still honor the longest match already found by that rule.
		const (
			lexTypeVeto   = textlexer.LexemeType("VETO")
			lexTypeHonest = textlexer.LexemeType("HONEST")
		)
		input := "abc;"

		var vetoRule textlexer.Rule
		vetoFinalizer := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == ';' {
				// Veto: After seeing ';', push it back and end the chain with
				// the rule's own StateReject. This does NOT erase the lastAccept
				// value already recorded for this rule.
				return excludeLast(1, textlexer.StateReject)
			}
			return nil, textlexer.StateReject
		}
		match_c := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'c' {
				return vetoFinalizer, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_b := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'b' {
				return match_c, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		vetoRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return match_b, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeVeto, vetoRule)
		lx.MustAddRule(lexTypeHonest, matchString("abc"))

		// Expected: Both VETO and HONEST rules achieve a maximal match of length 3.
		// The VETO rule's subsequent StateReject does not retroactively cancel
		// its match. Since VETO was added first, it wins the tie.
		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeVeto, "abc", 0),
			textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, ";", 3),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Rule Returns nil with StatePushBack", func(t *testing.T) {
		// An invalid user rule returns (nil, StatePushBack). The lexer must handle
		// this by deactivating the rule after the pushback to prevent errors.
		const lexTypeBad = textlexer.LexemeType("BAD_PUSHBACK")
		input := "ab"

		badRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					// Push back 'b' but provide no next rule.
					return nil, textlexer.StatePushBack
				}, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeBad, badRule)
		// This honest rule will be chosen after the bad one deactivates.
		lx.MustAddRule("A", matchString("a"))

		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(textlexer.LexemeType("A"), "a", 0),
			textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Always Reject Rule", func(t *testing.T) {
		const lexTypeReject = textlexer.LexemeType("REJECT")
		input := "abc"

		alwaysRejectRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeReject, alwaysRejectRule)

		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
			textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
			textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "c", 2),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Always Accept Rule", func(t *testing.T) {
		const lexTypeAccept = textlexer.LexemeType("ACCEPT")
		input := "abc"

		alwaysAcceptRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateAccept
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeAccept, alwaysAcceptRule)

		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeAccept, "a", 0),
			textlexer.NewLexemeFromString(lexTypeAccept, "b", 1),
			textlexer.NewLexemeFromString(lexTypeAccept, "c", 2),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Exponential State Explosion", func(t *testing.T) {
		// This simulates a badly written regex like (a*)* that could cause
		// exponential state explosion.
		const lexTypeExponential = textlexer.LexemeType("EXPONENTIAL")
		input := strings.Repeat("a", 20) + "b"

		callCount := 0
		var exponentialRule textlexer.Rule
		exponentialRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			callCount++
			if callCount > 1000 { // Safety limit.
				return nil, textlexer.StateReject
			}

			if s.Rune() == 'a' {
				return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					if s.Rune() == 'a' {
						// Branch: either continue or accept.
						return exponentialRule, textlexer.StateAccept
					}
					return nil, textlexer.StateReject
				}, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		// The rule takes one invocation per symbol read. The first token reads
		// the whole input, then "b" is read again and the EOF symbol once, so
		// a run takes len+2 invocations. A budget linear in the input fails
		// the test, from inside the rule, on an explosion or a lexer that
		// stops terminating.
		budget := 4 * (len(input) + 1)

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeExponential, boundedRule(t, budget, exponentialRule))

		for {
			_, err := lx.Next()
			if err == io.EOF {
				break
			}
		}

		assert.Less(t, callCount, 1001, "Rule call count should be limited")
	})

	t.Run("Memory Exhaustion Attack", func(t *testing.T) {
		const lexTypeMemory = textlexer.LexemeType("MEMORY")
		input := "start"

		var memoryTracker [][]byte
		memoryExhaustionRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if len(memoryTracker) < 10 { // Limit to prevent actual exhaustion in test.
				memoryTracker = append(memoryTracker, make([]byte, 1024*1024)) // 1MB
			}

			if s.Rune() >= 'a' && s.Rune() <= 'z' {
				return nil, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeMemory, memoryExhaustionRule)

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, lexTypeMemory, lex.Type())
		assert.Equal(t, "s", lex.Text())

		memoryTracker = nil
	})

	t.Run("Recursive Rule Cycles", func(t *testing.T) {
		const lexTypeCycle = textlexer.LexemeType("CYCLE")
		input := "abc"

		var ruleA, ruleB, ruleC textlexer.Rule

		// Create a cycle: A -> B -> C -> A
		ruleA = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return ruleB, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		ruleB = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'b' {
				return ruleC, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		ruleC = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'c' {
				return ruleA, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeCycle, ruleA)

		expected := []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeCycle, "abc", 0),
		}

		var found []*textlexer.Lexeme
		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			found = append(found, lex)
		}
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(found))
	})

	t.Run("Buffer Overflow Attempt", func(t *testing.T) {
		const lexTypeOverflow = textlexer.LexemeType("OVERFLOW")
		input := strings.Repeat("a", 65536)

		overflowRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				var deepChain func(int) textlexer.Rule
				deepChain = func(depth int) textlexer.Rule {
					if depth > 100 { // Limit depth for test.
						return nil
					}
					return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
						if s.Rune() == 'a' {
							return deepChain(depth + 1), textlexer.StateAccept
						}
						return nil, textlexer.StateReject
					}
				}
				return deepChain(0), textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeOverflow, overflowRule)

		tokenCount := 0
		for {
			_, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			tokenCount++
		}
		assert.Greater(t, tokenCount, 0, "Should process at least one token")
	})

	t.Run("Concurrent State Mutation", func(t *testing.T) {
		// A rule that mutates shared state should be safe as long as the state is
		// managed correctly (e.g., with atomics).
		const lexTypeMutate = textlexer.LexemeType("MUTATE")
		input := "test test test "

		sharedCounter := int32(0)

		var mutatingRule textlexer.Rule
		mutatingRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			atomic.AddInt32(&sharedCounter, 1)
			r := s.Rune()
			if r >= 'a' && r <= 'z' {
				return mutatingRule, textlexer.StateAccept
			}
			if r == ' ' {
				return nil, textlexer.StateReject
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeMutate, mutatingRule)
		lx.MustAddRule("WS", whitespaceRule)

		tokenCount := 0
		for {
			_, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			tokenCount++
		}

		assert.Greater(t, atomic.LoadInt32(&sharedCounter), int32(0), "Rule should have been called")
		assert.Equal(t, 6, tokenCount, "Should produce 6 tokens (3 words + 3 spaces)")
	})
}

// TestLexerChaos is a non-deterministic stress test that uses a rule with
// random behavior to process random input. Its primary purpose is to detect
// panics or deadlocks under unpredictable conditions. Run with -race.
func TestLexerChaos(t *testing.T) {
	const (
		lexTypeChaos = textlexer.LexemeType("CHAOS")
		inputSize    = 10000
	)

	inputBytes := make([]byte, inputSize)
	_, err := io.ReadFull(rand.Reader, inputBytes)
	require.NoError(t, err)
	for i, b := range inputBytes {
		inputBytes[i] = (b % 94) + 32 // Keep it to printable ASCII
	}
	input := string(inputBytes)

	lx := textlexer.New(strings.NewReader(input))

	rng := mrand.New(mrand.NewSource(42))

	matchLength := 0
	var chaosRule textlexer.Rule
	chaosRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		matchLength++
		u := rng.Float64()

		if u < 0.1 && matchLength > 1 {
			// Exclude the symbol just read and accept the rest.
			return excludeLast(1, textlexer.StateAccept)
		}

		if u < 0.7 && matchLength < 100 {
			return chaosRule, textlexer.StateContinue
		}

		matchLength = 0
		if u < 0.85 {
			return nil, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}

	// The rule continues only while matchLength is below 100, so a token reads
	// at most 100 symbols. The excludeLast branch does not reset matchLength,
	// which only makes the next token shorter. Each read takes at most one
	// extra pushback invocation, and every token consumes at least one symbol,
	// so a run over the input and the EOF symbol takes at most
	// 2*100*(inputSize+1) invocations. This is a worst-case bound, far above
	// the count a correct run takes; a lexer that stops terminating exceeds it
	// and fails the test.
	//
	// The rule ignores the symbol it reads and its RNG has a fixed seed, so a
	// run does not depend on the random input: a failure reproduces with any
	// input.
	budget := 2 * 100 * (inputSize + 1)
	lx.MustAddRule(lexTypeChaos, boundedRule(t, budget, chaosRule))

	tokenCount := 0
	for {
		_, err := lx.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Errors are expected in a chaos test; the goal is to ensure no panics or hangs.
			t.Logf("Chaos test encountered error (expected): %v", err)
			break
		}
		tokenCount++

		if tokenCount > inputSize*2 {
			t.Logf("Chaos test produced maximum tokens, stopping")
			break
		}
	}

	assert.Greater(t, tokenCount, 0, "Chaos test should have produced at least one token")
}

// TestLexerInfiniteLoopProtection specifically tests that the lexer protects against
// various types of infinite loops.
func TestLexerInfiniteLoopProtection(t *testing.T) {
	testCases := []struct {
		name string
		rule textlexer.Rule
	}{
		{
			name: "Always Continue",
			rule: func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				var loop textlexer.Rule
				loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return loop, textlexer.StateContinue
				}
				return loop(s)
			},
		},
		{
			name: "Always PushBack",
			rule: func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				var loop textlexer.Rule
				loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					return loop, textlexer.StatePushBack
				}
				return loop(s)
			},
		},
		{
			name: "Zero Length Accept Loop",
			// One pushback on the first symbol excludes it: a zero-length accept.
			rule: func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return excludeLast(1, textlexer.StateAccept)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := "test"

			// A rule that never settles is retired at EOF and yields a
			// one-rune UNKNOWN token, so the k-th Next from the end reads at
			// most k symbols (the rest of the input and the EOF symbol). Each
			// of these rules takes at most two invocations per symbol read,
			// so a run takes at most (len+1)*(len+2) invocations. A lexer
			// stuck in a loop exceeds the budget and fails the test.
			n := len(input)
			budget := (n + 1) * (n + 2)

			lx := textlexer.New(strings.NewReader(input))
			lx.MustAddRule("LOOP", boundedRule(t, budget, tc.rule))

			iterations := 0
			maxIterations := 100

			for iterations < maxIterations {
				_, err := lx.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					// Expected - the lexer should error out of an infinite loop.
					break
				}
				iterations++
			}
		})
	}
}

// TestLexerResourceExhaustion tests the lexer's handling of resource-intensive scenarios.
func TestLexerResourceExhaustion(t *testing.T) {
	t.Run("Stack Depth Exhaustion", func(t *testing.T) {
		// This test verifies that the lexer's processing loop is iterative and does not
		// exhaust the call stack when processing a rule with a very long chain of
		// StateContinue transitions.
		const maxDepth = 10000

		// This rule is implemented iteratively using a closure to track depth,
		// making it stack-safe. This correctly tests the lexer's loop, not the
		// Go runtime's stack limit.
		stackSafeDeepRule := func() textlexer.Rule {
			depth := 0
			var ruleLoop textlexer.Rule

			ruleLoop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() != 'a' {
					return nil, textlexer.StateReject
				}
				depth++
				if depth >= maxDepth {
					return nil, textlexer.StateAccept
				}
				return ruleLoop, textlexer.StateContinue
			}
			return ruleLoop
		}

		input := strings.Repeat("a", maxDepth)
		lx := textlexer.New(strings.NewReader(input))
		// The first Next invokes the rule once per symbol and the EOF probe
		// once more, so the run takes maxDepth+1 invocations. The budget
		// doubles that; a lexer stuck in a loop exceeds it and fails the test.
		budget := 2 * (maxDepth + 1)
		lx.MustAddRule("DEEP", boundedRule(t, budget, stackSafeDeepRule()))

		// Next runs on the test goroutine, so a panic fails the test. A Go
		// stack overflow is fatal and cannot be recovered in any case.
		lex, err := lx.Next()
		require.NoError(t, err)
		require.NotNil(t, lex)
		assert.Equal(t, maxDepth, lex.Len())

		_, err = lx.Next()
		require.Equal(t, io.EOF, err)
	})

	t.Run("Memory Compaction Under Pressure", func(t *testing.T) {
		// Alternate between large and small tokens to test memory compaction.
		var b strings.Builder
		for i := 0; i < 100; i++ {
			b.WriteString(strings.Repeat("a", 10000))
			b.WriteString(" ")
			for j := 0; j < 100; j++ {
				b.WriteString("b ")
			}
		}
		input := b.String()

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("ID", identifierRule)
		lx.MustAddRule("WS", whitespaceRule)

		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)
		initialMem := m1.Alloc

		tokenCount := 0
		for {
			_, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			tokenCount++

			if tokenCount%1000 == 0 {
				runtime.GC()
				var m2 runtime.MemStats
				runtime.ReadMemStats(&m2)
				memGrowth := int64(0)
				if m2.Alloc > initialMem {
					memGrowth = int64(m2.Alloc - initialMem)
				}
				// Memory growth should be bounded, not linear with input.
				maxExpectedGrowth := int64(100 * 1024 * 1024) // 100MB max
				assert.LessOrEqual(t, memGrowth, maxExpectedGrowth,
					"Memory grew by %d bytes, suggesting a leak", memGrowth)
			}
		}
		assert.Greater(t, tokenCount, 10000, "Should process many tokens")
	})
}

// TestLexerEdgeCasesAtEOF tests various edge cases that occur at end of file.
func TestLexerEdgeCasesAtEOF(t *testing.T) {
	t.Run("Rule in StateAccept at EOF", func(t *testing.T) {
		input := "test"

		var greedyRule textlexer.Rule
		greedyRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() >= 'a' && s.Rune() <= 'z' {
				return greedyRule, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("GREEDY", greedyRule)

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, "test", lex.Text())

		_, err = lx.Next()
		assert.Equal(t, io.EOF, err)
	})

	t.Run("Multiple Rules in Different States at EOF", func(t *testing.T) {
		input := "ab"

		// Rule A accepts 'a', continues on 'b', but never accepts the final state.
		ruleA := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'a' {
				return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					if s.Rune() == 'b' {
						return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
							return nil, textlexer.StateReject
						}, textlexer.StateContinue // Continue without accepting 'ab'.
					}
					return nil, textlexer.StateReject
				}, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		// Rule B accepts both 'a' and 'b'.
		var ruleB textlexer.Rule
		ruleB = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			r := s.Rune()
			if r == 'a' || r == 'b' {
				return ruleB, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("A", ruleA)
		lx.MustAddRule("B", ruleB)

		// Rule B should win with "ab" since its accepted match is longer than A's ("a").
		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("B"), lex.Type())
		assert.Equal(t, "ab", lex.Text())

		_, err = lx.Next()
		assert.Equal(t, io.EOF, err)
	})

	t.Run("Empty Input with Complex Rules", func(t *testing.T) {
		input := ""

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("FLOAT", unsignedFloatRule)
		lx.MustAddRule("INT", unsignedIntegerRule)
		lx.MustAddRule("ID", identifierRule)
		lx.MustAddRule("COMMENT", newSlashStarCommentRule())

		_, err := lx.Next()
		assert.Equal(t, io.EOF, err, "Empty input should immediately return EOF")
	})
}

// TestLexerBoundaryConditions tests various boundary conditions.
func TestLexerBoundaryConditions(t *testing.T) {
	t.Run("Single Rune Input", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected textlexer.LexemeType
		}{
			{"1", textlexer.LexemeType("INT")},
			{"a", textlexer.LexemeType("ID")},
			{" ", textlexer.LexemeType("WS")},
			{"€", textlexer.LexemeTypeUnknown},
		}

		for _, tc := range testCases {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule("INT", unsignedIntegerRule)
			lx.MustAddRule("ID", identifierRule)
			lx.MustAddRule("WS", whitespaceRule)

			lex, err := lx.Next()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, lex.Type())
			assert.Equal(t, tc.input, lex.Text())

			_, err = lx.Next()
			assert.Equal(t, io.EOF, err)
		}
	})

	t.Run("Maximum Rule Count", func(t *testing.T) {
		const maxRules = 10000
		input := "test"

		lx := textlexer.New(strings.NewReader(input))

		for i := 0; i < maxRules; i++ {
			ruleType := textlexer.LexemeType(fmt.Sprintf("RULE_%d", i))
			ruleText := fmt.Sprintf("rule%d", i)
			err := lx.AddRule(ruleType, matchString(ruleText))
			require.NoError(t, err, "Should handle %d rules", i+1)
		}

		lx.MustAddRule("TEST", matchString("test"))

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("TEST"), lex.Type())
		assert.Equal(t, "test", lex.Text())
	})

	t.Run("Maximum Token Length", func(t *testing.T) {
		sizes := []int{
			255,     // 8-bit boundary
			256,     // 8-bit boundary + 1
			65535,   // 16-bit boundary
			65536,   // 16-bit boundary + 1
			1048576, // 1MB
		}

		for _, size := range sizes {
			t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
				input := strings.Repeat("a", size) + " "

				lx := textlexer.New(strings.NewReader(input))
				lx.MustAddRule("ID", identifierRule)
				lx.MustAddRule("WS", whitespaceRule)

				lex, err := lx.Next()
				require.NoError(t, err)
				assert.Equal(t, size, lex.Len())
				assert.Equal(t, textlexer.LexemeType("ID"), lex.Type())

				lex, err = lx.Next()
				require.NoError(t, err)
				assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())
			})
		}
	})
}

// TestLexerComplexInteractions tests complex interactions between multiple rules.
func TestLexerComplexInteractions(t *testing.T) {
	t.Run("Overlapping Keywords and Identifiers", func(t *testing.T) {
		input := "if iffy while whiley"

		lx := textlexer.New(strings.NewReader(input))
		// Keywords first for higher priority on same-length matches.
		lx.MustAddRule("IF", matchString("if"))
		lx.MustAddRule("WHILE", matchString("while"))
		lx.MustAddRule("ID", identifierRule)
		lx.MustAddRule("WS", whitespaceRule)

		expected := []textlexer.LexemeType{
			"IF",    // "if"
			"WS",    // " "
			"ID",    // "iffy" - longer match wins over "if"
			"WS",    // " "
			"WHILE", // "while"
			"WS",    // " "
			"ID",    // "whiley" - longer match wins over "while"
		}

		for _, expectedType := range expected {
			lex, err := lx.Next()
			require.NoError(t, err)
			assert.Equal(t, expectedType, lex.Type())
		}

		_, err := lx.Next()
		assert.Equal(t, io.EOF, err)
	})

	t.Run("Ambiguous Number Formats", func(t *testing.T) {
		input := "123 123. .123 123.456"

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("FLOAT", unsignedFloatRule)
		lx.MustAddRule("INT", unsignedIntegerRule)
		lx.MustAddRule("WS", whitespaceRule)

		expected := []struct {
			typ  textlexer.LexemeType
			text string
		}{
			{"INT", "123"},
			{"WS", " "},
			{"FLOAT", "123."},
			{"WS", " "},
			{"FLOAT", ".123"},
			{"WS", " "},
			{"FLOAT", "123.456"},
		}

		for _, exp := range expected {
			lex, err := lx.Next()
			require.NoError(t, err)
			assert.Equal(t, exp.typ, lex.Type())
			assert.Equal(t, exp.text, lex.Text())
		}

		_, err := lx.Next()
		assert.Equal(t, io.EOF, err)
	})
}

// BenchmarkLexerPerformance provides performance benchmarks for common scenarios.
func BenchmarkLexerPerformance(b *testing.B) {
	b.Run("SimpleTokens", func(b *testing.B) {
		input := strings.Repeat("abc def 123 ", 1000)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lx := textlexer.New(strings.NewReader(input))
			lx.MustAddRule("ID", identifierRule)
			lx.MustAddRule("INT", unsignedIntegerRule)
			lx.MustAddRule("WS", whitespaceRule)

			for {
				_, err := lx.Next()
				if err == io.EOF {
					break
				}
			}
		}
	})

	b.Run("ComplexRules", func(b *testing.B) {
		input := strings.Repeat("123.456 'string' /* comment */ identifier ", 1000)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lx := textlexer.New(strings.NewReader(input))
			lx.MustAddRule("FLOAT", unsignedFloatRule)
			lx.MustAddRule("STRING", newSingleQuotedStringRule())
			lx.MustAddRule("COMMENT", newSlashStarCommentRule())
			lx.MustAddRule("ID", identifierRule)
			lx.MustAddRule("WS", whitespaceRule)

			for {
				_, err := lx.Next()
				if err == io.EOF {
					break
				}
			}
		}
	})

	b.Run("LargeTokens", func(b *testing.B) {
		largeToken := strings.Repeat("a", 10000)
		input := strings.Repeat(largeToken+" ", 100)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lx := textlexer.New(strings.NewReader(input))
			lx.MustAddRule("ID", identifierRule)
			lx.MustAddRule("WS", whitespaceRule)

			for {
				_, err := lx.Next()
				if err == io.EOF {
					break
				}
			}
		}
	})

	b.Run("ManyRules", func(b *testing.B) {
		input := strings.Repeat("test ", 1000)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lx := textlexer.New(strings.NewReader(input))

			for j := 0; j < 100; j++ {
				ruleType := textlexer.LexemeType(fmt.Sprintf("RULE_%d", j))
				ruleText := fmt.Sprintf("keyword%d", j)
				lx.MustAddRule(ruleType, matchString(ruleText))
			}
			lx.MustAddRule("TEST", matchString("test"))
			lx.MustAddRule("WS", whitespaceRule)

			for {
				_, err := lx.Next()
				if err == io.EOF {
					break
				}
			}
		}
	})
}

// TestLexerErrorConditions comprehensively tests error handling.
func TestLexerErrorConditions(t *testing.T) {
	t.Run("Duplicate Rule Types", func(t *testing.T) {
		lx := textlexer.New(strings.NewReader("test"))

		err := lx.AddRule("TEST", matchString("test"))
		require.NoError(t, err)

		err = lx.AddRule("TEST", matchString("other"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TEST")
	})

	t.Run("Nil Rule", func(t *testing.T) {
		lx := textlexer.New(strings.NewReader("test"))

		err := lx.AddRule("NIL", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("Empty LexemeType", func(t *testing.T) {
		lx := textlexer.New(strings.NewReader("test"))

		err := lx.AddRule("", matchString("test"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("Reader Error Propagation", func(t *testing.T) {
		testCases := []struct {
			name      string
			failAfter int
			input     string
		}{
			{"Immediate failure", 0, "test"},
			{"Mid-token failure", 2, "test"},
			{"Between tokens failure", 5, "test test"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				reader := &faultyReader{
					reader:     strings.NewReader(tc.input),
					failAfterN: tc.failAfter,
				}

				lx := textlexer.New(reader)
				lx.MustAddRule("ID", identifierRule)
				lx.MustAddRule("WS", whitespaceRule)

				var lastErr error
				for {
					_, err := lx.Next()
					if err != nil {
						lastErr = err
						break
					}
				}

				assert.Error(t, lastErr)
				if lastErr != io.EOF {
					assert.Contains(t, lastErr.Error(), "simulated")
				}
			})
		}
	})

	t.Run("Rule Panic Types Are Recovered", func(t *testing.T) {
		// A panic with any value is recovered and retires the rule, a nil
		// value included (on Go 1.21+ panic(nil) raises a
		// *runtime.PanicNilError). A sibling rule's match survives.
		testCases := []struct {
			name       string
			panicValue interface{}
		}{
			{"String panic", "rule panic: string"},
			{"Error panic", errors.New("rule panic: error")},
			{"Integer panic", 42},
			{"Nil panic", nil},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				input := "test"

				panicRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					panic(tc.panicValue)
				}

				t.Run("only rule", func(t *testing.T) {
					lx := textlexer.New(strings.NewReader(input))
					lx.MustAddRule("PANIC", panicRule)

					assertRetireStream(t, lx, unknownLexemes(input, 0))
				})

				t.Run("with a sibling", func(t *testing.T) {
					lx := textlexer.New(strings.NewReader(input))
					lx.MustAddRule("PANIC", panicRule)
					lx.MustAddRule("WORD", newLettersRule())

					assertRetireStream(t, lx, []*textlexer.Lexeme{
						textlexer.NewLexemeFromString("WORD", input, 0),
					})
				})
			})
		}
	})
}

// TestLexerStateMachineInvariants verifies that the state machine maintains its invariants.
func TestLexerStateMachineInvariants(t *testing.T) {
	t.Run("State Transition Validation", func(t *testing.T) {
		input := "test"

		var transitions []string
		var mu sync.Mutex

		trackingRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			mu.Lock()
			transitions = append(transitions, fmt.Sprintf("Start(%c)", s.Rune()))
			mu.Unlock()

			if s.Rune() >= 'a' && s.Rune() <= 'z' {
				var continueRule textlexer.Rule
				continueRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					mu.Lock()
					transitions = append(transitions, fmt.Sprintf("Continue(%c)", s.Rune()))
					mu.Unlock()

					if s.Rune() >= 'a' && s.Rune() <= 'z' {
						return continueRule, textlexer.StateAccept
					}
					return nil, textlexer.StateReject
				}
				return continueRule, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("TRACK", trackingRule)

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, "test", lex.Text())

		assert.Equal(t, "Start(t)", transitions[0])
		assert.Contains(t, transitions, "Continue(e)")
		assert.Contains(t, transitions, "Continue(s)")
		assert.Contains(t, transitions, "Continue(t)")
	})

	t.Run("Scanner Lifecycle", func(t *testing.T) {
		input := "ab cd"

		type lifecycleEvent struct {
			event string
			rune  rune
		}
		var events []lifecycleEvent
		var mu sync.Mutex

		lifecycleRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			mu.Lock()
			events = append(events, lifecycleEvent{"create", s.Rune()})
			mu.Unlock()

			if s.Rune() >= 'a' && s.Rune() <= 'z' {
				return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					mu.Lock()
					events = append(events, lifecycleEvent{"continue", s.Rune()})
					mu.Unlock()
					r := s.Rune()
					if r >= 'a' && r <= 'z' {
						return nil, textlexer.StateAccept
					}
					if r == ' ' {
						mu.Lock()
						events = append(events, lifecycleEvent{"reject_space", r})
						mu.Unlock()
						return nil, textlexer.StateReject
					}
					return nil, textlexer.StateReject
				}, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("LIFECYCLE", lifecycleRule)
		lx.MustAddRule("WS", whitespaceRule)

		for {
			_, err := lx.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		assert.Greater(t, len(events), 0, "Should have lifecycle events")
		assert.Equal(t, lifecycleEvent{"create", 'a'}, events[0])

		foundContinueB := false
		for _, e := range events {
			if e.event == "continue" && e.rune == 'b' {
				foundContinueB = true
				break
			}
		}
		assert.True(t, foundContinueB, "Should have continued with 'b'")
	})

	t.Run("Accept Position Tracking", func(t *testing.T) {
		input := "aaabbb"

		var acceptPositions []int
		var mu sync.Mutex

		positionTrackingRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			currentPos := 0
			var track func(s textlexer.Symbol) (textlexer.Rule, textlexer.State)
			track = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				currentPos++
				r := s.Rune()
				if r == 'a' {
					mu.Lock()
					acceptPositions = append(acceptPositions, currentPos)
					mu.Unlock()
					return track, textlexer.StateAccept
				}
				if r == 'b' {
					// Don't accept 'b', forcing backtrack to last 'a'.
					return nil, textlexer.StateReject
				}
				return nil, textlexer.StateReject
			}
			return track(s)
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("POSITION", positionTrackingRule)
		lx.MustAddRule("B", matchString("bbb"))

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, "aaa", lex.Text())

		assert.Contains(t, acceptPositions, 1)
		assert.Contains(t, acceptPositions, 2)
		assert.Contains(t, acceptPositions, 3)

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, "bbb", lex.Text())
	})
}

// TestLexerRecoveryMechanisms tests the lexer's ability to recover from various error conditions.
func TestLexerRecoveryMechanisms(t *testing.T) {
	t.Run("Recovery After Unknown Tokens", func(t *testing.T) {
		input := "valid @#$ valid"

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("ID", identifierRule)
		lx.MustAddRule("WS", whitespaceRule)

		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("ID"), lex.Type())
		assert.Equal(t, "valid", lex.Text())

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())

		for i := 0; i < 3; i++ {
			lex, err = lx.Next()
			require.NoError(t, err)
			assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type())
		}

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())

		// Should recover and parse next valid token.
		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("ID"), lex.Type())
		assert.Equal(t, "valid", lex.Text())
	})

	t.Run("Recovery After Rule Deactivation", func(t *testing.T) {
		input := "test test"
		callCount := 0

		var selfDeactivatingRule textlexer.Rule
		// Rule that deactivates itself after the first match.
		selfDeactivatingRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			callCount++
			if callCount > 4 { // Deactivate after matching "test".
				return nil, textlexer.StateReject
			}
			if s.Rune() >= 'a' && s.Rune() <= 'z' {
				return selfDeactivatingRule, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("DEACTIVATE", selfDeactivatingRule)
		lx.MustAddRule("ID", identifierRule) // Backup rule.
		lx.MustAddRule("WS", whitespaceRule)

		// First token uses the deactivating rule.
		lex, err := lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("DEACTIVATE"), lex.Type())
		assert.Equal(t, "test", lex.Text())

		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("WS"), lex.Type())

		// Second token should use the backup rule since the first one deactivated.
		lex, err = lx.Next()
		require.NoError(t, err)
		assert.Equal(t, textlexer.LexemeType("ID"), lex.Type())
		assert.Equal(t, "test", lex.Text())
	})

	t.Run("Recovery from Reader Errors (if reader recovers)", func(t *testing.T) {
		// This test verifies that the lexer correctly propagates an error from the
		// underlying reader and halts.

		// Create a reader that fails once on the first character.
		reader := &recoverableReader{
			reader:   strings.NewReader("test"),
			maxFails: 1,
		}

		lx := textlexer.New(reader)
		lx.MustAddRule("ID", identifierRule)

		// The first call to Next() will attempt to read the first rune 't', which fails.
		lex, err := lx.Next()

		// The lexer should propagate this error and return no lexeme.
		require.Error(t, err, "Expected an error from the failing reader")
		assert.Nil(t, lex, "Lexeme should be nil on reader error")
		assert.Contains(t, err.Error(), "temporary reader error", "Error message did not match")
	})
}

func TestLexerBacktrackingAndBufferState(t *testing.T) {
	const (
		lexTypeFor = textlexer.LexemeType("FOR")
		lexTypeID  = textlexer.LexemeType("ID")
	)

	// This rule matches "for" but only if it's not followed by another letter.
	// It does not accept at 'r', because a later pushback could not withdraw that
	// accept. It continues and decides on the symbol after "for": a letter makes
	// it reject; any other symbol is pushed back and "for" is accepted.
	forKeywordRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == 'f' {
			return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == 'o' {
					return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
						if s.Rune() == 'r' {
							return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
								// Lookahead: if the next char is a letter, this is not a keyword.
								if isLetter(s) {
									return nil, textlexer.StateReject
								}
								// It's a keyword. Push back the non-letter symbol.
								return excludeLast(1, textlexer.StateAccept)
							}, textlexer.StateContinue
						}
						return nil, textlexer.StateReject
					}, textlexer.StateContinue
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	input := "for format"
	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeFor, forKeywordRule)
	lx.MustAddRule(lexTypeID, newIdentifierRule())
	lx.MustAddRule("WS", newWhitespaceRule())

	// 1. "for" followed by space. The rule should accept "for" and push back the space.
	lex, err := lx.Next()
	require.NoError(t, err)
	assert.Equal(t, lexTypeFor, lex.Type())
	assert.Equal(t, "for", lex.Text())
	assert.Equal(t, 0, lex.Offset())

	// 2. Whitespace. Should be correctly processed from the backtracked buffer.
	lex, err = lx.Next()
	require.NoError(t, err)
	assert.Equal(t, " ", lex.Text())
	assert.Equal(t, 3, lex.Offset())

	// 3. "format". The keyword rule reads "for" and decides on the next
	//    symbol, 'm': a letter, so it rejects without having accepted. The ID
	//    rule wins with the longest match, "format".
	lex, err = lx.Next()
	require.NoError(t, err)
	assert.Equal(t, lexTypeID, lex.Type())
	assert.Equal(t, "format", lex.Text())
	assert.Equal(t, 4, lex.Offset())

	// 4. End of file.
	_, err = lx.Next()
	assert.Equal(t, io.EOF, err)
}

func TestLexerProcessorRecreationWithNewRules(t *testing.T) {
	const (
		lexTypeA = textlexer.LexemeType("A")
		lexTypeB = textlexer.LexemeType("B")
	)

	input := "a a a b b b"
	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeA, matchString("a"))
	lx.MustAddRule("WS", newWhitespaceRule())

	// Process the first token to ensure the initial processor is created.
	lex, err := lx.Next()
	require.NoError(t, err)
	assert.Equal(t, lexTypeA, lex.Type())

	// Now, add a new rule. This should invalidate the internal processor.
	err = lx.AddRule(lexTypeB, matchString("b"))
	require.NoError(t, err)

	// Continue lexing. The lexer should seamlessly create a new processor
	// that includes the new rule for "b".
	var foundTypes []textlexer.LexemeType
	for {
		lex, err := lx.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		foundTypes = append(foundTypes, lex.Type())
	}

	// The first 'a' was consumed before the rule change.
	// The rest should be processed with the new rule set.
	expectedTypes := []textlexer.LexemeType{
		"WS",
		lexTypeA,
		"WS",
		lexTypeA,
		"WS",
		lexTypeB, // This verifies the new rule was active.
		"WS",
		lexTypeB,
		"WS",
		lexTypeB,
	}
	assert.Equal(t, expectedTypes, foundTypes)
}

// literalThen returns a Rule that continues over the runes of lit without
// accepting and hands the symbol after them to then. Any other symbol rejects.
func literalThen(lit string, then textlexer.Rule) textlexer.Rule {
	runes := []rune(lit)
	var step func(i int) textlexer.Rule
	step = func(i int) textlexer.Rule {
		if i == len(runes) {
			return then
		}
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() != runes[i] {
				return nil, textlexer.StateReject
			}
			return step(i + 1), textlexer.StateContinue
		}
	}
	return step(0)
}

// unknownLexemes returns one single-rune UNKNOWN lexeme for each rune of s, the
// first at rune offset offset.
func unknownLexemes(s string, offset int) []*textlexer.Lexeme {
	var lexemes []*textlexer.Lexeme
	for i, r := range []rune(s) {
		lexemes = append(lexemes, textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, string(r), offset+i))
	}
	return lexemes
}

// lexemesWith returns head followed by the single-rune UNKNOWN lexemes of rest,
// which starts right after head.
func lexemesWith(head *textlexer.Lexeme, rest string) []*textlexer.Lexeme {
	return append([]*textlexer.Lexeme{head}, unknownLexemes(rest, head.Offset()+head.Len())...)
}

type lookaheadCase struct {
	name     string
	input    string
	expected []*textlexer.Lexeme
}

// runLookaheadCases lexes each case's input with the rules that setup
// registers and compares type, text, and offset of every lexeme.
func runLookaheadCases(t *testing.T, cases []lookaheadCase, setup func(lx *textlexer.TextLexer)) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			setup(lx)
			assert.Equal(t, lexemeKeys(tc.expected), lexemeKeys(lexAll(t, lx)))
		})
	}
}

// TestLookaheadMultiSymbol: a rule reads four symbols past its match to decide,
// then excludes all four. "http" is a PROTOCOL only when "://" and a letter
// follow.
func TestLookaheadMultiSymbol(t *testing.T) {
	const lexTypeProtocol = textlexer.LexemeType("PROTOCOL")

	// The rule continues through "http://" and one more symbol. If that symbol
	// is an ASCII letter, four pushbacks exclude it and "://", and the rule
	// accepts "http". It never accepts before it has decided.
	protocolRule := literalThen("http://", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isLetter(s) {
			return excludeLast(4, textlexer.StateAccept)
		}
		return nil, textlexer.StateReject
	})

	runLookaheadCases(t, []lookaheadCase{
		{
			name:     "letter after scheme",
			input:    "http://example.com",
			expected: lexemesWith(textlexer.NewLexemeFromString(lexTypeProtocol, "http", 0), "://example.com"),
		},
		{
			name:     "non-letter after scheme",
			input:    "http://$",
			expected: unknownLexemes("http://$", 0),
		},
	}, func(lx *textlexer.TextLexer) {
		lx.MustAddRule(lexTypeProtocol, protocolRule)
	})
}

// TestLookaheadVariableLength: the number of symbols a rule excludes depends on
// the input. "https" is a PROTOCOL only when "://wikipedia.org/wiki/" and a
// non-empty page name follow.
func TestLookaheadVariableLength(t *testing.T) {
	const (
		lexTypeProtocol = textlexer.LexemeType("PROTOCOL")
		prefix          = "://wikipedia.org/wiki/"
	)

	isPageChar := func(s textlexer.Symbol) bool {
		r := s.Rune()
		return isLetter(s) || isDigit(s) || r == '_' || r == '-'
	}

	// page(n) has read n page-name characters; the count lives in the Rule
	// values, not in a shared variable. The rule scans the whole page name. On
	// the first other symbol, or the EOF symbol, it excludes that symbol, the
	// page name, and the prefix, and accepts "https".
	var page func(n int) textlexer.Rule
	page = func(n int) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if isPageChar(s) {
				return page(n + 1), textlexer.StateContinue
			}
			if n == 0 {
				return nil, textlexer.StateReject
			}
			return excludeLast(len(prefix)+n+1, textlexer.StateAccept)
		}
	}
	protocolRule := literalThen("https"+prefix, page(0))

	https := textlexer.NewLexemeFromString(lexTypeProtocol, "https", 0)

	runLookaheadCases(t, []lookaheadCase{
		{
			name:     "valid URL at EOF",
			input:    "https://wikipedia.org/wiki/Lexical_analysis",
			expected: lexemesWith(https, "://wikipedia.org/wiki/Lexical_analysis"),
		},
		{
			name:     "valid URL followed by a space",
			input:    "https://wikipedia.org/wiki/Go-2 next",
			expected: lexemesWith(https, "://wikipedia.org/wiki/Go-2 next"),
		},
		{
			name:     "non-Wikipedia URL",
			input:    "https://example.org/wiki/Go",
			expected: unknownLexemes("https://example.org/wiki/Go", 0),
		},
		{
			name:     "partial URL",
			input:    "https://wikipedia.org/wi",
			expected: unknownLexemes("https://wikipedia.org/wi", 0),
		},
		{
			name:     "empty page name at EOF",
			input:    "https://wikipedia.org/wiki/",
			expected: unknownLexemes("https://wikipedia.org/wiki/", 0),
		},
		{
			name:     "empty page name before a space",
			input:    "https://wikipedia.org/wiki/ x",
			expected: unknownLexemes("https://wikipedia.org/wiki/ x", 0),
		},
	}, func(lx *textlexer.TextLexer) {
		lx.MustAddRule(lexTypeProtocol, protocolRule)
	})
}

// TestLookaheadTwoCandidateEnds: one rule chooses between two match ends after
// it has seen what follows. After "abcd" and any number of '.', 'X' yields "ab"
// and 'Y' yields "abcd".
func TestLookaheadTwoCandidateEnds(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	// dots(n) has read n '.' symbols. 'X' excludes itself, the n dots, and
	// "cd". 'Y' excludes itself and the n dots. Any other symbol, or the EOF
	// symbol, rejects.
	var dots func(n int) textlexer.Rule
	dots = func(n int) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			switch s.Rune() {
			case '.':
				return dots(n + 1), textlexer.StateContinue
			case 'X':
				return excludeLast(n+3, textlexer.StateAccept)
			case 'Y':
				return excludeLast(n+1, textlexer.StateAccept)
			}
			return nil, textlexer.StateReject
		}
	}
	rule := literalThen("abcd", dots(0))

	ab := textlexer.NewLexemeFromString(lexTypeR, "ab", 0)
	abcd := textlexer.NewLexemeFromString(lexTypeR, "abcd", 0)

	runLookaheadCases(t, []lookaheadCase{
		{name: "X right after abcd", input: "abcdX", expected: lexemesWith(ab, "cdX")},
		{name: "X after dots", input: "abcd...X", expected: lexemesWith(ab, "cd...X")},
		{name: "Y right after abcd", input: "abcdY", expected: lexemesWith(abcd, "Y")},
		{name: "Y after dots", input: "abcd..Y", expected: lexemesWith(abcd, "..Y")},
		{name: "other symbol", input: "abcd..Z", expected: unknownLexemes("abcd..Z", 0)},
		{name: "end of input", input: "abcd", expected: unknownLexemes("abcd", 0)},
	}, func(lx *textlexer.TextLexer) {
		lx.MustAddRule(lexTypeR, rule)
	})
}

// TestLookaheadKeywordBoundary: a keyword rule with no identifier rule beside
// it decides by one symbol of lookahead. "for" matches before a non-identifier
// symbol and at end of input, and not inside "format".
func TestLookaheadKeywordBoundary(t *testing.T) {
	const lexTypeFor = textlexer.LexemeType("FOR")

	// After "for", an identifier character rejects. Any other symbol, the EOF
	// symbol included, is excluded by one pushback and "for" is accepted.
	forRule := literalThen("for", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isLetter(s) || isDigit(s) || s.Rune() == '_' {
			return nil, textlexer.StateReject
		}
		return excludeLast(1, textlexer.StateAccept)
	})

	keyword := textlexer.NewLexemeFromString(lexTypeFor, "for", 0)

	runLookaheadCases(t, []lookaheadCase{
		{name: "before a parenthesis", input: "for (", expected: lexemesWith(keyword, " (")},
		{name: "at end of input", input: "for", expected: []*textlexer.Lexeme{keyword}},
		{name: "inside a longer word", input: "format", expected: unknownLexemes("format", 0)},
	}, func(lx *textlexer.TextLexer) {
		lx.MustAddRule(lexTypeFor, forRule)
	})
}

// TestLookaheadFloatVersusRange: newUnsignedFloatRule decides by lookahead
// whether "1." is a float. A second '.' makes it reject, so the integer rule
// wins "1" in a range such as "1..5".
func TestLookaheadFloatVersusRange(t *testing.T) {
	const (
		lexTypeInteger = textlexer.LexemeType("INT")
		lexTypeFloat   = textlexer.LexemeType("FLOAT")
	)

	cases := []lookaheadCase{
		{
			name:  "trailing radix before a space",
			input: "1. ",
			expected: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "1.", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, " ", 2),
			},
		},
		{
			name:  "range",
			input: "1..5",
			expected: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "1", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, ".", 1),
				textlexer.NewLexemeFromString(lexTypeFloat, ".5", 2),
			},
		},
		{
			name:  "float",
			input: "1.5",
			expected: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "1.5", 0),
			},
		},
	}

	t.Run("float registered first", func(t *testing.T) {
		runLookaheadCases(t, cases, func(lx *textlexer.TextLexer) {
			lx.MustAddRule(lexTypeFloat, newUnsignedFloatRule())
			lx.MustAddRule(lexTypeInteger, newUnsignedIntegerRule())
		})
	})
	t.Run("integer registered first", func(t *testing.T) {
		runLookaheadCases(t, cases, func(lx *textlexer.TextLexer) {
			lx.MustAddRule(lexTypeInteger, newUnsignedIntegerRule())
			lx.MustAddRule(lexTypeFloat, newUnsignedFloatRule())
		})
	})
}

// TestPushBackSymbolOrder: each pushback in a chain shows `next` the next symbol
// down the stack, starting with the symbol just read. It also shows the count
// rule: a helper that issues two pushbacks, entered through one more
// StatePushBack, excludes three symbols.
func TestPushBackSymbolOrder(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	var seen []rune
	// record(k) records the symbol it is shown, then pushes back k more times.
	var record func(k int) textlexer.Rule
	record = func(k int) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			seen = append(seen, s.Rune())
			if k == 0 {
				return nil, textlexer.StateAccept
			}
			return record(k - 1), textlexer.StatePushBack
		}
	}
	rule := literalThen("abc", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() != 'd' {
			return nil, textlexer.StateReject
		}
		return record(2), textlexer.StatePushBack
	})

	lx := textlexer.New(strings.NewReader("abcd"))
	lx.MustAddRule(lexTypeR, rule)

	expected := lexemesWith(textlexer.NewLexemeFromString(lexTypeR, "a", 0), "bcd")
	assert.Equal(t, lexemeKeys(expected), lexemeKeys(lexAll(t, lx)))
	assert.Equal(t, []rune{'d', 'c', 'b'}, seen, "the chain must see the stack from the top down")
}

// TestPushBackIsPerRule: one rule's pushback does not change what another rule
// is shown. The stack is the same for every rule within a step.
func TestPushBackIsPerRule(t *testing.T) {
	const (
		lexTypePush = textlexer.LexemeType("PUSH")
		lexTypeSeen = textlexer.LexemeType("SEEN")
	)

	// PUSH reads "ab" and, on 'c', excludes 'c' and 'b' and accepts "a".
	pushRule := literalThen("ab", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() != 'c' {
			return nil, textlexer.StateReject
		}
		return excludeLast(2, textlexer.StateAccept)
	})

	// SEEN runs after PUSH in each step. It records every symbol it is shown and
	// accepts every letter.
	var seen []rune
	var seenRule textlexer.Rule
	seenRule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		seen = append(seen, s.Rune())
		if isLetter(s) {
			return seenRule, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}

	lx := textlexer.New(strings.NewReader("abc"))
	lx.MustAddRule(lexTypePush, pushRule)
	lx.MustAddRule(lexTypeSeen, seenRule)

	lex, err := lx.Next()
	require.NoError(t, err)
	assert.Equal(t, lexemeKeys([]*textlexer.Lexeme{textlexer.NewLexemeFromString(lexTypeSeen, "abc", 0)}),
		lexemeKeys([]*textlexer.Lexeme{lex}))
	assert.Equal(t, []rune{'a', 'b', 'c', textlexer.RuneEOF}, seen,
		"PUSH's chain on 'c' must not change the symbols SEEN is shown")

	_, err = lx.Next()
	assert.Equal(t, io.EOF, err)
}

// TestPushBackAtEOF: at end of input the stack also holds the EOF symbol, and
// the first pushback removes it. On input "a", one pushback leaves "a"; two are a
// zero-length match; three pass the bottom of the stack. Two and three give the
// same lexeme here; TestProcessPushBackAtEOF tells them apart.
func TestPushBackAtEOF(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	for _, tc := range []struct {
		pushbacks int
		expected  *textlexer.Lexeme
	}{
		{1, textlexer.NewLexemeFromString(lexTypeR, "a", 0)},
		{2, textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0)},
		{3, textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0)},
	} {
		t.Run(fmt.Sprintf("%d pushbacks", tc.pushbacks), func(t *testing.T) {
			rule := literalThen("a", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if !s.IsEOF() {
					return nil, textlexer.StateReject
				}
				return excludeLast(tc.pushbacks, textlexer.StateAccept)
			})

			lx := textlexer.New(strings.NewReader("a"))
			lx.MustAddRule(lexTypeR, rule)
			assert.Equal(t, lexemeKeys([]*textlexer.Lexeme{tc.expected}), lexemeKeys(lexAll(t, lx)))
		})
	}
}

// TestPushBackRetiresRuleKeepsEarlierAccept: a pushback past the bottom of the
// stack, or (nil, StatePushBack), retires the rule. The accept it recorded
// earlier stays.
func TestPushBackRetiresRuleKeepsEarlierAccept(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	// more accepts every real symbol. It is reached only if the VM does not
	// retire the rule, and then the rule would match all of "abc".
	var more textlexer.Rule
	more = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		return more, textlexer.StateAccept
	}

	// over(k) pushes back k more times and then hands the rule to more.
	var over func(k int) textlexer.Rule
	over = func(k int) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if k == 0 {
				return more, textlexer.StateAccept
			}
			return over(k - 1), textlexer.StatePushBack
		}
	}

	for _, tc := range []struct {
		name string
		onB  textlexer.Rule
	}{
		{
			// On 'b' the stack holds two symbols, and the chain issues three
			// pushbacks.
			name: "pushback past the bottom of the stack",
			onB: func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return over(2), textlexer.StatePushBack
			},
		},
		{
			name: "nil next with StatePushBack",
			onB: func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return nil, textlexer.StatePushBack
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() != 'a' {
					return nil, textlexer.StateReject
				}
				return tc.onB, textlexer.StateAccept
			}

			lx := textlexer.New(strings.NewReader("abc"))
			lx.MustAddRule(lexTypeR, rule)

			expected := lexemesWith(textlexer.NewLexemeFromString(lexTypeR, "a", 0), "bc")
			assert.Equal(t, lexemeKeys(expected), lexemeKeys(lexAll(t, lx)))
		})
	}
}

// TestPushBackChainEndingContinue pins the v0.2.3 result of an unsupported chain
// ending: a chain that ends in StateContinue. The VM keeps the pushback count
// for the current symbol step only, so the rule stays active and the excluded
// symbol is counted back in by later accepts. A chain must end in
// (nil, StateAccept) or StateReject; see KD-5 in docs/specs/core-contract.md.
func TestPushBackChainEndingContinue(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	// tail accepts every real symbol and rejects the EOF symbol.
	var tail textlexer.Rule
	tail = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		return tail, textlexer.StateAccept
	}

	// The rule reads "ab" and, on the real symbol 'c', pushes 'c' back and
	// continues into tail. The pushback is forgotten when the step ends.
	rule := literalThen("ab", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return tail, textlexer.StateContinue
		}, textlexer.StatePushBack
	})

	lx := textlexer.New(strings.NewReader("abcde"))
	lx.MustAddRule(lexTypeR, rule)

	expected := []*textlexer.Lexeme{textlexer.NewLexemeFromString(lexTypeR, "abcde", 0)}
	assert.Equal(t, lexemeKeys(expected), lexemeKeys(lexAll(t, lx)))
}

// TestPushBackChainEndingAcceptWithNext pins the v0.2.3 result of the other
// unsupported chain ending: (non-nil next, StateAccept). The VM records the
// shorter length, keeps the rule active, and drops the pushback count when the
// step ends, so a later accept counts the excluded symbol back in. The
// supported ending, (nil, StateAccept), is shown for contrast. See KD-5 in
// docs/specs/core-contract.md.
func TestPushBackChainEndingAcceptWithNext(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	// more accepts every real symbol and rejects the EOF symbol.
	var more textlexer.Rule
	more = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		return more, textlexer.StateAccept
	}

	// The rule continues at 'a' and, on the real symbol 'X', pushes 'X' back
	// and accepts with next.
	ruleWithNext := func(next textlexer.Rule) textlexer.Rule {
		return literalThen("a", func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() != 'X' {
				return nil, textlexer.StateReject
			}
			return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return next, textlexer.StateAccept
			}, textlexer.StatePushBack
		})
	}

	runLookaheadCases(t, []lookaheadCase{
		{
			name:     "unsupported: non-nil next",
			input:    "aXbc",
			expected: []*textlexer.Lexeme{textlexer.NewLexemeFromString(lexTypeR, "aXbc", 0)},
		},
	}, func(lx *textlexer.TextLexer) {
		lx.MustAddRule(lexTypeR, ruleWithNext(more))
	})

	runLookaheadCases(t, []lookaheadCase{
		{
			name:     "supported: nil next",
			input:    "aXbc",
			expected: lexemesWith(textlexer.NewLexemeFromString(lexTypeR, "a", 0), "Xbc"),
		},
	}, func(lx *textlexer.TextLexer) {
		lx.MustAddRule(lexTypeR, ruleWithNext(nil))
	})
}

// assertRetireStream lexes all of lx and compares each lexeme's type, text, rune
// offset, and byte span with expected. The inputs are ASCII, so an expected
// lexeme's byte span is [offset, offset+len). lexAll fails the test on any
// error, so the stream must end in io.EOF.
func assertRetireStream(t *testing.T, lx *textlexer.TextLexer, expected []*textlexer.Lexeme) {
	t.Helper()

	key := func(typ textlexer.LexemeType, text string, offset, start, end int) string {
		return fmt.Sprintf("%s|%q|%d|%d-%d", typ, text, offset, start, end)
	}

	want := make([]string, len(expected))
	for i, lex := range expected {
		want[i] = key(lex.Type(), lex.Text(), lex.Offset(), lex.Offset(), lex.Offset()+lex.Len())
	}

	found := lexAll(t, lx)
	got := make([]string, len(found))
	for i, lex := range found {
		got[i] = key(lex.Type(), lex.Text(), lex.Offset(), lex.Span().Start().ByteOffset(), lex.Span().End().ByteOffset())
	}

	assert.Equal(t, want, got)

	_, err := lx.Next()
	assert.Equal(t, io.EOF, err, "lexing must stay at io.EOF")
}

// newLettersRule matches a run of ASCII letters. It accepts every letter and
// rejects any other symbol, the EOF symbol included, so it settles at end of
// input.
func newLettersRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isLetter(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return loop
}

// TestInvalidStateRetiresRule: a State outside the four constants retires only
// the rule that returned it, as StateReject does. The VM does not panic, and a
// sibling rule's match is emitted. See KD-1 in docs/specs/core-contract.md.
func TestInvalidStateRetiresRule(t *testing.T) {
	const (
		lexTypeWord = textlexer.LexemeType("WORD")
		lexTypeBad  = textlexer.LexemeType("BAD")
		invalid     = textlexer.State(9)
	)

	// direct continues on its first symbol and returns the invalid State on
	// its second.
	direct := func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return nil, invalid
		}, textlexer.StateContinue
	}

	// chained continues on its first symbol. On its second it starts a
	// pushback chain whose last step returns the invalid State, so the VM
	// meets the value at the end of the chain, not as a direct verdict.
	chained := func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				return nil, invalid
			}, textlexer.StatePushBack
		}, textlexer.StateContinue
	}

	for _, tc := range []struct {
		name string
		bad  textlexer.Rule
	}{
		{"direct", direct},
		{"end of pushback chain", chained},
	} {
		t.Run(tc.name+", with a sibling", func(t *testing.T) {
			// BAD is registered first, so it steps before WORD on each symbol.
			lx := textlexer.New(strings.NewReader("ab"))
			lx.MustAddRule(lexTypeBad, tc.bad)
			lx.MustAddRule(lexTypeWord, newLettersRule())

			assertRetireStream(t, lx, []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
			})
		})

		t.Run(tc.name+", only rule", func(t *testing.T) {
			// BAD never accepts, so each token is one UNKNOWN rune. On the
			// last rune its second symbol is the EOF symbol.
			lx := textlexer.New(strings.NewReader("ab"))
			lx.MustAddRule(lexTypeBad, tc.bad)

			assertRetireStream(t, lx, unknownLexemes("ab", 0))
		})
	}
}

// TestAcceptOfEOFSymbolIsDiscarded: an accept whose length counts the synthetic
// EOF symbol is longer than the symbols read. The VM discards that verdict and
// retires the rule; an earlier accept by the same rule stays, and a sibling
// rule's match is emitted. An accept whose pushback chain excludes the EOF
// symbol is valid. See KD-3 in docs/specs/core-contract.md.
func TestAcceptOfEOFSymbolIsDiscarded(t *testing.T) {
	const (
		lexTypeWord = textlexer.LexemeType("WORD")
		lexTypeR    = textlexer.LexemeType("R")
	)

	t.Run("sibling", func(t *testing.T) {
		// continueThenAcceptEOF continues on every real symbol and accepts the
		// EOF symbol. It records no other accept.
		var continueThenAcceptEOF textlexer.Rule
		continueThenAcceptEOF = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.IsEOF() {
				return nil, textlexer.StateAccept
			}
			return continueThenAcceptEOF, textlexer.StateContinue
		}

		// R is registered first. Its discarded accept of three symbols would
		// otherwise be the longest.
		lx := textlexer.New(strings.NewReader("ab"))
		lx.MustAddRule(lexTypeR, continueThenAcceptEOF)
		lx.MustAddRule(lexTypeWord, newLettersRule())

		assertRetireStream(t, lx, []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
		})
	})

	t.Run("earlier accept by the same rule", func(t *testing.T) {
		// acceptAll accepts every letter and also the EOF symbol. Its accept
		// of "ab" is recorded; its accept of the EOF symbol is discarded.
		var acceptAll textlexer.Rule
		acceptAll = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.IsEOF() {
				return nil, textlexer.StateAccept
			}
			if isLetter(s) {
				return acceptAll, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader("ab"))
		lx.MustAddRule(lexTypeR, acceptAll)

		assertRetireStream(t, lx, []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeR, "ab", 0),
		})
	})

	// The regression guards: a pushback chain at EOF that excludes the EOF
	// symbol accepts the real symbols. With a nil next the rule stops; with a
	// non-nil next the accept is recorded and the rule is retired, because no
	// input follows the EOF symbol.
	for _, tc := range []struct {
		name string
		next func(self textlexer.Rule) textlexer.Rule
	}{
		{"pushback excludes the EOF symbol, nil next", func(textlexer.Rule) textlexer.Rule { return nil }},
		{"pushback excludes the EOF symbol, non-nil next", func(self textlexer.Rule) textlexer.Rule { return self }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rule textlexer.Rule
			rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.IsEOF() {
					return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
						return tc.next(rule), textlexer.StateAccept
					}, textlexer.StatePushBack
				}
				if isLetter(s) {
					return rule, textlexer.StateContinue
				}
				return nil, textlexer.StateReject
			}

			lx := textlexer.New(strings.NewReader("ab"))
			lx.MustAddRule(lexTypeR, rule)

			assertRetireStream(t, lx, []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeR, "ab", 0),
			})
		})
	}
}

// TestStillActiveAtEOFRetiresRule: after the EOF symbol every rule that is still
// active is retired, and the winner is chosen from the recorded accepts. A
// sibling that never settles does not fail the token. See KD-4 in
// docs/specs/core-contract.md; the single never-settling rule is the
// TestLexerProcessor case "Rule Never Settles (UNKNOWN per rune)".
func TestStillActiveAtEOFRetiresRule(t *testing.T) {
	const (
		lexTypeWord = textlexer.LexemeType("WORD")
		lexTypeSib  = textlexer.LexemeType("SIB")
	)

	// alwaysContinue returns itself with StateContinue on every symbol, the EOF
	// symbol included. It records no accept.
	var alwaysContinue textlexer.Rule
	alwaysContinue = func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return alwaysContinue, textlexer.StateContinue
	}

	// alwaysAccept returns itself with StateAccept on every symbol. It records
	// the accept of "ab"; its accept of the EOF symbol is discarded.
	var alwaysAccept textlexer.Rule
	alwaysAccept = func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return alwaysAccept, textlexer.StateAccept
	}

	t.Run("StateContinue sibling", func(t *testing.T) {
		lx := textlexer.New(strings.NewReader("ab"))
		lx.MustAddRule(lexTypeSib, alwaysContinue)
		lx.MustAddRule(lexTypeWord, newLettersRule())

		assertRetireStream(t, lx, []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
		})
	})

	t.Run("(self, StateAccept) sibling", func(t *testing.T) {
		// SIB also records "ab", so the two rules tie and WORD, registered
		// first, wins.
		lx := textlexer.New(strings.NewReader("ab"))
		lx.MustAddRule(lexTypeWord, newLettersRule())
		lx.MustAddRule(lexTypeSib, alwaysAccept)

		assertRetireStream(t, lx, []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
		})
	})

	t.Run("(self, StateAccept) sibling registered first keeps its accept", func(t *testing.T) {
		// Registered first, SIB wins the tie with the accept it recorded
		// before the EOF symbol.
		lx := textlexer.New(strings.NewReader("ab"))
		lx.MustAddRule(lexTypeSib, alwaysAccept)
		lx.MustAddRule(lexTypeWord, newLettersRule())

		assertRetireStream(t, lx, []*textlexer.Lexeme{
			textlexer.NewLexemeFromString(lexTypeSib, "ab", 0),
		})
	})
}

// alwaysPanic panics on every symbol it is shown.
func alwaysPanic(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	panic("rule panicked")
}

// TestPanicRetiresRule: a rule that panics while it processes a symbol is
// retired for the current token, as StateReject does, whether it panics on a
// direct call or as the next rule of a pushback chain. The panic does not
// reach the caller, Next returns no error, and a sibling rule's match is
// emitted. See KD-2 in docs/specs/core-contract.md.
func TestPanicRetiresRule(t *testing.T) {
	const (
		lexTypeWord = textlexer.LexemeType("WORD")
		lexTypeBad  = textlexer.LexemeType("BAD")
	)

	// direct continues on its first symbol and panics on its second.
	direct := func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return alwaysPanic, textlexer.StateContinue
	}

	// chained continues on its first symbol. On its second it pushes back,
	// and the next rule of the chain panics on the pushed-back symbol.
	chained := func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return alwaysPanic, textlexer.StatePushBack
		}, textlexer.StateContinue
	}

	for _, tc := range []struct {
		name string
		bad  textlexer.Rule
	}{
		{"direct", direct},
		{"in a pushback chain", chained},
	} {
		t.Run(tc.name+", with a sibling", func(t *testing.T) {
			// BAD is registered first, so it steps before WORD on each symbol.
			lx := textlexer.New(strings.NewReader("ab"))
			lx.MustAddRule(lexTypeBad, tc.bad)
			lx.MustAddRule(lexTypeWord, newLettersRule())

			assertRetireStream(t, lx, []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
			})
		})

		t.Run(tc.name+", sibling registered first", func(t *testing.T) {
			lx := textlexer.New(strings.NewReader("ab"))
			lx.MustAddRule(lexTypeWord, newLettersRule())
			lx.MustAddRule(lexTypeBad, tc.bad)

			assertRetireStream(t, lx, []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
			})
		})

		t.Run(tc.name+", only rule", func(t *testing.T) {
			// BAD never accepts, so each token is one UNKNOWN rune. On the
			// last rune its second symbol is the EOF symbol.
			lx := textlexer.New(strings.NewReader("ab"))
			lx.MustAddRule(lexTypeBad, tc.bad)

			assertRetireStream(t, lx, unknownLexemes("ab", 0))
		})
	}
}

// TestPanicKeepsEarlierAccept: a rule accepts "a" and then panics on the next
// symbol, directly or in a pushback chain. It has no sibling. Its earlier
// accept stands, as for StateReject, so the first lexeme is that rule's "a".
// The next token starts fresh: the rule rejects "b", which becomes UNKNOWN,
// and then accepts the last "a" and panics on the EOF symbol.
func TestPanicKeepsEarlierAccept(t *testing.T) {
	const lexTypeA = textlexer.LexemeType("A")

	panicNext := map[string]textlexer.Rule{
		"direct": alwaysPanic,
		"in a pushback chain": func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return alwaysPanic, textlexer.StatePushBack
		},
	}

	for name, next := range panicNext {
		t.Run(name, func(t *testing.T) {
			acceptThenPanic := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == 'a' {
					return next, textlexer.StateAccept
				}
				return nil, textlexer.StateReject
			}

			lx := textlexer.New(strings.NewReader("aba"))
			lx.MustAddRule(lexTypeA, acceptThenPanic)

			assertRetireStream(t, lx, []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeA, "a", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
				textlexer.NewLexemeFromString(lexTypeA, "a", 2),
			})
		})
	}
}

// TestPanicLeavesLexerUsable is the KD-2 probe from docs/specs/core-contract.md,
// inverted. The input is longer than the cursor's retention budget
// (WithMaxTokenBytes(8) gives 4 × 8 = 32 bytes), and the WORD rule panics once,
// on the third symbol of the first token. The panic leaves no orphaned
// checkpoint and loses no symbol: no later Next fails, and every input byte
// appears in exactly one lexeme, in order.
func TestPanicLeavesLexerUsable(t *testing.T) {
	const (
		lexTypeWord = textlexer.LexemeType("WORD")
		lexTypeWS   = textlexer.LexemeType("WS")
	)

	input := "abcd " + strings.Repeat("ab ", 40)

	panicked := false
	var word func(depth int) textlexer.Rule
	word = func(depth int) textlexer.Rule {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if depth == 2 && !panicked {
				panicked = true
				panic("third symbol")
			}
			if isLetter(s) {
				return word(depth + 1), textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
	}

	lx := textlexer.NewWithOptions(strings.NewReader(input), textlexer.WithMaxTokenBytes(8))
	lx.MustAddRule(lexTypeWord, word(0))
	lx.MustAddRule(lexTypeWS, newWhitespaceRule())

	// WORD recorded "ab" before it panicked on "c", so its accept stands. The
	// next token starts at "c".
	expected := []*textlexer.Lexeme{
		textlexer.NewLexemeFromString(lexTypeWord, "ab", 0),
		textlexer.NewLexemeFromString(lexTypeWord, "cd", 2),
		textlexer.NewLexemeFromString(lexTypeWS, " ", 4),
	}
	for i := 0; i < 40; i++ {
		expected = append(expected,
			textlexer.NewLexemeFromString(lexTypeWord, "ab", 5+3*i),
			textlexer.NewLexemeFromString(lexTypeWS, " ", 7+3*i),
		)
	}

	assertRetireStream(t, lx, expected)
	assert.True(t, panicked, "the WORD rule must have panicked")
}

// TestPanickingRuleConcurrentDeterminism: a rule that panics in every
// identifier token does not break concurrent Next calls. Concurrent callers
// interleave tokens, so, as in TestLexerConcurrentDeterminism, the lexemes are
// collected in any order and sorted by offset to restore the stream order
// before the full sequence is compared. Run with -race.
func TestPanickingRuleConcurrentDeterminism(t *testing.T) {
	const (
		lexTypeBoom = textlexer.LexemeType("BOOM")
		lexTypeID   = textlexer.LexemeType("ID")
		lexTypeWS   = textlexer.LexemeType("WS")
		numTokens   = 100
	)

	var b strings.Builder
	var expected []*textlexer.Lexeme
	for i := 0; i < numTokens; i++ {
		id := fmt.Sprintf("id%d", i)
		expected = append(expected,
			textlexer.NewLexemeFromString(lexTypeID, id, b.Len()),
			textlexer.NewLexemeFromString(lexTypeWS, " ", b.Len()+len(id)),
		)
		b.WriteString(id + " ")
	}
	input := b.String()

	for run := 0; run < 3; run++ {
		// BOOM accepts letters and panics on the first digit, so it panics
		// once in every identifier token. Its accept of "id" stands, but ID's
		// longer match wins.
		var panics atomic.Int64
		var boom textlexer.Rule
		boom = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if isDigit(s) {
				panics.Add(1)
				panic("digit")
			}
			if isLetter(s) {
				return boom, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeBoom, boom)
		lx.MustAddRule(lexTypeID, identifierRule)
		lx.MustAddRule(lexTypeWS, whitespaceRule)

		var (
			mu     sync.Mutex
			tokens []*textlexer.Lexeme
			errs   []error
			wg     sync.WaitGroup
		)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					lex, err := lx.Next()
					if err == io.EOF {
						return
					}
					mu.Lock()
					if err != nil {
						errs = append(errs, err)
						mu.Unlock()
						return
					}
					tokens = append(tokens, lex)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()

		require.Empty(t, errs, "run %d", run)
		sort.Slice(tokens, func(i, j int) bool {
			return tokens[i].Offset() < tokens[j].Offset()
		})
		assert.Equal(t, lexemeKeys(expected), lexemeKeys(tokens), "run %d", run)
		assert.Equal(t, int64(numTokens), panics.Load(), "run %d", run)
	}
}
