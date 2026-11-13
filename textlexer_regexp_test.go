package textlexer_test

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xiam/textlexer"
)

// TestLiteralAlternationPrefixAlternative: an alternative that is a prefix of
// a longer one ends its match at its own last rune.
// newLiteralAlternationRule("a", "ab") on "ax" matches "a", not "ax".
func TestLiteralAlternationPrefixAlternative(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	cases := []struct {
		alternatives []string
		input        string
		match        string
	}{
		{alternatives: []string{"a", "ab"}, input: "ax", match: "a"},
		{alternatives: []string{"a", "ab"}, input: "ab", match: "ab"},
		{alternatives: []string{"ab", "a"}, input: "ax", match: "a"},
		{alternatives: []string{"if", "ifelse", "for"}, input: "ifels", match: "if"},
		{alternatives: []string{"if", "ifelse", "for"}, input: "ifelsex", match: "ifelse"},
		{alternatives: []string{"é", "éa"}, input: "éx", match: "é"},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.alternatives, "|")+"/"+tc.input, func(t *testing.T) {
			lx := textlexer.New(strings.NewReader(tc.input))
			lx.MustAddRule(lexTypeR, newLiteralAlternationRule(tc.alternatives...))

			lex, err := lx.Next()
			require.NoError(t, err)
			assert.Equal(t, lexTypeR, lex.Type())
			assert.Equal(t, tc.match, lex.Text())
		})
	}
}

// Rules for the regular expressions in this file. They follow
// docs/rule-authoring.md, as the shared rule helpers in textlexer_test.go do;
// newRunePlusRule, newLiteralThenRule and the generic predicates live there.

// Rune predicates. The names say whether a class is ASCII-only or Unicode.

// isPerlSpace reports whether r is in \s, which is ASCII-only: [\t\n\f\r ].
// It excludes '\v'.
func isPerlSpace(r rune) bool {
	return r == '\t' || r == '\n' || r == '\f' || r == '\r' || r == ' '
}

// isUnicodeNumber reports whether r is in \p{N}.
func isUnicodeNumber(r rune) bool { return unicode.IsNumber(r) }

// isGreek reports whether r is in \p{Greek}.
func isGreek(r rune) bool { return unicode.Is(unicode.Greek, r) }

// isNotUnicodeLetter reports whether r is in [^\pL].
func isNotUnicodeLetter(r rune) bool { return !unicode.IsLetter(r) }

// isQuote reports whether r is in ['"].
func isQuote(r rune) bool { return r == '\'' || r == '"' }

// newCountedDigitsRule returns a rule for [0-9]{lo,hi}, where 1 <= lo <= hi.
// At end of input it rejects the EOF symbol, so it matches the digits read so
// far if there are at least lo of them, and nothing otherwise.
func newCountedDigitsRule(lo, hi int) textlexer.Rule {
	if lo < 1 || hi < lo {
		panic("newCountedDigitsRule: want 1 <= lo <= hi")
	}
	return countedDigits(lo, hi, 0)
}

// countedDigits is the state of [0-9]{lo,hi} after n digits. It continues
// below lo digits, accepts from lo digits on, and stops at hi digits.
func countedDigits(lo, hi, n int) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if isASCIIDigit(s.Rune()) {
			read := n + 1
			switch {
			case read < lo:
				return countedDigits(lo, hi, read), textlexer.StateContinue
			case read < hi:
				return countedDigits(lo, hi, read), textlexer.StateAccept
			default:
				return nil, textlexer.StateAccept
			}
		}
		return nil, textlexer.StateReject
	}
}

// newEscapedStringRule returns a rule for (?s)'(?:\\.|[^'\\])*': a
// single-quoted string in which a backslash escapes the next rune, a newline
// included. It accepts at the closing quote and stops. At end of input inside
// the string it rejects the EOF symbol, so an unterminated string matches
// nothing.
func newEscapedStringRule() textlexer.Rule {
	var start, body, escaped textlexer.Rule
	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == '\'' {
			return body, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	body = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		switch s.Rune() {
		case '\'':
			return nil, textlexer.StateAccept
		case '\\':
			return escaped, textlexer.StateContinue
		}
		if isAnyRune(s.Rune()) {
			return body, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	escaped = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if isAnyRune(s.Rune()) {
			return body, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	return start
}

// newGroupPlusRule returns a rule for (?:ab)+. It accepts after each "ab". At
// end of input it rejects the EOF symbol, so the "ab" groups read so far stay
// the match.
func newGroupPlusRule() textlexer.Rule {
	var expectA, expectB textlexer.Rule
	expectA = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == 'a' {
			return expectB, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	expectB = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == 'b' {
			return expectA, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return expectA
}

// newAlternationPlusRule returns a rule for (?:a|bc)+. It accepts after each
// "a" and after each "bc". At end of input it rejects the EOF symbol, so the
// groups read so far stay the match.
func newAlternationPlusRule() textlexer.Rule {
	var start, afterB textlexer.Rule
	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		switch s.Rune() {
		case 'a':
			return start, textlexer.StateAccept
		case 'b':
			return afterB, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	afterB = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == 'c' {
			return start, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return start
}

// newFoldLiteralRule returns a rule for (?i)lit, where lit is not empty. It
// accepts at the last rune of lit and stops. At end of input before that rune
// it rejects the EOF symbol, so it matches nothing.
func newFoldLiteralRule(lit string) textlexer.Rule {
	if lit == "" {
		panic("newFoldLiteralRule: lit must not be empty")
	}
	return foldLiteralAt([]rune(lit), 0)
}

// foldLiteralAt is the state of (?i)lit that reads rune i of lit.
// strings.EqualFold uses Unicode simple case folding, as regexp does, so 'ſ'
// (U+017F) matches 's'.
func foldLiteralAt(lit []rune, i int) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if strings.EqualFold(string(s.Rune()), string(lit[i])) {
			if i == len(lit)-1 {
				return nil, textlexer.StateAccept
			}
			return foldLiteralAt(lit, i+1), textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

// newStartOfInputFooRule returns a rule for \Afoo. The 'f' must be the first
// symbol of the input. It accepts at the last 'o' and stops. At end of input
// before that it rejects the EOF symbol, so it matches nothing.
func newStartOfInputFooRule() textlexer.Rule {
	var start, afterF, afterFO textlexer.Rule
	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.IsBOF() && s.Rune() == 'f' {
			return afterF, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	afterF = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == 'o' {
			return afterFO, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	afterFO = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == 'o' {
			return nil, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return start
}

// newBackreferenceQuoteRule returns a rule for (['"]).*?\1: the closing quote
// must equal the opening quote. The start state passes the opening quote to
// quotedBody as a parameter, so it lives in the returned Rule values. The
// first equal quote ends the match, as the lazy .*? does, and . excludes a
// newline. At end of input before the closing quote it rejects the EOF symbol,
// so it matches nothing.
func newBackreferenceQuoteRule() textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if isQuote(s.Rune()) {
			return quotedBody(s.Rune()), textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

// quotedBody is the state of (['"]).*?\1 after the opening quote open.
func quotedBody(open rune) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		switch r := s.Rune(); {
		case r == open:
			return nil, textlexer.StateAccept
		case isNotNewline(r):
			return quotedBody(open), textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

// excludeEFOnF is the lookahead state of (?=ef) after the 'e': an 'f' confirms
// the lookahead. excludeLast(2) excludes the 'f' just read and the 'e' before
// it, and accepts the match that ends before "ef".
func excludeEFOnF(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	if s.IsEOF() {
		return nil, textlexer.StateReject
	}
	if s.Rune() == 'f' {
		return excludeLast(2, textlexer.StateAccept)
	}
	return nil, textlexer.StateReject
}

// newGrowingAcceptRule returns a rule for ab|abcd(?=ef). It accepts "ab" and
// keeps reading; after "abcde" an 'f' grows the accept to "abcd". Any other
// symbol rejects, and the earlier accept of "ab" stays. The eager accept of
// "ab" is safe under practice 5 of docs/rule-authoring.md: the pending
// lookahead can only grow the match. At end of input it rejects the EOF
// symbol, so after "ab" the match is "ab".
func newGrowingAcceptRule() textlexer.Rule {
	var start, afterA, afterAB textlexer.Rule
	afterAB = newLiteralThenRule("cde", excludeEFOnF)
	afterA = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == 'b' {
			return afterAB, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	start = newLiteralThenRule("a", afterA)
	return start
}

// newABCDBeforeEFRule returns a rule for abcd(?=ef). It never accepts before
// the lookahead has decided. At end of input it rejects the EOF symbol, so it
// matches nothing.
func newABCDBeforeEFRule() textlexer.Rule {
	return newLiteralThenRule("abcde", excludeEFOnF)
}

// regexpRow is one row of TestRegexpConformance.
type regexpRow struct {
	expr  string
	rule  textlexer.Rule
	input []string

	// matchesEveryInput exempts a row from the non-matching input and from
	// the match that stops before the end of input: its pattern matches every
	// non-empty input completely.
	matchesEveryInput bool
}

// TestRegexpConformance: a hand-written rule returns the same first match as
// Go's regexp for \A(?:expr) under leftmost-longest semantics. No match on a
// non-empty input is a first lexeme of type UNKNOWN. The empty input gives
// io.EOF and no lexeme. Each row must include a matching input and, unless it
// is exempt, a non-matching input and a match that stops before the end.
func TestRegexpConformance(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	rows := []regexpRow{
		{
			expr:  `[0-9]{2,4}`,
			rule:  newCountedDigitsRule(2, 4),
			input: []string{"1", "12", "123", "1234", "12345", "12a", "1a", "a12"},
		},
		{
			expr:  `.+`,
			rule:  newRunePlusRule(isNotNewline),
			input: []string{"a", "ab\ncd", "\nab", "é x\r\n", "\r"},
		},
		{
			expr:              `(?s).+`,
			rule:              newRunePlusRule(isAnyRune),
			input:             []string{"a", "a\nb", "\n", "\r\n", "é\x00ß"},
			matchesEveryInput: true,
		},
		{
			expr: `(?s)'(?:\\.|[^'\\])*'`,
			rule: newEscapedStringRule(),
			input: []string{
				`''`, `'abc' x`, `'a\'b' x`, `'a\\' x`, `'\n\'' '`, "'a\nb'", "'a\\\nb'",
				`'abc`, `'a\`, `'a\'`, `x'a'`,
			},
		},
		{
			expr:  `(?:ab)+`,
			rule:  newGroupPlusRule(),
			input: []string{"ab", "abab", "ababa", "abb", "a", "ba"},
		},
		{
			expr:  `(?:a|bc)+`,
			rule:  newAlternationPlusRule(),
			input: []string{"a", "bc", "abca", "abcb", "bcbcx", "aab", "b", "c"},
		},
		{
			expr:  `(?i)select`,
			rule:  newFoldLiteralRule("select"),
			input: []string{"select", "SeLeCt", "ſELECT", "SELECTx", "s", "se", "sel", "sele", "selec", "xselect"},
		},
		{
			expr:  `\p{N}+`,
			rule:  newRunePlusRule(isUnicodeNumber),
			input: []string{"42", "٣४5a", "Ⅷx", "½", "a1"},
		},
		{
			expr:  `\p{Greek}+`,
			rule:  newRunePlusRule(isGreek),
			input: []string{"αβγ", "αβγd", "Ωmega", "dα", "a"},
		},
		{
			expr:  `[^\pL]+`,
			rule:  newRunePlusRule(isNotUnicodeLetter),
			input: []string{"12 é", "1\n2", "-_ ", "é", "a1"},
		},
		{
			expr:  `\w+`,
			rule:  newRunePlusRule(isASCIIWord),
			input: []string{"a_1", "a_1-b", "abcé", "-a", "é"},
		},
		{
			expr:  `\s+`,
			rule:  newRunePlusRule(isPerlSpace),
			input: []string{" ", " \t\r\n\fx", "\n\n", "\v", "x "},
		},
		{
			expr:  `a|ab`,
			rule:  newLiteralAlternationRule("a", "ab"),
			input: []string{"a", "ab", "ax", "abx", "b"},
		},
		{
			expr:  `if|ifelse|for`,
			rule:  newLiteralAlternationRule("if", "ifelse", "for"),
			input: []string{"if", "ifelse", "for", "ifels", "if(", "forx", "f", "fo", "i", "ife", "ifel"},
		},

		// The shared rules of textlexer_test.go and textlexer_cursor_test.go.
		{
			expr:  `select`,
			rule:  newLiteralRule("select"),
			input: []string{"select", "selectx", "s", "se", "sel", "sele", "selec", "SELECT", "xselect"},
		},
		{
			expr:  `[ \t\n\r]+`,
			rule:  newWhitespaceRule(),
			input: []string{" ", " \t\r\n", " \tx", "\f", "x "},
		},
		{
			expr:  `[0-9]+`,
			rule:  newUnsignedIntegerRule(),
			input: []string{"7", "123", "12a", "a1", "٣"},
		},
		{
			expr:  `[A-Za-z]+`,
			rule:  newASCIILettersRule(),
			input: []string{"a", "abZ", "ab1", "1a", "é"},
		},
		{
			expr:  `\pL+`,
			rule:  newUnicodeLettersRule(),
			input: []string{"a", "héllo", "世界 x", "1a"},
		},
		{
			expr:  `[^\x00-\x7F]+`,
			rule:  newNonASCIIRunesRule(),
			input: []string{"é", "世界", "世a", "a世"},
		},
		{
			expr:  ` `,
			rule:  newSpaceRule(),
			input: []string{" ", "  ", "x"},
		},
		{
			expr:  `[A-Za-z_][A-Za-z0-9_]*`,
			rule:  newIdentifierRule(),
			input: []string{"a", "_", "a_1", "a-b", "1a", "_é"},
		},
		{
			expr:  `[+-]?[0-9]+`,
			rule:  newSignedIntegerRule(),
			input: []string{"1", "-12", "+12x", "+", "-x", "x"},
		},
		{
			expr:  `[-+*/=;()]`,
			rule:  newSymbolRule(),
			input: []string{"+", "()", ";x", "a"},
		},
		{
			expr:  `'[^']*'`,
			rule:  newSingleQuotedStringRule(),
			input: []string{"''", "'a'", "'a\nb' x", "'", "'abc", "x'a'"},
		},
		{
			expr: `(?s)/\*(?:[^*]|\*[^/])*\*/`,
			rule: newSlashStarCommentRule(),
			input: []string{
				"/**/", "/* a */", "/* a */ b */", "/* a\n*/", "/* ** */",
				"/", "/*", "/* a *", "/***/", "/x", "x",
			},
		},
		{
			// Without "..": the rule rejects "1." before a second radix point,
			// which regexp cannot express. TestLookaheadFloatVersusRange checks
			// that case.
			expr:  `[0-9]+\.[0-9]*|\.[0-9]+`,
			rule:  newUnsignedFloatRule(),
			input: []string{"1.5", "1.", "12.x", "1.2.3", ".5", ".x", ".", "1", "x"},
		},
		{
			expr:  `#[^\n]*`,
			rule:  newBOLCommentRule(),
			input: []string{"#", "# a", "# a\nb", "x#", "\n#"},
		},
		{
			expr:  `(?s)a`,
			rule:  newABeforeAnySymbolRule(),
			input: []string{"a", "ab", "a\n", "b"},
		},
		{
			expr:  `a(?:aa)*`,
			rule:  newOddARunRule(),
			input: []string{"a", "aa", "aaa", "aaab", "b"},
		},
		{
			expr:  `(?:aa)+`,
			rule:  newEvenARunRule(),
			input: []string{"a", "aa", "aaa", "aaaab", "b"},
		},
		{
			expr:  `(?:abc)*(?:abc|ab|a)`,
			rule:  newABCCycleRule(),
			input: []string{"a", "ab", "abc", "abca", "abcab", "abx", "b"},
		},
		{
			expr:  `a{1,3}`,
			rule:  newARunUpToRule(3),
			input: []string{"a", "aa", "aaa", "aaaa", "b"},
		},
		{
			expr:  `a{3}`,
			rule:  newARunOfLengthRule(3),
			input: []string{"a", "aa", "aaa", "aaaa", "b"},
		},
		{
			expr:  `[a-z]+`,
			rule:  newRunePlusRule(isASCIILowercase),
			input: []string{"test", "teSt", "T"},
		},
		{
			expr:  `[ab]+`,
			rule:  newRunePlusRule(isAOrB),
			input: []string{"ab", "abc", "c"},
		},
	}

	for _, row := range rows {
		t.Run(row.expr, func(t *testing.T) {
			re := regexp.MustCompile(`\A(?:` + row.expr + `)`)
			re.Longest()

			var matched, unmatched, stoppedEarly bool
			for _, input := range row.input {
				want := re.FindString(input)
				switch {
				case want == "":
					unmatched = true
				case len(want) < len(input):
					matched, stoppedEarly = true, true
				default:
					matched = true
				}

				t.Run(input, func(t *testing.T) {
					require.NotEmpty(t, input, "the empty input is checked separately")

					lx := textlexer.New(strings.NewReader(input))
					lx.MustAddRule(lexTypeR, row.rule)

					lex, err := lx.Next()
					require.NoError(t, err)
					if want == "" {
						assert.Equal(t, textlexer.LexemeTypeUnknown, lex.Type())
						return
					}
					assert.Equal(t, lexTypeR, lex.Type())
					assert.Equal(t, want, lex.Text())
				})
			}

			t.Run("empty input", func(t *testing.T) {
				lx := textlexer.New(strings.NewReader(""))
				lx.MustAddRule(lexTypeR, row.rule)

				lex, err := lx.Next()
				assert.True(t, errors.Is(err, io.EOF), "Next on empty input: %v", err)
				assert.Nil(t, lex)
			})

			assert.True(t, matched, "row has no matching input")
			if row.matchesEveryInput {
				assert.True(t, strings.ContainsRune(strings.Join(row.input, ""), '\n'), "row has no input with a newline")
				return
			}
			assert.True(t, unmatched, "row has no non-matching input")
			assert.True(t, stoppedEarly, "row has no match that stops before the end of input")
		})
	}
}

// TestRegexpWithoutOracle: constructs that Go's regexp cannot check, compared
// with hand-written lexeme streams.
func TestRegexpWithoutOracle(t *testing.T) {
	const lexTypeR = textlexer.LexemeType("R")

	t.Run(`\Afoo`, func(t *testing.T) {
		runLookaheadCases(t, []lookaheadCase{
			{
				name:     "foofoo",
				input:    "foofoo",
				expected: lexemesWith(textlexer.NewLexemeFromString(lexTypeR, "foo", 0), "foo"),
			},
			{name: "xfoo", input: "xfoo", expected: unknownLexemes("xfoo", 0)},
		}, func(lx *textlexer.TextLexer) {
			lx.MustAddRule(lexTypeR, newStartOfInputFooRule())
		})
	})

	t.Run(`(['"]).*?\1`, func(t *testing.T) {
		runLookaheadCases(t, []lookaheadCase{
			{
				name:  "each quote closes its own kind",
				input: `"a'b" 'c"d'`,
				expected: []*textlexer.Lexeme{
					textlexer.NewLexemeFromString(lexTypeR, `"a'b"`, 0),
					textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, " ", 5),
					textlexer.NewLexemeFromString(lexTypeR, `'c"d'`, 6),
				},
			},
			{
				name:  "first equal quote ends the match",
				input: `'a''b'`,
				expected: []*textlexer.Lexeme{
					textlexer.NewLexemeFromString(lexTypeR, `'a'`, 0),
					textlexer.NewLexemeFromString(lexTypeR, `'b'`, 3),
				},
			},
			{name: "no equal closing quote", input: `"abc'`, expected: unknownLexemes(`"abc'`, 0)},
			{name: "dot does not match a newline", input: "'a\nb'", expected: unknownLexemes("'a\nb'", 0)},
		}, func(lx *textlexer.TextLexer) {
			lx.MustAddRule(lexTypeR, newBackreferenceQuoteRule())
		})
	})

	t.Run(`ab|abcd(?=ef)`, func(t *testing.T) {
		runLookaheadCases(t, []lookaheadCase{
			{
				name:     "abcdef",
				input:    "abcdef",
				expected: lexemesWith(textlexer.NewLexemeFromString(lexTypeR, "abcd", 0), "ef"),
			},
			{
				name:     "abcdxx",
				input:    "abcdxx",
				expected: lexemesWith(textlexer.NewLexemeFromString(lexTypeR, "ab", 0), "cdxx"),
			},
			{
				name:     "ab",
				input:    "ab",
				expected: []*textlexer.Lexeme{textlexer.NewLexemeFromString(lexTypeR, "ab", 0)},
			},
		}, func(lx *textlexer.TextLexer) {
			lx.MustAddRule(lexTypeR, newGrowingAcceptRule())
		})
	})
}

// TestRegexpLexerLevelPatterns: constructs shown with a pattern of several
// rules that together give the token stream of the regular expression.
func TestRegexpLexerLevelPatterns(t *testing.T) {
	// abcd(?=ef)|abcdefgh, the two-rule form: one rule matches "abcd" when
	// "ef" follows, and a second rule matches "abcdefgh". Longest match picks
	// the second rule when the input has "gh" after "abcdef".
	t.Run(`abcd(?=ef)|abcdefgh`, func(t *testing.T) {
		const (
			lexTypeLookahead = textlexer.LexemeType("LOOKAHEAD")
			lexTypeLong      = textlexer.LexemeType("LONG")
		)

		cases := []lookaheadCase{
			{
				name:     "abcdefgh",
				input:    "abcdefgh",
				expected: []*textlexer.Lexeme{textlexer.NewLexemeFromString(lexTypeLong, "abcdefgh", 0)},
			},
			{
				name:     "abcdefxx",
				input:    "abcdefxx",
				expected: lexemesWith(textlexer.NewLexemeFromString(lexTypeLookahead, "abcd", 0), "efxx"),
			},
			{name: "abcdxx", input: "abcdxx", expected: unknownLexemes("abcdxx", 0)},
		}

		t.Run("lookahead rule registered first", func(t *testing.T) {
			runLookaheadCases(t, cases, func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeLookahead, newABCDBeforeEFRule())
				lx.MustAddRule(lexTypeLong, newLiteralRule("abcdefgh"))
			})
		})
		t.Run("long rule registered first", func(t *testing.T) {
			runLookaheadCases(t, cases, func(lx *textlexer.TextLexer) {
				lx.MustAddRule(lexTypeLong, newLiteralRule("abcdefgh"))
				lx.MustAddRule(lexTypeLookahead, newABCDBeforeEFRule())
			})
		})
	})

	// \bfor\b: a keyword rule for "for" registered before a rule for a run of
	// ASCII word runes, \w+. A word boundary is a change between \w and not
	// \w, so the competitor must consume every \w run, including one that
	// starts with a digit. On "for" both rules match three runes, and
	// registration order gives the keyword. Where a \w rune precedes or
	// follows "for", the word-run rule matches a longer token and wins. Go's
	// regexp checks the keyword offsets that the lexer produces.
	t.Run(`\bfor\b`, func(t *testing.T) {
		const (
			lexTypeKeyword = textlexer.LexemeType("KEYWORD")
			lexTypeWordRun = textlexer.LexemeType("WORD_RUN")
		)

		keyword := func(offset int) *textlexer.Lexeme {
			return textlexer.NewLexemeFromString(lexTypeKeyword, "for", offset)
		}
		wordRun := func(text string, offset int) *textlexer.Lexeme {
			return textlexer.NewLexemeFromString(lexTypeWordRun, text, offset)
		}
		unknown := func(text string, offset int) *textlexer.Lexeme {
			return textlexer.NewLexemeFromString(textlexer.LexemeTypeUnknown, text, offset)
		}

		cases := []lookaheadCase{
			{name: "for", input: "for", expected: []*textlexer.Lexeme{keyword(0)}},
			{name: "for(", input: "for(", expected: []*textlexer.Lexeme{keyword(0), unknown("(", 3)}},
			{name: " for ", input: " for ", expected: []*textlexer.Lexeme{unknown(" ", 0), keyword(1), unknown(" ", 4)}},
			{name: "xfor", input: "xfor", expected: []*textlexer.Lexeme{wordRun("xfor", 0)}},
			{name: "format", input: "format", expected: []*textlexer.Lexeme{wordRun("format", 0)}},
			{name: "for_1", input: "for_1", expected: []*textlexer.Lexeme{wordRun("for_1", 0)}},
			{name: "1for", input: "1for", expected: []*textlexer.Lexeme{wordRun("1for", 0)}},
			{name: "99for", input: "99for", expected: []*textlexer.Lexeme{wordRun("99for", 0)}},
			{name: "1for(", input: "1for(", expected: []*textlexer.Lexeme{wordRun("1for", 0), unknown("(", 4)}},
		}

		setup := func(lx *textlexer.TextLexer) {
			lx.MustAddRule(lexTypeKeyword, newLiteralRule("for"))
			lx.MustAddRule(lexTypeWordRun, newRunePlusRule(isASCIIWord))
		}

		runLookaheadCases(t, cases, setup)

		// The inputs are ASCII, so byte offsets equal rune offsets.
		re := regexp.MustCompile(`\bfor\b`)
		for _, tc := range cases {
			t.Run(tc.name+" against regexp", func(t *testing.T) {
				var want []int
				for _, loc := range re.FindAllStringIndex(tc.input, -1) {
					want = append(want, loc[0])
				}

				lx := textlexer.New(strings.NewReader(tc.input))
				setup(lx)
				var got []int
				for _, lex := range lexAll(t, lx) {
					if lex.Type() == lexTypeKeyword {
						got = append(got, lex.Offset())
					}
				}
				assert.Equal(t, want, got, "keyword offsets in %q", tc.input)
			})
		}
	})
}
