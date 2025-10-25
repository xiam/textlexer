package rules_test

import (
	"io"
	"strings"
	"testing"

	spew "github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

const (
	lexTypeUnknown = textlexer.LexemeTypeUnknown
)

func TestWhitespaceRule(t *testing.T) {
	const lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")

	testCases := []struct {
		name            string
		input           string
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Single Space",
			input: " ",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 0),
			},
		},
		{
			name:  "Multiple Spaces",
			input: "   ",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWhitespace, "   ", 0),
			},
		},
		{
			name:  "Mixed Whitespace",
			input: " \t\r\n\f ",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWhitespace, " \t\r\n\f ", 0),
			},
		},
		{
			name:  "Whitespace with leading and trailing text",
			input: "start  \t  end",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "s", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "t", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "a", 2),
				textlexer.NewLexemeFromString(lexTypeUnknown, "r", 3),
				textlexer.NewLexemeFromString(lexTypeUnknown, "t", 4),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  \t  ", 5),
				textlexer.NewLexemeFromString(lexTypeUnknown, "e", 10),
				textlexer.NewLexemeFromString(lexTypeUnknown, "n", 11),
				textlexer.NewLexemeFromString(lexTypeUnknown, "d", 12),
			},
		},
		{
			name:  "Input with no whitespace",
			input: "abc",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "b", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "c", 2),
			},
		},
		{
			name:            "Empty Input",
			input:           "",
			expectedLexemes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err, "Lexer returned an unexpected error")
				foundLexemes = append(foundLexemes, lex)
			}

			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)

			_, err := lx.Next()
			require.Equal(t, io.EOF, err, "Expected EOF after consuming all tokens")
		})
	}
}

func TestUnsignedIntegerRule(t *testing.T) {
	const lexTypeInteger = textlexer.LexemeType("INTEGER")

	testCases := []struct {
		name            string
		input           string
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Single Digit",
			input: "9",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "9", 0),
			},
		},
		{
			name:  "Multiple Digits",
			input: "12345",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "12345", 0),
			},
		},
		{
			name:  "Leading Zero",
			input: "042",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "042", 0),
			},
		},
		{
			name:  "Surrounded by text",
			input: "a123b",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(lexTypeInteger, "123", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "b", 4),
			},
		},
		{
			name:  "No digits",
			input: "abc",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "b", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "c", 2),
			},
		},
		{
			name:            "Empty Input",
			input:           "",
			expectedLexemes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)
			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
			_, err := lx.Next()
			require.Equal(t, io.EOF, err)
		})
	}
}

func TestWordRule(t *testing.T) {
	const lexTypeWord = textlexer.LexemeType("WORD")

	testCases := []struct {
		name            string
		input           string
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Lowercase ASCII word",
			input: "hello",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWord, "hello", 0),
			},
		},
		{
			name:  "MixedCase ASCII word",
			input: "World",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWord, "World", 0),
			},
		},
		{
			name:  "Unicode word",
			input: "세계",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWord, "세계", 0),
			},
		},
		{
			name:  "Surrounded by non-letters",
			input: "1word2",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "1", 0),
				textlexer.NewLexemeFromString(lexTypeWord, "word", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "2", 5),
			},
		},
		{
			name:  "No letters",
			input: "123_456",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "1", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "2", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "3", 2),
				textlexer.NewLexemeFromString(lexTypeUnknown, "_", 3),
				textlexer.NewLexemeFromString(lexTypeUnknown, "4", 4),
				textlexer.NewLexemeFromString(lexTypeUnknown, "5", 5),
				textlexer.NewLexemeFromString(lexTypeUnknown, "6", 6),
			},
		},
		{
			name:            "Empty Input",
			input:           "",
			expectedLexemes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypeWord, rules.Word)
			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
			_, err := lx.Next()
			require.Equal(t, io.EOF, err)
		})
	}
}

func TestASCIIWordRule(t *testing.T) {
	const lexTypeASCIIWord = textlexer.LexemeType("ASCII_WORD")

	testCases := []struct {
		name            string
		input           string
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Lowercase ASCII word",
			input: "hello",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeASCIIWord, "hello", 0),
			},
		},
		{
			name:  "MixedCase ASCII word",
			input: "World",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeASCIIWord, "World", 0),
			},
		},
		{
			name:  "Unicode is rejected",
			input: "세계",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "세", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "계", 1),
			},
		},
		{
			name:  "Mixed ASCII and Unicode",
			input: "word세계word",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeASCIIWord, "word", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "세", 4),
				textlexer.NewLexemeFromString(lexTypeUnknown, "계", 5),
				textlexer.NewLexemeFromString(lexTypeASCIIWord, "word", 6),
			},
		},
		{
			name:            "Empty Input",
			input:           "",
			expectedLexemes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypeASCIIWord, rules.ASCIIWord)
			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
			_, err := lx.Next()
			require.Equal(t, io.EOF, err)
		})
	}
}

func TestIdentifierRule(t *testing.T) {
	const (
		lexTypeIdentifier = textlexer.LexemeType("IDENTIFIER")
		lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
	)

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Simple Identifier",
			input: "hello",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "hello", 0),
			},
		},
		{
			name:  "Starts with Underscore",
			input: "_privateVar",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "_privateVar", 0),
			},
		},
		{
			name:  "Contains Digits",
			input: "var123",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "var123", 0),
			},
		},
		{
			name:  "Single Character Letter",
			input: "i",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "i", 0),
			},
		},
		{
			name:  "Single Character Underscore",
			input: "_",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "_", 0),
			},
		},
		{
			name:  "Starts with Digit (Invalid)",
			input: "123var",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "1", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "2", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "3", 2),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "var", 3),
			},
		},
		{
			name:  "Contains Invalid Character",
			input: "my-var",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "my", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "-", 2),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "var", 3),
			},
		},
		{
			name:  "Surrounded by Whitespace",
			input: "  id  ",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  ", 0),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "id", 2),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  ", 4),
			},
		},
		{
			name:  "Empty Input",
			input: "",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err, "Lexer returned an unexpected error")
				foundLexemes = append(foundLexemes, lex)
			}

			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)

			_, err := lx.Next()
			require.Equal(t, io.EOF, err, "Expected EOF after consuming all tokens")
		})
	}
}

func TestLiteralRule(t *testing.T) {
	const (
		lexTypeIf         = textlexer.LexemeType("IF")
		lexTypeFunc       = textlexer.LexemeType("FUNC")
		lexTypeArrow      = textlexer.LexemeType("ARROW")
		lexTypeIdentifier = textlexer.LexemeType("IDENTIFIER")
	)

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Simple Keyword Match",
			input: "if",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIf, rules.Literal("if"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIf, "if", 0),
			},
		},
		{
			name:  "Multi-character Operator",
			input: "=>",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeArrow, rules.Literal("=>"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeArrow, "=>", 0),
			},
		},
		{
			name:  "Mid-match Failure",
			input: "iffy",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIf, rules.Literal("if"))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "iffy", 0),
			},
		},
		{
			name:  "Keyword vs Identifier Precedence",
			input: "if func function",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIf, rules.Literal("if"))
				lx.MustAddRule(lexTypeFunc, rules.Literal("func"))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule("WHITESPACE", rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIf, "if", 0),
				textlexer.NewLexemeFromString("WHITESPACE", " ", 2),
				textlexer.NewLexemeFromString(lexTypeFunc, "func", 3),
				textlexer.NewLexemeFromString("WHITESPACE", " ", 7),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "function", 8),
			},
		},
		{
			name:  "No Match",
			input: "else",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeIf, rules.Literal("if"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, "e", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "l", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "s", 2),
				textlexer.NewLexemeFromString(lexTypeUnknown, "e", 3),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}

	t.Run("Panics on Empty Literal", func(t *testing.T) {
		assert.Panics(t, func() {
			rules.Literal("")
		}, "Literal factory should panic if given an empty string")
	})
}

func TestDelimitedRule(t *testing.T) {
	const lexTypeString = textlexer.LexemeType("STRING")
	const lexTypeComment = textlexer.LexemeType("COMMENT")

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Simple Double Quoted String",
			input: `"hello world"`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeString, rules.Delimited(`"`, `"`, '\\'))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeString, `"hello world"`, 0),
			},
		},
		{
			name:  "Empty String",
			input: `""`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeString, rules.Delimited(`"`, `"`, '\\'))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeString, `""`, 0),
			},
		},
		{
			name:  "String with Internal Escaped Quote",
			input: `"He said, \"Hello!\""`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeString, rules.Delimited(`"`, `"`, '\\'))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeString, `"He said, \"Hello!\""`, 0),
			},
		},
		{
			name:  "String with Escaped Backslash",
			input: `"path\\to\\file"`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeString, rules.Delimited(`"`, `"`, '\\'))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeString, `"path\\to\\file"`, 0),
			},
		},
		{
			name:  "Multi-character Delimiter Comment",
			input: `/* this is a comment */`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComment, rules.Delimited(`/*`, `*/`, 0)) // No escape rune
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComment, `/* this is a comment */`, 0),
			},
		},
		{
			name:  "Multi-character Delimiter with False End",
			input: `/* comment with a * in it */`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComment, rules.Delimited(`/*`, `*/`, 0))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComment, `/* comment with a * in it */`, 0),
			},
		},
		{
			name:  "Unterminated String at EOF",
			input: `"hello`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeString, rules.Delimited(`"`, `"`, '\\'))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, `"`, 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "h", 1),
				textlexer.NewLexemeFromString(lexTypeUnknown, "e", 2),
				textlexer.NewLexemeFromString(lexTypeUnknown, "l", 3),
				textlexer.NewLexemeFromString(lexTypeUnknown, "l", 4),
				textlexer.NewLexemeFromString(lexTypeUnknown, "o", 5),
			},
		},
		{
			name:  "Different Start and End Delimiters",
			input: `(a test)`,
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule("PARENS", rules.Delimited(`(`, `)`, 0))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString("PARENS", `(a test)`, 0),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestSignedIntegerRule(t *testing.T) {
	const lexTypeSignedInt = textlexer.LexemeType("SIGNED_INT")
	const lexTypeSymbol = textlexer.LexemeType("SYMBOL")

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Positive Integer with Sign",
			input: "+42",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeSignedInt, rules.SignedInteger())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSignedInt, "+42", 0),
			},
		},
		{
			name:  "Negative Integer",
			input: "-123",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeSignedInt, rules.SignedInteger())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSignedInt, "-123", 0),
			},
		},
		{
			name:  "Integer without Sign",
			input: "100",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeSignedInt, rules.SignedInteger())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSignedInt, "100", 0),
			},
		},
		{
			name:  "Zero",
			input: "0",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeSignedInt, rules.SignedInteger())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSignedInt, "0", 0),
			},
		},
		{
			name:  "Sign Not Followed by Digit",
			input: "-a",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeSignedInt, rules.SignedInteger())
				lx.MustAddRule(lexTypeSymbol, rules.Literal("-"))
				lx.MustAddRule("ID", rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSymbol, "-", 0),
				textlexer.NewLexemeFromString("ID", "a", 1),
			},
		},
		{
			name:  "Standalone Sign",
			input: "+",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeSignedInt, rules.SignedInteger())
				lx.MustAddRule(lexTypeSymbol, rules.Literal("+"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSymbol, "+", 0),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestUnsignedFloatRule(t *testing.T) {
	const lexTypeFloat = textlexer.LexemeType("FLOAT")
	const lexTypeInt = textlexer.LexemeType("INT")

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Integer and Fractional Parts",
			input: "12.12",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "12.12", 0),
			},
		},
		{
			name:  "Integer Part Only",
			input: "12.",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "12.", 0),
			},
		},
		{
			name:  "Fractional Part Only",
			input: ".12",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, ".12", 0),
			},
		},
		{
			name:  "Zero Point Zero",
			input: "0.0",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "0.0", 0),
			},
		},
		{
			name:  "Zero with trailing decimal",
			input: "0.",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "0.", 0),
			},
		},
		{
			name:  "Float vs Integer (longest match)",
			input: "123.456",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, rules.UnsignedFloat())
				lx.MustAddRule(lexTypeInt, rules.UnsignedInteger)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "123.456", 0),
			},
		},
		{
			name:  "Standalone Decimal is not a float",
			input: ".",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeUnknown, ".", 0),
			},
		},
		{
			name:  "Integer should not match as float",
			input: "123",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, rules.UnsignedFloat())
				lx.MustAddRule(lexTypeInt, rules.UnsignedInteger)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInt, "123", 0),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			if tc.setupRules == nil {
				tc.setupRules = func(lx *textlexer.TextLexer) {
					lx.MustAddRule(lexTypeFloat, rules.UnsignedFloat())
				}
			}
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestSignedFloatRule(t *testing.T) {
	const lexTypeFloat = textlexer.LexemeType("FLOAT")
	const lexTypeSymbol = textlexer.LexemeType("SYMBOL")

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Negative with integer and fractional",
			input: "-12.22",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "-12.22", 0),
			},
		},
		{
			name:  "Negative with trailing decimal",
			input: "-12.",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "-12.", 0),
			},
		},
		{
			name:  "Negative with leading decimal",
			input: "-.1",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "-.1", 0),
			},
		},
		{
			name:  "Negative Zero with trailing decimal",
			input: "-0.",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "-0.", 0),
			},
		},
		{
			name:  "Positive with trailing decimal",
			input: "+12.",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, "+12.", 0),
			},
		},
		{
			name:  "Unsigned float is also matched",
			input: ".1",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeFloat, ".1", 0),
			},
		},
		{
			name:  "Standalone sign is rejected",
			input: "-",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, rules.SignedFloat())
				lx.MustAddRule(lexTypeSymbol, rules.Literal("-"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSymbol, "-", 0),
			},
		},
		{
			name:  "Sign followed by non-number is rejected",
			input: "+a",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeFloat, rules.SignedFloat())
				lx.MustAddRule(lexTypeSymbol, rules.Literal("+"))
				lx.MustAddRule("ID", rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSymbol, "+", 0),
				textlexer.NewLexemeFromString("ID", "a", 1),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			if tc.setupRules == nil {
				tc.setupRules = func(lx *textlexer.TextLexer) {
					lx.MustAddRule(lexTypeFloat, rules.SignedFloat())
				}
			}
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestChoiceAndHexadecimalRule(t *testing.T) {
	const (
		lexTypeHex = textlexer.LexemeType("HEX")
		lexTypeInt = textlexer.LexemeType("INT")
	)

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Lowercase Hex Prefix",
			input: "0xdeadbeef",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeHex, rules.Hexadecimal())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeHex, "0xdeadbeef", 0),
			},
		},
		{
			name:  "Uppercase Hex Prefix",
			input: "0X123ABC",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeHex, rules.Hexadecimal())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeHex, "0X123ABC", 0),
			},
		},
		{
			name:  "Handles both upper and lower prefixes in one stream",
			input: "0xabc 0XDEF",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeHex, rules.Hexadecimal())
				lx.MustAddRule("WS", rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeHex, "0xabc", 0),
				textlexer.NewLexemeFromString("WS", " ", 5),
				textlexer.NewLexemeFromString(lexTypeHex, "0XDEF", 6),
			},
		},
		{
			name:  "Choice fails on prefix",
			input: "0zFFF",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeHex, rules.Hexadecimal())
				lx.MustAddRule(lexTypeInt, rules.UnsignedInteger) // Fallback for the '0'
				lx.MustAddRule("ID", rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInt, "0", 0),
				textlexer.NewLexemeFromString("ID", "zFFF", 1),
			},
		},
		{
			name:  "Standalone '0' does not match hex",
			input: "0",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeHex, rules.Hexadecimal())
				lx.MustAddRule(lexTypeInt, rules.UnsignedInteger) // Fallback for the '0'
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInt, "0", 0),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestChoiceRule_ComparisonOperators(t *testing.T) {
	const (
		lexTypeComparisonOp = textlexer.LexemeType("COMPARISON_OP")
		lexTypeAssign       = textlexer.LexemeType("ASSIGN") // For fallback testing
		lexTypeIdentifier   = textlexer.LexemeType("IDENTIFIER")
		lexTypeWhitespace   = textlexer.LexemeType("WHITESPACE")
	)

	comparisonOperatorRule := rules.Choice(
		rules.Literal("=="),
		rules.Literal("!="),
		rules.Literal("<="),
		rules.Literal(">="),
		rules.Literal("<"),
		rules.Literal(">"),
	)

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Equals",
			input: "==",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComparisonOp, "==", 0),
			},
		},
		{
			name:  "Not Equals",
			input: "!=",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComparisonOp, "!=", 0),
			},
		},
		{
			name:  "Less Than Or Equal",
			input: "<=",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComparisonOp, "<=", 0),
			},
		},
		{
			name:  "Greater Than Or Equal",
			input: ">=",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComparisonOp, ">=", 0),
			},
		},
		{
			name:  "Less Than (single character)",
			input: "<",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComparisonOp, "<", 0),
			},
		},
		{
			name:  "Greater Than (single character)",
			input: ">",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeComparisonOp, ">", 0),
			},
		},
		{
			name:  "Sequence of mixed operators",
			input: "a > b <= c",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "a", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 1),
				textlexer.NewLexemeFromString(lexTypeComparisonOp, ">", 2),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 3),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "b", 4),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 5),
				textlexer.NewLexemeFromString(lexTypeComparisonOp, "<=", 6),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 8),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "c", 9),
			},
		},
		{
			name:  "Partial match failure with fallback",
			input: "=a",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeComparisonOp, comparisonOperatorRule)
				lx.MustAddRule(lexTypeAssign, rules.Literal("="))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeAssign, "=", 0),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "a", 1),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestLookahead(t *testing.T) {
	const (
		lexTypeKeyword     = textlexer.LexemeType("KEYWORD")
		lexTypeIdentifier  = textlexer.LexemeType("IDENTIFIER")
		lexTypeWhitespace  = textlexer.LexemeType("WHITESPACE")
		lexTypeInteger     = textlexer.LexemeType("INTEGER")
		lexTypeCSSValue    = textlexer.LexemeType("CSS_VALUE")
		lexTypeVersionPart = textlexer.LexemeType("VERSION_PART")
		lexTypeDot         = textlexer.LexemeType("DOT")
	)

	testCases := []struct {
		name            string
		input           string
		setupRules      func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Keyword followed by whitespace (should match)",
			input: "if\n",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("if"),
					rules.Whitespace,
				))
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeKeyword, "if", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "\n", 2),
			},
		},
		{
			name:  "Keyword followed by identifier (should not match)",
			input: "ifx ",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("if"),
					rules.Whitespace,
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "ifx", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 3),
			},
		},
		{
			name:  "Keyword at end of input without whitespace (should not match)",
			input: "if",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("if"),
					rules.Whitespace,
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "if", 0),
			},
		},
		{
			name:  "Ensure longest match (identifier)",
			input: "cat cats caterpillar catsup",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("cat"),
					rules.Except('s'),
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeKeyword, "cat", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 3),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "cats", 4),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 8),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "caterpillar", 9),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 20),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "catsup", 21),
			},
		},
		{
			name:  "CSS value",
			input: "10px",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeCSSValue, rules.Lookahead(
					rules.UnsignedInteger,
					rules.Literal("px"),
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeCSSValue, "10", 0),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "px", 2),
			},
		},
		{
			name:  "CSS values",
			input: "10px 11 12em 13%% 14px",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeCSSValue, rules.Lookahead(
					rules.UnsignedInteger,
					rules.Choice(
						rules.Literal("px"),
						rules.Literal("em"),
					),
				))
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
				lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeCSSValue, "10", 0),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "px", 2),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 4),
				textlexer.NewLexemeFromString(lexTypeInteger, "11", 5),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 7),
				textlexer.NewLexemeFromString(lexTypeCSSValue, "12", 8),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "em", 10),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 12),
				textlexer.NewLexemeFromString(lexTypeInteger, "13", 13),
				textlexer.NewLexemeFromString(lexTypeUnknown, "%", 15),
				textlexer.NewLexemeFromString(lexTypeUnknown, "%", 16),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 17),
				textlexer.NewLexemeFromString(lexTypeCSSValue, "14", 18),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "px", 20),
			},
		},
		{
			name:  "Versioning part followed by dot",
			input: "1.2.3",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeVersionPart, rules.Lookahead(
					rules.UnsignedInteger,
					rules.Literal("."),
				))
				lx.MustAddRule(lexTypeDot, rules.Literal("."))
				lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeVersionPart, "1", 0),
				textlexer.NewLexemeFromString(lexTypeDot, ".", 1),
				textlexer.NewLexemeFromString(lexTypeVersionPart, "2", 2),
				textlexer.NewLexemeFromString(lexTypeDot, ".", 3),
				textlexer.NewLexemeFromString(lexTypeInteger, "3", 4),
			},
		},
		{
			name:  "Multi-character lookahead success",
			input: "start--end",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("start"),
					rules.Literal("--"),
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule("OP", rules.Literal("--"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeKeyword, "start", 0),
				textlexer.NewLexemeFromString("OP", "--", 5),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "end", 7),
			},
		},
		{
			name:  "Lookahead fails at EOF",
			input: "start",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("start"),
					rules.Literal("--"),
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "start", 0),
			},
		},
		{
			name:  "Lookahead fails after partial match",
			input: "12em",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeCSSValue, rules.Lookahead(
					rules.UnsignedInteger,
					rules.Literal("rem"), // Look for 'rem'
				))
				lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeInteger, "12", 0),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "em", 2),
			},
		},
		{
			name:  "Longest match principle wins over lookahead",
			input: "forall ",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("for"),
					rules.Whitespace,
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "forall", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 6),
			},
		},
		{
			name:  "Main rule fails immediately",
			input: "bar",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("foo"), // This will fail on 'b'
					rules.Whitespace,
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "bar", 0),
			},
		},
		{
			name:  "EOF during main rule match",
			input: "star",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("start"), // This rule will fail at EOF
					rules.Literal("--"),
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "star", 0),
			},
		},
		{
			name:  "Nested Lookahead (success)",
			input: "version 123",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("version"),
					rules.Lookahead(
						rules.Whitespace,
						rules.UnsignedInteger,
					),
				))
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
				lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeKeyword, "version", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 7),
				textlexer.NewLexemeFromString(lexTypeInteger, "123", 8),
			},
		},
		{
			name:  "Nested Lookahead (failure)",
			input: "version alpha",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("version"),
					rules.Lookahead(
						rules.Whitespace,
						rules.UnsignedInteger,
					),
				))
				lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "version", 0),
				textlexer.NewLexemeFromString(lexTypeWhitespace, " ", 7),
				textlexer.NewLexemeFromString(lexTypeIdentifier, "alpha", 8),
			},
		},
		{
			name:  "Edge Case: UnsignedInteger with lookahead for decimal point",
			input: "12.3",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeVersionPart, rules.Lookahead(
					rules.UnsignedInteger,
					rules.Literal("."),
				))
				lx.MustAddRule(lexTypeDot, rules.Literal("."))
				lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeVersionPart, "12", 0),
				textlexer.NewLexemeFromString(lexTypeDot, ".", 2),
				textlexer.NewLexemeFromString(lexTypeInteger, "3", 3),
			},
		},
		{
			name:  "Edge Case: Lookahead failure allows longer non-lookahead match to win",
			input: "abcde",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Literal("ab"),
					rules.Literal("cx"),
				))
				lx.MustAddRule(lexTypeIdentifier, rules.Literal("abcde"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "abcde", 0),
			},
		},
		{
			name:  "Edge Case: Lookahead for EOF",
			input: "word.",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeKeyword, rules.Lookahead(
					rules.Identifier(),
					func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
						if s.IsEOF() {
							return nil, textlexer.StateAccept
						}
						return nil, textlexer.StateReject
					},
				))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeKeyword, "word", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, ".", 4),
			},
		},
		{
			name:  "Edge Case: Lookahead as a failing branch in a Choice",
			input: "ac",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule("CHOICE", rules.Choice(
					rules.Lookahead(rules.Literal("a"), rules.Literal("b")),
					rules.Literal("ac"),
				))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString("CHOICE", "ac", 0),
			},
		},
		{
			name:  "Edge Case: Longest match wins over same-start lookahead",
			input: "ab",
			setupRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(
					lexTypeKeyword,
					rules.Lookahead(
						rules.Literal("a"),
						rules.Literal("b"),
					),
				)
				lx.MustAddRule(lexTypeIdentifier, rules.Identifier())
				lx.MustAddRule("LITERAL_B", rules.Literal("b"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeIdentifier, "ab", 0),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			tc.setupRules(lx)

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}
			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

func TestSequenceRule(t *testing.T) {
	const (
		lexTypeSequence   = textlexer.LexemeType("SEQUENCE")
		lexTypeIdentifier = textlexer.LexemeType("IDENTIFIER")
		lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
		lexTypeAB         = textlexer.LexemeType("AB")
		lexTypeInt        = textlexer.LexemeType("INT")
		lexTypeUnknown    = lexTypeUnknown
	)

	testCases := []struct {
		name            string
		input           string
		rule            textlexer.Rule
		setupExtraRules func(lx *textlexer.TextLexer)
		expectedLexemes []*textlexer.Lexeme
	}{
		{
			name:  "Simple sequence of two literals",
			input: "ab",
			rule: rules.Sequence(
				rules.Literal("a"),
				rules.Literal("b"),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "ab", 0),
			},
		},
		{
			name:  "Sequence fails on second element",
			input: "ac",
			rule: rules.Sequence(
				rules.Literal("a"),
				rules.Literal("b"),
			),
			setupExtraRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule("A", rules.Literal("a"))
				lx.MustAddRule("C", rules.Literal("c"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString("A", "a", 0),
				textlexer.NewLexemeFromString("C", "c", 1),
			},
		},
		{
			name:  "Sequence with multi-character sub-rules",
			input: "if cond",
			rule: rules.Sequence(
				rules.Literal("if"),
				rules.Whitespace,
				rules.Identifier(),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "if cond", 0),
			},
		},
		{
			name:  "Sequence where first rule pushes back",
			input: "123x",
			rule: rules.Sequence(
				rules.UnsignedInteger,
				rules.Literal("x"),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "123x", 0),
			},
		},
		{
			name:  "Sequence where a middle rule pushes back",
			input: "let 123 go",
			rule: rules.Sequence(
				rules.Literal("let"),
				rules.Whitespace,
				rules.UnsignedInteger,
				rules.Whitespace,
				rules.Literal("go"),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "let 123 go", 0),
			},
		},
		{
			name:  "Nested sequence",
			input: "abc",
			rule: rules.Sequence(
				rules.Sequence(
					rules.Literal("a"),
					rules.Literal("b"),
				),
				rules.Literal("c"),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "abc", 0),
			},
		},
		{
			name:  "Sequence with choice - first choice",
			input: "ab",
			rule: rules.Sequence(
				rules.Literal("a"),
				rules.Choice(rules.Literal("b"), rules.Literal("c")),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "ab", 0),
			},
		},
		{
			name:  "Sequence with choice - second choice",
			input: "ac",
			rule: rules.Sequence(
				rules.Literal("a"),
				rules.Choice(rules.Literal("b"), rules.Literal("c")),
			),
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeSequence, "ac", 0),
			},
		},
		{
			name:  "Sequence fails on last element with fallback",
			input: "abx",
			rule: rules.Sequence(
				rules.Literal("a"),
				rules.Literal("b"),
				rules.Literal("c"),
			),
			setupExtraRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeAB, rules.Literal("ab"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeAB, "ab", 0),
				textlexer.NewLexemeFromString(lexTypeUnknown, "x", 2),
			},
		},
		{
			name:  "EOF in middle of sequence with fallback",
			input: "ab",
			rule:  rules.Sequence(rules.Literal("a"), rules.Literal("b"), rules.Literal("c")),
			setupExtraRules: func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeAB, rules.Literal("ab"))
			},
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(lexTypeAB, "ab", 0),
			},
		},
	}

	t.Run("Panics on empty sequence", func(t *testing.T) {
		assert.Panics(t, func() {
			rules.Sequence()
		}, "Sequence with no rules should panic")
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypeSequence, tc.rule)
			if tc.setupExtraRules != nil {
				tc.setupExtraRules(lx)
			}

			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				foundLexemes = append(foundLexemes, lex)
			}

			assertLexemesEqual(t, tc.expectedLexemes, foundLexemes)
		})
	}
}

// assertLexemesEqual compares a slice of expected lexemes with a slice of
// actual lexemes.
func assertLexemesEqual(t *testing.T, expected []*textlexer.Lexeme, actual []*textlexer.Lexeme) {
	areLengthsEqual := assert.Equal(t, len(expected), len(actual), "Number of lexemes mismatch")
	if !areLengthsEqual {
		spew.Dump(map[string][]*textlexer.Lexeme{
			"expected": expected,
			"actual":   actual,
		})
	}

	for i := range expected {
		assertLexemeEqual(t, expected[i], actual[i])
	}
}

// assertLexemeEqual compares a single expected lexeme with a single actual
// lexeme.
func assertLexemeEqual(t *testing.T, expected *textlexer.Lexeme, actual *textlexer.Lexeme) {
	require.NotNil(t, actual, "Actual lexeme should not be nil")
	require.NotNil(t, expected, "Expected lexeme should not be nil")

	assert.Equal(t, expected.Type(), actual.Type(), "Lexeme type mismatch")
	assert.Equal(t, expected.Text(), actual.Text(), "Lexeme text mismatch")
	assert.Equal(t, expected.Offset(), actual.Offset(), "Lexeme offset mismatch")
	assert.Equal(t, expected.Len(), actual.Len(), "Lexeme length mismatch")
}
