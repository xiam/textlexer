package rules_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

// inputAndMatchesCase represents a test case with input text and expected matches
type inputAndMatchesCase struct {
	Input       string
	Matches     []string
	Description string // Added description field for better test documentation
}

func TestUnsignedInteger(t *testing.T) {
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
	}
	runTestInputAndMatches(t, "UnsignedInteger", testCases, rules.UnsignedInteger)
}

func TestSignedInteger(t *testing.T) {
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
			Description: "Minus sign separated from number",
		},
		{
			Input:       "-   1",
			Matches:     []string{"-1"},
			Description: "Minus sign with whitespace before number",
		},
		{
			Input:       "- \n  1",
			Matches:     []string{"-1"},
			Description: "Minus sign with newline before number",
		},
		{
			Input:       "+123.+ 456 78 - 10",
			Matches:     []string{"+123", "+456", "78", "-10"},
			Description: "Multiple signed numbers with separators",
		},
		{
			Input:       "0.",
			Matches:     []string{"0"},
			Description: "Zero with decimal point",
		},
		{
			Input:       "+  \n 0. 455",
			Matches:     []string{"+0", "455"},
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
	}

	runTestInputAndMatches(t, "SignedInteger", testCases, rules.SignedInteger)
}

func TestUnsignedFloat(t *testing.T) {
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
	}

	runTestInputAndMatches(t, "UnsignedFloat", testCases, rules.UnsignedFloat)
}

func TestSignedFloat(t *testing.T) {
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
			Matches:     []string{"-134.1"},
			Description: "Negative float with whitespace after sign",
		},
		{
			Input:       "  + 134.1 ",
			Matches:     []string{"+134.1"},
			Description: "Positive float with whitespace after sign and surrounding spaces",
		},
	}

	runTestInputAndMatches(t, "SignedFloat", testCases, rules.SignedFloat)
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
	}

	runTestInputAndMatches(t, "Numeric", testCases, rules.Numeric)
}

func TestUnsignedNumeric(t *testing.T) {
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
	}

	runTestInputAndMatches(t, "UnsignedNumeric", testCases, rules.UnsignedNumeric)
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
	}

	runTestInputAndMatches(t, "Whitespace", testCases, rules.Whitespace)
}

func TestInvert(t *testing.T) {
	// Breaking down the large test function into smaller sub-tests
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
		}

		invertRule := rules.Invert(rules.Whitespace)
		runTestInputAndMatches(t, "InvertWhitespace", testCases, invertRule)
	})

	t.Run("InvertSignedInteger", func(t *testing.T) {
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
				Matches:     []string{"-  ", "a", "e"},
				Description: "Minus sign with text and digit",
			},
		}

		invertRule := rules.Invert(rules.SignedInteger)
		runTestInputAndMatches(t, "InvertSignedInteger", testCases, invertRule)
	})

	t.Run("InvertSignedFloat", func(t *testing.T) {
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
				Matches:     []string{"ABC", "-123", "ABC", "-123.", "abc", "ABC"},
				Description: "Complex mix of text, integers, and floats",
			},
			{
				Input:       "aBCd0-1234.444EDFT11",
				Matches:     []string{"aBCd", "0", "EDFT", "11"},
				Description: "Mixed case text with numbers and a negative float",
			},
			{
				Input:       "AB-1234.-12.3",
				Matches:     []string{"AB", "-1234."},
				Description: "Text with invalid float format",
			},
			{
				Input:       "-12.34ABC",
				Matches:     []string{"ABC"},
				Description: "Negative float followed by text",
			},
			{
				Input:       "ABC-12.3 4AAAA",
				Matches:     []string{"ABC", " ", "4", "AAAA"},
				Description: "Text with space-separated negative float and number",
			},
			{
				Input:       "ABC",
				Matches:     []string{"ABC"},
				Description: "Text only",
			},
			{
				Input:       "0000",
				Matches:     []string{"0000"},
				Description: "Integer with leading zeros",
			},
			{
				Input:       "00001.2",
				Matches:     nil,
				Description: "Float with leading zeros (should not match when inverted)",
			},
			{
				Input:       "00001.2a",
				Matches:     []string{"a"},
				Description: "Float with leading zeros followed by letter",
			},
			{
				Input:       "a00001.2",
				Matches:     []string{"a"},
				Description: "Letter followed by float with leading zeros",
			},
		}

		invertRule := rules.Invert(rules.SignedFloat)
		runTestInputAndMatches(t, "InvertSignedFloat", testCases, invertRule)
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
				Input:       "ABCabcABC",
				Matches:     []string{"ABC", "ABC"},
				Description: "Different case surrounding exact match",
			},
			{
				Input:       "ABCabcABCabc",
				Matches:     []string{"ABC", "ABC"},
				Description: "Alternating different case and exact match",
			},
			{
				Input:       "ABCabcABCabcABC",
				Matches:     []string{"ABC", "ABC", "ABC"},
				Description: "Multiple alternating patterns",
			},
		}

		invertRule := rules.Invert(rules.NewLiteralMatch("abc"))
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
				Matches:     []string{"124", "A", "dBefC"},
				Description: "Complex mix of matching and non-matching text",
			},
		}

		invertRule := rules.Invert(rules.NewCaseInsensitiveLiteralMatch("abc"))
		runTestInputAndMatches(t, "InvertCaselessLiteralMatch", testCases, invertRule)
	})

	t.Run("InvertInvertedSignedFloat", func(t *testing.T) {
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
				Input:       "-   123.4a",
				Matches:     []string{"-   123.4"},
				Description: "Negative float with whitespace after sign",
			},
		}

		// Double inversion should return to original behavior
		invertRule := rules.Invert(rules.Invert(rules.SignedFloat))
		runTestInputAndMatches(t, "InvertInvertedSignedFloat", testCases, invertRule)
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
	}

	runTestInputAndMatches(t, "Word", testCases, rules.Word)
}

func TestDoubleQuotedString(t *testing.T) {
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
			Description: "Quoted string with escaped quote",
		},
	}

	runTestInputAndMatches(t, "DoubleQuotedString", testCases, rules.DoubleQuotedString)
}

func TestSingleQuotedString(t *testing.T) {
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
			Description: "Quoted string with escaped quote",
		},
	}

	runTestInputAndMatches(t, "SingleQuotedString", testCases, rules.SingleQuotedString)
}

func TestDoubleQuotedFormattedString(t *testing.T) {
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
	}

	runTestInputAndMatches(t, "DoubleQuotedFormattedString", testCases, rules.DoubleQuotedFormattedString)
}

func TestInlineComment(t *testing.T) {
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
			Matches:     []string{"//"},
			Description: "Empty comment with newline",
		},
		{
			Input:       "aaaa // eeee\t\n\n",
			Matches:     []string{"// eeee\t"},
			Description: "Comment with text and tab",
		},
		{
			Input:       "aaaa // // //\n\n\n",
			Matches:     []string{"// // //"},
			Description: "Comment containing comment syntax",
		},
		{
			Input:       "aaaaaaaa//bbbbbbbb\ncccc\n\nddd\n\n    // tttt",
			Matches:     []string{"//bbbbbbbb", "// tttt"},
			Description: "Multiple comments in multi-line text",
		},
		{
			Input:       "aaaa // eeee\t\n\n//\n",
			Matches:     []string{"// eeee\t", "//"},
			Description: "Multiple comments with empty comment",
		},
	}

	runTestInputAndMatches(t, "InlineComment", testCases, rules.InlineComment)
}

func TestSlashStarComment(t *testing.T) {
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
	}

	runTestInputAndMatches(t, "SlashStarComment", testCases, rules.SlashStarComment)
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
	}

	matchDefKeywordRule := rules.NewLiteralMatch("abc")
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
	}

	matchDefKeywordRule := rules.NewCaseInsensitiveLiteralMatch("abc")
	runTestInputAndMatches(t, "CaseInsensitiveLiteralMatch", testCases, matchDefKeywordRule)
}

func TestAlways(t *testing.T) {
	// Breaking down into sub-tests for clarity
	t.Run("AlwaysReject", func(t *testing.T) {
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
		}

		runTestInputAndMatches(t, "AlwaysReject", testCases, rules.AlwaysReject)
	})

	t.Run("AlwaysContinue", func(t *testing.T) {
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
		}

		runTestInputAndMatches(t, "AlwaysContinue", testCases, rules.AlwaysContinue)
	})

	t.Run("AlwaysAccept", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     []string{""},
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     []string{"", "", "", ""},
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     []string{"", "", "", "", "", "", ""},
				Description: "Longer text",
			},
		}

		runTestInputAndMatches(t, "AlwaysAccept", testCases, rules.AlwaysAccept)
	})

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
		}

		runTestInputAndMatches(t, "InvertReject", testCases, rules.Invert(rules.AlwaysReject))
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
		}

		runTestInputAndMatches(t, "InvertContinue", testCases, rules.Invert(rules.AlwaysContinue))
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
		}

		runTestInputAndMatches(t, "InvertAccept", testCases, rules.Invert(rules.AlwaysAccept))
	})
}

func TestCompose(t *testing.T) {
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
	}

	orderByRule := rules.Compose(
		rules.NewCaseInsensitiveLiteralMatch("ORDER"),
		rules.Whitespace,
		rules.NewCaseInsensitiveLiteralMatch("BY"),
	)

	runTestInputAndMatches(t, "Compose", testCases, orderByRule)
}

func TestAnyMatch(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			Input:       "",
			Matches:     nil,
			Description: "Empty input",
		},
		{
			Input:       "abc",
			Matches:     []string{"abc"},
			Description: "Simple match",
		},
		{
			Input:       "abcdeg defabcghiabc",
			Matches:     []string{"abc", "def", "abc", "ghi", "abc"},
			Description: "Multiple different matches",
		},
	}

	anyMatchRule := rules.NewMatchAnyOf(
		rules.NewCaseInsensitiveLiteralMatch("ABCDEF"), // will never match
		rules.NewCaseInsensitiveLiteralMatch("ABC"),
		rules.NewCaseInsensitiveLiteralMatch("DEF"),
		rules.NewCaseInsensitiveLiteralMatch("GHI"),
	)

	runTestInputAndMatches(t, "AnyMatch", testCases, anyMatchRule)
}

// runTestInputAndMatches tests a rule against a set of test cases
// Added rule type parameter for better error messages
func runTestInputAndMatches(t *testing.T, ruleType string, testCases []inputAndMatchesCase, initialRule textlexer.Rule) {
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%s_case_%03d_%s", ruleType, i, tc.Description), func(t *testing.T) {
			times := 0

			var state textlexer.State
			var rule textlexer.Rule

			input := append([]rune(tc.Input), textlexer.RuneEOF)

			var matches []string

			buf := make([]rune, 0, len(tc.Input))
			for j := 0; j < len(input); j++ {
				// Increased limit for more complex inputs
				times++
				require.True(t, times < 1000, "Out of control loop. Aborting after 1000 iterations.")

				r := input[j]

				atEOF := textlexer.IsEOF(r)

				if rule == nil {
					rule = initialRule
					buf = buf[:0]
				}

				rule, state = rule(r)

				switch state {
				case textlexer.StateAccept:
					if matches == nil {
						matches = []string{}
					}

					matches = append(matches, string(buf))
					if len(buf) > 0 {
						// The current rune 'r' caused the acceptance.
						// Reprocess 'r' with the default rule to check if it
						// starts a new token immediately after the accepted one.
						j = j - 1
					}

					buf = buf[:0]
				case textlexer.StateContinue:
					buf = append(buf, r)
				case textlexer.StateReject:
					if rule == nil {
						if len(buf) > 0 {
							// The rule failed completely after consuming
							// characters in 'buf'. Discard 'buf' and reprocess
							// the current rune 'r' with the default rule.
							j = j - 1
							buf = buf[:0]
						}
					}
				}

				if atEOF {
					break
				}
			}

			assert.Equal(t, tc.Matches, matches,
				"Rule: %s, Input: %q, Expected: %q, Got: %q",
				ruleType, tc.Input, tc.Matches, matches)
		})
	}
}
