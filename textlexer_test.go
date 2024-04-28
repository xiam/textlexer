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

		assert.Equal(t, expected.Type, lex.Type)
		assert.Equal(t, expected.Text, lex.Text())

		seen[lex.Type] = true
	}

	assert.True(t, seen[lexTypeInteger])
	assert.True(t, seen[lexTypeFloat])
	assert.True(t, seen[lexTypeWhitespace])
}

func TestNumericAndMathOperators(t *testing.T) {
	t.Run("basic math operators", func(t *testing.T) {
		const (
			lexTypeNumeric = textlexer.LexemeType("NUMERIC")
			lexTypeMathOp  = textlexer.LexemeType("MATH-OPERATOR")
		)

		in := `-1.23+-12++2-6+7.45*4/-2+3.33`

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{lexTypeNumeric, "-1.23"},
			{lexTypeMathOp, "+"},
			{lexTypeNumeric, "-12"},
			{lexTypeMathOp, "+"},
			{lexTypeNumeric, "+2"},
			{lexTypeNumeric, "-6"},
			{lexTypeNumeric, "+7.45"},
			{lexTypeMathOp, "*"},
			{lexTypeNumeric, "4"},
			{lexTypeMathOp, "/"},
			{lexTypeNumeric, "-2"},
			{lexTypeNumeric, "+3.33"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexTypeNumeric, rules.Numeric)
		lx.MustAddRule(lexTypeMathOp, rules.BasicMathOperator)

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
			lexTypeNumeric    = textlexer.LexemeType("NUMERIC")
			lexTypeMathOp     = textlexer.LexemeType("MATH-OPERATOR")
			lexTypeWhitespace = textlexer.LexemeType("WHITE-SPACE")
		)

		in := `- 1.1 + 2.2 + 3.3 - 4.4`

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{lexTypeNumeric, "- 1.1"},
			{lexTypeNumeric, "+ 2.2"},
			{lexTypeNumeric, "+ 3.3"},
			{lexTypeNumeric, "- 4.4"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexTypeNumeric, rules.Numeric)
		lx.MustAddRule(lexTypeMathOp, rules.BasicMathOperator)
		lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)

		matches := 0
		for {
			lex, err := lx.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}

			if lex.Type == lexTypeWhitespace {
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
			lexTypeWhitespace = textlexer.LexemeType("WHITESPACE")
			lexTypeComma      = textlexer.LexemeType("COMMA")
			lexTypeSemicolon  = textlexer.LexemeType("SEMICOLON")
			lexTypeMathOp     = textlexer.LexemeType("MATH-OPERATOR")
			lexTypeInteger    = textlexer.LexemeType("INT")
			lexTypeWord       = textlexer.LexemeType("WORD")
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
			{lexTypeWhitespace, "\n\t\t"},
			{lexTypeWord, "SELECT"},
			{lexTypeWhitespace, "\n\t\t\t"},
			{lexTypeWord, "id"},
			{lexTypeComma, ","},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "book"},
			{lexTypeComma, ","},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "major"},
			{lexTypeWhitespace, " "},
			{lexTypeMathOp, "-"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "fees"},
			{lexTypeComma, ","},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "major"},
			{lexTypeComma, ","},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "minor"},
			{lexTypeComma, ","},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "price"},
			{lexTypeComma, ","},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "side"},
			{lexTypeWhitespace, "\n\t\t"},
			{lexTypeWord, "FROM"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "trades"},
			{lexTypeWhitespace, "\n\t\t"},
			{lexTypeWord, "ORDER"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "BY"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "id"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "DESC"},
			{lexTypeWhitespace, "\n\t\t"},
			{lexTypeWord, "LIMIT"},
			{lexTypeWhitespace, " "},
			{lexTypeInteger, "50"},
			{lexTypeSemicolon, ";"},
			{lexTypeWhitespace, "\n\t"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
		lx.MustAddRule(lexTypeWord, rules.Word)
		lx.MustAddRule(lexTypeComma, rules.Comma)
		lx.MustAddRule(lexTypeMathOp, rules.BasicMathOperator)
		lx.MustAddRule(lexTypeInteger, rules.SignedInteger)
		lx.MustAddRule(lexTypeSemicolon, rules.Semicolon)

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
			lexTypeWhitespace = textlexer.LexemeType("WHITE-SPACE")
			lexTypeWord       = textlexer.LexemeType("WORD")
			lexTypeStar       = textlexer.LexemeType("STAR")
			lexTypeComment    = textlexer.LexemeType("COMMENT")
		)

		in := "SELECT\n\t* FROM clients AS c /* hello word */ ORDER \n\n\n\t BY id ASC"

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{lexTypeWord, "SELECT"},
			{lexTypeWhitespace, "\n\t"},
			{lexTypeStar, "*"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "FROM"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "clients"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "AS"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "c"},
			{lexTypeWhitespace, " "},
			{lexTypeComment, "/* hello word */"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "ORDER"},
			{lexTypeWhitespace, " \n\n\n\t "},
			{lexTypeWord, "BY"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "id"},
			{lexTypeWhitespace, " "},
			{lexTypeWord, "ASC"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
		lx.MustAddRule(lexTypeWord, rules.Word)
		lx.MustAddRule(lexTypeStar, rules.Star)
		lx.MustAddRule(lexTypeComment, rules.SlashStarComment)

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
			lexTypeWhitespace    = textlexer.LexemeType("WHITE-SPACE")
			lexTypeWord          = textlexer.LexemeType("WORD")
			lexTypeReservedWord  = textlexer.LexemeType("RESERVED-WORD")
			lexTypeStringLiteral = textlexer.LexemeType("STRING-LITERAL")
			lexTypeStar          = textlexer.LexemeType("STAR")
			lexTypeSymbol        = textlexer.LexemeType("SYMBOL")
			lexTypeSemicolon     = textlexer.LexemeType("SEMICOLON")
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
			{lexTypeReservedWord, "SELECT"},
			{lexTypeStar, "*"},
			{lexTypeReservedWord, "FROM"},
			{lexTypeWord, "clients"},
			{lexTypeReservedWord, "AS"},
			{lexTypeWord, "c"},
			{lexTypeReservedWord, "WHERE"},
			{lexTypeWord, "name"},
			{lexTypeReservedWord, "LIKE"},
			{lexTypeStringLiteral, "'%john%'"},
			{lexTypeReservedWord, "AND"},
			{lexTypeWord, "birthdate"},
			{lexTypeSymbol, ">"},
			{lexTypeStringLiteral, "'1990-01-01'"},
			{lexTypeReservedWord, "ORDER BY"},
			{lexTypeWord, "id"},
			{lexTypeWord, "ASC"},
			{lexTypeSemicolon, ";"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexTypeWhitespace, rules.Whitespace)
		lx.MustAddRule(lexTypeWord, rules.Word)

		lx.MustAddRule(lexTypeReservedWord, rules.NewMatchAnyOf(
			rules.NewCaseInsensitiveLiteralMatch("SELECT"),
			rules.NewCaseInsensitiveLiteralMatch("FROM"),
			rules.NewCaseInsensitiveLiteralMatch("AS"),
			rules.NewCaseInsensitiveLiteralMatch("WHERE"),
			rules.NewCaseInsensitiveLiteralMatch("LIKE"),
			rules.NewCaseInsensitiveLiteralMatch("AND"),
			rules.Compose(
				rules.NewCaseInsensitiveLiteralMatch("ORDER"),
				rules.Whitespace,
				rules.NewCaseInsensitiveLiteralMatch("BY"),
			),
		))

		lx.MustAddRule(lexTypeStringLiteral, rules.SingleQuotedString)
		lx.MustAddRule(lexTypeStar, rules.Star)
		lx.MustAddRule(lexTypeSymbol, rules.RAngle)
		lx.MustAddRule(lexTypeSemicolon, rules.Semicolon)

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

			if lex.Type == lexTypeWhitespace {
				continue
			}

			t.Logf("%v", lex)

			assert.Equal(t, out[matches].Type, lex.Type)
			assert.Equal(t, out[matches].Text, lex.Text())

			matches++
		}

		assert.True(t, seen[lexTypeReservedWord])
		assert.True(t, seen[lexTypeWord])
		assert.True(t, seen[lexTypeStringLiteral])
		assert.True(t, seen[lexTypeStar])
		assert.True(t, seen[lexTypeSymbol])
		assert.True(t, seen[lexTypeSemicolon])
	})
}

func TestSpecificCases(t *testing.T) {
	t.Run("disambiguation", func(t *testing.T) {
		in := `ABCDEMANL1.23ABDEGHI`

		const (
			lexType1 = textlexer.LexemeType("TYPE-1")
			lexType2 = textlexer.LexemeType("TYPE-2")
			lexType3 = textlexer.LexemeType("TYPE-3")
			lexType4 = textlexer.LexemeType("TYPE-4")
		)

		out := []struct {
			Type textlexer.LexemeType
			Text string
		}{
			{lexType1, "ABC"},
			{lexType4, "GHI"},
		}

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexType1, rules.NewCaseInsensitiveLiteralMatch("ABC"))
		lx.MustAddRule(lexType2, rules.NewCaseInsensitiveLiteralMatch("ABCDEF"))
		lx.MustAddRule(lexType3, rules.NewCaseInsensitiveLiteralMatch("DEF"))
		lx.MustAddRule(lexType4, rules.NewCaseInsensitiveLiteralMatch("GHI"))

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

		assert.True(t, seen[lexType1])
		assert.True(t, seen[lexType4])
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
			lexType = textlexer.LexemeType("A")
		)

		in := `2hacgh6bAks3aSklvVVa1aaa`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexType, rules.NewCaseInsensitiveLiteralMatch("a"))

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
			if lex.Type == lexType {
				assert.Equal(t, "a", strings.ToLower(lex.Text()))
			}
		}

		assert.True(t, seen[lexType])
	})

	t.Run("pair of AA", func(t *testing.T) {

		const (
			lexType = textlexer.LexemeType("AA")
		)

		in := `xxxaxxaxaxaaaaxaxaxaxaaaxaaaaxaaaaaa`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexType, rules.NewCaseInsensitiveLiteralMatch("AA"))

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
			if lex.Type == lexType {
				assert.Equal(t, "aa", strings.ToLower(lex.Text()))
			}
		}

		assert.True(t, seen[lexType])
	})

	t.Run("always reject", func(t *testing.T) {
		const (
			lexType = textlexer.LexemeType("TYPE_1")
		)

		in := `abcdef`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexType, rules.AlwaysReject)

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
			lexType = textlexer.LexemeType("TYPE_1")
		)

		in := `abcdef`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexType, rules.AlwaysAccept)

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

		assert.True(t, seen[lexType])
	})

	t.Run("always continue", func(t *testing.T) {
		const (
			lexType = textlexer.LexemeType("TYPE_1")
		)

		in := `abcdef`

		lx := textlexer.New(strings.NewReader(in))

		lx.MustAddRule(lexType, rules.AlwaysContinue)

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
		lexTypeChaos1 = textlexer.LexemeType("CHAOS-1")
		lexTypeChaos2 = textlexer.LexemeType("CHAOS-2")
		lexTypeChaos3 = textlexer.LexemeType("CHAOS-3")
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

	lx.MustAddRule(lexTypeChaos1, chaosRule)
	lx.MustAddRule(lexTypeChaos2, chaosRule)
	lx.MustAddRule(lexTypeChaos3, chaosRule)

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

	assert.True(t, seen[lexTypeChaos1])
	assert.True(t, seen[lexTypeChaos2])
	assert.True(t, seen[lexTypeChaos3])
}
