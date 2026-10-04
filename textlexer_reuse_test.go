package textlexer_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xiam/textlexer"
	"github.com/xiam/textreader"
)

// TestLexerMarkReuseLongSequence covers a long stream of tokens on a finite
// cursor whose budget is exactly the context window (markWindow+1 tokens). A
// cursor with the Remark capability re-arms a bounded set of mark slots, so the
// retained region stays bounded and the whole stream lexes. If the lexer leaked
// an extra active mark — the reuse path growing beyond the window — the budget
// would be exceeded and the stream would stop early.
func TestLexerMarkReuseLongSequence(t *testing.T) {
	const n = 512

	// Budget is exactly the window: three committed tokens plus the in-flight
	// token, each one byte. A fifth active mark would push the retained region
	// to five bytes and trip the limit.
	cur := textreader.NewReader(strings.NewReader(strings.Repeat("a", n)),
		textreader.WithMaxRetained(4))

	lx := textlexer.NewWithCursor(cur)
	lx.MustAddRule(lexTypeWord, newExactRuneRule('a'))

	got := 0
	for {
		lex, err := lx.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		assert.Equal(t, "a", lex.Text())
		got++
	}

	assert.Equal(t, n, got, "the full stream must lex without a retention error")
}

// TestLexerMarkEvictionExact covers the exact three-lexeme window: after enough
// tokens commit, the three most recent remain readable and every earlier one is
// evicted, with the retained region clamped to exactly those three.
func TestLexerMarkEvictionExact(t *testing.T) {
	const input = "a b c d e f"

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())
	lx.MustAddRule(lexTypeSpace, newSpaceRule())

	lexes := lexAll(t, lx)
	require.Len(t, lexes, 11)

	// After the final commit the window holds the three most recent tokens
	// (d, " ", e); everything earlier has fallen out and its input is released.
	for i := 8; i < 11; i++ {
		_, err := lx.Context(lexes[i], 0, 0)
		assert.NoError(t, err, "token %d (%q) is inside the window", i, lexes[i].Text())
	}
	for i := 0; i < 8; i++ {
		_, err := lx.Context(lexes[i], 0, 0)
		assert.ErrorIs(t, err, textreader.ErrPositionOutOfBuffer,
			"token %d (%q) has fallen out of the window", i, lexes[i].Text())
	}

	// The retained region is exactly the window: a wider `before` clamps to the
	// oldest retained token (e) and never reaches the evicted d or c.
	ctx, err := lx.Context(lexes[10], 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "e f", ctx, "before reaches exactly to the oldest retained token")

	ctx, err = lx.Context(lexes[8], 0, 8)
	require.NoError(t, err)
	assert.Equal(t, "e f", ctx, "after reaches to the end of the stream")
}

// TestLexerMarkReuseAfterFailures covers a failed Next (a token that grows past
// the cursor's finite retention): the call commits nothing, the previous
// lexeme's context survives, and the failure is repeatable without leaking a
// mark or corrupting the window.
func TestLexerMarkReuseAfterFailures(t *testing.T) {
	// "ccccc" grows past the budget while "ab " is committed.
	cur := textreader.NewReader(strings.NewReader("ab ccccc"), textreader.WithMaxRetained(6))

	lx := textlexer.NewWithCursor(cur)
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())
	lx.MustAddRule(lexTypeSpace, newSpaceRule())

	first, err := lx.Next()
	require.NoError(t, err)
	require.Equal(t, "ab", first.Text())

	second, err := lx.Next()
	require.NoError(t, err)
	require.Equal(t, " ", second.Text())

	// The next token exceeds the retention budget, so it fails and consumes
	// nothing. Repeated failures must keep the previous context intact and not
	// leak a mark that would widen the retained region.
	for i := 0; i < 3; i++ {
		lex, err := lx.Next()
		require.Error(t, err)
		assert.Nil(t, lex)
		assert.ErrorIs(t, err, textreader.ErrRetentionExceeded)

		ctx, err := lx.Context(second, 0, 0)
		require.NoError(t, err)
		assert.Equal(t, " ", ctx, "a failed Next must not release the previous context")
	}
}

// TestLexerMarkReuseAfterRelease covers Release followed by further tokens: the
// lexer must drop its window, then re-arm the mark slots and keep tokenizing
// correctly with the exact window restored.
func TestLexerMarkReuseAfterRelease(t *testing.T) {
	const input = "a b c d e f"

	lx := textlexer.New(strings.NewReader(input))
	lx.MustAddRule(lexTypeWord, newAsciiWordRule())
	lx.MustAddRule(lexTypeSpace, newSpaceRule())

	// Commit six tokens (a, " ", b, " ", c, " "), then release the window.
	lexes := make([]*textlexer.Lexeme, 0, 11)
	for i := 0; i < 6; i++ {
		lex, err := lx.Next()
		require.NoError(t, err)
		lexes = append(lexes, lex)
	}

	lx.Release()

	// Release drops every mark, so no previously committed token resolves any
	// more.
	for i := 0; i < 6; i++ {
		_, err := lx.Context(lexes[i], 0, 0)
		assert.ErrorIs(t, err, textreader.ErrPositionOutOfBuffer, "token %d must be dropped by Release", i)
	}

	// Further tokens still lex. Release left all slots empty, so the next four
	// commits re-arm fresh checkpoints and the fifth re-arms one of the values
	// it just evicted — the ring is back in steady state either way.
	for i := 6; i < 11; i++ {
		lex, err := lx.Next()
		require.NoError(t, err)
		lexes = append(lexes, lex)
	}

	// The window is restored exactly around the last three commits (e, " ", f);
	// the two oldest post-Release marks (d, " ") are evicted.
	for i := 6; i < 8; i++ {
		_, err := lx.Context(lexes[i], 0, 0)
		assert.ErrorIs(t, err, textreader.ErrPositionOutOfBuffer, "token %d is evicted", i)
	}
	for i := 8; i < 11; i++ {
		_, err := lx.Context(lexes[i], 0, 0)
		assert.NoError(t, err, "token %d is inside the restored window", i)
	}

	ctx, err := lx.Context(lexes[10], 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "e f", ctx, "the window is restored around the new tokens")
}

// plainCursor wraps a *textreader.TextReader and exposes only the four-method
// Cursor interface — deliberately not the Remark capability. It models an
// injected cursor without reuse support, which must keep the existing
// allocating behavior.
type plainCursor struct {
	inner *textreader.TextReader
}

func (p *plainCursor) ReadLocatedRune() (textreader.LocatedRune, error) {
	return p.inner.ReadLocatedRune()
}

func (p *plainCursor) Cursor() textreader.Pos { return p.inner.Cursor() }

func (p *plainCursor) Checkpoint() *textreader.Checkpoint {
	return p.inner.Checkpoint()
}

func (p *plainCursor) Seek(offset int64, whence int) (int64, error) {
	return p.inner.Seek(offset, whence)
}

// TestLexerMarkFallbackWithoutRemark covers an injected cursor that lacks the
// Remark capability: the lexer must still tokenize correctly and keep the
// existing allocating behavior — allocating a checkpoint per token rather than
// re-arming a bounded set of slots.
func TestLexerMarkFallbackWithoutRemark(t *testing.T) {
	const input = "a b c d e f g h"

	newFallback := func() *textlexer.TextLexer {
		lx := textlexer.NewWithCursor(&plainCursor{inner: textreader.NewReader(strings.NewReader(input))})
		lx.MustAddRule(lexTypeWord, newAsciiWordRule())
		lx.MustAddRule(lexTypeSpace, newSpaceRule())
		return lx
	}
	newReuse := func() *textlexer.TextLexer {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule(lexTypeWord, newAsciiWordRule())
		lx.MustAddRule(lexTypeSpace, newSpaceRule())
		return lx
	}

	// Behavior is unchanged: the fallback lexes the identical stream to the
	// reuse path.
	assert.Equal(t, lexemeKeys(lexAll(t, newReuse())), lexemeKeys(lexAll(t, newFallback())),
		"a cursor without the Remark capability must lex identically to one with it")

	// Warm both to steady state (past the first pass over the four slots), so
	// the reuse path is re-arming slots and the fallback path is allocating.
	fallback := newFallback()
	reuse := newReuse()
	for i := 0; i < 4; i++ {
		_, err := fallback.Next()
		require.NoError(t, err, "fallback warm %d", i)
		_, err = reuse.Next()
		require.NoError(t, err, "reuse warm %d", i)
	}

	// Measure the allocations of one further, steady-state token on each. The
	// fallback must still allocate a checkpoint per token; the reuse path
	// re-arms a slot and does not.
	fbAllocs := testing.AllocsPerRun(1, func() {
		_, err := fallback.Next()
		require.NoError(t, err)
	})
	reAllocs := testing.AllocsPerRun(1, func() {
		_, err := reuse.Next()
		require.NoError(t, err)
	})

	assert.Greater(t, fbAllocs, reAllocs,
		"a cursor without the Remark capability must still allocate a checkpoint per token (fallback=%d, reuse=%d)",
		fbAllocs, reAllocs)
}
