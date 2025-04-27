package rules_test

import (
	"fmt"
	stdlog "log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

const outOfControlLimit = 1e2

type inputAndMatchesCase struct {
	Input       string
	Matches     []string
	Description string
}

func TestMatchUnsignedInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "1",
			Matches:     []string{"1"},
			Description: "Single digit",
		},
		{
			Input:       " 2",
			Matches:     []string{"2"},
			Description: "Digit with leading space",
		},
		{
			Input:       "1a",
			Matches:     []string{"1"},
			Description: "Digit followed by letter",
		},
		{
			Input:       "123",
			Matches:     []string{"123"},
			Description: "Multi-digit number",
		},
		{
			Input:       "0",
			Matches:     []string{"0"},
			Description: "Zero",
		},
		{
			Input:       "-",
			Matches:     nil,
			Description: "Minus sign only",
		},
		{
			Input:       "-1",
			Matches:     []string{"1"},
			Description: "Negative number (should match only digit)",
		},
		{
			Input:       "0001",
			Matches:     []string{"0001"},
			Description: "Number with leading zeros",
		},
		{
			Input:       "12 34 567 8.9",
			Matches:     []string{"12", "34", "567", "8", "9"},
			Description: "Multiple numbers with separators",
		},
		{
			Input:       "aaa123.#1113.123",
			Matches:     []string{"123", "1113", "123"},
			Description: "Numbers embedded in non-numeric text",
		},
		{
			Input:       "+1",
			Matches:     []string{"1"},
			Description: "Positive sign (should match only digit)",
		},
		{
			Input:       "1.",
			Matches:     []string{"1"},
			Description: "Digit followed by dot",
		},
		{
			Input:       "1..2",
			Matches:     []string{"1", "2"},
			Description: "Digits separated by multiple dots",
		},
		{
			Input:       "99999999999999999999999999999999999999999999999999",
			Matches:     []string{"99999999999999999999999999999999999999999999999999"},
			Description: "Extremely long unsigned integer",
		},
	}
	runTestInputAndMatches(t, "MatchUnsignedInteger", testCases, rules.MatchUnsignedInteger)
}

func TestMatchSignedInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "1",
			Matches:     []string{"1"},
			Description: "Positive number without sign",
		},
		{
			Input:       "-1",
			Matches:     []string{"-1"},
			Description: "Negative number",
		},
		{
			Input:       "- a  1",
			Matches:     []string{"1"},
			Description: "Minus sign separated from number by non-whitespace",
		},
		{
			Input:       "-   1",
			Matches:     []string{"-   1"},
			Description: "Minus sign with whitespace before number",
		},
		{
			Input:       "- \n  1",
			Matches:     []string{"- \n  1"},
			Description: "Minus sign with newline before number",
		},
		{
			Input:       "+123.+ 456 78 - 10",
			Matches:     []string{"+123", "+ 456", "78", "- 10"},
			Description: "Multiple signed numbers with separators",
		},
		{
			Input:       "0.",
			Matches:     []string{"0"},
			Description: "Zero with decimal point",
		},
		{
			Input:       "+  \n 0. 455",
			Matches:     []string{"+  \n 0", "455"},
			Description: "Signed zero with whitespace and newline",
		},
		{
			Input:       "-",
			Matches:     nil,
			Description: "Minus sign only",
		},
		{
			Input:       "-1-2-3-4--5",
			Matches:     []string{"-1", "-2", "-3", "-4", "-5"},
			Description: "Sequence of negative numbers without separators",
		},
		{
			Input:       "+",
			Matches:     nil,
			Description: "Plus sign only",
		},
		{
			Input:       "++1",
			Matches:     []string{"+1"},
			Description: "Double plus sign",
		},
		{
			Input:       "--1",
			Matches:     []string{"-1"},
			Description: "Double minus sign",
		},
		{
			Input:       "+-1",
			Matches:     []string{"-1"},
			Description: "Plus minus sign",
		},
		{
			Input:       "-+1",
			Matches:     []string{"+1"},
			Description: "Minus plus sign",
		},
		{
			Input:       "+000",
			Matches:     []string{"+000"},
			Description: "Signed zero with leading zeros",
		},
		{
			Input:       "-0",
			Matches:     []string{"-0"},
			Description: "Negative zero",
		},
	}

	runTestInputAndMatches(t, "MatchSignedInteger", testCases, rules.MatchSignedInteger)
}

func TestMatchUnsignedFloat(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "1",
			Matches:     nil,
			Description: "Integer (should not match float)",
		},
		{
			Input:       "123",
			Matches:     nil,
			Description: "Multi-digit integer (should not match float)",
		},
		{
			Input:       "0",
			Matches:     nil,
			Description: "Zero (should not match float)",
		},
		{
			Input:       "-1.0",
			Matches:     []string{"1.0"},
			Description: "Negative float (should match only the numeric part)",
		},
		{
			Input:       "-01",
			Matches:     nil,
			Description: "Negative integer with leading zero (should not match float)",
		},
		{
			Input:       "aaaa0.1xxxx",
			Matches:     []string{"0.1"},
			Description: "Float embedded in text",
		},
		{
			Input:       "0.001",
			Matches:     []string{"0.001"},
			Description: "Float with leading zero",
		},
		{
			Input:       "123.",
			Matches:     nil,
			Description: "Number with trailing decimal point (not a valid float)",
		},
		{
			Input:       "123 .45 6",
			Matches:     []string{".45"},
			Description: "Decimal part separated from whole number",
		},
		{
			Input:       "123.0",
			Matches:     []string{"123.0"},
			Description: "Float with zero decimal",
		},
		{
			Input:       "123.45",
			Matches:     []string{"123.45"},
			Description: "Standard float",
		},
		{
			Input:       "123.456",
			Matches:     []string{"123.456"},
			Description: "Float with multiple decimal digits",
		},
		{
			Input:       "ajt.23",
			Matches:     []string{".23"},
			Description: "Decimal part after non-numeric text",
		},
		{
			Input:       "ssdddd123.456.76755",
			Matches:     []string{"123.456", ".76755"},
			Description: "Multiple floats with text",
		},
		{
			Input:       ".",
			Matches:     nil,
			Description: "Dot only",
		},
		{
			Input:       "..1",
			Matches:     []string{".1"},
			Description: "Double dot before digit",
		},
		{
			Input:       "1..2",
			Matches:     []string{".2"},
			Description: "Digit followed by double dot and digit",
		},
		{
			Input:       "1.a",
			Matches:     nil,
			Description: "Digit, dot, letter",
		},
		{
			Input:       ".a",
			Matches:     nil,
			Description: "Dot, letter",
		},
		{
			Input:       "-.1",
			Matches:     []string{".1"},
			Description: "Negative sign before float without leading zero",
		},
		{
			Input:       "+.1",
			Matches:     []string{".1"},
			Description: "Positive sign before float without leading zero",
		},
		{
			Input:       "1.2.3",
			Matches:     []string{"1.2", ".3"},
			Description: "Multiple decimal points",
		},
	}

	runTestInputAndMatches(t, "MatchUnsignedFloat", testCases, rules.MatchUnsignedFloat)
}

func TestMatchSignedFloat(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "1.1a",
			Matches:     []string{"1.1"},
			Description: "Float followed by letter",
		},
		{
			Input:       "a+45.222",
			Matches:     []string{"+45.222"},
			Description: "Positive float with text prefix",
		},
		{
			Input:       "-134.1",
			Matches:     []string{"-134.1"},
			Description: "Negative float",
		},
		{
			Input:       "aBCd0-1234.444EDFT11",
			Matches:     []string{"-1234.444"},
			Description: "Negative float embedded in mixed-case text",
		},
		{
			Input:       "-  134.1",
			Matches:     []string{"-  134.1"},
			Description: "Negative float with whitespace after sign",
		},
		{
			Input:       "  + 134.1 ",
			Matches:     []string{"+ 134.1"},
			Description: "Positive float with whitespace after sign and surrounding spaces",
		},
		{
			Input:       "+.",
			Matches:     nil,
			Description: "Plus dot only",
		},
		{
			Input:       "-.",
			Matches:     nil,
			Description: "Minus dot only",
		},
		{
			Input:       "+.1",
			Matches:     []string{"+.1"},
			Description: "Positive float without leading zero",
		},
		{
			Input:       "-.1",
			Matches:     []string{"-.1"},
			Description: "Negative float without leading zero",
		},
		{
			Input:       "+ .1",
			Matches:     []string{"+ .1"},
			Description: "Plus space dot digit",
		},
		{
			Input:       "- .1",
			Matches:     []string{"- .1"},
			Description: "Minus space dot digit",
		},
		{
			Input:       "++1.0",
			Matches:     []string{"+1.0"},
			Description: "Double plus sign float",
		},
		{
			Input:       "--1.0",
			Matches:     []string{"-1.0"},
			Description: "Double minus sign float",
		},
		{
			Input:       "-0.0",
			Matches:     []string{"-0.0"},
			Description: "Negative zero float",
		},
		{
			Input:       "+0.0",
			Matches:     []string{"+0.0"},
			Description: "Positive zero float",
		},
		{
			Input:       "-.0",
			Matches:     []string{"-.0"},
			Description: "Negative zero float without leading zero digit",
		},
		{
			Input:       "+.0",
			Matches:     []string{"+.0"},
			Description: "Positive zero float without leading zero digit",
		},
	}

	runTestInputAndMatches(t, "MatchSignedFloat", testCases, rules.MatchSignedFloat)
}

func TestNumeric(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "1",
			Matches:     []string{"1"},
			Description: "Integer",
		},
		{
			Input:       "1.23",
			Matches:     []string{"1.23"},
			Description: "Float",
		},
		{
			Input:       "  1 . 23. 21.0",
			Matches:     []string{"1", "23.", "21.0"},
			Description: "Multiple numbers with whitespace and decimal points",
		},
		{
			Input:       "1.23.45",
			Matches:     []string{"1.23", ".45"},
			Description: "Float followed by decimal part",
		},
		{
			Input:       "+1.23.45",
			Matches:     []string{"+1.23", ".45"},
			Description: "Signed float followed by decimal part",
		},
		{
			Input:       "-1.23+ \t.45",
			Matches:     []string{"-1.23", "+ \t.45"},
			Description: "Negative float followed by positive decimal with whitespace",
		},
		{
			Input:       " -   1 ",
			Matches:     []string{"-   1"},
			Description: "Negative integer with whitespace",
		},
		{
			Input:       "-   1.23",
			Matches:     []string{"-   1.23"},
			Description: "Negative float with whitespace after sign",
		},
		{
			Input:       ".",
			Matches:     nil,
			Description: "Dot only",
		},
		{
			Input:       "+",
			Matches:     nil,
			Description: "Plus only",
		},
		{
			Input:       "-",
			Matches:     nil,
			Description: "Minus only",
		},
		{
			Input:       "1.",
			Matches:     []string{"1."},
			Description: "Numeric with trailing dot",
		},
		{
			Input:       "-1.",
			Matches:     []string{"-1."},
			Description: "Negative numeric with trailing dot",
		},
		{
			Input:       "-.1",
			Matches:     []string{"-.1"},
			Description: "Negative float starting with dot",
		},
		{
			Input:       "+.1",
			Matches:     []string{"+.1"},
			Description: "Positive float starting with dot",
		},
	}

	runTestInputAndMatches(t, "Numeric", testCases, rules.MatchSignedNumeric)
}

func TestMatchUnsignedNumeric(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "1",
			Matches:     []string{"1"},
			Description: "Integer",
		},
		{
			Input:       "1.23",
			Matches:     []string{"1.23"},
			Description: "Float",
		},
		{
			Input:       "  1 . 23. 21.0",
			Matches:     []string{"1", "23.", "21.0"},
			Description: "Multiple numbers with whitespace and decimal points",
		},
		{
			Input:       "1.23.45",
			Matches:     []string{"1.23", ".45"},
			Description: "Float followed by decimal part",
		},
		{
			Input:       "1.23+    .45",
			Matches:     []string{"1.23", ".45"},
			Description: "Float followed by decimal part with plus sign and whitespace",
		},
		{
			Input:       " -   1 ",
			Matches:     []string{"1"},
			Description: "Negative integer (should match only the digit)",
		},
		{
			Input:       "-   1.23",
			Matches:     []string{"1.23"},
			Description: "Negative float (should match only the numeric part)",
		},
		{
			Input:       ".",
			Matches:     nil,
			Description: "Dot only",
		},
		{
			Input:       "+1",
			Matches:     []string{"1"},
			Description: "Positive integer (matches only digit)",
		},
		{
			Input:       "+.1",
			Matches:     []string{".1"},
			Description: "Positive float starting with dot (matches only float part)",
		},
		{
			Input:       "1.",
			Matches:     []string{"1."},
			Description: "Float with trailing dot",
		},
	}

	runTestInputAndMatches(t, "MatchUnsignedNumeric", testCases, rules.MatchUnsignedNumeric)
}

func TestWhitespace(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       " ",
			Matches:     []string{" "},
			Description: "Single space",
		},
		{
			Input:       "\t",
			Matches:     []string{"\t"},
			Description: "Tab character",
		},
		{
			Input:       "\n",
			Matches:     []string{"\n"},
			Description: "Newline character",
		},
		{
			Input:       "q \t\nq",
			Matches:     []string{" \t\n"},
			Description: "Mixed whitespace between characters",
		},
		{
			Input:       "a b c \n d",
			Matches:     []string{" ", " ", " \n "},
			Description: "Words separated by whitespace",
		},
		{
			Input:       "\r",
			Matches:     []string{"\r"},
			Description: "Carriage return",
		},
		{
			Input:       "\r\n",
			Matches:     []string{"\r\n"},
			Description: "CRLF newline",
		},
		{
			Input:       " \t\r\n \t",
			Matches:     []string{" \t\r\n \t"},
			Description: "Complex mix of whitespace",
		},
		{
			Input:       "\u00a0",
			Matches:     nil,
			Description: "Non-breaking space (unicode)",
		},
	}

	runTestInputAndMatches(t, "Whitespace", testCases, rules.MatchWhitespace)
}

func TestNewMatchInvertedRule(t *testing.T) {
	t.Skip("Skipping test for NewMatchInvertedRule")

	t.Run("InvertWhitespace", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       " ",
				Matches:     nil,
				Description: "Single space (should not match when inverted)",
			},
			{
				Input:       "abc\t \t\t\nBCD",
				Matches:     []string{"abc", "BCD"},
				Description: "Text separated by whitespace",
			},
			{
				Input:       "qaaaa \t\nqa",
				Matches:     []string{"qaaaa", "qa"},
				Description: "Words separated by mixed whitespace",
			},
			{
				Input:       "a b c \n d",
				Matches:     []string{"a", "b", "c", "d"},
				Description: "Single characters separated by whitespace",
			},
			{
				Input:       " leading",
				Matches:     []string{"leading"},
				Description: "Invert whitespace with leading space",
			},
			{
				Input:       "trailing ",
				Matches:     []string{"trailing"},
				Description: "Invert whitespace with trailing space",
			},
			{
				Input:       "nospaces",
				Matches:     []string{"nospaces"},
				Description: "Invert whitespace with no spaces",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.MatchWhitespace)
		runTestInputAndMatches(t, "InvertWhitespace", testCases, invertRule)
	})

	t.Run("InvertMatchSignedInteger", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "1",
				Matches:     nil,
				Description: "Integer (should not match when inverted)",
			},
			{
				Input:       "-1a",
				Matches:     []string{"a"},
				Description: "Negative number followed by letter (should match only letter)",
			},
			{
				Input:       "t-   \n\n\n 1ea",
				Matches:     []string{"t", "ea"},
				Description: "Text around a negative number with whitespace",
			},
			{
				Input:       "-  a1e",
				Matches:     []string{"-  a", "e"},
				Description: "Minus sign with text and digit",
			},
			{
				Input:       "+",
				Matches:     []string{"+"},
				Description: "Invert signed int with plus only",
			},
			{
				Input:       "abc",
				Matches:     []string{"abc"},
				Description: "Invert signed int with text only",
			},
			{
				Input:       "a+1b-2c",
				Matches:     []string{"a", "b", "c"},
				Description: "Invert signed int with mixed text and numbers",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.MatchSignedInteger)
		runTestInputAndMatches(t, "InvertMatchSignedInteger", testCases, invertRule)
	})

	t.Run("InvertMatchSignedFloat", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       " ",
				Matches:     []string{" "},
				Description: "Single space",
			},
			{
				Input:       "-12.34",
				Matches:     nil,
				Description: "Negative float (should not match when inverted)",
			},
			{
				Input:       "A-12.34B",
				Matches:     []string{"A", "B"},
				Description: "Text around a negative float",
			},
			{
				Input:       "ABC-12.34DEF-12.0HIJ",
				Matches:     []string{"ABC", "DEF", "HIJ"},
				Description: "Text between multiple negative floats",
			},
			{
				Input:       "ABC-123ABC-123.abc-123.4ABC",
				Matches:     []string{"ABC", "-123ABC", "-123.abc", "ABC"},
				Description: "Complex mix of text, integers, and floats",
			},
			{
				Input:       "aBCd0-1234.444EDFT11",
				Matches:     []string{"aBCd", "0", "EDFT", "11"},
				Description: "Mixed case text with numbers and a negative float",
			},
			{
				Input:       ".",
				Matches:     []string{"."},
				Description: "Invert float with dot only",
			},
			{
				Input:       "1.",
				Matches:     []string{"1."},
				Description: "Invert float with integer and trailing dot",
			},
			{
				Input:       "a.1",
				Matches:     []string{"a"},
				Description: "Invert float with letter dot digit",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.MatchSignedFloat)
		runTestInputAndMatches(t, "InvertMatchSignedFloat", testCases, invertRule)
	})

	t.Run("InvertLiteralMatch", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       " ",
				Matches:     []string{" "},
				Description: "Single space",
			},
			{
				Input:       "abc",
				Matches:     nil,
				Description: "Exact match (should not match when inverted)",
			},
			{
				Input:       "ABC",
				Matches:     []string{"ABC"},
				Description: "Different case (should match when inverted)",
			},
			{
				Input:       "ABCabc",
				Matches:     []string{"ABC"},
				Description: "Different case followed by exact match",
			},
			{
				Input:       "ABC abc",
				Matches:     []string{"ABC "},
				Description: "Different case, space, and exact match",
			},
			{
				Input:       "ab",
				Matches:     []string{"ab"},
				Description: "Invert literal with prefix input",
			},
			{
				Input:       "abcd",
				Matches:     []string{"d"},
				Description: "Invert literal with suffix input",
			},
			{
				Input:       "xabcyabcz",
				Matches:     []string{"x", "y", "z"},
				Description: "Invert literal with surrounding chars",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.NewMatchString("abc"))
		runTestInputAndMatches(t, "InvertLiteralMatch", testCases, invertRule)
	})

	t.Run("InvertCaselessLiteralMatch", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       " ",
				Matches:     []string{" "},
				Description: "Single space",
			},
			{
				Input:       "abc",
				Matches:     nil,
				Description: "Lowercase match (should not match when inverted)",
			},
			{
				Input:       "ABC",
				Matches:     nil,
				Description: "Uppercase match (should not match when inverted)",
			},
			{
				Input:       "ABCabdef",
				Matches:     []string{"ab", "def"},
				Description: "Partial match with suffix",
			},
			{
				Input:       "ABC abc",
				Matches:     []string{" "},
				Description: "Space between two matches",
			},
			{
				Input:       "ABCabcABC",
				Matches:     nil,
				Description: "Multiple matches with no non-matching text",
			},
			{
				Input:       "ABCabc124ABCabcAdBefC",
				Matches:     []string{"124", "AdBefC"},
				Description: "Complex mix of matching and non-matching text",
			},
			{
				Input:       "xABCyabcz",
				Matches:     []string{"x", "y", "z"},
				Description: "Invert caseless literal with surrounding chars",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.NewMatchStringIgnoreCase("abc"))
		runTestInputAndMatches(t, "InvertCaselessLiteralMatch", testCases, invertRule)
	})

	t.Run("InvertInvertedMatchSignedFloat", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       " ",
				Matches:     nil,
				Description: "Single space (should not match double-inverted float)",
			},
			{
				Input:       "123.4",
				Matches:     []string{"123.4"},
				Description: "Simple float",
			},
			{
				Input:       "abc123.4def",
				Matches:     []string{"123.4"},
				Description: "Float surrounded by text",
			},
			{
				Input:       "abc123.4def123.4",
				Matches:     []string{"123.4", "123.4"},
				Description: "Multiple floats with text",
			},
			{
				Input:       "abc- 123.4",
				Matches:     []string{"- 123.4"},
				Description: "Negative float with space after sign",
			},
			{
				Input:       "+.1",
				Matches:     []string{"+.1"},
				Description: "Double invert float with positive float starting with dot",
			},
			{
				Input:       "-.1",
				Matches:     []string{"-.1"},
				Description: "Double invert float with negative float starting with dot",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.NewMatchInvertedRule(rules.MatchSignedFloat))
		runTestInputAndMatches(t, "InvertInvertedMatchSignedFloat", testCases, invertRule)
	})
}

func TestWord(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "hello world",
			Matches:     []string{"hello", "world"},
			Description: "Simple two-word phrase",
		},
		{
			Input:       "hello world\n",
			Matches:     []string{"hello", "world"},
			Description: "Two words with trailing newline",
		},
		{
			Input:       "a n C  \t d ....../*. Ef \"G123        AA.BB.CC",
			Matches:     []string{"a", "n", "C", "d", "Ef", "G123", "AA", "BB", "CC"},
			Description: "Complex mix of words, symbols, and punctuation",
		},
		{
			Input:       "Build simple, secure, scalable systems with Go",
			Matches:     []string{"Build", "simple", "secure", "scalable", "systems", "with", "Go"},
			Description: "Sentence with commas",
		},
		{
			Input:       "a-b-c",
			Matches:     []string{"a", "b", "c"},
			Description: "Words separated by hyphens",
		},
		{
			Input:       "/abc123.",
			Matches:     []string{"abc123"},
			Description: "Word with surrounding punctuation",
		},
		{
			Input:       "123 456",
			Matches:     nil,
			Description: "Numbers as words",
		},
		{
			Input:       "word_with_underscore",
			Matches:     []string{"word", "with", "underscore"},
			Description: "Word with underscore",
		},
		{
			Input:       "word123",
			Matches:     []string{"word123"},
			Description: "Word with trailing digits",
		},
		{
			Input:       "123word",
			Matches:     []string{"word"},
			Description: "Word with leading digits",
		},
		{
			Input:       " a ",
			Matches:     []string{"a"},
			Description: "Single letter word",
		},
	}

	runTestInputAndMatches(t, "MatchIdentifier", testCases, rules.MatchIdentifier)
}

func TestMatchDoubleQuotedString(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       ``,
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       `"`,
			Matches:     nil,
			Description: "Unclosed quote",
		},
		{
			Input:       `""`,
			Matches:     []string{`""`},
			Description: "Empty quoted string",
		},
		{
			Input:       `a"b"c`,
			Matches:     []string{`"b"`},
			Description: "Quoted string with surrounding text",
		},
		{
			Input:       `a"bcd`,
			Matches:     nil,
			Description: "Unclosed quote with text",
		},
		{
			Input:       "a b \" cdef \" ghji",
			Matches:     []string{`" cdef "`},
			Description: "Quoted string with spaces inside and outside",
		},
		{
			Input:       `aaa " aaaa aaaa \" aaaaaa`,
			Matches:     []string{`" aaaa aaaa \"`},
			Description: "Quoted string with potentially escaped quote",
		},
		{
			Input:       `"\""`,
			Matches:     []string{`"\"`},
			Description: "String with only escaped quote",
		},
		{
			Input:       `"\\"`,
			Matches:     []string{`"\\"`},
			Description: "String with escaped backslash",
		},
		{
			Input:       `"a\\b"`,
			Matches:     []string{`"a\\b"`},
			Description: "String with internal escaped backslash",
		},
		{
			Input:       `"a\"b"`,
			Matches:     []string{`"a\"`},
			Description: "String with internal escaped quote",
		},
		{
			Input:       `"String with 'single' quotes"`,
			Matches:     []string{`"String with 'single' quotes"`},
			Description: "Double quoted string containing single quotes",
		},
		{
			Input:       `"Adjacent""Strings"`,
			Matches:     []string{`"Adjacent"`, `"Strings"`},
			Description: "Adjacent double quoted strings",
		},
	}

	runTestInputAndMatches(t, "MatchDoubleQuotedString", testCases, rules.MatchDoubleQuotedString)
}

func TestMatchSingleQuotedString(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       ``,
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       `'`,
			Matches:     nil,
			Description: "Unclosed quote",
		},
		{
			Input:       `''`,
			Matches:     []string{`''`},
			Description: "Empty quoted string",
		},
		{
			Input:       `a'b'c`,
			Matches:     []string{`'b'`},
			Description: "Quoted string with surrounding text",
		},
		{
			Input:       `a'bcd`,
			Matches:     nil,
			Description: "Unclosed quote with text",
		},
		{
			Input:       "a b ' cdef ' ghji",
			Matches:     []string{`' cdef '`},
			Description: "Quoted string with spaces inside and outside",
		},
		{
			Input:       `aaa ' aaaa aaaa \' aaaaaa`,
			Matches:     []string{`' aaaa aaaa \'`},
			Description: "Quoted string with potentially escaped quote",
		},
		{
			Input:       `'\''`,
			Matches:     []string{`'\'`},
			Description: "String with only escaped quote",
		},
		{
			Input:       `'\\'`,
			Matches:     []string{`'\\'`},
			Description: "String with escaped backslash",
		},
		{
			Input:       `'a\\b'`,
			Matches:     []string{`'a\\b'`},
			Description: "String with internal escaped backslash",
		},
		{
			Input:       `'a\'b'`,
			Matches:     []string{`'a\'`},
			Description: "String with internal escaped quote",
		},
		{
			Input:       `'String with "double" quotes'`,
			Matches:     []string{`'String with "double" quotes'`},
			Description: "Single quoted string containing double quotes",
		},
		{
			Input:       `'Adjacent''Strings'`,
			Matches:     []string{`'Adjacent'`, `'Strings'`},
			Description: "Adjacent single quoted strings",
		},
	}

	runTestInputAndMatches(t, "MatchSingleQuotedString", testCases, rules.MatchSingleQuotedString)
}

func TestMatchEscapedDoubleQuotedString(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       ``,
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       `"`,
			Matches:     nil,
			Description: "Unclosed quote",
		},
		{
			Input:       `"\""`,
			Matches:     []string{`"\""`},
			Description: "String with escaped quote",
		},
		{
			Input:       `a"b\"\"c"c`,
			Matches:     []string{`"b\"\"c"`},
			Description: "String with multiple escaped quotes",
		},
		{
			Input:       `""`,
			Matches:     []string{`""`},
			Description: "Empty formatted string",
		},
		{
			Input:       `"\\"`,
			Matches:     []string{`"\\"`},
			Description: "Formatted string with escaped backslash",
		},
		{
			Input:       `"\n\t\r"`,
			Matches:     []string{`"\n\t\r"`},
			Description: "Formatted string with common escapes",
		},
		{
			Input:       `"\x41"`,
			Matches:     []string{`"\x41"`},
			Description: "Formatted string with hex escape",
		},
		{
			Input:       `"\u0041"`,
			Matches:     []string{`"\u0041"`},
			Description: "Formatted string with unicode escape (4 hex)",
		},
		{
			Input:       `"Invalid escape \z"`,
			Matches:     []string{`"Invalid escape \z"`},
			Description: "Formatted string with invalid escape",
		},
		{
			Input:       `"Ends with escape\"`,
			Matches:     nil,
			Description: "Formatted string ending mid-escape",
		},
	}

	runTestInputAndMatches(t, "MatchEscapedDoubleQuotedString", testCases, rules.MatchEscapedDoubleQuotedString)
}

func TestMatchInlineComment(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "aaaa //",
			Matches:     []string{"//"},
			Description: "Empty comment",
		},
		{
			Input:       "aaaa //\n",
			Matches:     []string{"//\n"},
			Description: "Empty comment with newline",
		},
		{
			Input:       "aaaa // eeee\t\n\n",
			Matches:     []string{"// eeee\t\n"},
			Description: "Comment with text and tab",
		},
		{
			Input:       "aaaa // // //\n\n\n",
			Matches:     []string{"// // //\n"},
			Description: "Comment containing comment syntax",
		},
		{
			Input:       "aaaaaaaa//bbbbbbbb\ncccc\n\nddd\n\n    // tttt",
			Matches:     []string{"//bbbbbbbb\n", "// tttt"},
			Description: "Multiple comments in multi-line text",
		},
		{
			Input:       "//",
			Matches:     []string{"//"},
			Description: "Comment only, no newline",
		},
		{
			Input:       "//\n",
			Matches:     []string{"//\n"},
			Description: "Comment only, with newline",
		},
		{
			Input:       "code//comment",
			Matches:     []string{"//comment"},
			Description: "Comment immediately after code, no newline",
		},
		{
			Input:       "code // comment with // nested slashes\n",
			Matches:     []string{"// comment with // nested slashes\n"},
			Description: "Comment containing nested slashes",
		},
		{
			Input:       "//\r\n",
			Matches:     []string{"//\r\n"},
			Description: "Comment followed by CRLF",
		},
		{
			Input:       "//\ra\rb\r\n//",
			Matches:     []string{"//\ra\rb\r\n", "//"},
			Description: "Comment with CR and text, followed by another comment",
		},
	}

	runTestInputAndMatches(t, "MatchInlineComment", testCases, rules.MatchInlineComment)
}

func TestMatchSlashStarComment(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "aaaa /*",
			Matches:     nil,
			Description: "Unclosed comment",
		},
		{
			Input:       "aaaa /* */",
			Matches:     []string{"/* */"},
			Description: "Empty block comment",
		},
		{
			Input:       "aaaa /* eeee\t\n\n",
			Matches:     nil,
			Description: "Unclosed multi-line comment",
		},
		{
			Input:       "aaaa /* eeee\t\n\n*/",
			Matches:     []string{"/* eeee\t\n\n*/"},
			Description: "Multi-line comment",
		},
		{
			Input:       "aaaa /* eeee\t\n\n*/xxxx",
			Matches:     []string{"/* eeee\t\n\n*/"},
			Description: "Multi-line comment with trailing text",
		},
		{
			Input:       "aa /* b\n\nb */ c\t\n\t\nc\n\n/* dd */",
			Matches:     []string{"/* b\n\nb */", "/* dd */"},
			Description: "Multiple block comments in multi-line text",
		},
		{
			Input:       "/**/",
			Matches:     []string{"/**/"},
			Description: "Empty block comment variation 1",
		},
		{
			Input:       "/***/",
			Matches:     []string{"/***/"},
			Description: "Empty block comment variation 2",
		},
		{
			Input:       "/* */ */",
			Matches:     []string{"/* */"},
			Description: "Closed comment followed by spurious end",
		},
		{
			Input:       "/* /* */ */",
			Matches:     []string{"/* /* */"},
			Description: "Nested block comment syntax (standard C behavior)",
		},
		{
			Input:       "code/**/code",
			Matches:     []string{"/**/"},
			Description: "Empty comment between code",
		},
		{
			Input:       "/* Comment with * / tricky sequence */",
			Matches:     []string{"/* Comment with * / tricky sequence */"},
			Description: "Comment containing space in potential end sequence",
		},
	}

	runTestInputAndMatches(t, "MatchSlashStarComment", testCases, rules.MatchSlashStarComment)
}

func TestLiteralMatch(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       " abc ",
			Matches:     []string{"abc"},
			Description: "Match with surrounding spaces",
		},
		{
			Input:       " abc",
			Matches:     []string{"abc"},
			Description: "Match with leading space",
		},
		{
			Input:       "abc.abc",
			Matches:     []string{"abc", "abc"},
			Description: "Multiple matches separated by period",
		},
		{
			Input:       "abcabc",
			Matches:     []string{"abc", "abc"},
			Description: "Adjacent matches",
		},
		{
			Input:       "aaabababcabc",
			Matches:     []string{"abc", "abc"},
			Description: "Matches with prefix text",
		},
		{
			Input:       "abaabababcaabababcccabc",
			Matches:     []string{"abc", "abc", "abc"},
			Description: "Multiple matches with complex surrounding text",
		},
		{
			Input:       "abc\nabc",
			Matches:     []string{"abc", "abc"},
			Description: "Matches separated by newline",
		},
		{
			Input:       "ab",
			Matches:     nil,
			Description: "Prefix only",
		},
		{
			Input:       "abcd",
			Matches:     []string{"abc"},
			Description: "Match is prefix of input",
		},
		{
			Input:       "ABC",
			Matches:     nil,
			Description: "Different case",
		},
	}

	matchDefKeywordRule := rules.NewMatchString("abc")
	runTestInputAndMatches(t, "LiteralMatch", testCases, matchDefKeywordRule)
}

func TestCaseInsensitiveLiteralMatch(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       " aBc ",
			Matches:     []string{"aBc"},
			Description: "Mixed-case match with surrounding spaces",
		},
		{
			Input:       " aBC",
			Matches:     []string{"aBC"},
			Description: "Mixed-case match with leading space",
		},
		{
			Input:       "abc.ABC",
			Matches:     []string{"abc", "ABC"},
			Description: "Different case matches separated by period",
		},
		{
			Input:       "ABCabc",
			Matches:     []string{"ABC", "abc"},
			Description: "Adjacent different case matches",
		},
		{
			Input:       "aaababABCabc",
			Matches:     []string{"ABC", "abc"},
			Description: "Different case matches with prefix text",
		},
		{
			Input:       "abaababABCaababABCccABC",
			Matches:     []string{"ABC", "ABC", "ABC"},
			Description: "Multiple matches with complex surrounding text",
		},
		{
			Input:       "ab",
			Matches:     nil,
			Description: "Prefix only (caseless)",
		},
		{
			Input:       "abcd",
			Matches:     []string{"abc"},
			Description: "Match is prefix of input (caseless)",
		},
		{
			Input:       "ABCD",
			Matches:     []string{"ABC"},
			Description: "Match is prefix of input uppercase (caseless)",
		},
	}

	matchDefKeywordRule := rules.NewMatchStringIgnoreCase("abc")
	runTestInputAndMatches(t, "CaseInsensitiveLiteralMatch", testCases, matchDefKeywordRule)
}

func TestAlways(t *testing.T) {
	t.Run("InvertReject", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     []string{"abc"},
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     []string{"abcdef"},
				Description: "Longer text",
			},
			{
				Input:       "a\nb\tc",
				Matches:     []string{"a\nb\tc"},
				Description: "InvertReject with whitespace",
			},
			{
				Input:       " ",
				Matches:     []string{" "},
				Description: "InvertReject with space only",
			},
		}
		runTestInputAndMatches(t, "InvertReject", testCases, rules.NewMatchInvertedRule(rules.RejectCurrentAndStop))
	})

	t.Run("InvertContinue", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     nil,
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     nil,
				Description: "Longer text",
			},
			{
				Input:       " ",
				Matches:     nil,
				Description: "InvertContinue with space",
			},
		}

		runTestInputAndMatches(t, "InvertContinue", testCases, rules.NewMatchInvertedRule(rules.MatchAnyCharacter))
	})

	t.Run("InvertAccept", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     nil,
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     nil,
				Description: "Longer text",
			},
			{
				Input:       " ",
				Matches:     nil,
				Description: "InvertAccept with space",
			},
		}
		runTestInputAndMatches(t, "InvertAccept", testCases, rules.NewMatchInvertedRule(rules.AcceptCurrentAndStop))
	})
}

func TestCompose(t *testing.T) {
	orderByRule := rules.NewMatchRuleSequence(
		rules.NewMatchStringIgnoreCase("ORDER"),
		rules.MatchWhitespace,
		rules.NewMatchStringIgnoreCase("BY"),
	)

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "ORDER \n BY",
			Matches:     []string{"ORDER \n BY"},
			Description: "ORDER BY with newline",
		},
		{
			Input:       " oRDER \n BY ",
			Matches:     []string{"oRDER \n BY"},
			Description: "Mixed-case ORDER BY with whitespace",
		},
		{
			Input:       "SELECT * FROM trades ORDER BY id DESC LIMIT 50;",
			Matches:     []string{"ORDER BY"},
			Description: "ORDER BY in SQL query",
		},
		{
			Input:       "ORDERBY",
			Matches:     nil,
			Description: "Compose without required whitespace",
		},
		{
			Input:       "ORDER  BY",
			Matches:     []string{"ORDER  BY"},
			Description: "Compose with multiple spaces",
		},
		{
			Input:       "ORDER\tBY",
			Matches:     []string{"ORDER\tBY"},
			Description: "Compose with tab whitespace",
		},
		{
			Input:       "order by",
			Matches:     []string{"order by"},
			Description: "Compose with lowercase",
		},
		{
			Input:       "ORDER",
			Matches:     nil,
			Description: "Compose first part only",
		},
		{
			Input:       "ORDER BYX",
			Matches:     []string{"ORDER BY"},
			Description: "Compose second part has extra char",
		},
		{
			Input:       "ORDER BY ORDER BY",
			Matches:     []string{"ORDER BY", "ORDER BY"},
			Description: "Adjacent composed matches",
		},
	}

	runTestInputAndMatches(t, "Compose", testCases, orderByRule)

	t.Run("Compose_Always", func(t *testing.T) {
		composeContinue := rules.NewMatchRuleSequence(
			rules.NewMatchString("A"),
			rules.MatchAnyCharacter,
			rules.NewMatchString("B"),
		)

		runTestInputAndMatches(
			t,
			"ComposeContinue",
			[]inputAndMatchesCase{
				{"AB", nil, "Compose with Continue"},
				{"A B", nil, "Compose with Continue space"},
			},
			composeContinue,
		)

		composeReject := rules.NewMatchRuleSequence(
			rules.NewMatchString("A"),
			rules.RejectCurrentAndStop,
			rules.NewMatchString("B"),
		)
		runTestInputAndMatches(t,
			"ComposeReject",
			[]inputAndMatchesCase{
				{"AB", nil, "Compose with Reject"},
				{"A B", nil, "Compose with Reject space"},
			},
			composeReject,
		)

		composeAccept := rules.NewMatchRuleSequence(
			rules.NewMatchString("A"),
			rules.AcceptCurrentAndStop,
		)

		runTestInputAndMatches(
			t,
			"ComposeAccept",
			[]inputAndMatchesCase{
				{"AB", []string{"AB"}, "Compose with B"},
				{"A", []string{"A\x00"}, "Compose with EOF"},
				{"a", nil, "Not a match"},
			}, composeAccept,
		)
	})
}

func TestAnyMatch(t *testing.T) {
	anyMatchRule := rules.NewMatchAnyRule(
		rules.NewMatchStringIgnoreCase("ABC"),
		rules.NewMatchStringIgnoreCase("DEF"),
		rules.NewMatchStringIgnoreCase("GHI"),
		rules.NewMatchStringIgnoreCase("ABCDEF"),
	)

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "abc",
			Matches:     []string{"abc"},
			Description: "Simple match (first rule)",
		},
		{
			Input:       "abcdeg defabcghiabc",
			Matches:     []string{"abc", "def", "abc", "ghi", "abc"},
			Description: "Multiple different matches",
		},
		{
			Input:       "ABCDEF",
			Matches:     []string{"ABC", "DEF"},
			Description: "Input matches longer rule, but shorter rule is first",
		},
		{
			Input:       "defabc",
			Matches:     []string{"def", "abc"},
			Description: "Adjacent matches of different rules",
		},
		{
			Input:       "ab",
			Matches:     nil,
			Description: "Prefix of a potential match",
		},
		{
			Input:       "abchi",
			Matches:     []string{"abc"},
			Description: "Match followed by partial match of another rule",
		},
		{
			Input:       "xyz",
			Matches:     nil,
			Description: "Input with no matching rules",
		},
		{
			Input:       " abc def ",
			Matches:     []string{"abc", "def"},
			Description: "Matches with surrounding spaces",
		},
		{
			Input:       "ABCDEFGHI",
			Matches:     []string{"ABC", "DEF", "GHI"},
			Description: "Concatenation of all matches",
		},
	}

	runTestInputAndMatches(t, "AnyMatch", testCases, anyMatchRule)

	t.Run("AnyMatch", func(t *testing.T) {
		firstRule := rules.NewMatchAnyRule(
			rules.NewMatchStringIgnoreCase("ABCDEF"),
			rules.NewMatchStringIgnoreCase("ABC"),
			rules.NewMatchStringIgnoreCase("DEF"),
		)
		testCasesLonger := []inputAndMatchesCase{
			{
				Input:   "ABCDEF",
				Matches: []string{"ABC", "DEF"},
			},
			{
				Input:       "abcdef",
				Matches:     []string{"abc", "def"},
				Description: "Lowercase longer rule listed first matches full string",
			},
			{
				Input:       "ABC DEF",
				Matches:     []string{"ABC", "DEF"},
				Description: "Shorter rules match when separated",
			},
		}

		runTestInputAndMatches(t, "AnyMatchLongerFirst", testCasesLonger, firstRule)
	})

	t.Run("AnyMatch_EmptyList", func(t *testing.T) {
		emptyAny := rules.NewMatchAnyRule()
		testCasesEmpty := []inputAndMatchesCase{
			{"abc", nil, "Empty rule list"},
			{"", nil, "Empty rule list empty input"},
		}
		runTestInputAndMatches(t, "AnyMatchEmpty", testCasesEmpty, emptyAny)
	})
}

func TestMatchIdentifierWithUnderscore(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "_variable",
			Matches:     []string{"_variable"},
			Description: "Identifier starting with underscore",
		},
		{
			Input:       "variable_name",
			Matches:     []string{"variable_name"},
			Description: "Identifier with underscore in middle",
		},
		{
			Input:       "var_",
			Matches:     []string{"var_"},
			Description: "Identifier ending with underscore",
		},
		{
			Input:       "_",
			Matches:     []string{"_"},
			Description: "Single underscore",
		},
		{
			Input:       "__private",
			Matches:     []string{"__private"},
			Description: "Double underscore prefix",
		},
		{
			Input:       "a_1_b_2",
			Matches:     []string{"a_1_b_2"},
			Description: "Identifier with mixed letters, numbers, and underscores",
		},
		{
			Input:       "1_invalid",
			Matches:     []string{"_invalid"},
			Description: "Starting with digit (should only match from underscore)",
		},
		{
			Input:       "var1 _var2",
			Matches:     []string{"var1", "_var2"},
			Description: "Multiple identifiers separated by space",
		},
		{
			Input:       "snake_case camelCase _private",
			Matches:     []string{"snake_case", "camelCase", "_private"},
			Description: "Different identifier styles",
		},
	}

	runTestInputAndMatches(t, "MatchIdentifierWithUnderscore", testCases, rules.MatchIdentifierWithUnderscore)
}

func TestMatchHexInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "0x1A",
			Matches:     []string{"0x1A"},
			Description: "Simple hex number",
		},
		{
			Input:       "0XFF",
			Matches:     []string{"0XFF"},
			Description: "Uppercase X and hex digits",
		},
		{
			Input:       "0x0",
			Matches:     []string{"0x0"},
			Description: "Hex zero",
		},
		{
			Input:       "0xabcdef",
			Matches:     []string{"0xabcdef"},
			Description: "Lowercase hex digits",
		},
		{
			Input:       "0xABCDEF",
			Matches:     []string{"0xABCDEF"},
			Description: "Uppercase hex digits",
		},
		{
			Input:       "0x123ABC",
			Matches:     []string{"0x123ABC"},
			Description: "Mixed digits and letters",
		},
		{
			Input:       "0x",
			Matches:     nil,
			Description: "Incomplete hex number",
		},
		{
			Input:       "0xG",
			Matches:     nil,
			Description: "Invalid hex digit",
		},
		{
			Input:       "0x1G",
			Matches:     []string{"0x1"},
			Description: "Valid hex followed by invalid character",
		},
		{
			Input:       "00x0x1 00xFF",
			Matches:     []string{"0x0", "0xFF"},
			Description: "Multiple hex numbers",
		},
		{
			Input:       "int value = 0xA0;",
			Matches:     []string{"0xA0"},
			Description: "Hex in code context",
		},
	}

	runTestInputAndMatches(t, "MatchHexInteger", testCases, rules.MatchHexInteger)
}

func TestMatchBinaryInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "0b1010",
			Matches:     []string{"0b1010"},
			Description: "Simple binary number",
		},
		{
			Input:       "0B11",
			Matches:     []string{"0B11"},
			Description: "Uppercase B",
		},
		{
			Input:       "0b0",
			Matches:     []string{"0b0"},
			Description: "Binary zero",
		},
		{
			Input:       "0b1111111",
			Matches:     []string{"0b1111111"},
			Description: "Longer binary number",
		},
		{
			Input:       "0b",
			Matches:     nil,
			Description: "Incomplete binary number",
		},
		{
			Input:       "0b2",
			Matches:     nil,
			Description: "Invalid binary digit",
		},
		{
			Input:       "0b10102",
			Matches:     []string{"0b1010"},
			Description: "Valid binary followed by invalid digit",
		},
		{
			Input:       "0b10 0b01",
			Matches:     []string{"0b10", "0b01"},
			Description: "Multiple binary numbers",
		},
		{
			Input:       "int mask = 0b1101;",
			Matches:     []string{"0b1101"},
			Description: "Binary in code context",
		},
	}

	runTestInputAndMatches(t, "MatchBinaryInteger", testCases, rules.MatchBinaryInteger)
}

func TestMatchOctalInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "0o12",
			Matches:     []string{"0o12"},
			Description: "Simple octal number",
		},
		{
			Input:       "0O7",
			Matches:     []string{"0O7"},
			Description: "Uppercase O",
		},
		{
			Input:       "0o0",
			Matches:     []string{"0o0"},
			Description: "Octal zero",
		},
		{
			Input:       "0o1234567",
			Matches:     []string{"0o1234567"},
			Description: "All valid octal digits",
		},
		{
			Input:       "0o",
			Matches:     nil,
			Description: "Incomplete octal number",
		},
		{
			Input:       "0o8",
			Matches:     nil,
			Description: "Invalid octal digit",
		},
		{
			Input:       "0o128",
			Matches:     []string{"0o12"},
			Description: "Valid octal followed by invalid digit",
		},
		{
			Input:       "0o12 0o34",
			Matches:     []string{"0o12", "0o34"},
			Description: "Multiple octal numbers",
		},
		{
			Input:       "int perm = 0o755;",
			Matches:     []string{"0o755"},
			Description: "Octal in code context",
		},
	}

	runTestInputAndMatches(t, "MatchOctalInteger", testCases, rules.MatchOctalInteger)
}

func TestMatchHashComment(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "#",
			Matches:     []string{"#"},
			Description: "Empty comment",
		},
		{
			Input:       "# This is a comment",
			Matches:     []string{"# This is a comment"},
			Description: "Simple comment",
		},
		{
			Input:       "#\n",
			Matches:     []string{"#\n"},
			Description: "Empty comment with newline",
		},
		{
			Input:       "# Comment\n",
			Matches:     []string{"# Comment\n"},
			Description: "Comment with newline",
		},
		{
			Input:       "code # Comment",
			Matches:     []string{"# Comment"},
			Description: "Comment after code",
		},
		{
			Input:       "# Comment 1\n# Comment 2",
			Matches:     []string{"# Comment 1\n", "# Comment 2"},
			Description: "Multiple comments",
		},
		{
			Input:       "# Comment with # inside",
			Matches:     []string{"# Comment with # inside"},
			Description: "Comment with hash inside",
		},
		{
			Input:       "# Comment with // inside",
			Matches:     []string{"# Comment with // inside"},
			Description: "Comment with other comment syntax inside",
		},
		{
			Input:       "#\r\n",
			Matches:     []string{"#\r\n"},
			Description: "Comment with CRLF",
		},
		{
			Input:       "#\ra\rb\r\n#",
			Matches:     []string{"#\ra\rb\r\n", "#"},
			Description: "Comment with CR and text, followed by another comment",
		},
	}

	runTestInputAndMatches(t, "MatchHashComment", testCases, rules.MatchHashComment)
}

func TestMatchExceptEOF(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "abc",
			Matches:     []string{"abc"},
			Description: "Simple text",
		},
		{
			Input:       "Multiple\nLines\nOf\nText",
			Matches:     []string{"Multiple\nLines\nOf\nText"},
			Description: "Multi-line text",
		},
		{
			Input:       "Text with special chars: !@#$%^&*()",
			Matches:     []string{"Text with special chars: !@#$%^&*()"},
			Description: "Text with special characters",
		},
	}

	runTestInputAndMatches(t, "MatchExceptEOF", testCases, rules.MatchExceptEOF)
}

func TestMatchExceptEOL(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "abc",
			Matches:     []string{"abc"},
			Description: "Simple text without newline",
		},
		{
			Input:       "line1\nline2",
			Matches:     []string{"line1\n", "line2"},
			Description: "Two lines",
		},
		{
			Input:       "line1\r\nline2",
			Matches:     []string{"line1\r\n", "line2"},
			Description: "Two lines with CRLF",
		},
		{
			Input:       "\n",
			Matches:     []string{"\n"},
			Description: "Empty line",
		},
		{
			Input:       "line with\ttab",
			Matches:     []string{"line with\ttab"},
			Description: "Line with tab",
		},
	}

	runTestInputAndMatches(t, "MatchExceptEOL", testCases, rules.MatchExceptEOL)
}

func TestMatchBasicMathOperator(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "+",
			Matches:     []string{"+"},
			Description: "Plus operator",
		},
		{
			Input:       "-",
			Matches:     []string{"-"},
			Description: "Minus operator",
		},
		{
			Input:       "*",
			Matches:     []string{"*"},
			Description: "Multiply operator",
		},
		{
			Input:       "/",
			Matches:     []string{"/"},
			Description: "Divide operator",
		},
		{
			Input:       "+-*/",
			Matches:     []string{"+", "-", "*", "/"},
			Description: "All operators in sequence",
		},
		{
			Input:       "a + b",
			Matches:     []string{"+"},
			Description: "Operator in expression",
		},
		{
			Input:       "a+b-c*d/e",
			Matches:     []string{"+", "-", "*", "/"},
			Description: "Operators in complex expression",
		},
		{
			Input:       "%",
			Matches:     nil,
			Description: "Non-basic math operator",
		},
	}

	runTestInputAndMatches(t, "MatchBasicMathOperator", testCases, rules.MatchBasicMathOperator)
}

func TestLogicalOperators(t *testing.T) {
	t.Run("MatchLogicalAnd", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "&&",
				Matches:     []string{"&&"},
				Description: "Logical AND operator",
			},
			{
				Input:       "a && b",
				Matches:     []string{"&&"},
				Description: "Logical AND in expression",
			},
			{
				Input:       "&",
				Matches:     nil,
				Description: "Single ampersand",
			},
		}
		runTestInputAndMatches(t, "MatchLogicalAnd", testCases, rules.MatchLogicalAnd)
	})

	t.Run("MatchLogicalOr", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "||",
				Matches:     []string{"||"},
				Description: "Logical OR operator",
			},
			{
				Input:       "a || b",
				Matches:     []string{"||"},
				Description: "Logical OR in expression",
			},
			{
				Input:       "|",
				Matches:     nil,
				Description: "Single pipe",
			},
		}
		runTestInputAndMatches(t, "MatchLogicalOr", testCases, rules.MatchLogicalOr)
	})
}

func TestArrowOperators(t *testing.T) {
	t.Run("MatchArrow", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "->",
				Matches:     []string{"->"},
				Description: "Arrow operator",
			},
			{
				Input:       "obj->method()",
				Matches:     []string{"->"},
				Description: "Arrow in method call",
			},
			{
				Input:       "-",
				Matches:     nil,
				Description: "Minus only",
			},
		}
		runTestInputAndMatches(t, "MatchArrow", testCases, rules.MatchArrow)
	})

	t.Run("MatchFatArrow", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "=>",
				Matches:     []string{"=>"},
				Description: "Fat arrow operator",
			},
			{
				Input:       "(x) => x + 1",
				Matches:     []string{"=>"},
				Description: "Fat arrow in lambda",
			},
			{
				Input:       "=",
				Matches:     nil,
				Description: "Equals only",
			},
		}
		runTestInputAndMatches(t, "MatchFatArrow", testCases, rules.MatchFatArrow)
	})
}

func TestSingleCharacterAcceptors(t *testing.T) {
	testSingleCharacter := func(name string, rule textlexer.Rule, char rune) {
		t.Run(name, func(t *testing.T) {
			testCases := []inputAndMatchesCase{
				{
					Input:       "",
					Matches:     nil,
					Description: "Empty input",
				},
				{
					Input:       string(char),
					Matches:     []string{string(char)},
					Description: "Single character",
				},
				{
					Input:       fmt.Sprintf("a%cb", char),
					Matches:     []string{string(char)},
					Description: "Character in context",
				},
				{
					Input:       fmt.Sprintf("%c%c%c", char, char, char),
					Matches:     []string{string(char), string(char), string(char)},
					Description: "Multiple characters",
				},
			}
			runTestInputAndMatches(t, name, testCases, rule)
		})
	}

	testSingleCharacter("AcceptLParen", rules.AcceptLParen, '(')
	testSingleCharacter("AcceptRParen", rules.AcceptRParen, ')')
	testSingleCharacter("AcceptLBrace", rules.AcceptLBrace, '{')
	testSingleCharacter("AcceptRBrace", rules.AcceptRBrace, '}')
	testSingleCharacter("AcceptLBracket", rules.AcceptLBracket, '[')
	testSingleCharacter("AcceptRBracket", rules.AcceptRBracket, ']')
	testSingleCharacter("AcceptLAngle", rules.AcceptLAngle, '<')
	testSingleCharacter("AcceptRAngle", rules.AcceptRAngle, '>')
	testSingleCharacter("AcceptComma", rules.AcceptComma, ',')
	testSingleCharacter("AcceptColon", rules.AcceptColon, ':')
	testSingleCharacter("AcceptSemicolon", rules.AcceptSemicolon, ';')
	testSingleCharacter("AcceptPeriod", rules.AcceptPeriod, '.')
	testSingleCharacter("AcceptPlus", rules.AcceptPlus, '+')
	testSingleCharacter("AcceptMinus", rules.AcceptMinus, '-')
	testSingleCharacter("AcceptStar", rules.AcceptStar, '*')
	testSingleCharacter("AcceptSlash", rules.AcceptSlash, '/')
	testSingleCharacter("AcceptPercent", rules.AcceptPercent, '%')
	testSingleCharacter("AcceptEqual", rules.AcceptEqual, '=')
	testSingleCharacter("AcceptExclamation", rules.AcceptExclamation, '!')
	testSingleCharacter("AcceptPipe", rules.AcceptPipe, '|')
	testSingleCharacter("AcceptAmpersand", rules.AcceptAmpersand, '&')
	testSingleCharacter("AcceptQuestionMark", rules.AcceptQuestionMark, '?')
}

func TestAcceptAnyParen(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "(",
			Matches:     []string{"("},
			Description: "Left parenthesis",
		},
		{
			Input:       ")",
			Matches:     []string{")"},
			Description: "Right parenthesis",
		},
		{
			Input:       "()",
			Matches:     []string{"(", ")"},
			Description: "Both parentheses",
		},
		{
			Input:       "a(b)c",
			Matches:     []string{"(", ")"},
			Description: "Parentheses in context",
		},
		{
			Input:       "[",
			Matches:     nil,
			Description: "Non-parenthesis bracket",
		},
	}
	runTestInputAndMatches(t, "AcceptAnyParen", testCases, rules.AcceptAnyParen)
}

func TestNewMatchExceptString(t *testing.T) {
	// Test matching until a specific string is found
	matchUntilEnd := rules.NewMatchExceptString("END", rules.AcceptCurrentAndStop)

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "END",
			Matches:     []string{"END"},
			Description: "Just the target string",
		},
		{
			Input:       "before END",
			Matches:     []string{"before END"},
			Description: "Text before target",
		},
		{
			Input:       "text with END in middle END again",
			Matches:     []string{"text with END", " in middle END"},
			Description: "Multiple occurrences of target",
		},
		{
			Input:       "text without target",
			Matches:     nil,
			Description: "No target string",
		},
		{
			Input:       "partial EN match",
			Matches:     nil,
			Description: "Partial match of target",
		},
		{
			Input:       "overlapping ENEN match",
			Matches:     nil,
			Description: "Overlapping potential matches",
		},
	}

	runTestInputAndMatches(t, "NewMatchExceptString", testCases, matchUntilEnd)
}

func TestNewMatchStartingWithString(t *testing.T) {
	// Test requiring input to start with a specific string
	matchStartingWithHello := rules.NewMatchStartingWithString(
		"Hello",
		func(r rune) (textlexer.Rule, textlexer.State) {
			stdlog.Printf("SECOND: %q", r)
			if rules.IsCommonWhitespace(r) || rules.IsEOF(r) {
				stdlog.Printf("SECOND: IS EOF or whitespace")
				return rules.PushBackCurrentAndAccept(r)
			}

			stdlog.Printf("SECOND: NOT EOF or whitespace")
			return rules.MatchUntilCommonWhitespaceOrEOF(r)
		},
	)

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "Hello",
			Matches:     []string{"Hello"},
			Description: "Just the prefix",
		},
		{
			Input:       "HelloWorld",
			Matches:     []string{"HelloWorld"},
			Description: "Prefix with continuation",
		},
		{
			Input:       "Hello World",
			Matches:     []string{"Hello"},
			Description: "Prefix followed by space",
		},
		{
			Input:       "Hi Hello",
			Matches:     []string{"Hello"},
			Description: "Prefix not at start",
		},
		{
			Input:       "Hell",
			Matches:     nil,
			Description: "Partial prefix",
		},
	}

	runTestInputAndMatches(t, "NewMatchStartingWithString", testCases, matchStartingWithHello)
}

func TestNewCharacterMatcher(t *testing.T) {
	// Test custom single character matcher
	matchAt := rules.NewCharacterMatcher('@')

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "@",
			Matches:     []string{"@"},
			Description: "Single @ character",
		},
		{
			Input:       "a@b",
			Matches:     []string{"@"},
			Description: "@ in context",
		},
		{
			Input:       "@@",
			Matches:     []string{"@", "@"},
			Description: "Multiple @ characters",
		},
		{
			Input:       "a",
			Matches:     nil,
			Description: "Different character",
		},
	}

	runTestInputAndMatches(t, "NewCharacterMatcher", testCases, matchAt)
}

func TestMatchUntilCommonWhitespaceOrEOF(t *testing.T) {
	// This rule has a specific behavior that should be tested
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input (EOF)",
		},
		{
			Input:       "   ",
			Matches:     nil,
			Description: "Just whitespace",
		},
		{
			Input:       "word ",
			Matches:     []string{"word"},
			Description: "Word followed by space",
		},
		{
			Input:       " word",
			Matches:     []string{"word"},
			Description: "Space followed by word",
		},
	}

	runTestInputAndMatches(t, "MatchUntilCommonWhitespaceOrEOF", testCases, rules.MatchUntilCommonWhitespaceOrEOF)
}

func TestNewMatchRuleSequence(t *testing.T) {

	// Test a more complex rule sequence
	complexSequence := rules.NewMatchRuleSequence(
		rules.MatchZeroOrMoreWhitespaces,
		rules.AcceptLParen,
		rules.MatchZeroOrMoreWhitespaces,
		rules.MatchIdentifier,
		rules.MatchZeroOrMoreWhitespaces,
		rules.AcceptComma,
		rules.MatchZeroOrMoreWhitespaces,
		rules.MatchUnsignedInteger,
		rules.MatchZeroOrMoreWhitespaces,
		rules.AcceptRParen,
		rules.MatchZeroOrMoreWhitespaces,
	)

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "(name,123)",
			Matches:     []string{"(name,123)"},
			Description: "Valid sequence",
		},
		{
			Input:       "(  name, 123  )",
			Matches:     []string{"(  name, 123  )"},
			Description: "Sequence with extra whitespace",
		},
		{
			Input:       "(name,)",
			Matches:     nil,
			Description: "Incomplete sequence",
		},
		{
			Input:       "name,123)",
			Matches:     nil,
			Description: "Missing start of sequence",
		},
		{
			Input:       "(name,123",
			Matches:     nil,
			Description: "Missing end of sequence",
		},
		{
			Input:       "(123,name)",
			Matches:     nil,
			Description: "Incorrect order in sequence",
		},
	}

	runTestInputAndMatches(t, "NewMatchRuleSequence_Complex", testCases, complexSequence)
}

/*
func TestNewMatchWithLookahead(t *testing.T) {
	// Test lookahead functionality with keyword boundaries
	// This tests if a word is followed by a non-identifier character
	keywordIf := rules.NewMatchWithLookahead(
		// Match the keyword "if"
		rules.NewMatchString("if"),

		// Accept if the next character is not a letter, digit, or underscore
		func(r rune) (textlexer.Rule, textlexer.State) {
			isASCIILetter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
			isASCIIDigit := r >= '0' && r <= '9'
			isUnderscore := r == '_'

			if isASCIILetter || isASCIIDigit || isUnderscore {
				return nil, textlexer.StateReject
			}

			return nil, textlexer.StateAccept
		},
	)

	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "if ",
			Matches:     []string{"if"},
			Description: "Keyword followed by space",
		},
		{
			Input:       "if(",
			Matches:     []string{"if"},
			Description: "Keyword followed by opening parenthesis",
		},
		{
			Input:       "if\n",
			Matches:     []string{"if"},
			Description: "Keyword followed by newline",
		},
		{
			Input:       "if;",
			Matches:     []string{"if"},
			Description: "Keyword followed by semicolon",
		},
		{
			Input:       "ifelse",
			Matches:     nil,
			Description: "Keyword as prefix of longer identifier",
		},
		{
			Input:       "ifififfi",
			Matches:     nil,
			Description: "Many keywords in sequence",
		},
		{
			Input:       "if./if#if(fi(",
			Matches:     []string{"if", "if", "if"},
			Description: "Many keywords in sequence",
		},
		{
			Input:       "prefix_if ",
			Matches:     []string{"if"},
			Description: "Keyword after prefix",
		},
		{
			Input:       "if_suffix",
			Matches:     nil,
			Description: "Keyword with suffix",
		},
		{
			Input:       "if (x == 0) return; else if (y > 0) continue;",
			Matches:     []string{"if", "if"},
			Description: "Multiple keywords in context",
		},
	}

	runTestInputAndMatches(t, "NewMatchWithLookahead", testCases, keywordIf)

	// Test lookahead with more complex patterns
	t.Run("LookaheadComplex", func(t *testing.T) {
		// Match a number only if followed by a unit (px, em, %)
		numberWithUnit := rules.NewMatchWithLookahead(
			rules.MatchUnsignedNumeric,

			rules.NewMatchAnyRule(
				rules.NewMatchString("px"),
				rules.NewMatchString("em"),
				rules.NewMatchString("%"),
			),
		)

		testCasesComplex := []inputAndMatchesCase{
			{
				Input:       "10px",
				Matches:     []string{"10"},
				Description: "Number with pixel unit",
			},
			{
				Input:       "1px 2px 3px",
				Matches:     []string{"1", "2", "3"},
				Description: "Numbers with pixel units",
			},
			{
				Input:       "px 1 2px px3 4px 5 6px px7 px",
				Matches:     []string{"2", "4", "6"},
				Description: "Numbers with pixel units",
			},
			{
				Input:       "1.5em",
				Matches:     []string{"1.5"},
				Description: "Float with em unit",
			},
			{
				Input:       "50%",
				Matches:     []string{"50"},
				Description: "Number with percent unit",
			},
			{
				Input:       "1 100",
				Matches:     nil,
				Description: "Number without unit",
			},
			{
				Input:       "10pt",
				Matches:     nil,
				Description: "Number with unsupported unit",
			},
			{
				Input:       "width: 20px; height: 30em; opacity: 75%;",
				Matches:     []string{"20", "30", "75"},
				Description: "Multiple measurements in CSS-like context",
			},
		}

		runTestInputAndMatches(t, "LookaheadComplex", testCasesComplex, numberWithUnit)
	})

	t.Run("Chained simple rules", func(t *testing.T) {
		// Match "a" only if followed by "b"
		chainedLookahead := rules.NewMatchWithLookahead(
			rules.NewMatchString("a"),
			rules.NewMatchString("b"),
		)

		testCasesNested := []inputAndMatchesCase{
			{
				Input:       "abc",
				Matches:     []string{"a"},
				Description: "Matching sequence",
			},
			{
				Input:       "ab",
				Matches:     []string{"a"},
				Description: "Incomplete sequence",
			},
			{
				Input:       "ba",
				Matches:     nil,
				Description: "Wrong final character",
			},
			{
				Input:       "adc",
				Matches:     nil,
				Description: "Wrong middle character",
			},
			{
				Input:       "abcabc aaa aaaaa aaaaaaa ba ba babab",
				Matches:     []string{"a", "a", "a", "a"},
				Description: "Multiple matching sequences",
			},
		}

		runTestInputAndMatches(t, "NestedLookahead", testCasesNested, chainedLookahead)
	})

	t.Run("Chained Lookahead", func(t *testing.T) {
		chainedLookahead1 := rules.NewMatchWithLookahead(
			rules.NewMatchString("abc"),
			rules.NewMatchString("123"),
		)

		runTestInputAndMatches(t,
			"chainedLookahead1",
			[]inputAndMatchesCase{
				{
					Input:       "abc123",
					Matches:     []string{"abc"},
					Description: "Matching sequence",
				},
			},
			chainedLookahead1,
		)

		chainedLookahead2 := rules.NewMatchWithLookahead(
			rules.NewMatchString("123"),
			rules.NewMatchString("def"),
		)

		runTestInputAndMatches(t,
			"chainedLookahead2",
			[]inputAndMatchesCase{
				{
					Input:       "123def",
					Matches:     []string{"123"},
					Description: "Matching sequence",
				},
			},
			chainedLookahead2,
		)

		chainedLookahead := rules.NewMatchWithLookahead(
			chainedLookahead1, // "abc" followed by "123"
			chainedLookahead2, // "123" followed by "def"
		)

		testCasesNested := []inputAndMatchesCase{
			{
				Input:       "yabc123defz",
				Matches:     []string{"abc"},
				Description: "Matching sequence",
			},
			{
				Input:       "abc123",
				Matches:     []string{},
				Description: "Incomplete sequence",
			},
			{
				Input:       "abce",
				Matches:     nil,
				Description: "Wrong final character",
			},
			{
				Input:       "abde",
				Matches:     nil,
				Description: "Wrong middle character",
			},
			{
				Input:       "abc123def abc123def",
				Matches:     []string{"abc", "abc"},
				Description: "Consecutive matching sequences",
			},
			{
				Input:       "abc123defxabc123defyabc123def",
				Matches:     []string{"abc", "abc", "abc"},
				Description: "Consecutive matching sequences",
			},
			{
				Input:       "zabc123defxabc123defyabc123def!",
				Matches:     []string{"abc", "abc", "abc"},
				Description: "Multiple matching sequences",
			},
		}

		runTestInputAndMatches(t, "ChainedLookahead", testCasesNested, chainedLookahead)
	})

	// Test nested lookahead
	t.Run("NestedLookahead", func(t *testing.T) {
		// Match "a" only if followed by "b" which is followed by "c"
		nestedLookahead := rules.NewMatchWithLookahead(
			rules.NewMatchString("a"),

			rules.NewMatchWithLookahead(
				rules.NewMatchString("b"),
				rules.NewMatchString("c"),
			),
		)

		testCasesNested := []inputAndMatchesCase{
			{
				Input:       "abc",
				Matches:     []string{"a"},
				Description: "Matching sequence",
			},
			{
				Input:       "ab",
				Matches:     nil,
				Description: "Incomplete sequence",
			},
			{
				Input:       "abd",
				Matches:     nil,
				Description: "Wrong final character",
			},
			{
				Input:       "adc",
				Matches:     nil,
				Description: "Wrong middle character",
			},
			{
				Input:       "abcabc",
				Matches:     []string{"a", "a"},
				Description: "Multiple matching sequences",
			},
		}

		runTestInputAndMatches(t, "NestedLookahead", testCasesNested, nestedLookahead)
	})
}
*/

func runTestInputAndMatches(t *testing.T, ruleType string, testCases []inputAndMatchesCase, initialRule textlexer.Rule) {
	for ti, tc := range testCases {
		t.Run(fmt.Sprintf("%s_case_%03d_%s", ruleType, ti+1, tc.Description), func(t *testing.T) {
			var rule textlexer.Rule
			var state textlexer.State

			var matches []string

			// append the EOF rune to the input
			input := []rune(tc.Input)

			times := 0

			t.Logf("### input: %q", input)

			for current, lookahead := 0, 0; current < len(input); {

				require.True(t, times < outOfControlLimit, "Out of control loop. Aborting after %v iterations", outOfControlLimit)
				times++

				// next character
				var r rune
				if current+lookahead < len(input) {
					r = input[current+lookahead]
				} else {
					r = textlexer.RuneEOF
				}

				if rule == nil {
					// reset the rule to the initial rule
					rule = initialRule
				}

				// execute the rule
				rule, state = rule(r)

				t.Logf("### iteration [%02d]: current: %02d, lookahead: %02d, rune: %q, state: %v", times, current, lookahead, r, state)

				switch state {
				case textlexer.StateContinue:
					lookahead = lookahead + 1

				case textlexer.StateAccept:
					lookahead = lookahead + 1

					// rule current, store the match
					if matches == nil {
						matches = []string{}
					}

					term := string(input[current : current+lookahead])
					matches = append(matches, term)

					t.Logf("PUSH. current: %d, lookahead: %d, end: %d, term: %q", current, lookahead, current+lookahead, term)

					// record matcn and move the current position
					current = current + lookahead

					lookahead = 0 // reset lookahead to 0

				case textlexer.StateReject:
					// advance the position without a match
					current = current + lookahead

					if lookahead == 0 {
						// this rule failed immediately, and there is no other rule to try,
						// so we advance by one
						current = current + 1
					}

					lookahead = 0 // reset lookahead to 0

				case textlexer.StatePushBack:
					lookahead = lookahead - 1
					if lookahead < 0 {
						t.Fatalf("Pushback underflow: %d", lookahead)
					}

				default:
					t.Fatalf("Unexpected state: %v", state)
				}

				if rules.IsEOF(r) && rule == nil {
					t.Logf("### EOF reached")
					break
				}
			}

			// check if the number of matches is as expected
			if len(matches) != len(tc.Matches) {
				require.Equal(t, len(tc.Matches), len(matches),
					"Rule: %q, Input: %q, Expected matches: %v, got: %v",
					ruleType, tc.Input, tc.Matches, matches,
				)
			}

			// check if the matches are as expected
			for i, match := range matches {
				assert.Equal(t, tc.Matches[i], match,
					"Rule: %q, Input: %q, Expected match %d: %q, got %q",
					ruleType, tc.Input, i, tc.Matches[i], match)
			}
		})
	}
}

func OLDrunTestInputAndMatches(t *testing.T, ruleType string, testCases []inputAndMatchesCase, initialRule textlexer.Rule) {
	for ti, tc := range testCases {
		t.Run(fmt.Sprintf("%s_case_%03d_%s", ruleType, ti+1, tc.Description), func(t *testing.T) {
			var rule textlexer.Rule
			var state textlexer.State

			var matches []string

			// append the EOF rune to the input
			input := append([]rune(tc.Input), textlexer.RuneEOF)

			times := 0

			t.Logf("Input: %q", input)

			for current, lookahead := 0, 0; current+lookahead < len(input); {
				require.True(t, times < outOfControlLimit, "Out of control loop. Aborting after %v iterations", outOfControlLimit)
				times++

				// next character
				r := input[current+lookahead]

				t.Logf("iteration: %d ## current: %d, lookahead: %d, rune: %q", times, current, lookahead, r)

				if rule == nil {
					// reset the rule to the initial rule
					rule = initialRule
				}

				// execute the rule
				rule, state = rule(r)

				t.Logf("iteration: %d -> rule(%q) => %v", times, r, state)

				switch state {
				case textlexer.StateContinue:
					lookahead = lookahead + 1

				case textlexer.StateAccept:
					t.Logf("** Accept **")

					// rule accepted, store the match
					if matches == nil {
						matches = []string{}
					}

					term := string(input[current : current+lookahead])
					matches = append(matches, term)

					t.Logf("PUSH: %q", term)

					// advance the position
					if lookahead == 0 {
						current = current + 1
					} else {
						current = current + lookahead
					}

					lookahead = 0

				case textlexer.StateReject:

					if rule != nil {
						lookahead = 0
						t.Logf("** Reject **: rule changed")
						continue
					}

					// advance the position
					if lookahead == 0 {
						t.Logf("** Reject **: current+1")
						current = current + 1
					} else {
						t.Logf("** Reject **: current+lookahead")
						current = current + lookahead
					}

					lookahead = 0

				/*
					case textlexer.StateBacktrack:
						// reset state
						lookahead = 0

						// continue to the next iteration immediately to prevent being stopped by EOF
						continue
				*/
				default:
					t.Fatalf("Unexpected state: %v", state)
				}

				if rules.IsEOF(r) && rule == nil {
					t.Logf("EOF reached")
					break
				}
			}

			// check if the number of matches is as expected
			if len(matches) != len(tc.Matches) {
				require.Equal(t, len(tc.Matches), len(matches),
					"Rule: %q, Input: %q, Expected matches: %v, got: %v",
					ruleType, tc.Input, tc.Matches, matches,
				)
			}

			// check if the matches are as expected
			for i, match := range matches {
				assert.Equal(t, tc.Matches[i], match,
					"Rule: %q, Input: %q, Expected match %d: %q, got %q",
					ruleType, tc.Input, i, tc.Matches[i], match)
			}
		})
	}
}
