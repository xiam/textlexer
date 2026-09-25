package textlexer_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xiam/textlexer"
	"github.com/xiam/textreader"
)

// lexemeKeys projects lexemes onto the fields a NewLexemeFromString-built
// expectation can express. The source span is asserted by the span-aware tests
// in this file, which build expectations from the text itself.
func lexemeKeys(lexemes []*textlexer.Lexeme) []string {
	keys := make([]string, len(lexemes))
	for i, lex := range lexemes {
		keys[i] = fmt.Sprintf("%s|%q|%d", lex.Type(), lex.Text(), lex.Offset())
	}
	return keys
}

func lexAll(t *testing.T, lx *textlexer.TextLexer) []*textlexer.Lexeme {
	t.Helper()

	var found []*textlexer.Lexeme
	for {
		lex, err := lx.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		found = append(found, lex)
	}

	return found
}

const (
	lexTypeNumber  = textlexer.LexemeType("NUMBER")
	lexTypeWord    = textlexer.LexemeType("WORD")
	lexTypeSpace   = textlexer.LexemeType("SPACE")
	lexTypeNewline = textlexer.LexemeType("NEWLINE")
)

func newSimpleLexer(t *testing.T, input string) *textlexer.TextLexer {
	t.Helper()

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeWord, newLetterRule())
	lx.MustAddRule(lexTypeNumber, newAsciiNumberRule())
	lx.MustAddRule(lexTypeNewline, newExactRuneRule('\n'))
	lx.MustAddRule(lexTypeSpace, newSpaceRule())

	return lx
}

// TestLexerLexemeSpans checks that every lexeme reports the source span it
// covers, in bytes and runes, for ASCII, multi-byte UTF-8, and newlines.
func TestLexerLexemeSpans(t *testing.T) {
	const input = "héllo 世界\n42"

	lx := newSimpleLexer(t, input)
	found := lexAll(t, lx)

	// Every byte of the input must be covered by exactly one lexeme, in order.
	var rebuilt strings.Builder
	for i, lex := range found {
		rebuilt.WriteString(lex.Text())

		if i > 0 {
			prev := found[i-1].Span()
			assert.Equal(t, prev.End(), lex.Span().Start(),
				"lexeme %d must start where lexeme %d ended", i, i-1)
		}
	}

	assert.Equal(t, input, rebuilt.String())
	assert.Equal(t, len(input), found[len(found)-1].Span().End().ByteOffset())

	// The first word is multi-byte: 5 runes in 6 bytes.
	first := found[0]
	assert.Equal(t, lexTypeWord, first.Type())
	assert.Equal(t, "héllo", first.Text())
	assert.Equal(t, 0, first.ByteOffset())
	assert.Equal(t, 0, first.RuneOffset())
	assert.Equal(t, 1, first.Line())
	assert.Equal(t, 0, first.Column())
	assert.Equal(t, 5, first.Span().Runes())
	assert.Equal(t, 6, first.Span().Bytes())
	assert.Equal(t, first.Len(), first.Span().Runes(), "Len and the span must agree")
	assert.Equal(t, first.Offset(), first.RuneOffset())

	// The third word starts on line 2, at column 0, after the newline.
	var onLineTwo *textlexer.Lexeme
	for _, lex := range found {
		if lex.Line() == 2 {
			onLineTwo = lex
			break
		}
	}
	require.NotNil(t, onLineTwo)
	assert.Equal(t, lexTypeNumber, onLineTwo.Type())
	assert.Equal(t, "42", onLineTwo.Text())
	assert.Equal(t, 0, onLineTwo.Column())
	assert.Equal(t, 2, onLineTwo.Line())
}

// TestLexerSpanIsCommittedNotSpeculated drives a rule that reads ahead past the
// token it matches, then checks that the committed spans cover the input
// exactly. A token's start must not drift to where speculation stopped.
func TestLexerSpanIsCommittedNotSpeculated(t *testing.T) {
	const input = "aXbYcZ"

	// ruleOne matches "a" only when followed by "X": the lexer reads one symbol
	// past the token to decide, then must commit over exactly one byte.
	var ruleOne textlexer.Rule
	ruleOne = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		switch s.Rune() {
		case 'a':
			return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == 'X' {
					return nil, textlexer.StatePushBack
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		case 'b':
			return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == 'Y' {
					return nil, textlexer.StatePushBack
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		case 'c':
			return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
				if s.Rune() == 'Z' {
					return nil, textlexer.StatePushBack
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule("LOOKAHEAD", ruleOne)

	found := lexAll(t, lx)

	require.Len(t, found, 6)
	for i, lex := range found {
		assert.Equal(t, string(input[i]), lex.Text())
		assert.Equal(t, i, lex.ByteOffset(), "token %d must start at its own byte", i)
		assert.Equal(t, i, lex.RuneOffset())
		assert.Equal(t, 1, lex.Span().Bytes())
		assert.Equal(t, i+1, lex.Span().End().ByteOffset())
	}
}

// TestLexerRuneReaderSourceSpans checks the compatibility path: a source that
// only implements io.RuneReader still yields correct byte, rune, line, and
// column coordinates.
func TestLexerRuneReaderSourceSpans(t *testing.T) {
	lx := textlexer.New(&runeSource{runes: []rune("ab\n世界")})
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())
	lx.MustAddRule("CJK", newNonAsciiRule())
	lx.MustAddRule(lexTypeNewline, newExactRuneRule('\n'))
	lx.MustAddRule(lexTypeSpace, newSpaceRule())

	found := lexAll(t, lx)
	require.Len(t, found, 3)

	assert.Equal(t, "ab", found[0].Text())
	assert.Equal(t, 0, found[0].ByteOffset())
	assert.Equal(t, 2, found[0].Span().End().ByteOffset())

	newline := found[1]
	assert.Equal(t, lexTypeNewline, newline.Type())
	assert.Equal(t, 2, newline.ByteOffset())
	assert.Equal(t, 1, newline.Line())
	assert.Equal(t, 2, newline.Column())

	// '世界' is two runes in six bytes, starting one byte after the newline.
	cjk := found[2]
	assert.Equal(t, "世界", cjk.Text())
	assert.Equal(t, 3, cjk.ByteOffset())
	assert.Equal(t, 3, cjk.RuneOffset())
	assert.Equal(t, 2, cjk.Line())
	assert.Equal(t, 0, cjk.Column())
	assert.Equal(t, 6, cjk.Span().Bytes())
	assert.Equal(t, 2, cjk.Span().Runes())
	assert.Equal(t, 9, cjk.Span().End().ByteOffset())
}

// TestLexerMaxTokenBytes checks that the retention bound is enforced with a
// deterministic error and no partial progress, and that it does not fire when
// the token fits.
func TestLexerMaxTokenBytes(t *testing.T) {
	t.Run("within the bound", func(t *testing.T) {
		lx := textlexer.NewWithOptions(strings.NewReader("12345 abc"), textlexer.WithMaxTokenBytes(64))
		lx.MustAddRule(lexTypeWord, newAsciiWordRule())
		lx.MustAddRule(lexTypeNumber, newAsciiNumberRule())
		lx.MustAddRule(lexTypeSpace, newSpaceRule())

		found := lexAll(t, lx)
		require.Len(t, found, 3)
		assert.Equal(t, "12345", found[0].Text())
		assert.Equal(t, "abc", found[2].Text())
	})

	t.Run("over the bound", func(t *testing.T) {
		lx := textlexer.NewWithOptions(strings.NewReader(strings.Repeat("a", 1024)),
			textlexer.WithMaxTokenBytes(32))
		lx.MustAddRule(lexTypeWord, newAsciiWordRule())

		lex, err := lx.Next()
		require.Error(t, err)
		assert.Nil(t, lex)
		assert.ErrorIs(t, err, textreader.ErrRetentionExceeded)
	})
}

// TestLexerContext checks the diagnostic context accessor: it reaches back
// across the preceding lexemes while the lexeme is inside the context window,
// and stops working once later tokens push it out.
func TestLexerContext(t *testing.T) {
	lx := textlexer.New(strings.NewReader("one two three"))
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())
	lx.MustAddRule(lexTypeSpace, newSpaceRule())

	first, err := lx.Next()
	require.NoError(t, err)
	require.Equal(t, "one", first.Text())

	ctx, err := lx.Context(first, 0, 4)
	require.NoError(t, err)
	assert.Equal(t, "one two", ctx)

	// One more committed token does not release it: the lexer retains a window
	// of recent lexemes so `before` and `after` can reach across them.
	second, err := lx.Next()
	require.NoError(t, err)
	require.Equal(t, " ", second.Text())

	ctx, err = lx.Context(first, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, "one", ctx, "the first lexeme is still inside the window")

	// Once enough later tokens commit, it falls out of the window and its
	// retained input is released.
	for i := 0; i < 3; i++ {
		_, err = lx.Next()
		require.NoError(t, err)
	}

	_, err = lx.Context(first, 0, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, textreader.ErrPositionOutOfBuffer)
}

// TestLexerNewWithCursor checks the cursor-injection constructor.
func TestLexerNewWithCursor(t *testing.T) {
	cur := textreader.NewReader(strings.NewReader("ab\ncd"))

	lx := textlexer.NewWithCursor(cur)
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())
	lx.MustAddRule(lexTypeNewline, newExactRuneRule('\n'))

	found := lexAll(t, lx)
	require.Len(t, found, 3)
	assert.Equal(t, "ab", found[0].Text())
	assert.Equal(t, 1, found[0].Line())
	assert.Equal(t, "cd", found[2].Text())
	assert.Equal(t, 2, found[2].Line())
	assert.Equal(t, 3, found[2].ByteOffset())
}

func TestLexerContextRejectsNilLexeme(t *testing.T) {
	lx := textlexer.New(strings.NewReader("x"))
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())

	_, err := lx.Context(nil, 1, 1)
	require.Error(t, err)
}

// newAsciiWordRule matches a run of ASCII letters.
func newAsciiWordRule() textlexer.Rule {
	var rule textlexer.Rule
	rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return rule, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return rule
}

// newLetterRule matches a run of Unicode letters.
func newLetterRule() textlexer.Rule {
	var rule textlexer.Rule
	rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if unicode.IsLetter(s.Rune()) {
			return rule, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return rule
}

// newNonAsciiRule matches a run of non-ASCII runes.
func newNonAsciiRule() textlexer.Rule {
	var rule textlexer.Rule
	rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() >= 0x80 {
			return rule, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return rule
}

// newAsciiNumberRule matches a run of ASCII digits.
func newAsciiNumberRule() textlexer.Rule {
	var rule textlexer.Rule
	rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if r := s.Rune(); r >= '0' && r <= '9' {
			return rule, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return rule
}

// newSpaceRule matches a single space.
func newSpaceRule() textlexer.Rule {
	return newExactRuneRule(' ')
}

// newExactRuneRule matches exactly one occurrence of r.
func newExactRuneRule(r rune) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.Rune() == r {
			return nil, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

// runeSource implements io.RuneReader and nothing else.
type runeSource struct {
	runes []rune
	pos   int
}

func (s *runeSource) ReadRune() (rune, int, error) {
	if s.pos >= len(s.runes) {
		return 0, 0, io.EOF
	}

	r := s.runes[s.pos]
	s.pos++

	return r, len(string(r)), nil
}

func BenchmarkLexerCursor(b *testing.B) {
	input := strings.Repeat("the quick brown fox 12345 jumps over 42 lazy dogs\n", 256)

	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeWord, newAsciiWordRule())
		lx.MustAddRule(lexTypeNumber, newAsciiNumberRule())
		lx.MustAddRule(lexTypeNewline, newExactRuneRule('\n'))
		lx.MustAddRule(lexTypeSpace, newSpaceRule())

		for {
			if _, err := lx.Next(); err != nil {
				break
			}
		}
	}
}
