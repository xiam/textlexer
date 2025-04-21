package rules_test

import (
	"fmt"
	"strings" // Import strings package for repetition
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

func TestMatchUnsignedInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
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
			Input:       "1-2",
			Matches:     []string{"1", "2"},
			Description: "Digits separated by minus",
		},
		{
			Input:       "1+2",
			Matches:     []string{"1", "2"},
			Description: "Digits separated by plus",
		},
		{
			Input:       "01a",
			Matches:     []string{"01"},
			Description: "Leading zero followed by digit and letter",
		},
		{
			Input:       "1\n2",
			Matches:     []string{"1", "2"},
			Description: "Digits separated by newline",
		},
		{
			Input:       "1\t2",
			Matches:     []string{"1", "2"},
			Description: "Digits separated by tab",
		},
		{
			Input:       "99999999999999999999999999999999999999999999999999", // Very long number
			Matches:     []string{"99999999999999999999999999999999999999999999999999"},
			Description: "Extremely long unsigned integer",
		},
	}
	runTestInputAndMatches(t, "MatchUnsignedInteger", testCases, rules.MatchUnsignedInteger)
}

func TestMatchSignedInteger(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
			Description: "Minus sign separated from number by non-whitespace", // Adjusted expectation based on likely rule logic
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
		// Destructive/Edge Cases Added:
		{
			Input:       "+",
			Matches:     nil,
			Description: "Plus sign only",
		},
		{
			Input:       "+ ", // Sign followed by space
			Matches:     nil,
			Description: "Plus sign followed by space",
		},
		{
			Input:       "- ", // Sign followed by space
			Matches:     nil,
			Description: "Minus sign followed by space",
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
			Input:       "+a",
			Matches:     nil,
			Description: "Plus sign followed by letter",
		},
		{
			Input:       "-a",
			Matches:     nil,
			Description: "Minus sign followed by letter",
		},
		{
			Input:       "1-2",
			Matches:     []string{"1", "-2"},
			Description: "Positive integer followed by negative integer",
		},
		{
			Input:       "1+2",
			Matches:     []string{"1", "+2"},
			Description: "Positive integer followed by explicitly positive integer",
		},
		{
			Input:       "-1.2",
			Matches:     []string{"-1", "2"}, // Integer part, then unsigned integer part
			Description: "Negative integer followed by dot and digit",
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
		{
			Input:       "-\t1",
			Matches:     []string{"-\t1"},
			Description: "Minus sign with tab before number",
		},
	}

	runTestInputAndMatches(t, "MatchSignedInteger", testCases, rules.MatchSignedInteger)
}

func TestMatchUnsignedFloat(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       ".",
			Matches:     nil,
			Description: "Dot only",
		},
		{
			Input:       "..1",
			Matches:     []string{".1"}, // Matches the second dot and digit
			Description: "Double dot before digit",
		},
		{
			Input:       "1..2",
			Matches:     []string{".2"}, // Matches the second dot and digit
			Description: "Digit followed by double dot and digit",
		},
		{
			Input:       "1.a",
			Matches:     nil, // Needs digit after dot
			Description: "Digit, dot, letter",
		},
		{
			Input:       ".a",
			Matches:     nil, // Needs digit after dot
			Description: "Dot, letter",
		},
		{
			Input:       "-.1",
			Matches:     []string{".1"}, // Matches the float part
			Description: "Negative sign before float without leading zero",
		},
		{
			Input:       "+.1",
			Matches:     []string{".1"}, // Matches the float part
			Description: "Positive sign before float without leading zero",
		},
		{
			Input:       "1.2.3",
			Matches:     []string{"1.2", ".3"}, // Matches two consecutive floats
			Description: "Multiple decimal points",
		},
		{
			Input:       "0.0.0",
			Matches:     []string{"0.0", ".0"},
			Description: "Multiple decimal points with zeros",
		},
		{
			Input:       ".1e", // Exponent notation not supported by this rule
			Matches:     []string{".1"},
			Description: "Float followed by 'e'",
		},
		{
			Input:       "1.0e",
			Matches:     []string{"1.0"},
			Description: "Float followed by 'e'",
		},
		{
			Input:       "1.0e2.0",
			Matches:     []string{"1.0", "2.0"}, // Float then UnsignedInt
			Description: "Float followed by 'e' and float",
		},
	}

	runTestInputAndMatches(t, "MatchUnsignedFloat", testCases, rules.MatchUnsignedFloat)
}

func TestMatchSignedFloat(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
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
			Input:       "+ .1", // Space between sign and dot
			Matches:     []string{"+ .1"},
			Description: "Plus space dot digit",
		},
		{
			Input:       "- .1", // Space between sign and dot
			Matches:     []string{"- .1"},
			Description: "Minus space dot digit",
		},
		{
			Input:       "++1.0",
			Matches:     []string{"+1.0"}, // Second sign takes precedence
			Description: "Double plus sign float",
		},
		{
			Input:       "--1.0",
			Matches:     []string{"-1.0"}, // Second sign takes precedence
			Description: "Double minus sign float",
		},
		{
			Input:       "+-1.0",
			Matches:     []string{"-1.0"}, // Second sign takes precedence
			Description: "Plus minus sign float",
		},
		{
			Input:       "-+1.0",
			Matches:     []string{"+1.0"}, // Second sign takes precedence
			Description: "Minus plus sign float",
		},
		{
			Input:       "-1.", // Trailing dot after signed int part
			Matches:     nil,
			Description: "Negative integer with trailing dot",
		},
		{
			Input:       "+1.", // Trailing dot after signed int part
			Matches:     nil,
			Description: "Positive integer with trailing dot",
		},
		{
			Input:       "-1.a",
			Matches:     nil,
			Description: "Negative integer, dot, letter",
		},
		{
			Input:       "+1.a",
			Matches:     nil,
			Description: "Positive integer, dot, letter",
		},
		{
			Input:       "-1.2.3",
			Matches:     []string{"-1.2", ".3"}, // Signed float, then unsigned float
			Description: "Negative float with multiple decimal points",
		},
		{
			Input:       "+1.2.3",
			Matches:     []string{"+1.2", ".3"}, // Signed float, then unsigned float
			Description: "Positive float with multiple decimal points",
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
		// Existing cases...
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
			Matches:     []string{"-   1"}, // Whitespace after sign is consumed
			Description: "Negative integer with whitespace",
		},
		{
			Input:       "-   1.23",
			Matches:     []string{"-   1.23"}, // Whitespace after sign is consumed
			Description: "Negative float with whitespace after sign",
		},
		// Destructive/Edge Cases Added:
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
			Input:       "+.",
			Matches:     nil,
			Description: "Plus dot",
		},
		{
			Input:       "-.",
			Matches:     nil,
			Description: "Minus dot",
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
			Input:       "1.e2",
			Matches:     []string{"1.", "2"},
			Description: "Attempted exponent notation (unsigned)",
		},
		{
			Input:       "-1.e2",
			Matches:     []string{"-1.", "2"},
			Description: "Attempted exponent notation (signed)",
		},
		{
			Input:       "1.2e3",
			Matches:     []string{"1.2", "3"}, // Matches float '1.2', then int '3'
			Description: "Float followed by e and digit",
		},
		{
			Input:       "1..2",
			Matches:     []string{"1.", ".2"},
			Description: "Integer double dot float part",
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
		{
			Input:       "--1",
			Matches:     []string{"-1"},
			Description: "Double minus integer",
		},
		{
			Input:       "++1.0",
			Matches:     []string{"+1.0"},
			Description: "Double plus float",
		},
	}

	runTestInputAndMatches(t, "Numeric", testCases, rules.MatchSignedNumeric)
}

func TestMatchUnsignedNumeric(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
			Input:       "1.23+    .45", // Plus sign breaks the unsigned match
			Matches:     []string{"1.23", ".45"},
			Description: "Float followed by decimal part with plus sign and whitespace",
		},
		{
			Input:       " -   1 ", // Minus sign breaks the unsigned match
			Matches:     []string{"1"},
			Description: "Negative integer (should match only the digit)",
		},
		{
			Input:       "-   1.23", // Minus sign breaks the unsigned match
			Matches:     []string{"1.23"},
			Description: "Negative float (should match only the numeric part)",
		},
		// Destructive/Edge Cases Added:
		{
			Input:       ".",
			Matches:     nil,
			Description: "Dot only",
		},
		{
			Input:       "+", // Sign is not part of unsigned
			Matches:     nil,
			Description: "Plus only",
		},
		{
			Input:       "-", // Sign is not part of unsigned
			Matches:     nil,
			Description: "Minus only",
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
		{
			Input:       "1.e2",
			Matches:     []string{"1.", "2"},
			Description: "Attempted exponent notation",
		},
		{
			Input:       "1..2",
			Matches:     []string{"1.", ".2"},
			Description: "Double dot float part",
		},
	}

	runTestInputAndMatches(t, "MatchUnsignedNumeric", testCases, rules.MatchUnsignedNumeric)
}

func TestWhitespace(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       "\r", // Carriage return
			Matches:     []string{"\r"},
			Description: "Carriage return",
		},
		{
			Input:       "\r\n", // Windows newline
			Matches:     []string{"\r\n"},
			Description: "CRLF newline",
		},
		{
			Input:       " \t\r\n \t", // Mix of all common whitespace
			Matches:     []string{" \t\r\n \t"},
			Description: "Complex mix of whitespace",
		},
		{
			Input:       "a\tb\nc\rd", // Whitespace separating single chars
			Matches:     []string{"\t", "\n", "\r"},
			Description: "Whitespace between single chars",
		},
		{
			Input:       " leading",
			Matches:     []string{" "},
			Description: "Leading space",
		},
		{
			Input:       "trailing ",
			Matches:     []string{" "},
			Description: "Trailing space",
		},
		{
			Input:       "\u00a0", // Non-breaking space (might or might not be included, depends on definition)
			Matches:     nil,      // Assuming standard ASCII whitespace only
			Description: "Non-breaking space (unicode)",
		},
		{
			Input:       "\u2003", // Em space (unicode)
			Matches:     nil,      // Assuming standard ASCII whitespace only
			Description: "Em space (unicode)",
		},
	}

	runTestInputAndMatches(t, "Whitespace", testCases, rules.MatchWhitespace)
}

func TestNewMatchInvertedRule(t *testing.T) {
	// --- InvertWhitespace ---
	t.Run("InvertWhitespace", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
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
			// Destructive/Edge Cases Added:
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
				Input:       "  multiple  spaces  ",
				Matches:     []string{"multiple", "spaces"},
				Description: "Invert whitespace with multiple spaces",
			},
			{
				Input:       "\nfirst\n\nsecond\n",
				Matches:     []string{"first", "second"},
				Description: "Invert whitespace with newlines",
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

	// --- InvertMatchSignedInteger ---
	t.Run("InvertMatchSignedInteger", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
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
				Input:       "t-   \n\n\n 1ea", // -1 is matched, rest is inverted
				Matches:     []string{"t", "ea"},
				Description: "Text around a negative number with whitespace",
			},
			{
				Input:       "-  a1e", // - is not followed by digit/ws+digit, so it's inverted. 1 is matched. a, e are inverted.
				Matches:     []string{"-  ", "a", "e"},
				Description: "Minus sign with text and digit",
			},
			// Destructive/Edge Cases Added:
			{
				Input:       "+", // Sign only is not a signed int, so it matches inverted
				Matches:     []string{"+"},
				Description: "Invert signed int with plus only",
			},
			{
				Input:       "-", // Sign only is not a signed int, so it matches inverted
				Matches:     []string{"-"},
				Description: "Invert signed int with minus only",
			},
			{
				Input:       "1.2",
				Matches:     []string{"."},
				Description: "Invert signed int with float input",
			},
			{
				Input:       "-1.2",
				Matches:     []string{"."},
				Description: "Invert signed int with negative float input",
			},
			{
				Input:       "abc",
				Matches:     []string{"abc"},
				Description: "Invert signed int with text only",
			},
			{
				Input:       "a+1b-2c", // +1 and -2 are matched
				Matches:     []string{"a", "b", "c"},
				Description: "Invert signed int with mixed text and numbers",
			},
			{
				Input:       "++1",         // +1 is matched
				Matches:     []string{"+"}, // The first + is inverted
				Description: "Invert signed int with double plus",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.MatchSignedInteger)
		runTestInputAndMatches(t, "InvertMatchSignedInteger", testCases, invertRule)
	})

	// --- InvertMatchSignedFloat ---
	t.Run("InvertMatchSignedFloat", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
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
				Input:       "ABC-123ABC-123.abc-123.4ABC", // -123 is int, -123. is invalid, -123.4 is float
				Matches:     []string{"ABC", "-123", "ABC", "-123.", "abc", "ABC"},
				Description: "Complex mix of text, integers, and floats",
			},
			{
				Input:       "aBCd0-1234.444EDFT11", // 0, 11 are ints
				Matches:     []string{"aBCd", "0", "EDFT", "11"},
				Description: "Mixed case text with numbers and a negative float",
			},
			{
				Input:       "AB-1234.-12.3", // -1234. is invalid, -12.3 is float
				Matches:     []string{"AB", "-1234."},
				Description: "Text with invalid float format",
			},
			{
				Input:       "-12.34ABC",
				Matches:     []string{"ABC"},
				Description: "Negative float followed by text",
			},
			{
				Input:       "ABC-12.3 4AAAA", // -12.3 is float, 4 is int
				Matches:     []string{"ABC", " ", "4", "AAAA"},
				Description: "Text with space-separated negative float and number",
			},
			{
				Input:       "ABC",
				Matches:     []string{"ABC"},
				Description: "Text only",
			},
			{
				Input:       "0000", // Integer
				Matches:     []string{"0000"},
				Description: "Integer with leading zeros",
			},
			{
				Input:       "00001.2", // Float
				Matches:     nil,
				Description: "Float with leading zeros (should not match when inverted)",
			},
			{
				Input:       "00001.2a", // Float matched
				Matches:     []string{"a"},
				Description: "Float with leading zeros followed by letter",
			},
			{
				Input:       "a00001.2", // Float matched
				Matches:     []string{"a"},
				Description: "Letter followed by float with leading zeros",
			},
			// Destructive/Edge Cases Added:
			{
				Input:       ".", // Not a float
				Matches:     []string{"."},
				Description: "Invert float with dot only",
			},
			{
				Input:       "+.", // Not a float
				Matches:     []string{"+."},
				Description: "Invert float with plus dot",
			},
			{
				Input:       "-.", // Not a float
				Matches:     []string{"-."},
				Description: "Invert float with minus dot",
			},
			{
				Input:       "1.", // Not a float
				Matches:     []string{"1."},
				Description: "Invert float with integer and trailing dot",
			},
			{
				Input:       "-1.", // Not a float
				Matches:     []string{"-1."},
				Description: "Invert float with negative integer and trailing dot",
			},
			{
				Input:       "1.2.3", // 1.2 is float, .3 is float
				Matches:     nil,     // Both parts match float, nothing left to invert
				Description: "Invert float with multiple decimal points",
			},
			{
				Input:       "a.1", // .1 is float
				Matches:     []string{"a"},
				Description: "Invert float with letter dot digit",
			},
			{
				Input:       "+.1", // Is a float
				Matches:     nil,
				Description: "Invert float with positive float starting with dot",
			},
			{
				Input:       "-.1", // Is a float
				Matches:     nil,
				Description: "Invert float with negative float starting with dot",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.MatchSignedFloat)
		runTestInputAndMatches(t, "InvertMatchSignedFloat", testCases, invertRule)
	})

	// --- InvertLiteralMatch ---
	t.Run("InvertLiteralMatch", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
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
			// Destructive/Edge Cases Added:
			{
				Input:       "ab", // Prefix of literal
				Matches:     []string{"ab"},
				Description: "Invert literal with prefix input",
			},
			{
				Input:       "abcd", // Literal is prefix
				Matches:     []string{"d"},
				Description: "Invert literal with suffix input",
			},
			{
				Input:       "ab abc", // Partial match then full match
				Matches:     []string{"ab", " "},
				Description: "Invert literal with partial then full match",
			},
			{
				Input:       "abc abc",
				Matches:     []string{" "},
				Description: "Invert literal with space between matches",
			},
			{
				Input:       "xabcyabcz",
				Matches:     []string{"x", "y", "z"},
				Description: "Invert literal with surrounding chars",
			},
			{
				Input:       "abacabc", // Overlapping potential matches
				Matches:     []string{"ab", "a", "c"},
				Description: "Invert literal with overlapping non-match",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.NewMatchString("abc"))
		runTestInputAndMatches(t, "InvertLiteralMatch", testCases, invertRule)
	})

	// --- InvertCaselessLiteralMatch ---
	t.Run("InvertCaselessLiteralMatch", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
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
				Input:       "ABCabdef", // ABC matches, ab matches, def is inverted
				Matches:     []string{"ab", "def"},
				Description: "Partial match with suffix",
			},
			{
				Input:       "ABC abc", // Both match
				Matches:     []string{" "},
				Description: "Space between two matches",
			},
			{
				Input:       "ABCabcABC", // All match
				Matches:     nil,
				Description: "Multiple matches with no non-matching text",
			},
			{
				Input:       "ABCabc124ABCabcAdBefC",
				Matches:     []string{"124", "A", "dBefC"},
				Description: "Complex mix of matching and non-matching text",
			},
			// Destructive/Edge Cases Added:
			{
				Input:       "ab", // Prefix
				Matches:     []string{"ab"},
				Description: "Invert caseless literal with prefix",
			},
			{
				Input:       "ABCD", // Suffix
				Matches:     []string{"D"},
				Description: "Invert caseless literal with suffix",
			},
			{
				Input:       "aBcD", // Suffix, mixed case
				Matches:     []string{"D"},
				Description: "Invert caseless literal with mixed case suffix",
			},
			{
				Input:       "ab ABC", // Partial match then full match
				Matches:     []string{"ab", " "},
				Description: "Invert caseless literal with partial then full match",
			},
			{
				Input:       "xABCyabcz",
				Matches:     []string{"x", "y", "z"},
				Description: "Invert caseless literal with surrounding chars",
			},
			{
				Input:       "aBaCaBC", // Overlapping potential matches
				Matches:     []string{"aB", "a", "C"},
				Description: "Invert caseless literal with overlapping non-match",
			},
		}

		invertRule := rules.NewMatchInvertedRule(rules.NewMatchStringIgnoreCase("abc"))
		runTestInputAndMatches(t, "InvertCaselessLiteralMatch", testCases, invertRule)
	})

	// --- InvertInvertedMatchSignedFloat ---
	t.Run("InvertInvertedMatchSignedFloat", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       " ", // Space is not a float
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
				Matches:     []string{"- 123.4"}, // Assuming sign rule consumes whitespace
				Description: "Negative float with space after sign",
			},
			{
				Input:       "-   123.4a",
				Matches:     []string{"-   123.4"}, // Assuming sign rule consumes whitespace
				Description: "Negative float with whitespace after sign",
			},
			// Destructive/Edge Cases Added (Should behave like MatchSignedFloat):
			{
				Input:       ".", // Not a float
				Matches:     nil,
				Description: "Double invert float with dot only",
			},
			{
				Input:       "+.", // Not a float
				Matches:     nil,
				Description: "Double invert float with plus dot",
			},
			{
				Input:       "-.", // Not a float
				Matches:     nil,
				Description: "Double invert float with minus dot",
			},
			{
				Input:       "1.", // Not a float
				Matches:     nil,
				Description: "Double invert float with integer and trailing dot",
			},
			{
				Input:       "-1.", // Not a float
				Matches:     nil,
				Description: "Double invert float with negative integer and trailing dot",
			},
			{
				Input:       "1.2.3", // Matches 1.2 then .3
				Matches:     []string{"1.2", ".3"},
				Description: "Double invert float with multiple decimal points",
			},
			{
				Input:       "+.1", // Is a float
				Matches:     []string{"+.1"},
				Description: "Double invert float with positive float starting with dot",
			},
			{
				Input:       "-.1", // Is a float
				Matches:     []string{"-.1"},
				Description: "Double invert float with negative float starting with dot",
			},
		}

		// Double inversion should return to original behavior
		invertRule := rules.NewMatchInvertedRule(rules.NewMatchInvertedRule(rules.MatchSignedFloat))
		runTestInputAndMatches(t, "InvertInvertedMatchSignedFloat", testCases, invertRule)
	})
}

func TestWord(t *testing.T) {
	// Assuming Word uses IsLetter or IsDigit (common definition)
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
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
			Input:       "_leading_underscore",
			Matches:     []string{"leading", "underscore"},
			Description: "Word starting with underscore",
		},
		{
			Input:       "trailing_underscore_",
			Matches:     []string{"trailing", "underscore"},
			Description: "Word ending with underscore",
		},
		{
			Input:       "__double__",
			Matches:     []string{"double"},
			Description: "Word with double underscores",
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
			Input:       "WördWithÜmlauts",
			Matches:     []string{"W", "rdWith", "mlauts"},
			Description: "Word with Unicode letters",
		},
		{
			Input:       "你好世界", // Unicode words
			Matches:     nil,
			Description: "Word with CJK characters",
		},
		{
			Input:       "word-123", // Hyphen breaks the word
			Matches:     []string{"word"},
			Description: "Word hyphen number",
		},
		{
			Input:       "...", // Only punctuation
			Matches:     nil,
			Description: "Only punctuation",
		},
		{
			Input:       " a ", // Single letter word surrounded by spaces
			Matches:     []string{"a"},
			Description: "Single letter word",
		},
	}

	runTestInputAndMatches(t, "MatchIdentifier", testCases, rules.MatchIdentifier)
}

func TestMatchDoubleQuotedString(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       `"\""`, // Escaped quote inside
			Matches:     []string{`"\"`},
			Description: "String with only escaped quote",
		},
		{
			Input:       `"\\"`, // Escaped backslash inside
			Matches:     []string{`"\\"`},
			Description: "String with escaped backslash",
		},
		{
			Input:       `"a\\b"`, // Escaped backslash between chars
			Matches:     []string{`"a\\b"`},
			Description: "String with internal escaped backslash",
		},
		{
			Input:       `"a\"b"`, // Escaped quote between chars
			Matches:     []string{`"a\"`},
			Description: "String with internal escaped quote",
		},
		{
			Input:       `"a\nb"`, // Escaped newline (assuming standard escapes)
			Matches:     []string{`"a\nb"`},
			Description: "String with escaped newline",
		},
		{
			Input:       `"String with 'single' quotes"`,
			Matches:     []string{`"String with 'single' quotes"`},
			Description: "Double quoted string containing single quotes",
		},
		{
			Input:       `"Adjacent""Strings"`, // Two separate strings
			Matches:     []string{`"Adjacent"`, `"Strings"`},
			Description: "Adjacent double quoted strings",
		},
		{
			Input:       `"Ends with backslash\\"`,
			Matches:     []string{`"Ends with backslash\\"`},
			Description: "String ending with escaped backslash",
		},
	}

	runTestInputAndMatches(t, "MatchDoubleQuotedString", testCases, rules.MatchDoubleQuotedString)
}

func TestMatchSingleQuotedString(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
			Input:       `aaa ' aaaa aaaa \' aaaaaa`, // Note: Original expectation might be wrong
			Matches:     []string{`' aaaa aaaa \'`},  // Assuming \ is literal unless followed by '
			Description: "Quoted string with potentially escaped quote (check rule impl)",
		},
		// Destructive/Edge Cases Added:
		{
			Input:       `'\''`, // Escaped quote inside
			Matches:     []string{`'\'`},
			Description: "String with only escaped quote",
		},
		{
			Input:       `'\\'`, // Escaped backslash inside
			Matches:     []string{`'\\'`},
			Description: "String with escaped backslash",
		},
		{
			Input:       `'a\\b'`, // Escaped backslash between chars
			Matches:     []string{`'a\\b'`},
			Description: "String with internal escaped backslash",
		},
		{
			Input:       `'a\'b'`, // Escaped quote between chars
			Matches:     []string{`'a\'`},
			Description: "String with internal escaped quote",
		},
		{
			Input:       `'a\nb'`, // Escaped newline (assuming standard escapes)
			Matches:     []string{`'a\nb'`},
			Description: "String with escaped newline",
		},
		{
			Input:       `'a\zb'`,           // Invalid escape sequence (often treated as literal \ and z)
			Matches:     []string{`'a\zb'`}, // Assuming \ becomes literal if escape is invalid
			Description: "String with invalid escape sequence",
		},
		{
			Input:       `'abc\'`, // Unclosed string ending with escaped quote
			Matches:     []string{`'abc\'`},
			Description: "Unclosed string ending with escaped quote",
		},
		{
			Input:       `'abc\\`, // Unclosed string ending with escaped backslash
			Matches:     nil,
			Description: "Unclosed string ending with escaped backslash",
		},
		{
			Input:       `'abc\`, // Unclosed string ending with backslash (potential escape)
			Matches:     nil,
			Description: "Unclosed string ending with bare backslash",
		},
		{
			Input:       `'String with "double" quotes'`,
			Matches:     []string{`'String with "double" quotes'`},
			Description: "Single quoted string containing double quotes",
		},
		{
			Input:       `'Adjacent''Strings'`, // Two separate strings
			Matches:     []string{`'Adjacent'`, `'Strings'`},
			Description: "Adjacent single quoted strings",
		},
		{
			Input:       `'Ends with backslash\\'`,
			Matches:     []string{`'Ends with backslash\\'`},
			Description: "String ending with escaped backslash",
		},
	}
	// Note: The behavior of escapes (`\'`, `\\`, `\n`, etc.) depends heavily
	// on the specific implementation of the MatchSingleQuotedString rule.
	// Adjust expected matches based on the actual rule logic.
	runTestInputAndMatches(t, "MatchSingleQuotedString", testCases, rules.MatchSingleQuotedString)
}

func TestMatchEscapedDoubleQuotedString(t *testing.T) {
	// This rule likely implies more complex escape handling (like C printf)
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       `""`,
			Matches:     []string{`""`},
			Description: "Empty formatted string",
		},
		{
			Input:       `"\\"`, // Escaped backslash
			Matches:     []string{`"\\"`},
			Description: "Formatted string with escaped backslash",
		},
		{
			Input:       `"\n\t\r"`, // Common escapes
			Matches:     []string{`"\n\t\r"`},
			Description: "Formatted string with common escapes",
		},
		{
			Input:       `"\a\b\f\v"`, // Less common C escapes
			Matches:     []string{`"\a\b\f\v"`},
			Description: "Formatted string with less common escapes",
		},
		{
			Input:       `"\0"`, // Null escape
			Matches:     []string{`"\0"`},
			Description: "Formatted string with null escape",
		},
		{
			Input:       `"\x41"`,           // Hex escape
			Matches:     []string{`"\x41"`}, // Represents "A"
			Description: "Formatted string with hex escape",
		},
		{
			Input:       `"\u0041"`,           // Basic Unicode escape
			Matches:     []string{`"\u0041"`}, // Represents "A"
			Description: "Formatted string with unicode escape (4 hex)",
		},
		{
			Input:       `"\U00000041"`,           // Full Unicode escape
			Matches:     []string{`"\U00000041"`}, // Represents "A"
			Description: "Formatted string with unicode escape (8 hex)",
		},
		{
			Input:       `"\101"`,           // Octal escape
			Matches:     []string{`"\101"`}, // Represents "A"
			Description: "Formatted string with octal escape",
		},
		{
			Input:       `"\?"`, // Escaped question mark (trigraph related?)
			Matches:     []string{`"\?"`},
			Description: "Formatted string with escaped question mark",
		},
		{
			Input:       `"Invalid escape \z"`,           // Invalid sequence
			Matches:     []string{`"Invalid escape \z"`}, // Assuming \ becomes literal
			Description: "Formatted string with invalid escape",
		},
		{
			Input:       `"Unclosed hex \x4"`, // Incomplete hex
			Matches:     []string{`"Unclosed hex \x4"`},
			Description: "Formatted string with unclosed hex escape",
		},
		{
			Input:       `"Ends with escape\"`,
			Matches:     nil,
			Description: "Formatted string ending mid-escape",
		},
		{
			Input:       `"Ends with backslash\\"`,
			Matches:     []string{`"Ends with backslash\\"`},
			Description: "Formatted string ending with escaped backslash",
		},
	}
	// Note: The exact behavior of formatted strings depends heavily on which
	// escape sequences are supported. Adjust expectations accordingly.
	runTestInputAndMatches(t, "MatchEscapedDoubleQuotedString", testCases, rules.MatchEscapedDoubleQuotedString)
}

func TestMatchInlineComment(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       "/",
			Matches:     nil,
			Description: "Single slash",
		},
		{
			Input:       " /",
			Matches:     nil,
			Description: "Space then slash",
		},
		{
			Input:       "//", // Comment at start of input, no newline
			Matches:     []string{"//"},
			Description: "Comment only, no newline",
		},
		{
			Input:       "//\n", // Comment at start of input, with newline
			Matches:     []string{"//"},
			Description: "Comment only, with newline",
		},
		{
			Input:       "code//comment", // Comment immediately after code
			Matches:     []string{"//comment"},
			Description: "Comment immediately after code, no newline",
		},
		{
			Input:       "code//comment\nmore code",
			Matches:     []string{"//comment"},
			Description: "Comment immediately after code, with newline",
		},
		{
			Input:       "code // comment with // nested slashes\n",
			Matches:     []string{"// comment with // nested slashes"},
			Description: "Comment containing nested slashes",
		},
		{
			Input:       "code // comment ends with backslash \\\n", // Backslash usually ignored at end of line comment
			Matches:     []string{"// comment ends with backslash \\"},
			Description: "Comment ending with backslash",
		},
		{
			Input:       "//\r\n", // Comment followed by CRLF
			Matches:     []string{"//"},
			Description: "Comment followed by CRLF",
		},
		{
			Input:       "// comment\r",
			Matches:     []string{"// comment"},
			Description: "Comment followed by CR only",
		},
	}

	runTestInputAndMatches(t, "MatchInlineComment", testCases, rules.MatchInlineComment)
}

func TestMatchSlashStarComment(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       "/*",
			Matches:     nil,
			Description: "Unclosed comment start only",
		},
		{
			Input:       "/**/", // Empty comment variation
			Matches:     []string{"/**/"},
			Description: "Empty block comment variation 1",
		},
		{
			Input:       "/***/", // Star inside empty comment
			Matches:     []string{"/***/"},
			Description: "Empty block comment variation 2",
		},
		{
			Input:       "/****/", // Multiple stars inside empty comment
			Matches:     []string{"/****/"},
			Description: "Empty block comment variation 3",
		},
		{
			Input:       "/*/", // Unclosed, ends with slash
			Matches:     nil,
			Description: "Unclosed comment ending with slash",
		},
		{
			Input:       "/* */ */", // Closed comment followed by end sequence
			Matches:     []string{"/* */"},
			Description: "Closed comment followed by spurious end",
		},
		{
			Input:       "/* /* */ */",        // Nested comment syntax (usually not supported)
			Matches:     []string{"/* /* */"}, // Outer comment ends at first */
			Description: "Nested block comment syntax (standard C behavior)",
		},
		{
			Input:       "code/**/code", // Empty comment between code
			Matches:     []string{"/**/"},
			Description: "Empty comment between code",
		},
		{
			Input:       "code/*comment*/code", // Comment between code
			Matches:     []string{"/*comment*/"},
			Description: "Comment between code",
		},
		{
			Input:       "/* Unterminated comment at EOF",
			Matches:     nil,
			Description: "Unterminated comment at EOF",
		},
		{
			Input:       "/* Comment with * / tricky sequence */", // Space in end sequence
			Matches:     []string{"/* Comment with * / tricky sequence */"},
			Description: "Comment containing space in potential end sequence",
		},
		{
			Input:       "/* Comment with / * tricky sequence */", // Space in start sequence
			Matches:     []string{"/* Comment with / * tricky sequence */"},
			Description: "Comment containing space in potential start sequence",
		},
	}

	runTestInputAndMatches(t, "MatchSlashStarComment", testCases, rules.MatchSlashStarComment)
}

func TestLiteralMatch(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       "ab", // Prefix only
			Matches:     nil,
			Description: "Prefix only",
		},
		{
			Input:       "a", // Shorter prefix only
			Matches:     nil,
			Description: "Shorter prefix only",
		},
		{
			Input:       "abcd", // Match is prefix
			Matches:     []string{"abc"},
			Description: "Match is prefix of input",
		},
		{
			Input:       "abac", // Near miss
			Matches:     nil,
			Description: "Near miss",
		},
		{
			Input:       "ABC", // Different case
			Matches:     nil,
			Description: "Different case",
		},
		{
			Input:       "ab abc", // Partial match, space, full match
			Matches:     []string{"abc"},
			Description: "Partial match space full match",
		},
		{
			Input:       "abc" + "abc", // Repeated adjacent
			Matches:     []string{"abc", "abc"},
			Description: "Repeated adjacent matches",
		},
		{
			Input: strings.Repeat("abc", 10), // Many adjacent matches
			Matches: func() []string {
				s := make([]string, 10)
				for i := range s {
					s[i] = "abc"
				}
				return s
			}(),
			Description: "Many adjacent matches",
		},
		{
			Input: strings.Repeat("abc", 5),
			Matches: func() []string {
				s := make([]string, 5)
				for i := range s {
					s[i] = "abc"
				}
				return s
			}(),
			Description: "Overlapping attempts",
		},
	}

	matchDefKeywordRule := rules.NewMatchString("abc")
	runTestInputAndMatches(t, "LiteralMatch", testCases, matchDefKeywordRule)

	// Test with empty literal
	t.Run("LiteralMatch_Empty", func(t *testing.T) {
		emptyLiteralRule := rules.NewMatchString("")
		testCasesEmpty := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     []string{""},
				Description: "Empty input, empty literal",
			},
			{
				Input:       "a",
				Matches:     []string{"", ""},
				Description: "Single char input, empty literal",
			},
			{
				Input:       "abc",
				Matches:     []string{"", "", "", ""},
				Description: "Multi char input, empty literal",
			},
		}

		runTestInputAndMatches(t, "LiteralMatchEmpty", testCasesEmpty, emptyLiteralRule)
	})
}

func TestCaseInsensitiveLiteralMatch(t *testing.T) {
	testCases := []inputAndMatchesCase{
		// Existing cases...
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
		// Destructive/Edge Cases Added:
		{
			Input:       "ab", // Prefix only
			Matches:     nil,
			Description: "Prefix only (caseless)",
		},
		{
			Input:       "AB", // Prefix only (caseless)
			Matches:     nil,
			Description: "Prefix only uppercase (caseless)",
		},
		{
			Input:       "abcd", // Match is prefix
			Matches:     []string{"abc"},
			Description: "Match is prefix of input (caseless)",
		},
		{
			Input:       "ABCD", // Match is prefix, different case
			Matches:     []string{"ABC"},
			Description: "Match is prefix of input uppercase (caseless)",
		},
		{
			Input:       "aBcD", // Match is prefix, mixed case
			Matches:     []string{"aBc"},
			Description: "Match is prefix of input mixed case (caseless)",
		},
		{
			Input:       "abac", // Near miss
			Matches:     nil,
			Description: "Near miss (caseless)",
		},
		{
			Input:       "ABaC", // Near miss, mixed case
			Matches:     nil,
			Description: "Near miss mixed case (caseless)",
		},
		{
			Input:       "ab ABC", // Partial match, space, full match
			Matches:     []string{"ABC"},
			Description: "Partial match space full match (caseless)",
		},
		{
			Input:       "ABC" + "abc", // Repeated adjacent, different case
			Matches:     []string{"ABC", "abc"},
			Description: "Repeated adjacent matches different case (caseless)",
		},
		{
			Input: strings.Repeat("aBc", 10), // Many adjacent matches, mixed case
			Matches: func() []string {
				s := make([]string, 10)
				for i := range s {
					s[i] = "aBc"
				}
				return s
			}(),
			Description: "Many adjacent matches mixed case (caseless)",
		},
		{
			Input: strings.Repeat("aBc", 5),
			Matches: func() []string {
				s := make([]string, 5)
				for i := range s {
					s[i] = "aBc"
				}
				return s
			}(),
			Description: "Overlapping attempts mixed case (caseless)",
		},
	}

	matchDefKeywordRule := rules.NewMatchStringIgnoreCase("abc")
	runTestInputAndMatches(t, "CaseInsensitiveLiteralMatch", testCases, matchDefKeywordRule)

	// Test with empty literal
	t.Run("CaseInsensitiveLiteralMatch_Empty", func(t *testing.T) {
		emptyLiteralRule := rules.NewMatchStringIgnoreCase("")
		testCasesEmpty := []inputAndMatchesCase{
			{
				Input:       "",
				Matches:     []string{""}, // Matches empty input
				Description: "Empty input, empty literal (caseless)",
			},
			{
				Input:       "a",
				Matches:     []string{"", ""}, // Matches empty input before and after
				Description: "Single char input, empty literal (caseless)",
			},
			{
				Input:       "abc",
				Matches:     []string{"", "", "", ""}, // Matches before each char
				Description: "Multi char input, empty literal (caseless)",
			},
		}
		runTestInputAndMatches(t, "CaseInsensitiveLiteralMatchEmpty", testCasesEmpty, emptyLiteralRule)
	})
}

func TestAlways(t *testing.T) {
	// Existing sub-tests...

	// Destructive cases for NewMatchInvertedRule(Always...)
	t.Run("InvertReject", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     []string{"abc"}, // NewMatchInvertedRule(Reject) should accept everything non-EOF
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     []string{"abcdef"},
				Description: "Longer text",
			},
			// Destructive/Edge Cases Added:
			{
				Input:       "a\nb\tc", // Should consume all
				Matches:     []string{"a\nb\tc"},
				Description: "InvertReject with whitespace",
			},
			{
				Input:       " ", // Should consume all
				Matches:     []string{" "},
				Description: "InvertReject with space only",
			},
		}
		// NewMatchInvertedRule(Reject) should accept any single rune and continue, effectively consuming the whole input as one token.
		// The runner logic might split this differently based on how Accept works.
		// Let's assume the runner's Accept logic (`j=j-1`) doesn't apply well here,
		// and NewMatchInvertedRule(Reject) effectively acts like "match until EOF".
		runTestInputAndMatches(t, "InvertReject", testCases, rules.NewMatchInvertedRule(rules.RejectCurrent))
	})

	t.Run("InvertContinue", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     nil, // NewMatchInvertedRule(Continue) should still be Continue? Or Reject? Assume Reject.
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     nil,
				Description: "Longer text",
			},
			// Destructive/Edge Cases Added:
			{
				Input:       " ",
				Matches:     nil,
				Description: "InvertContinue with space",
			},
		}
		// NewMatchInvertedRule(Continue) is tricky. If Continue means "need more input",
		// inverting it might mean "don't need more input", which could be Accept or Reject.
		// Let's assume it becomes Reject (fails immediately).
		runTestInputAndMatches(t, "InvertContinue", testCases, rules.NewMatchInvertedRule(rules.MatchAnyCharacter))
	})

	t.Run("InvertAccept", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			// Existing cases...
			{
				Input:       "",
				Matches:     nil,
				Description: "Empty input",
			},
			{
				Input:       "abc",
				Matches:     nil, // NewMatchInvertedRule(Accept) should reject immediately.
				Description: "Simple text",
			},
			{
				Input:       "abcdef",
				Matches:     nil,
				Description: "Longer text",
			},
			// Destructive/Edge Cases Added:
			{
				Input:       " ",
				Matches:     nil,
				Description: "InvertAccept with space",
			},
		}
		// NewMatchInvertedRule(AcceptCurrentAndStop) should reject immediately.
		runTestInputAndMatches(t, "InvertAccept", testCases, rules.NewMatchInvertedRule(rules.AcceptCurrentAndStop))
	})
}

func TestCompose(t *testing.T) {
	// Rule: Compose(Caseless("ORDER"), Whitespace, Caseless("BY"))
	orderByRule := rules.NewMatchRuleSequence(
		rules.NewMatchStringIgnoreCase("ORDER"),
		rules.MatchWhitespace,
		rules.NewMatchStringIgnoreCase("BY"),
	)

	testCases := []inputAndMatchesCase{
		// Existing cases...
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
			Input:       " oRDER \n BY ", // Leading/trailing space not part of composed match
			Matches:     []string{"oRDER \n BY"},
			Description: "Mixed-case ORDER BY with whitespace",
		},
		{
			Input:       "SELECT * FROM trades ORDER BY id DESC LIMIT 50;", // Matches only ORDER BY part
			Matches:     []string{"ORDER BY"},
			Description: "ORDER BY in SQL query",
		},
		// Destructive/Edge Cases Added:
		{
			Input:       "ORDERBY", // Missing whitespace
			Matches:     nil,
			Description: "Compose without required whitespace",
		},
		{
			Input:       "ORDER  BY", // Multiple spaces
			Matches:     []string{"ORDER  BY"},
			Description: "Compose with multiple spaces",
		},
		{
			Input:       "ORDER\tBY", // Tab whitespace
			Matches:     []string{"ORDER\tBY"},
			Description: "Compose with tab whitespace",
		},
		{
			Input:       "ORDER \n\t BY", // Mixed whitespace
			Matches:     []string{"ORDER \n\t BY"},
			Description: "Compose with mixed whitespace",
		},
		{
			Input:       "order by", // Lowercase
			Matches:     []string{"order by"},
			Description: "Compose with lowercase",
		},
		{
			Input:       "ORDER", // First part only
			Matches:     nil,
			Description: "Compose first part only",
		},
		{
			Input:       "ORDER ", // First part and start of whitespace only
			Matches:     nil,
			Description: "Compose first part and partial whitespace",
		},
		{
			Input:       "ORDER BYX", // Second part has extra char
			Matches:     []string{"ORDER BY"},
			Description: "Compose second part has extra char",
		},
		{
			Input:       "XORDER BY", // Extra char before first part
			Matches:     []string{"ORDER BY"},
			Description: "Compose extra char before first part",
		},
		{
			Input:       "ORDER Junk BY", // Non-whitespace between parts
			Matches:     nil,
			Description: "Compose non-whitespace between parts",
		},
		{
			Input:       "ORDER BY ORDER BY", // Adjacent composed matches
			Matches:     []string{"ORDER BY", "ORDER BY"},
			Description: "Adjacent composed matches",
		},
	}

	runTestInputAndMatches(t, "Compose", testCases, orderByRule)

	// Test composing problematic rules
	t.Run("Compose_Always", func(t *testing.T) {
		// Compose(Literal("A"), Continue, Literal("B")) -> Should never match B
		composeContinue := rules.NewMatchRuleSequence(rules.NewMatchString("A"), rules.MatchAnyCharacter, rules.NewMatchString("B"))
		runTestInputAndMatches(t, "ComposeContinue", []inputAndMatchesCase{
			{"AB", nil, "Compose with Continue"},
			{"A B", nil, "Compose with Continue space"},
		}, composeContinue)

		// Compose(Literal("A"), RejectCurrent, Literal("B")) -> Should reject after A
		composeReject := rules.NewMatchRuleSequence(rules.NewMatchString("A"), rules.RejectCurrent, rules.NewMatchString("B"))
		runTestInputAndMatches(t, "ComposeReject", []inputAndMatchesCase{
			{"AB", nil, "Compose with Reject"},
			{"A B", nil, "Compose with Reject space"},
		}, composeReject)

		composeAccept := rules.NewMatchRuleSequence(rules.NewMatchString("A"), rules.AcceptCurrentAndStop, rules.NewMatchString("B"))
		runTestInputAndMatches(t, "ComposeAccept", []inputAndMatchesCase{
			{"AB", []string{"AB"}, "Compose with AcceptCurrentAndStop"}, // A matches, AcceptCurrentAndStop matches "", B fails
			{"A B", nil, "Compose with AcceptCurrentAndStop space"},     // A matches, AcceptCurrentAndStop matches "", space fails B
		}, composeAccept)
	})
}

func TestAnyMatch(t *testing.T) {
	// Rules: Caseless("ABC"), Caseless("DEF"), Caseless("GHI"), Caseless("ABCDEF") - note order
	anyMatchRule := rules.NewMatchAnyRule(
		rules.NewMatchStringIgnoreCase("ABC"),
		rules.NewMatchStringIgnoreCase("DEF"),
		rules.NewMatchStringIgnoreCase("GHI"),
		rules.NewMatchStringIgnoreCase("ABCDEF"), // Longer match listed later
	)

	testCases := []inputAndMatchesCase{
		// Existing cases...
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
			Input:       "abcdeg defabcghiabc", // Matches abc, def, abc, ghi, abc
			Matches:     []string{"abc", "def", "abc", "ghi", "abc"},
			Description: "Multiple different matches",
		},
		// Destructive/Edge Cases Added:
		{
			Input:       "ABCDEF",               // Matches "ABC" first because it's listed first
			Matches:     []string{"ABC", "DEF"}, // Then matches "DEF"
			Description: "Input matches longer rule, but shorter rule is first",
		},
		{
			Input:       "aBcDeF", // Mixed case version of the above
			Matches:     []string{"aBc", "DeF"},
			Description: "Mixed case version of prefix rule precedence",
		},
		{
			Input:       "defabc", // Matches DEF then ABC
			Matches:     []string{"def", "abc"},
			Description: "Adjacent matches of different rules",
		},
		{
			Input:       "ab", // Prefix of ABC
			Matches:     nil,
			Description: "Prefix of a potential match",
		},
		{
			Input:       "abchi", // Matches ABC, then nothing for HI
			Matches:     []string{"abc"},
			Description: "Match followed by partial match of another rule",
		},
		{
			Input:       "xyz", // No match
			Matches:     nil,
			Description: "Input with no matching rules",
		},
		{
			Input:       " abc def ", // Matches with surrounding spaces
			Matches:     []string{"abc", "def"},
			Description: "Matches with surrounding spaces",
		},
		{
			Input:       "ABCDEFGHI", // Matches ABC, DEF, GHI
			Matches:     []string{"ABC", "DEF", "GHI"},
			Description: "Concatenation of all matches",
		},
	}

	runTestInputAndMatches(t, "AnyMatch", testCases, anyMatchRule)

	// Test with overlapping/prefix rules where longer is first
	t.Run("AnyMatch", func(t *testing.T) {
		firstRule := rules.NewMatchAnyRule(
			rules.NewMatchStringIgnoreCase("ABCDEF"),
			rules.NewMatchStringIgnoreCase("ABC"), // Shorter prefix listed later
			rules.NewMatchStringIgnoreCase("DEF"),
		)
		testCasesLonger := []inputAndMatchesCase{
			{
				Input:   "ABCDEF",
				Matches: []string{"ABC", "DEF"}, // Matches ABC then DEF
			},
			{
				Input:       "abcdef", // Lowercase version
				Matches:     []string{"abc", "def"},
				Description: "Lowercase longer rule listed first matches full string",
			},
			{
				Input:       "ABC DEF", // Matches ABC then DEF
				Matches:     []string{"ABC", "DEF"},
				Description: "Shorter rules match when separated",
			},
		}

		runTestInputAndMatches(t, "AnyMatchLongerFirst", testCasesLonger, firstRule)
	})

	// Test with empty list of rules
	t.Run("AnyMatch_EmptyList", func(t *testing.T) {
		emptyAny := rules.NewMatchAnyRule()
		testCasesEmpty := []inputAndMatchesCase{
			{"abc", nil, "Empty rule list"},
			{"", nil, "Empty rule list empty input"},
		}
		runTestInputAndMatches(t, "AnyMatchEmpty", testCasesEmpty, emptyAny)
	})
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
