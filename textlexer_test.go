// LLM Programming Directive:
//
// Objective:
// This file provides a comprehensive, self-contained, and robust test suite for the `textlexer` package.
// Its primary purpose is to validate the correctness and resilience of the lexer's state machine.
//
// Core Abstractions for This File:
// - `Rule` (textlexer.Rule): A function `func(rune) (Rule, State)` representing a state in a state machine.
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
// - Resource Management: Dynamic buffer growth and memory compaction.
// - Concurrency: Goroutine safety of `Next()` and `AddRule()` (verified with the -race detector).
// - Error Handling: Propagation of reader errors and recovery from panics in user-provided rules.
// - Pathological Rules: A dedicated suite (`TestLexerPathologicalRules`) for complex, malicious,
//   or badly-behaved rules designed to stress the state machine's logic.
//
// Common Patterns and Helpers:
// - `matchString(s)`: Creates a rule that matches an exact string.
// - `matchAnyOf(ss...)`: Creates a composite rule that matches any of the provided strings.
// - `backtrack(n, state)`: Simulates n pushbacks before returning the given state.
// - Table-driven tests in `TestLexerProcessor` for basic functionality.
// - Adversarial sub-tests in `TestLexerPathologicalRules`.
//
// Known Constraints:
// - Each `LexemeType` can only have one rule associated with it.
// - Rules must handle EOF gracefully (no inconclusive states).
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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
)

// backtrack is a test stub that simulates a real backtrack function for state signaling.
func backtrack(n int, s textlexer.State) textlexer.Rule {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if n > 0 {
			return backtrack(n-1, s), textlexer.StatePushBack
		}
		return nil, s
	}
}

// matchString is a test helper that creates a simple Rule to match an exact string.
func matchString(s string) textlexer.Rule {
	var ruleFor func(sub string) textlexer.Rule
	ruleFor = func(sub string) textlexer.Rule {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			runes := []rune(sub)
			if r != runes[0] {
				return nil, textlexer.StateReject
			}
			if len(runes) == 1 {
				return nil, textlexer.StateAccept
			}
			return ruleFor(string(runes[1:])), textlexer.StateContinue
		}
	}
	if s == "" {
		return func(r rune) (textlexer.Rule, textlexer.State) {
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

	return func(r rune) (textlexer.Rule, textlexer.State) {
		var activeScanners []*scanner
		var overallState textlexer.State = textlexer.StateReject

		for _, s := range ss {
			if len(s) > 0 && rune(s[0]) == r {
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
		nextCompositeRule = func(nextRune rune) (textlexer.Rule, textlexer.State) {
			var nextActiveScanners []*scanner
			var nextOverallState textlexer.State = textlexer.StateReject

			for _, sc := range activeScanners {
				nextRule, state := sc.rule(nextRune)
				if state != textlexer.StateReject {
					nextActiveScanners = append(nextActiveScanners, &scanner{rule: nextRule})
					if state == textlexer.StateAccept {
						nextOverallState = textlexer.StateAccept
					} else if nextOverallState != textlexer.StateAccept {
						nextOverallState = textlexer.StateContinue
					}
				}
			}

			if len(nextActiveScanners) == 0 {
				return nil, textlexer.StateReject
			}
			activeScanners = nextActiveScanners
			return nextCompositeRule, nextOverallState
		}

		return nextCompositeRule, overallState
	}
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func newWhitespaceRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(r rune) (textlexer.Rule, textlexer.State) {
		if isWhitespace(r) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if isWhitespace(r) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newUnsignedIntegerRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newIdentifierRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(r rune) (textlexer.Rule, textlexer.State) {
		if isLetter(r) || isDigit(r) || r == '_' {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if isLetter(r) || r == '_' {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newUnsignedFloatRule() textlexer.Rule {
	var start, integerPart, afterInitialRadix, afterIntegerRadix, fractionalPart textlexer.Rule

	pushBackAndAccept := func(_ rune) (textlexer.Rule, textlexer.State) {
		return backtrack(1, textlexer.StateAccept), textlexer.StatePushBack
	}

	// State for matching digits after a decimal point.
	fractionalPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return fractionalPart, textlexer.StateAccept
		}
		return pushBackAndAccept(r)
	}

	// State after a radix point that followed an integer part (e.g., after "123.").
	afterIntegerRadix = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return fractionalPart, textlexer.StateAccept
		}
		return pushBackAndAccept(r)
	}

	// State after a radix point was the *first* character. A fractional part is required.
	afterInitialRadix = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return fractionalPart, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}

	// State for matching the initial integer part. This is NOT a float yet.
	integerPart = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return integerPart, textlexer.StateContinue
		}
		if r == '.' {
			return afterIntegerRadix, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}

	start = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return integerPart, textlexer.StateContinue
		}
		if r == '.' {
			return afterInitialRadix, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	return start
}

func newSignedIntegerRule() textlexer.Rule {
	var start, loop textlexer.Rule
	loop = func(r rune) (textlexer.Rule, textlexer.State) {
		if isDigit(r) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	start = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '+' || r == '-' {
			// A sign must be followed by at least one digit.
			return loop, textlexer.StateContinue
		}
		if isDigit(r) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return start
}

func newSymbolRule() textlexer.Rule {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		switch r {
		case '+', '-', '*', '/', '=', ';', '(', ')':
			return nil, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

func newSingleQuotedStringRule() textlexer.Rule {
	var loop, afterQuote textlexer.Rule
	loop = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\'' {
			return afterQuote, textlexer.StateAccept
		}
		// Note: This simple version doesn't handle escaped quotes.
		return loop, textlexer.StateContinue
	}
	afterQuote = func(r rune) (textlexer.Rule, textlexer.State) {
		return nil, textlexer.StateReject
	}
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '\'' {
			return loop, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

func newSlashStarCommentRule() textlexer.Rule {
	var inComment, afterStar textlexer.Rule
	inComment = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '*' {
			return afterStar, textlexer.StateContinue
		}
		return inComment, textlexer.StateContinue
	}
	afterStar = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '/' {
			return nil, textlexer.StateAccept
		}
		return inComment, textlexer.StateContinue
	}
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '/' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if r == '*' {
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
				textlexer.NewLexeme(lexTypeWhitespace, "  ", 0),
				textlexer.NewLexeme(lexTypeFloat, "12.3", 2),
				textlexer.NewLexeme(lexTypeWhitespace, "  \t   ", 6),
				textlexer.NewLexeme(lexTypeInteger, "4", 12),
				textlexer.NewLexeme(lexTypeWhitespace, " ", 13),
				textlexer.NewLexeme(lexTypeFloat, "5.6", 14),
				textlexer.NewLexeme(lexTypeWhitespace, " \t ", 17),
				textlexer.NewLexeme(lexTypeInteger, "7", 20),
				textlexer.NewLexeme(lexTypeWhitespace, "\n ", 21),
				textlexer.NewLexeme(lexTypeInteger, "8", 23),
				textlexer.NewLexeme(lexTypeWhitespace, " ", 24),
				textlexer.NewLexeme(lexTypeFloat, "9.0", 25),
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
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 1),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "c", 2),
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
				textlexer.NewLexeme(lexTypeFloat, "12.345", 0),
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
				textlexer.NewLexeme(lexTypeFloat, "42.", 0),
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
				textlexer.NewLexeme(lexTypeInteger, "42", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 2),
			},
		},
		{
			name:  "Tokens immediately at EOF",
			input: "123",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(lexTypeInteger, "123", 0),
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
				textlexer.NewLexeme(lexTypeKeywordIF, "if", 0),
				textlexer.NewLexeme(lexTypeWhitespace, " ", 2),
				textlexer.NewLexeme(lexTypeIdentifier, "identifier", 3),
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
				textlexer.NewLexeme(lexTypeIdentifier, "if", 0),
			},
		},
		{
			name:  "Unicode Characters",
			input: "123\n世界456",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeInteger, unsignedIntegerRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(lexTypeInteger, "123", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "\n", 3),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "世", 4),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "界", 5),
				textlexer.NewLexeme(lexTypeInteger, "456", 6),
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
				textlexer.NewLexeme(lexTypeFloat, "1.2", 0),
				textlexer.NewLexeme(lexTypeFloat, ".3", 3),
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
				textlexer.NewLexeme(textlexer.LexemeType("SIGNED_INT"), "-12", 0),
				textlexer.NewLexeme(textlexer.LexemeType("SYMBOL"), "+", 3),
				textlexer.NewLexeme(textlexer.LexemeType("SIGNED_INT"), "-3", 4),
				textlexer.NewLexeme(textlexer.LexemeType("SYMBOL"), "+", 6),
				textlexer.NewLexeme(textlexer.LexemeType("SIGNED_INT"), "+4", 7),
				textlexer.NewLexeme(textlexer.LexemeType("SIGNED_INT"), "-5", 9),
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
				textlexer.NewLexeme(textlexer.LexemeType("SELECT"), "SELECT", 0),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 6),
				textlexer.NewLexeme(textlexer.LexemeType("SYMBOL"), "*", 7),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 8),
				textlexer.NewLexeme(textlexer.LexemeType("FROM"), "FROM", 9),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 13),
				textlexer.NewLexeme(textlexer.LexemeType("IDENTIFIER"), "users", 14),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 19),
				textlexer.NewLexeme(textlexer.LexemeType("WHERE"), "WHERE", 20),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 25),
				textlexer.NewLexeme(textlexer.LexemeType("IDENTIFIER"), "name", 26),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 30),
				textlexer.NewLexeme(textlexer.LexemeType("SYMBOL"), "=", 31),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 32),
				textlexer.NewLexeme(textlexer.LexemeType("STRING"), "'John Doe'", 33),
				textlexer.NewLexeme(textlexer.LexemeType("WHITESPACE"), " ", 43),
				textlexer.NewLexeme(textlexer.LexemeType("COMMENT"), "/* a comment */", 44),
				textlexer.NewLexeme(textlexer.LexemeType("SYMBOL"), ";", 59),
			},
		},
		{
			name:          "No Rules Configured",
			input:         "abc",
			setupRules:    func(lx *textlexer.TextLexer) {},
			expectedError: "rules",
		},
		{
			name:  "Malformed Rule (Infinite Loop)",
			input: "aaaa",
			setupRules: func(lx *textlexer.TextLexer) {
				// This rule always continues without consuming input, which can cause an infinite loop.
				var brokenRule textlexer.Rule
				brokenRule = func(r rune) (textlexer.Rule, textlexer.State) {
					return brokenRule, textlexer.StateContinue
				}
				lx.MustAddRule(lexTypeLoop, brokenRule)
			},
			expectedError: "EOF",
		},
		{
			name:  "Excessive Pushback (Index Out of Bounds)",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				var pushbackRule textlexer.Rule
				pushbackRule = func(r rune) (textlexer.Rule, textlexer.State) {
					return backtrack(2, textlexer.StateAccept), textlexer.StatePushBack
				}
				lx.MustAddRule(lexTypeLoop, pushbackRule)
			},
			// The ruleProcessor should have a safety limit on pushbacks.
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
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
					lexemes[i] = textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "€", i)
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
				zeroLengthRule := func(r rune) (textlexer.Rule, textlexer.State) {
					return backtrack(1, textlexer.StateAccept), textlexer.StatePushBack
				}
				lx.MustAddRule(lexTypeLoop, zeroLengthRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
			},
		},
		{
			name:  "Pushback Infinite Loop",
			input: "abc",
			setupRules: func(lx *textlexer.TextLexer) {
				// This rule creates a loop by always pushing back and returning to itself.
				var pushbackLoopRule textlexer.Rule
				pushbackLoopRule = func(r rune) (textlexer.Rule, textlexer.State) {
					return pushbackLoopRule, textlexer.StatePushBack
				}
				lx.MustAddRule(lexTypeLoop, pushbackLoopRule)
			},
			// The processor's pushback limit should prevent an infinite loop.
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 1),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "c", 2),
			},
		},
		{
			name:  "Rule returns nil, StateContinue",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				// The processor must handle an invalid (nil, StateContinue) return
				// by deactivating the scanner to prevent undefined behavior.
				badRule := func(r rune) (textlexer.Rule, textlexer.State) {
					return nil, textlexer.StateContinue
				}
				lx.MustAddRule(lexTypeLoop, badRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
			},
		},
		{
			name:  "Rule in StateContinue at EOF",
			input: "ab",
			setupRules: func(lx *textlexer.TextLexer) {
				// Rule that continues without accepting, then hits EOF.
				ruleAtEOF := func(r rune) (textlexer.Rule, textlexer.State) {
					if r == 'a' {
						return func(r rune) (textlexer.Rule, textlexer.State) {
							if r == 'b' {
								// Continue without accepting - will hit EOF.
								return func(r rune) (textlexer.Rule, textlexer.State) {
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
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 1),
			},
		},
		{
			name:  "Pushback at EOF",
			input: "a",
			setupRules: func(lx *textlexer.TextLexer) {
				pushbackAtEOF := func(r rune) (textlexer.Rule, textlexer.State) {
					if r == 'a' {
						return func(r rune) (textlexer.Rule, textlexer.State) {
							return backtrack(1, textlexer.StateAccept), textlexer.StatePushBack
						}, textlexer.StateAccept
					}
					return nil, textlexer.StateReject
				}
				lx.MustAddRule("PUSHBACK_EOF", pushbackAtEOF)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexeme(textlexer.LexemeType("PUSHBACK_EOF"), "a", 0),
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

			ok := assert.Equal(t, tc.expectedLexemes, foundLexemes, "The stream of lexemes did not match the expected output.")
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

	wordOrSpaceRule := func(r rune) (textlexer.Rule, textlexer.State) {
		if r == 'w' {
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

// TestLexerPanickingRule demonstrates that a panic within a user-provided rule
// is correctly propagated.
func TestLexerPanickingRule(t *testing.T) {
	const lexTypePanic = textlexer.LexemeType("PANIC")
	input := "crash"

	panickingRule := func(r rune) (textlexer.Rule, textlexer.State) {
		panic("user rule panicked")
	}

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypePanic, panickingRule)

	assert.PanicsWithValue(t, "user rule panicked", func() {
		_, _ = lx.Next()
	})
}

// TestLexerPanicRecovery verifies the lexer's behavior when a rule panics.
func TestLexerPanicRecovery(t *testing.T) {
	const (
		lexTypePanic  = textlexer.LexemeType("PANIC")
		lexTypeNormal = textlexer.LexemeType("NORMAL")
	)
	input := "panic normal"

	panicCount := 0
	recoveringPanicRule := func(r rune) (textlexer.Rule, textlexer.State) {
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

	// This test verifies the current behavior, which is to propagate the panic.
	// The lexer itself does not recover from panics in user-provided rules.
	assert.Panics(t, func() {
		_, _ = lx.Next()
	})
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
		close(lexingStarted)
		for {
			_, err := lx.Next()
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

		// Add multiple rules with a small delay to increase chance of race detection.
		time.Sleep(time.Millisecond)

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

// TestLexerPathologicalRuleWithDeepBacktracking tests a complex rule that accepts
// early, continues, and then forces a deep backtrack.
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
				textlexer.NewLexeme(lexTypePathological, "aaa", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 3),
			},
		},
		{
			name:           "Medium backtrack",
			input:          "aaaaaaaab",
			backtrackDepth: 8,
			expected: []*textlexer.Lexeme{
				textlexer.NewLexeme(lexTypePathological, "aaaaaaaa", 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 8),
			},
		},
		{
			name:           "Deep backtrack",
			input:          strings.Repeat("a", 100) + "b",
			backtrackDepth: 100,
			expected: []*textlexer.Lexeme{
				textlexer.NewLexeme(lexTypePathological, strings.Repeat("a", 100), 0),
				textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 100),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var pathologicalRule, finalState textlexer.Rule

			finalState = func(r rune) (textlexer.Rule, textlexer.State) {
				return backtrack(tc.backtrackDepth, textlexer.StateAccept), textlexer.StatePushBack
			}

			// This rule accepts every 'a' but continues matching. When it sees a 'b',
			// it transitions to the final state to force the backtrack to the last accept.
			pathologicalRule = func(r rune) (textlexer.Rule, textlexer.State) {
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

			assert.Equal(t, tc.expected, found, "Backtrack depth %d failed", tc.backtrackDepth)
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
		textlexer.NewLexeme(lexTypeID, "hello", 0),
		textlexer.NewLexeme(textlexer.LexemeTypeUnknown, string(textlexer.RuneEOF), 5),
		textlexer.NewLexeme(lexTypeID, "world", 6),
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

	assert.Equal(t, expected, found)
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

		var deepPushbackRule textlexer.Rule
		deepPushbackRule = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return backtrack(maxPushback+5, textlexer.StateAccept), textlexer.StatePushBack
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("DEEP_PUSHBACK", deepPushbackRule)

		// The lexer should handle excessive pushback gracefully.
		lex, err := lx.Next()
		if err == nil {
			assert.NotNil(t, lex)
		} else {
			assert.Contains(t, err.Error(), "pushback")
		}
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
			// This rule always produces a zero-length match.
			zeroLengthRule := func(r rune) (textlexer.Rule, textlexer.State) {
				return backtrack(1, textlexer.StateAccept), textlexer.StatePushBack
			}

			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule("ZERO", zeroLengthRule)

			done := make(chan struct{})
			go func() {
				defer close(done)

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
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Test timeout - likely infinite loop in zero-length match handling")
			}
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
		ruleA = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return ruleA_continue, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		ruleA_continue = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return ruleA, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}

		var ruleB, ruleB_accept textlexer.Rule
		ruleB = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return ruleB_accept, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}
		ruleB_accept = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return ruleB, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeOdd, ruleA)
		lx.MustAddRule(lexTypeEven, ruleB)

		// Expected: Rule A's lastAccept is 5, Rule B's is 4. The processor must pick the longest.
		expected := []*textlexer.Lexeme{
			textlexer.NewLexeme(lexTypeOdd, "aaaaa", 0),
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
		assert.Equal(t, expected, found)
	})

	t.Run("Bait and Switch Rule", func(t *testing.T) {
		// A malicious rule ("Bait") finds a long match, but then backtracks to report a shorter one.
		// The lexer must not be fooled and should select the longest match recorded by any rule.
		const (
			lexTypeBait   = textlexer.LexemeType("BAIT")
			lexTypeHonest = textlexer.LexemeType("HONEST")
		)
		input := "token;"

		var baitRule textlexer.Rule
		baitFinalizer := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == ';' {
				return backtrack(4, textlexer.StateAccept), textlexer.StatePushBack
			}
			return nil, textlexer.StateReject
		}
		match_n := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'n' {
				return baitFinalizer, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_e := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'e' {
				return match_n, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_k := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'k' {
				return match_e, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_o := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'o' {
				return match_k, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		baitRule = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 't' {
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
			textlexer.NewLexeme(lexTypeBait, "token", 0),
			textlexer.NewLexeme(textlexer.LexemeTypeUnknown, ";", 5),
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
		assert.Equal(t, expected, found)
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
		vetoFinalizer := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == ';' {
				// Veto: After seeing ';', reject further matching.
				// This does NOT erase the lastAccept value already recorded for this rule.
				return backtrack(4, textlexer.StateReject), textlexer.StatePushBack
			}
			return nil, textlexer.StateReject
		}
		match_c := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'c' {
				return vetoFinalizer, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		match_b := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'b' {
				return match_c, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		vetoRule = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
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
			textlexer.NewLexeme(lexTypeVeto, "abc", 0),
			textlexer.NewLexeme(textlexer.LexemeTypeUnknown, ";", 3),
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
		assert.Equal(t, expected, found)
	})

	t.Run("Rule Returns nil with StatePushBack", func(t *testing.T) {
		// An invalid user rule returns (nil, StatePushBack). The lexer must handle
		// this by deactivating the rule after the pushback to prevent errors.
		const lexTypeBad = textlexer.LexemeType("BAD_PUSHBACK")
		input := "ab"

		badRule := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return func(r rune) (textlexer.Rule, textlexer.State) {
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
			textlexer.NewLexeme(textlexer.LexemeType("A"), "a", 0),
			textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 1),
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
		assert.Equal(t, expected, found)
	})

	t.Run("Always Reject Rule", func(t *testing.T) {
		const lexTypeReject = textlexer.LexemeType("REJECT")
		input := "abc"

		alwaysRejectRule := func(r rune) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeReject, alwaysRejectRule)

		expected := []*textlexer.Lexeme{
			textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "a", 0),
			textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "b", 1),
			textlexer.NewLexeme(textlexer.LexemeTypeUnknown, "c", 2),
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
		assert.Equal(t, expected, found)
	})

	t.Run("Always Accept Rule", func(t *testing.T) {
		const lexTypeAccept = textlexer.LexemeType("ACCEPT")
		input := "abc"

		alwaysAcceptRule := func(r rune) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateAccept
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeAccept, alwaysAcceptRule)

		expected := []*textlexer.Lexeme{
			textlexer.NewLexeme(lexTypeAccept, "a", 0),
			textlexer.NewLexeme(lexTypeAccept, "b", 1),
			textlexer.NewLexeme(lexTypeAccept, "c", 2),
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
		assert.Equal(t, expected, found)
	})

	t.Run("Exponential State Explosion", func(t *testing.T) {
		// This simulates a badly written regex like (a*)* that could cause
		// exponential state explosion.
		const lexTypeExponential = textlexer.LexemeType("EXPONENTIAL")
		input := strings.Repeat("a", 20) + "b"

		callCount := 0
		var exponentialRule textlexer.Rule
		exponentialRule = func(r rune) (textlexer.Rule, textlexer.State) {
			callCount++
			if callCount > 1000 { // Safety limit.
				return nil, textlexer.StateReject
			}

			if r == 'a' {
				return func(r rune) (textlexer.Rule, textlexer.State) {
					if r == 'a' {
						// Branch: either continue or accept.
						return exponentialRule, textlexer.StateAccept
					}
					return nil, textlexer.StateReject
				}, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeExponential, exponentialRule)

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, err := lx.Next()
				if err == io.EOF {
					break
				}
			}
		}()

		select {
		case <-done:
			assert.Less(t, callCount, 1001, "Rule call count should be limited")
		case <-time.After(2 * time.Second):
			t.Fatal("Test timeout - possible exponential explosion")
		}
	})

	t.Run("Memory Exhaustion Attack", func(t *testing.T) {
		const lexTypeMemory = textlexer.LexemeType("MEMORY")
		input := "start"

		var memoryTracker [][]byte
		memoryExhaustionRule := func(r rune) (textlexer.Rule, textlexer.State) {
			if len(memoryTracker) < 10 { // Limit to prevent actual exhaustion in test.
				memoryTracker = append(memoryTracker, make([]byte, 1024*1024)) // 1MB
			}

			if r >= 'a' && r <= 'z' {
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
		ruleA = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return ruleB, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		ruleB = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'b' {
				return ruleC, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}
		ruleC = func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'c' {
				return ruleA, textlexer.StateAccept
			}
			return nil, textlexer.StateReject
		}

		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeCycle, ruleA)

		expected := []*textlexer.Lexeme{
			textlexer.NewLexeme(lexTypeCycle, "abc", 0),
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
		assert.Equal(t, expected, found)
	})

	t.Run("Buffer Overflow Attempt", func(t *testing.T) {
		const lexTypeOverflow = textlexer.LexemeType("OVERFLOW")
		input := strings.Repeat("a", 65536)

		overflowRule := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				var deepChain func(int) textlexer.Rule
				deepChain = func(depth int) textlexer.Rule {
					if depth > 100 { // Limit depth for test.
						return nil
					}
					return func(r rune) (textlexer.Rule, textlexer.State) {
						if r == 'a' {
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
		mutatingRule = func(r rune) (textlexer.Rule, textlexer.State) {
			atomic.AddInt32(&sharedCounter, 1)

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
	chaosRule = func(r rune) (textlexer.Rule, textlexer.State) {
		matchLength++
		u := rng.Float64()

		if u < 0.1 && matchLength > 1 {
			return backtrack(1, textlexer.StateAccept), textlexer.StatePushBack
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

	lx.MustAddRule(lexTypeChaos, chaosRule)

	done := make(chan struct{})
	tokenCount := 0

	go func() {
		defer close(done)
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
	}()

	select {
	case <-done:
		assert.Greater(t, tokenCount, 0, "Chaos test should have produced at least one token")
	case <-time.After(10 * time.Second):
		t.Fatal("Chaos test timeout - possible infinite loop or deadlock")
	}
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
			rule: func(r rune) (textlexer.Rule, textlexer.State) {
				var loop textlexer.Rule
				loop = func(r rune) (textlexer.Rule, textlexer.State) {
					return loop, textlexer.StateContinue
				}
				return loop(r)
			},
		},
		{
			name: "Always PushBack",
			rule: func(r rune) (textlexer.Rule, textlexer.State) {
				var loop textlexer.Rule
				loop = func(r rune) (textlexer.Rule, textlexer.State) {
					return loop, textlexer.StatePushBack
				}
				return loop(r)
			},
		},
		{
			name: "Zero Length Accept Loop",
			rule: func(r rune) (textlexer.Rule, textlexer.State) {
				return backtrack(1, textlexer.StateAccept), textlexer.StatePushBack
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := "test"
			lx := textlexer.New(strings.NewReader(input))
			lx.MustAddRule("LOOP", tc.rule)

			done := make(chan struct{})
			go func() {
				defer close(done)

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
			}()

			select {
			case <-done:
				// Test completed, meaning the lexer handled the problematic rule.
			case <-time.After(2 * time.Second):
				t.Fatal("Infinite loop protection failed - lexer is stuck")
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

			ruleLoop = func(r rune) (textlexer.Rule, textlexer.State) {
				if r != 'a' {
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
		lx.MustAddRule("DEEP", stackSafeDeepRule())

		done := make(chan bool)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Panic recovered: %v", r)
					done <- false
				} else {
					done <- true
				}
			}()

			lex, err := lx.Next()
			require.NoError(t, err)
			require.NotNil(t, lex)
			assert.Equal(t, maxDepth, lex.Len())

			_, err = lx.Next()
			require.Equal(t, io.EOF, err)
		}()

		select {
		case success := <-done:
			assert.True(t, success, "Should not panic from stack overflow")
		case <-time.After(5 * time.Second):
			t.Fatal("Test timeout - lexer may be stuck in a loop")
		}
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
		greedyRule = func(r rune) (textlexer.Rule, textlexer.State) {
			if r >= 'a' && r <= 'z' {
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
		ruleA := func(r rune) (textlexer.Rule, textlexer.State) {
			if r == 'a' {
				return func(r rune) (textlexer.Rule, textlexer.State) {
					if r == 'b' {
						return func(r rune) (textlexer.Rule, textlexer.State) {
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
		ruleB = func(r rune) (textlexer.Rule, textlexer.State) {
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

	t.Run("Rule Panic Types", func(t *testing.T) {
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

				panicRule := func(r rune) (textlexer.Rule, textlexer.State) {
					panic(tc.panicValue)
				}

				lx := textlexer.New(strings.NewReader(input))
				lx.MustAddRule("PANIC", panicRule)

				if tc.panicValue == nil {
					assert.Panics(t, func() {
						_, _ = lx.Next()
					})
				} else {
					assert.PanicsWithValue(t, tc.panicValue, func() {
						_, _ = lx.Next()
					})
				}
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

		trackingRule := func(r rune) (textlexer.Rule, textlexer.State) {
			mu.Lock()
			transitions = append(transitions, fmt.Sprintf("Start(%c)", r))
			mu.Unlock()

			if r >= 'a' && r <= 'z' {
				var continueRule textlexer.Rule
				continueRule = func(r rune) (textlexer.Rule, textlexer.State) {
					mu.Lock()
					transitions = append(transitions, fmt.Sprintf("Continue(%c)", r))
					mu.Unlock()

					if r >= 'a' && r <= 'z' {
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

		lifecycleRule := func(r rune) (textlexer.Rule, textlexer.State) {
			mu.Lock()
			events = append(events, lifecycleEvent{"create", r})
			mu.Unlock()

			if r >= 'a' && r <= 'z' {
				return func(r rune) (textlexer.Rule, textlexer.State) {
					mu.Lock()
					events = append(events, lifecycleEvent{"continue", r})
					mu.Unlock()

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

		positionTrackingRule := func(r rune) (textlexer.Rule, textlexer.State) {
			currentPos := 0
			var track func(r rune) (textlexer.Rule, textlexer.State)
			track = func(r rune) (textlexer.Rule, textlexer.State) {
				currentPos++
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
			return track(r)
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
		selfDeactivatingRule = func(r rune) (textlexer.Rule, textlexer.State) {
			callCount++
			if callCount > 4 { // Deactivate after matching "test".
				return nil, textlexer.StateReject
			}
			if r >= 'a' && r <= 'z' {
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
