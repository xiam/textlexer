package textlexer_test

import (
	"bytes"
	crand "crypto/rand"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

func TestNumericAndWhitespace(t *testing.T) {
	const (
		integerRule    = textlexer.LexemeType("INT")
		floatRule      = textlexer.LexemeType("FLOAT")
		whitespaceRule = textlexer.LexemeType("WHITESPACE")
	)

	in := " 12.3  \t   4 5.6 \t 7\n 8 9.0"

	out := []struct {
		Type textlexer.LexemeType
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

	lx := textlexer.New(strings.NewReader(in))

	lx.MustAddRule(whitespaceRule, rules.WhitespaceLexemeRule)
	lx.MustAddRule(floatRule, rules.UnsignedFloatLexemeRule)
	lx.MustAddRule(integerRule, rules.UnsignedIntegerLexemeRule)

	seen := map[textlexer.LexemeType]bool{}

	for _, expected := range out {
		lex, err := lx.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		assert.Equal(t, expected.Type, lex.Type)
		assert.Equal(t, expected.Text, lex.Text())

		seen[lex.Type] = true
	}

	assert.True(t, seen[integerRule])
	assert.True(t, seen[floatRule])
	assert.True(t, seen[whitespaceRule])
}

func TestNumericAndMathOperators(t *testing.T) {
	t.Run("basic math operators", func(t *testing.T) {
		const (
			numericRule      = textlexer.LexemeType("NUMERIC")
			mathOperatorRule = textlexer.LexemeType("MATH-OPERATOR")
		)

		in := `-1.23+-12++2-6+7.45*4/-2+3.33`

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{numericRule, "-1.23"},
			{mathOperatorRule, "+"},
			{numericRule, "-12"},
			{mathOperatorRule, "+"},
			{numericRule, "+2"},
			{numericRule, "-6"},
			{numericRule, "+7.45"},
			{mathOperatorRule, "*"},
			{numericRule, "4"},
			{mathOperatorRule, "/"},
			{numericRule, "-2"},
			{numericRule, "+3.33"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(numericRule, rules.NumericLexemeRule)
		lx.MustAddRule(mathOperatorRule, rules.BasicMathOperatorLexemeRule)

		matches := 0
		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, out[matches].Type, lex.Type)
			assert.Equal(t, out[matches].Text, lex.Text())

			matches++
		}
	})

	t.Run("basic math operators and space", func(t *testing.T) {
		const (
			numericRule      = textlexer.LexemeType("NUMERIC")
			mathOperatorRule = textlexer.LexemeType("MATH-OPERATOR")
			whiteSpaceRule   = textlexer.LexemeType("WHITE-SPACE")
		)

		in := `- 1.1 + 2.2 + 3.3 - 4.4`

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{numericRule, "- 1.1"},
			{numericRule, "+ 2.2"},
			{numericRule, "+ 3.3"},
			{numericRule, "- 4.4"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(numericRule, rules.NumericLexemeRule)
		lx.MustAddRule(mathOperatorRule, rules.BasicMathOperatorLexemeRule)
		lx.MustAddRule(whiteSpaceRule, rules.WhitespaceLexemeRule)

		matches := 0
		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			if lex.Type == whiteSpaceRule {
				continue
			}

			assert.Equal(t, out[matches].Type, lex.Type)
			assert.Equal(t, out[matches].Text, lex.Text())

			matches++
		}
	})
}

func TestSQL(t *testing.T) {
	t.Run("statement 1", func(t *testing.T) {
		const (
			whiteSpaceLexeme   = textlexer.LexemeType("WHITESPACE")
			commaLexeme        = textlexer.LexemeType("COMMA")
			semicolonLexeme    = textlexer.LexemeType("SEMICOLON")
			mathOperatorLexeme = textlexer.LexemeType("MATH-OPERATOR")
			integerLexeme      = textlexer.LexemeType("INT")
			wordLexeme         = textlexer.LexemeType("WORD")
		)

		in := `
		SELECT
			id, book, major - fees, major, minor, price, side
		FROM trades
		ORDER BY id DESC
		LIMIT 50;
	`

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{whiteSpaceLexeme, "\n\t\t"},
			{wordLexeme, "SELECT"},
			{whiteSpaceLexeme, "\n\t\t\t"},
			{wordLexeme, "id"},
			{commaLexeme, ","},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "book"},
			{commaLexeme, ","},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "major"},
			{whiteSpaceLexeme, " "},
			{mathOperatorLexeme, "-"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "fees"},
			{commaLexeme, ","},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "major"},
			{commaLexeme, ","},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "minor"},
			{commaLexeme, ","},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "price"},
			{commaLexeme, ","},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "side"},
			{whiteSpaceLexeme, "\n\t\t"},
			{wordLexeme, "FROM"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "trades"},
			{whiteSpaceLexeme, "\n\t\t"},
			{wordLexeme, "ORDER"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "BY"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "id"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "DESC"},
			{whiteSpaceLexeme, "\n\t\t"},
			{wordLexeme, "LIMIT"},
			{whiteSpaceLexeme, " "},
			{integerLexeme, "50"},
			{semicolonLexeme, ";"},
			{whiteSpaceLexeme, "\n\t"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(whiteSpaceLexeme, rules.WhitespaceLexemeRule)
		lx.MustAddRule(wordLexeme, rules.WordLexemeRule)
		lx.MustAddRule(commaLexeme, rules.CommaLexemeRule)
		lx.MustAddRule(mathOperatorLexeme, rules.BasicMathOperatorLexemeRule)
		lx.MustAddRule(integerLexeme, rules.SignedIntegerLexemeRule)
		lx.MustAddRule(semicolonLexeme, rules.SemicolonLexemeRule)

		for _, expected := range out {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, expected.Type, lex.Type)
			assert.Equal(t, expected.Text, lex.Text())
		}
	})

	t.Run("statement 2", func(t *testing.T) {
		const (
			whiteSpaceLexeme = textlexer.LexemeType("WHITE-SPACE")
			wordLexeme       = textlexer.LexemeType("WORD")
			starLexeme       = textlexer.LexemeType("STAR")
			commentLexeme    = textlexer.LexemeType("COMMENT")
		)

		in := "SELECT\n\t* FROM clients AS c /* hello word */ ORDER \n\n\n\t BY id ASC"

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{wordLexeme, "SELECT"},
			{whiteSpaceLexeme, "\n\t"},
			{starLexeme, "*"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "FROM"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "clients"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "AS"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "c"},
			{whiteSpaceLexeme, " "},
			{commentLexeme, "/* hello word */"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "ORDER"},
			{whiteSpaceLexeme, " \n\n\n\t "},
			{wordLexeme, "BY"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "id"},
			{whiteSpaceLexeme, " "},
			{wordLexeme, "ASC"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(whiteSpaceLexeme, rules.WhitespaceLexemeRule)
		lx.MustAddRule(wordLexeme, rules.WordLexemeRule)
		lx.MustAddRule(starLexeme, rules.StarLexemeRule)
		lx.MustAddRule(commentLexeme, rules.SlashStarCommentLexemeRule)

		for _, expected := range out {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, expected.Type, lex.Type)
			assert.Equal(t, expected.Text, lex.Text())

			t.Logf("%v", lex)
		}
	})

	t.Run("statement 3", func(t *testing.T) {
		const (
			whitespaceLexeme = textlexer.LexemeType("WHITE-SPACE")
			wordLexeme       = textlexer.LexemeType("WORD")
			reservedWord     = textlexer.LexemeType("RESERVED-WORD")
			stringLiteral    = textlexer.LexemeType("STRING-LITERAL")
			starLexeme       = textlexer.LexemeType("STAR")
			symbolLexeme     = textlexer.LexemeType("SYMBOL")
			semicolonLexeme  = textlexer.LexemeType("SEMICOLON")
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

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{reservedWord, "SELECT"},
			{starLexeme, "*"},
			{reservedWord, "FROM"},
			{wordLexeme, "clients"},
			{reservedWord, "AS"},
			{wordLexeme, "c"},
			{reservedWord, "WHERE"},
			{wordLexeme, "name"},
			{reservedWord, "LIKE"},
			{stringLiteral, "'%john%'"},
			{reservedWord, "AND"},
			{wordLexeme, "birthdate"},
			{symbolLexeme, ">"},
			{stringLiteral, "'1990-01-01'"},
			{reservedWord, "ORDER BY"},
			{wordLexeme, "id"},
			{wordLexeme, "ASC"},
			{semicolonLexeme, ";"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(whitespaceLexeme, rules.WhitespaceLexemeRule)
		lx.MustAddRule(wordLexeme, rules.WordLexemeRule)

		lx.MustAddRule(reservedWord, rules.NewMatchAnyOf(
			rules.NewCaseInsensitiveLiteralMatchLexemeRule("SELECT"),
			rules.NewCaseInsensitiveLiteralMatchLexemeRule("FROM"),
			rules.NewCaseInsensitiveLiteralMatchLexemeRule("AS"),
			rules.NewCaseInsensitiveLiteralMatchLexemeRule("WHERE"),
			rules.NewCaseInsensitiveLiteralMatchLexemeRule("LIKE"),
			rules.NewCaseInsensitiveLiteralMatchLexemeRule("AND"),
			rules.ComposeLexemeRules(
				rules.NewCaseInsensitiveLiteralMatchLexemeRule("ORDER"),
				rules.WhitespaceLexemeRule,
				rules.NewCaseInsensitiveLiteralMatchLexemeRule("BY"),
			),
		))

		lx.MustAddRule(stringLiteral, rules.SingleQuotedStringLexemeRule)
		lx.MustAddRule(starLexeme, rules.StarLexemeRule)
		lx.MustAddRule(symbolLexeme, rules.RAngleLexemeRule)
		lx.MustAddRule(semicolonLexeme, rules.SemicolonLexemeRule)

		seen := map[textlexer.LexemeType]bool{}

		matches := 0
		for {
			lex, err := lx.Next()

			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			seen[lex.Type] = true

			if lex.Type == whitespaceLexeme {
				continue
			}

			t.Logf("%v", lex)

			assert.Equal(t, out[matches].Type, lex.Type)
			assert.Equal(t, out[matches].Text, lex.Text())

			matches++
		}

		assert.True(t, seen[reservedWord])
		assert.True(t, seen[wordLexeme])
		assert.True(t, seen[stringLiteral])
		assert.True(t, seen[starLexeme])
		assert.True(t, seen[symbolLexeme])
		assert.True(t, seen[semicolonLexeme])
	})
}

func TestSpecificCases(t *testing.T) {
	t.Run("disambiguation", func(t *testing.T) {
		in := `ABCDEMANL1.23ABDEGHI`

		const (
			lexemeType1 = textlexer.LexemeType("TYPE-1")
			lexemeType2 = textlexer.LexemeType("TYPE-2")
			lexemeType3 = textlexer.LexemeType("TYPE-3")
			lexemeType4 = textlexer.LexemeType("TYPE-4")
		)

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{lexemeType1, "ABC"},
			{lexemeType4, "GHI"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexemeType1, rules.NewCaseInsensitiveLiteralMatchLexemeRule("ABC"))
		lx.MustAddRule(lexemeType2, rules.NewCaseInsensitiveLiteralMatchLexemeRule("ABCDEF"))
		lx.MustAddRule(lexemeType3, rules.NewCaseInsensitiveLiteralMatchLexemeRule("DEF"))
		lx.MustAddRule(lexemeType4, rules.NewCaseInsensitiveLiteralMatchLexemeRule("GHI"))

		seen := map[textlexer.LexemeType]bool{}

		matches := 0
		for {
			lex, err := lx.Next()

			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			seen[lex.Type] = true

			if lex.Type == textlexer.LexemeTypeUnknown {
				continue
			}

			assert.Equal(t, out[matches].Type, lex.Type)
			assert.Equal(t, out[matches].Text, lex.Text())

			matches++
		}

		assert.True(t, seen[lexemeType1])
		assert.True(t, seen[lexemeType4])
	})

	t.Run("no rules", func(t *testing.T) {
		in := `ABCDEFG`

		lx := textlexer.New(strings.NewReader(in))

		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type)
		}
	})

	t.Run("no rules no input", func(t *testing.T) {
		lx := textlexer.New(bytes.NewReader(nil))

		lex, err := lx.Next()
		require.Error(t, err)

		assert.Nil(t, lex)
	})

	t.Run("only A", func(t *testing.T) {
		const (
			lexemeType = textlexer.LexemeType("A")
		)

		in := `2hacgh6bAks3aSklvVVa1aaa`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexemeType, rules.NewCaseInsensitiveLiteralMatchLexemeRule("a"))

		seen := map[textlexer.LexemeType]bool{}

		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			seen[lex.Type] = true
			if lex.Type == lexemeType {
				assert.Equal(t, "a", strings.ToLower(lex.Text()))
			}
		}

		assert.True(t, seen[lexemeType])
	})

	t.Run("pair of AA", func(t *testing.T) {

		const (
			lexemeType = textlexer.LexemeType("AA")
		)

		in := `xxxaxxaxaxaaaaxaxaxaxaaaxaaaaxaaaaaa`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexemeType, rules.NewCaseInsensitiveLiteralMatchLexemeRule("AA"))

		seen := map[textlexer.LexemeType]bool{}

		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			seen[lex.Type] = true
			if lex.Type == lexemeType {
				assert.Equal(t, "aa", strings.ToLower(lex.Text()))
			}
		}

		assert.True(t, seen[lexemeType])
	})

	t.Run("always reject", func(t *testing.T) {
		const (
			lexemeType = textlexer.LexemeType("TYPE_1")
		)

		in := `abcdef`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexemeType, rules.AlwaysReject)

		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type)
		}
	})

	t.Run("always accept", func(t *testing.T) {
		const (
			lexemeType = textlexer.LexemeType("TYPE_1")
		)

		in := `abcdef`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexemeType, rules.AlwaysAccept)

		seen := map[textlexer.LexemeType]bool{}

		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			seen[lex.Type] = true
		}

		assert.True(t, seen[lexemeType])
	})

	t.Run("always continue", func(t *testing.T) {
		const (
			lexemeType = textlexer.LexemeType("TYPE_1")
		)

		in := `abcdef`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexemeType, rules.AlwaysContinue)

		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type)
		}
	})
}

func TestChaosRules(t *testing.T) {
	const (
		chaosLexeme1 = textlexer.LexemeType("CHAOS-1")
		chaosLexeme2 = textlexer.LexemeType("CHAOS-2")
		chaosLexeme3 = textlexer.LexemeType("CHAOS-3")
	)

	garbage := func() string {
		charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

		buf := make([]byte, 10000)
		_, err := crand.Read(buf)
		if err != nil {
			panic(err)
		}

		for i := 0; i < len(buf); i++ {
			buf[i] = charset[int(buf[i])%len(charset)]
		}

		return string(buf)
	}

	lx := textlexer.New(strings.NewReader(garbage()))

	var chaosRule textlexer.Rule

	chaosRule = func(r rune) (textlexer.Rule, textlexer.State) {
		u := rand.Float64()

		if u < 0.8 {
			return chaosRule, textlexer.StateContinue
		}

		if u < 0.9 {
			return nil, textlexer.StateReject
		}

		return nil, textlexer.StateAccept
	}

	lx.MustAddRule(chaosLexeme1, chaosRule)
	lx.MustAddRule(chaosLexeme2, chaosRule)
	lx.MustAddRule(chaosLexeme3, chaosRule)

	seen := map[textlexer.LexemeType]bool{}

	for {
		lex, err := lx.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		seen[lex.Type] = true
	}

	assert.True(t, seen[chaosLexeme1])
	assert.True(t, seen[chaosLexeme2])
	assert.True(t, seen[chaosLexeme3])
}
