package textlexer_test

import (
	//"bytes"
	//crand "crypto/rand"
	"io"
	//"math/rand"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

func TestNumericAndWhitespace(t *testing.T) {
	const (
		lexTypeInteger    = textlexer.LexemeType("INT")
		lexTypeFloat      = textlexer.LexemeType("FLOAT")
		lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
	)

	in := " 12.3  \t   4 5.6 \t 7\n 8 9.0"

	out := []struct {
		Type textlexer.LexemeType
		Text string
	}{
		{lexTypeWhitespace, " "},
		{lexTypeFloat, "12.3"},
		{lexTypeWhitespace, "  \t   "},
		{lexTypeInteger, "4"},
		{lexTypeWhitespace, " "},
		{lexTypeFloat, "5.6"},
		{lexTypeWhitespace, " \t "},
		{lexTypeInteger, "7"},
		{lexTypeWhitespace, "\n "},
		{lexTypeInteger, "8"},
		{lexTypeWhitespace, " "},
		{lexTypeFloat, "9.0"},
	}

	lx := textlexer.New(strings.NewReader(in))

	lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
	lx.MustAddRule(lexTypeFloat, rules.UnsignedFloat)
	lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)

	seen := map[textlexer.LexemeType]bool{}

	for _, expected := range out {
		lex, err := lx.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		slog.Info("lexeme", "type", lex.Type(), "text", lex.Text())

		//assert.Equal(t, expected.Type, lex.Type())
		//assert.Equal(t, expected.Text, lex.Text())

		seen[lex.Type()] = true

		_ = expected
	}
	return

	assert.True(t, seen[lexTypeInteger])
	assert.True(t, seen[lexTypeFloat])
	assert.True(t, seen[lexTypeWhitespace])
}

func TestScanner(t *testing.T) {
	const (
		lexTypeInteger    = textlexer.LexemeType("INT")
		lexTypeFloat      = textlexer.LexemeType("FLOAT")
		lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
	)

	in := "  12.3  \t   4 5.6 \t 7\n 8 9.0"

	expected := []*textlexer.Lexeme{
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
	}

	lx := textlexer.New(strings.NewReader(in))

	lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
	lx.MustAddRule(lexTypeFloat, rules.UnsignedFloat)
	lx.MustAddRule(lexTypeInteger, rules.UnsignedInteger)

	found := []textlexer.Lexeme{}
	for {
		lex, err := lx.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		found = append(found, *lex)
	}

	lex, err := lx.Next()
	assert.Equal(t, io.EOF, err)
	assert.Nil(t, lex)

	for i, exp := range expected {
		if i >= len(found) {
			t.Fatalf("missing lexeme: %+v", exp)
		}
		act := found[i]

		text := in[act.Offset() : act.Offset()+act.Len()]

		assert.Equal(t, exp.Type(), act.Type(), "index %d", i)
		assert.Equal(t, exp.Text(), act.Text(), "index %d", i)
		assert.Equal(t, exp.Text(), text, "index %d", i)
		assert.Equal(t, exp.Offset(), act.Offset(), "index %d", i)
		assert.Equal(t, exp.Len(), act.Len(), "index %d", i)
	}
}
