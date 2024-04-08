package rules_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
	"github.com/xiam/textlexer/rules"
)

type inputAndMatchesCase struct {
	Input   string
	Matches []string
}

func TestUnsignedIntegerLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"1",
			[]string{"1"},
		},
		{
			" 2",
			[]string{"2"},
		},
		{
			"1a",
			[]string{"1"},
		},
		{
			"123",
			[]string{"123"},
		},
		{
			"0",
			[]string{"0"},
		},
		{
			"-",
			nil,
		},
		{
			"-1",
			[]string{"1"},
		},
		{
			"0001",
			[]string{"0001"},
		},
		{
			"12 34 567 8.9",
			[]string{
				"12",
				"34",
				"567",
				"8",
				"9",
			},
		},
		{
			"aaa123.#1113.123",
			[]string{
				"123",
				"1113",
				"123",
			},
		},
	}
	runTestInputAndMatches(t, testCases, rules.UnsignedIntegerLexemeRule)
}

func TestSignedIntegerLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"1",
			[]string{"1"},
		},
		{
			"-1",
			[]string{"-1"},
		},
		{
			"- a  1",
			[]string{"1"},
		},
		{
			"-   1",
			[]string{"-1"},
		},
		{
			"- \n  1",
			[]string{"-1"},
		},
		{
			"+123.+ 456 78 - 10",
			[]string{"+123", "+456", "78", "-10"},
		},
		{
			"0.",
			[]string{"0"},
		},
		{
			"+  \n 0. 455",
			[]string{"+0", "455"},
		},
		{
			"-",
			nil,
		},
		{
			"-1-2-3-4--5",
			[]string{"-1", "-2", "-3", "-4", "-5"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.SignedIntegerLexemeRule)
}

func TestUnsignedFloatLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"1",
			nil,
		},
		{
			"123",
			nil,
		},
		{
			"0",
			nil,
		},
		{
			"-1.0",
			[]string{"1.0"},
		},
		{
			"-01",
			nil,
		},
		{
			"aaaa0.1xxxx",
			[]string{"0.1"},
		},
		{
			"0.001",
			[]string{"0.001"},
		},
		{
			"123.",
			nil,
		},
		{
			"123 .45 6",
			[]string{".45"},
		},
		{
			"123.0",
			[]string{"123.0"},
		},
		{
			"123.45",
			[]string{"123.45"},
		},
		{
			"123.456",
			[]string{"123.456"},
		},
		{
			"ajt.23",
			[]string{".23"},
		},
		{
			"ssdddd123.456.76755",
			[]string{"123.456", ".76755"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.UnsignedFloatLexemeRule)
}

func TestSignedFloatLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"1.1a",
			[]string{"1.1"},
		},
		{
			"a+45.222",
			[]string{"+45.222"},
		},
		{
			"-134.1",
			[]string{"-134.1"},
		},
		{
			"aBCd0-1234.444EDFT11",
			[]string{"-1234.444"},
		},
		{
			"-  134.1",
			[]string{"-134.1"},
		},
		{
			"  + 134.1 ",
			[]string{"+134.1"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.SignedFloatLexemeRule)
}

func TestNumericLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"1",
			[]string{"1"},
		},
		{
			"1.23",
			[]string{"1.23"},
		},
		{
			"  1 . 23. 21.0",
			[]string{"1", "23.", "21.0"},
		},
		{
			"1.23.45",
			[]string{"1.23", ".45"},
		},
		{
			"+1.23.45",
			[]string{"+1.23", ".45"},
		},
		{
			"-1.23+    .45",
			[]string{"-1.23", "+.45"},
		},
		{
			"-   1.23",
			[]string{"-1.23"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.NumericLexemeRule)
}

func TestWhitespaceLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			" ",
			[]string{" "},
		},
		{
			"\t",
			[]string{"\t"},
		},
		{
			"\n",
			[]string{"\n"},
		},
		{
			"q \t\nq",
			[]string{" \t\n"},
		},
		{
			"a b c \n d",
			[]string{" ", " ", " \n "},
		},
	}

	runTestInputAndMatches(t, testCases, rules.WhitespaceLexemeRule)
}

func TestInvertLexemeRule(t *testing.T) {
	t.Run("invert whitespace", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				" ",
				nil,
			},
			{
				"abc\t \t\t\nBCD",
				[]string{"abc", "BCD"},
			},
			{
				"qaaaa \t\nqa",
				[]string{"qaaaa", "qa"},
			},
			{
				"a b c \n d",
				[]string{"a", "b", "c", "d"},
			},
		}

		invertLexemeRule := rules.InvertLexemeRule(rules.WhitespaceLexemeRule)

		runTestInputAndMatches(t, testCases, invertLexemeRule)
	})

	t.Run("invert signed integer", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				"1",
				nil,
			},
			{
				"-1a",
				[]string{"a"},
			},
			{
				"t-   \n\n\n 1ea",
				[]string{"t", "ea"},
			},

			{
				"-  a1e",
				[]string{"-  ", "a", "e"},
			},
		}

		invertLexemeRule := rules.InvertLexemeRule(rules.SignedIntegerLexemeRule)

		runTestInputAndMatches(t, testCases, invertLexemeRule)
	})

	t.Run("invert signed float", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				" ",
				[]string{" "},
			},
			{
				"-12.34",
				nil,
			},
			{
				"A-12.34B",
				[]string{"A", "B"},
			},
			{
				"ABC-12.34DEF-12.0HIJ",
				[]string{"ABC", "DEF", "HIJ"},
			},
			{
				"ABC-123ABC-123.abc-123.4ABC",
				[]string{"ABC", "-123", "ABC", "-123.", "abc", "ABC"},
			},
			{
				"aBCd0-1234.444EDFT11",
				[]string{"aBCd", "0", "EDFT", "11"},
			},
			{
				"AB-1234.-12.3",
				[]string{"AB", "-1234."},
			},
			{
				"-12.34ABC",
				[]string{"ABC"},
			},
			{
				"ABC-12.3 4AAAA",
				[]string{"ABC", " ", "4", "AAAA"},
			},
			{
				"ABC",
				[]string{"ABC"},
			},
			{
				"0000",
				[]string{"0000"},
			},
			{
				"00001.2",
				nil,
			},
			{
				"00001.2a",
				[]string{"a"},
			},
			{
				"a00001.2",
				[]string{"a"},
			},
		}

		invertLexemeRule := rules.InvertLexemeRule(rules.SignedFloatLexemeRule)

		runTestInputAndMatches(t, testCases, invertLexemeRule)
	})

	t.Run("invert literal match", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				" ",
				[]string{" "},
			},
			{
				"abc",
				nil,
			},
			{
				"ABC",
				[]string{"ABC"},
			},
			{
				"ABCabc",
				[]string{"ABC"},
			},
			{
				"ABC abc",
				[]string{"ABC "},
			},
			{
				"ABCabcABC",
				[]string{"ABC", "ABC"},
			},
			{
				"ABCabcABCabc",
				[]string{"ABC", "ABC"},
			},
			{
				"ABCabcABCabcABC",
				[]string{"ABC", "ABC", "ABC"},
			},
		}

		invertLexemeRule := rules.InvertLexemeRule(rules.NewLiteralMatchLexemeRule("abc"))

		runTestInputAndMatches(t, testCases, invertLexemeRule)
	})

	t.Run("invert caseless literal match", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				" ",
				[]string{" "},
			},
			{
				"abc",
				nil,
			},
			{
				"ABC",
				nil,
			},
			{
				"ABCabdef",
				[]string{"ab", "def"},
			},
			{
				"ABC abc",
				[]string{" "},
			},
			{
				"ABCabcABC",
				nil,
			},
			{
				"ABCabc124ABCabcAdBefC",
				[]string{"124", "A", "dBefC"},
			},
		}

		invertLexemeRule := rules.InvertLexemeRule(rules.NewCaseInsensitiveLiteralMatchLexemeRule("abc"))

		runTestInputAndMatches(t, testCases, invertLexemeRule)
	})

	t.Run("invert inverted signed float", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				" ",
				nil,
			},
			{
				"123.4",
				[]string{"123.4"},
			},
			{
				"abc123.4def",
				[]string{"123.4"},
			},
			{
				"abc123.4def123.4",
				[]string{"123.4", "123.4"},
			},
			{
				"abc- 123.4",
				[]string{"- 123.4"},
			},
			{
				"-   123.4a",
				[]string{"-   123.4"},
			},
		}

		invertLexemeRule := rules.InvertLexemeRule(rules.InvertLexemeRule(rules.SignedFloatLexemeRule))

		runTestInputAndMatches(t, testCases, invertLexemeRule)
	})
}

func TestWordLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"hello world",
			[]string{"hello", "world"},
		},
		{
			"hello world\n",
			[]string{"hello", "world"},
		},
		{
			"a n C  \t d ....../*. Ef \"G123        AA.BB.CC",
			[]string{"a", "n", "C", "d", "Ef", "G123", "AA", "BB", "CC"},
		},
		{
			"Build simple, secure, scalable systems with Go",
			[]string{"Build", "simple", "secure", "scalable", "systems", "with", "Go"},
		},
		{
			"a-b-c",
			[]string{"a", "b", "c"},
		},
		{
			"/abc123.",
			[]string{"abc123"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.WordLexemeRule)
}

func TestDoubleQuotedStringLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			``,
			nil,
		},
		{
			`"`,
			nil,
		},
		{
			`""`,
			[]string{`""`},
		},
		{
			`a"b"c`,
			[]string{`"b"`},
		},
		{
			`a"bcd`,
			nil,
		},
		{
			"a b \" cdef \" ghji",
			[]string{`" cdef "`},
		},
		{
			`aaa " aaaa aaaa \" aaaaaa`,
			[]string{`" aaaa aaaa \"`},
		},
	}

	runTestInputAndMatches(t, testCases, rules.DoubleQuotedStringLexemeRule)
}

func TestSingleQuotedStringLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			``,
			nil,
		},
		{
			`'`,
			nil,
		},
		{
			`''`,
			[]string{`''`},
		},
		{
			`a'b'c`,
			[]string{`'b'`},
		},
		{
			`a'bcd`,
			nil,
		},
		{
			"a b ' cdef ' ghji",
			[]string{`' cdef '`},
		},
		{
			`aaa ' aaaa aaaa \' aaaaaa`,
			[]string{`' aaaa aaaa \'`},
		},
	}

	runTestInputAndMatches(t, testCases, rules.SingleQuotedStringLexemeRule)
}

func TestDoubleQuotedFormattedStringLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			``,
			nil,
		},
		{
			`"`,
			nil,
		},
		{
			`"\""`,
			[]string{`"\""`},
		},
		{
			`a"b\"\"c"c`,
			[]string{`"b\"\"c"`},
		},
	}

	runTestInputAndMatches(t, testCases, rules.DoubleQuotedFormattedStringLexemeRule)
}

func TestInlineCommentLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"aaaa //",
			[]string{"//"},
		},
		{
			"aaaa //\n",
			[]string{"//"},
		},
		{
			"aaaa // eeee\t\n\n",
			[]string{"// eeee\t"},
		},
		{
			"aaaa // // //\n\n\n",
			[]string{"// // //"},
		},
		{
			"aaaaaaaa//bbbbbbbb\ncccc\n\nddd\n\n    // tttt",
			[]string{"//bbbbbbbb", "// tttt"},
		},
		{
			"aaaa // eeee\t\n\n//\n",
			[]string{"// eeee\t", "//"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.InlineCommentLexemeRule)
}

func TestSlashStarCommentLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"aaaa /*",
			nil,
		},
		{
			"aaaa /* */",
			[]string{"/* */"},
		},
		{
			"aaaa /* eeee\t\n\n",
			nil,
		},
		{
			"aaaa /* eeee\t\n\n*/",
			[]string{"/* eeee\t\n\n*/"},
		},
		{
			"aaaa /* eeee\t\n\n*/xxxx",
			[]string{"/* eeee\t\n\n*/"},
		},
		{
			"aa /* b\n\nb */ c\t\n\t\nc\n\n/* dd */",
			[]string{"/* b\n\nb */", "/* dd */"},
		},
	}

	runTestInputAndMatches(t, testCases, rules.SlashStarCommentLexemeRule)
}

func TestLiteralMatchLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			" abc ",
			[]string{"abc"},
		},
		{
			" abc",
			[]string{"abc"},
		},
		{
			"abc.abc",
			[]string{"abc", "abc"},
		},
		{
			"abcabc",
			[]string{"abc", "abc"},
		},
		{
			"aaabababcabc",
			[]string{"abc", "abc"},
		},
		{
			"abaabababcaabababcccabc",
			[]string{"abc", "abc", "abc"},
		},
		{
			"abc\nabc",
			[]string{"abc", "abc"},
		},
	}

	matchDefKeywordLexemeRule := rules.NewLiteralMatchLexemeRule("abc")

	runTestInputAndMatches(t, testCases, matchDefKeywordLexemeRule)
}

func TestCaseInsensitiveLiteralMatchLexemeRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			" aBc ",
			[]string{"aBc"},
		},
		{
			" aBC",
			[]string{"aBC"},
		},
		{
			"abc.ABC",
			[]string{"abc", "ABC"},
		},
		{
			"ABCabc",
			[]string{"ABC", "abc"},
		},
		{
			"aaababABCabc",
			[]string{"ABC", "abc"},
		},
		{
			"abaababABCaababABCccABC",
			[]string{"ABC", "ABC", "ABC"},
		},
	}

	matchDefKeywordLexemeRule := rules.NewCaseInsensitiveLiteralMatchLexemeRule("abc")

	runTestInputAndMatches(t, testCases, matchDefKeywordLexemeRule)
}

func TestAlways(t *testing.T) {

	t.Run("reject", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				"abc",
				nil,
			},
			{
				"abcdef",
				nil,
			},
		}

		runTestInputAndMatches(t, testCases, rules.AlwaysReject)
	})

	t.Run("continue", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				"abc",
				nil,
			},
			{
				"abcdef",
				nil,
			},
		}

		runTestInputAndMatches(t, testCases, rules.AlwaysContinue)
	})

	t.Run("accept", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				[]string{""},
			},
			{
				"abc",
				[]string{"", "", "", ""},
			},
			{
				"abcdef",
				[]string{"", "", "", "", "", "", ""},
			},
		}

		runTestInputAndMatches(t, testCases, rules.AlwaysAccept)
	})

	t.Run("invert reject", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				"abc",
				[]string{"abc"},
			},
			{
				"abcdef",
				[]string{"abcdef"},
			},
		}

		runTestInputAndMatches(t, testCases, rules.InvertLexemeRule(rules.AlwaysReject))
	})

	t.Run("invert continue", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				"abc",
				nil,
			},
			{
				"abcdef",
				nil,
			},
		}

		runTestInputAndMatches(t, testCases, rules.InvertLexemeRule(rules.AlwaysContinue))
	})

	t.Run("invert accept", func(t *testing.T) {
		testCases := []inputAndMatchesCase{
			{
				"",
				nil,
			},
			{
				"abc",
				nil,
			},
			{
				"abcdef",
				nil,
			},
		}

		runTestInputAndMatches(t, testCases, rules.InvertLexemeRule(rules.AlwaysAccept))
	})
}

func TestComposeLexemesRule(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"ORDER \n BY",
			[]string{"ORDER \n BY"},
		},
		{
			" oRDER \n BY ",
			[]string{"oRDER \n BY"},
		},
		{
			"SELECT * FROM trades ORDER BY id DESC LIMIT 50;",
			[]string{"ORDER BY"},
		},
	}

	orderByLexemeRule := rules.ComposeLexemeRules(
		rules.NewCaseInsensitiveLiteralMatchLexemeRule("ORDER"),
		rules.WhitespaceLexemeRule,
		rules.NewCaseInsensitiveLiteralMatchLexemeRule("BY"),
	)

	runTestInputAndMatches(t, testCases, orderByLexemeRule)
}

func TestAnyMatch(t *testing.T) {
	testCases := []inputAndMatchesCase{
		{
			"",
			nil,
		},
		{
			"abc",
			[]string{"abc"},
		},
		{
			"abcdeg defabcghiabc",
			[]string{"abc", "def", "abc", "ghi", "abc"},
		},
	}

	anyMatchLexemeRule := rules.NewMatchAnyOf(
		rules.NewCaseInsensitiveLiteralMatchLexemeRule("ABCDEF"), // will never match
		rules.NewCaseInsensitiveLiteralMatchLexemeRule("ABC"),
		rules.NewCaseInsensitiveLiteralMatchLexemeRule("DEF"),
		rules.NewCaseInsensitiveLiteralMatchLexemeRule("GHI"),
	)

	runTestInputAndMatches(t, testCases, anyMatchLexemeRule)
}

func runTestInputAndMatches(t *testing.T, testCases []inputAndMatchesCase, initialRule textlexer.Rule) {
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test case %03d", i), func(t *testing.T) {
			times := 0

			var state textlexer.State
			var rule textlexer.Rule

			input := append([]rune(tc.Input), textlexer.RuneEOF)

			var matches []string

			buf := make([]rune, 0, len(tc.Input))
			for j := 0; j < len(input); j++ {

				times++
				require.True(t, times < 100, "Out of control loop. Aborting.")

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
						j = j - 1
					}
					buf = buf[:0]
				case textlexer.StateContinue:
					buf = append(buf, r)
				case textlexer.StateReject:
					if rule == nil {
						if len(buf) > 0 {
							j = j - 1
							buf = buf[:0]
						}
					}
				}

				if atEOF {
					break
				}
			}

			assert.Equal(t, tc.Matches, matches, "input: %q. expected: %q, got: %q", tc.Input, tc.Matches, matches)
		})
	}
}
