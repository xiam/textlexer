package rules_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
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
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "s", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "t", 1),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 2),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "r", 3),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "t", 4),
				textlexer.NewLexemeFromString(lexTypeWhitespace, "  \t  ", 5),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "e", 10),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "n", 11),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "d", 12),
			},
		},
		{
			name:  "Input with no whitespace",
			input: "abc",
			expectedLexemes: []*textlexer.Lexeme{
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "a", 0),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "b", 1),
				textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, "c", 2),
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
			// 1. Create a new lexer for the test case input.
			lx := textlexer.New(strings.NewReader(tc.input))

			// 2. Add the rule we are testing.
			lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)

			// 3. Collect all lexemes from the lexer.
			var foundLexemes []*textlexer.Lexeme
			for {
				lex, err := lx.Next()
				if err == io.EOF {
					break
				}
				require.NoError(t, err, "Lexer returned an unexpected error")
				foundLexemes = append(foundLexemes, lex)
			}

			// 4. Assert that the collected lexemes match the expected output.
			assert.Equal(t, tc.expectedLexemes, foundLexemes, "The stream of lexemes did not match the expected output.")

			// 5. Verify the lexer is fully consumed.
			_, err := lx.Next()
			require.Equal(t, io.EOF, err, "Expected EOF after consuming all tokens")
		})
	}
}
