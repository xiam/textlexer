package textlexer_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

func TestLexer(t *testing.T) {

	const (
		integerRule    = textlexer.TokenType("INT")
		floatRule      = textlexer.TokenType("FLOAT")
		whitespaceRule = textlexer.TokenType("WHITESPACE")
	)

	t.Run("add rules", func(t *testing.T) {
		in := " 12.3  \t   4 5.6 \t 7\n 8 9.0"

		out := []struct {
			Type textlexer.TokenType
			Text string
		}{
			{whitespaceRule, " "},
			{floatRule, "12.3"},
			{whitespaceRule, "  \t   "},
			{integerRule, "4"},
			{whitespaceRule, " "},
			{floatRule, "5.6"},
			{whitespaceRule, " \t "},
			{integerRule, "7"},
			{whitespaceRule, "\n "},
			{integerRule, "8"},
			{whitespaceRule, " "},
			{floatRule, "9.0"},
		}

		lx := textlexer.NewTextLexer(strings.NewReader(in))

		lx.MustAddRule(whitespaceRule, rules.WhitespaceTokenRule)
		lx.MustAddRule(floatRule, rules.UnsignedFloatTokenRule)
		lx.MustAddRule(integerRule, rules.UnsignedIntegerTokenRule)

		for _, expected := range out {
			tok, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, expected.Type, tok.Type)
			assert.Equal(t, expected.Text, tok.Text())
		}
	})
}

func TestSnippets(t *testing.T) {
	{
		in := `
		SELECT
			id, book, major - fees, major, minor, price, side
		FROM trades
		ORDER BY id DESC
		LIMIT 50;
	`

		const (
			whiteSpaceToken   = textlexer.TokenType("WHITESPACE")
			commaToken        = textlexer.TokenType("COMMA")
			semicolonToken    = textlexer.TokenType("SEMICOLON")
			mathOperatorToken = textlexer.TokenType("MATH_OPERATOR")
			integerToken      = textlexer.TokenType("INT")
			wordToken         = textlexer.TokenType("WORD")
		)

		out := []struct {
			Type textlexer.TokenType
			Text string
		}{
			{whiteSpaceToken, "\n\t\t"},
			{wordToken, "SELECT"},
			{whiteSpaceToken, "\n\t\t\t"},
			{wordToken, "id"},
			{commaToken, ","},
			{whiteSpaceToken, " "},
			{wordToken, "book"},
			{commaToken, ","},
			{whiteSpaceToken, " "},
			{wordToken, "major"},
			{whiteSpaceToken, " "},
			{mathOperatorToken, "-"},
			{whiteSpaceToken, " "},
			{wordToken, "fees"},
			{commaToken, ","},
			{whiteSpaceToken, " "},
			{wordToken, "major"},
			{commaToken, ","},
			{whiteSpaceToken, " "},
			{wordToken, "minor"},
			{commaToken, ","},
			{whiteSpaceToken, " "},
			{wordToken, "price"},
			{commaToken, ","},
			{whiteSpaceToken, " "},
			{wordToken, "side"},
			{whiteSpaceToken, "\n\t\t"},
			{wordToken, "FROM"},
			{whiteSpaceToken, " "},
			{wordToken, "trades"},
			{whiteSpaceToken, "\n\t\t"},
			{wordToken, "ORDER"},
			{whiteSpaceToken, " "},
			{wordToken, "BY"},
			{whiteSpaceToken, " "},
			{wordToken, "id"},
			{whiteSpaceToken, " "},
			{wordToken, "DESC"},
			{whiteSpaceToken, "\n\t\t"},
			{wordToken, "LIMIT"},
			{whiteSpaceToken, " "},
			{integerToken, "50"},
			{semicolonToken, ";"},
			{whiteSpaceToken, "\n\t"},
		}

		lx := textlexer.NewTextLexer(strings.NewReader(in))

		lx.MustAddRule(whiteSpaceToken, rules.WhitespaceTokenRule)
		lx.MustAddRule(wordToken, rules.WordTokenRule)
		lx.MustAddRule(commaToken, rules.CommaTokenRule)
		lx.MustAddRule(mathOperatorToken, rules.BasicMathOperatorTokenRule)
		lx.MustAddRule(integerToken, rules.SignedIntegerTokenRule)
		lx.MustAddRule(semicolonToken, rules.SemicolonTokenRule)

		for _, expected := range out {
			tok, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, expected.Type, tok.Type)
			assert.Equal(t, expected.Text, tok.Text())
		}
	}
}

func TestSQLSelectOrderBy(t *testing.T) {
	in := "SELECT\n\t* FROM clients AS c /* hello word */ ORDER \n\n\n\t BY id ASC"

	const (
		whiteSpaceToken = textlexer.TokenType("WHITE_SPACE")
		wordToken       = textlexer.TokenType("WORD")
		starToken       = textlexer.TokenType("STAR")
		commentToken    = textlexer.TokenType("COMMENT")
	)

	lx := textlexer.NewTextLexer(strings.NewReader(in))

	lx.MustAddRule(whiteSpaceToken, rules.WhitespaceTokenRule)
	lx.MustAddRule(wordToken, rules.WordTokenRule)
	lx.MustAddRule(starToken, rules.StarTokenRule)
	lx.MustAddRule(commentToken, rules.SlashStarCommentTokenRule)

	out := []struct {
		Type textlexer.TokenType
		Text string
	}{
		{wordToken, "SELECT"},
		{whiteSpaceToken, "\n\t"},
		{starToken, "*"},
		{whiteSpaceToken, " "},
		{wordToken, "FROM"},
		{whiteSpaceToken, " "},
		{wordToken, "clients"},
		{whiteSpaceToken, " "},
		{wordToken, "AS"},
		{whiteSpaceToken, " "},
		{wordToken, "c"},
		{whiteSpaceToken, " "},
		{commentToken, "/* hello word */"},
		{whiteSpaceToken, " "},
		{wordToken, "ORDER"},
		{whiteSpaceToken, " \n\n\n\t "},
		{wordToken, "BY"},
		{whiteSpaceToken, " "},
		{wordToken, "id"},
		{whiteSpaceToken, " "},
		{wordToken, "ASC"},
	}

	for _, expected := range out {
		tok, err := lx.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		assert.Equal(t, expected.Type, tok.Type)
		assert.Equal(t, expected.Text, tok.Text())
	}

}

func TestWordsAndReservedWords(t *testing.T) {

	const (
		whitespaceToken = textlexer.TokenType("WHITE_SPACE")
		wordToken       = textlexer.TokenType("WORD")
		reservedWord    = textlexer.TokenType("RESERVED_WORD")
		stringLiteral   = textlexer.TokenType("STRING_LITERAL")
		starToken       = textlexer.TokenType("STAR")
		symbolToken     = textlexer.TokenType("SYMBOL")
		semicolonToken  = textlexer.TokenType("SEMICOLON")
	)

	in := `
		SELECT
			*
		FROM clients AS c
		WHERE
			name LIKE '%john%'
			AND birthdate > '1990-01-01'
		ORDER BY id ASC
		;`

	lx := textlexer.NewTextLexer(strings.NewReader(in))

	lx.MustAddRule(whitespaceToken, rules.WhitespaceTokenRule)
	lx.MustAddRule(wordToken, rules.WordTokenRule)

	lx.MustAddRule(reservedWord, rules.NewMatchAnyOf(
		rules.NewCaseInsensitiveLiteralMatchTokenRule("SELECT"),
		rules.NewCaseInsensitiveLiteralMatchTokenRule("FROM"),
		rules.NewCaseInsensitiveLiteralMatchTokenRule("AS"),
		rules.NewCaseInsensitiveLiteralMatchTokenRule("WHERE"),
		rules.NewCaseInsensitiveLiteralMatchTokenRule("LIKE"),
		rules.NewCaseInsensitiveLiteralMatchTokenRule("AND"),
		rules.ComposeTokenRules(
			rules.NewCaseInsensitiveLiteralMatchTokenRule("ORDER"),
			rules.WhitespaceTokenRule,
			rules.NewCaseInsensitiveLiteralMatchTokenRule("BY"),
		),
	))

	lx.MustAddRule(stringLiteral, rules.SingleQuotedStringTokenRule)
	lx.MustAddRule(starToken, rules.StarTokenRule)
	lx.MustAddRule(symbolToken, rules.RAngleTokenRule)
	lx.MustAddRule(semicolonToken, rules.SemicolonTokenRule)

	out := []struct {
		Type textlexer.TokenType
		Text string
	}{
		{reservedWord, "SELECT"},
		{starToken, "*"},
		{reservedWord, "FROM"},
		{wordToken, "clients"},
		{reservedWord, "AS"},
		{wordToken, "c"},
		{reservedWord, "WHERE"},
		{wordToken, "name"},
		{reservedWord, "LIKE"},
		{stringLiteral, "'%john%'"},
		{reservedWord, "AND"},
		{wordToken, "birthdate"},
		{symbolToken, ">"},
		{stringLiteral, "'1990-01-01'"},
		{reservedWord, "ORDER BY"},
		{wordToken, "id"},
		{wordToken, "ASC"},
		{semicolonToken, ";"},
	}

	matches := 0

	for {
		tok, err := lx.Next()

		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		if tok.Type == whitespaceToken {
			continue
		}

		t.Logf("Token: %s", tok)

		assert.Equal(t, out[matches].Type, tok.Type)
		assert.Equal(t, out[matches].Text, tok.Text())

		matches++

		//assert.Equal(t, expected.Type, tok.Type)
		//assert.Equal(t, expected.Text, tok.Text())
	}

}
