package textlexer

import "github.com/xiam/textreader"

// LexemeType is a string identifier for a class of lexeme, e.g., "IDENTIFIER".
type LexemeType string

const (
	// LexemeTypeUnknown is the default type for a lexeme that does not match
	// any defined rule.
	LexemeTypeUnknown LexemeType = "UNKNOWN"
)

// Lexeme represents a token identified by the lexer. It contains the token's
// type, its textual content, its starting position in the input source, and the
// source span it covers.
//
// The span is expressed in the source text's own coordinates — byte offset,
// rune offset, line, and rune column — and is derived from the located runes
// the lexer committed to the token, never from cursor state left over after
// speculative read-ahead.
type Lexeme struct {
	typ LexemeType

	text   []rune
	offset int

	span textreader.Span
}

// NewLexeme creates and returns a new Lexeme.
//
// The offset is a rune offset, matching Offset. The returned lexeme carries no
// span, so ByteOffset, Line, and Column report zero; use NewLexemeWithSpan to
// build a lexeme with full source coordinates.
func NewLexeme(typ LexemeType, text []rune, offset int) *Lexeme {
	return &Lexeme{
		typ:    typ,
		text:   text,
		offset: offset,
	}
}

// NewLexemeFromString creates and returns a new Lexeme from a string.
//
// Like NewLexeme, it records a rune offset only and leaves the span empty.
func NewLexemeFromString(typ LexemeType, text string, offset int) *Lexeme {
	return &Lexeme{
		typ:    typ,
		text:   []rune(text),
		offset: offset,
	}
}

// NewLexemeWithSpan creates a Lexeme from its text and the source span it
// covers. The rune offset reported by Offset is taken from the span, so the
// compatible accessor and the span agree.
func NewLexemeWithSpan(typ LexemeType, text []rune, span textreader.Span) *Lexeme {
	return &Lexeme{
		typ:    typ,
		text:   text,
		offset: span.Start().RuneOffset(),
		span:   span,
	}
}

// Type returns the type of the lexeme.
func (l *Lexeme) Type() LexemeType {
	return l.typ
}

// Text returns the textual content of the lexeme as a string.
func (l *Lexeme) Text() string {
	return string(l.text)
}

// Offset returns the zero-based starting rune offset of the lexeme in the
// original input source.
//
// Offset counts runes, not bytes: it is the rune offset of Span().Start(). Use
// ByteOffset for the byte offset of the same position.
func (l *Lexeme) Offset() int {
	return l.offset
}

// Len returns the length of the lexeme's text in runes. This is useful for
// calculating the end position (Offset + Len).
func (l *Lexeme) Len() int {
	return len(l.text)
}

// Span returns the source span the lexeme covers: a half-open range with byte
// offset, rune offset, line, and rune column at both ends.
func (l *Lexeme) Span() textreader.Span {
	return l.span
}

// ByteOffset returns the zero-based byte offset at which the lexeme starts.
func (l *Lexeme) ByteOffset() int {
	return l.span.Start().ByteOffset()
}

// RuneOffset returns the zero-based rune offset at which the lexeme starts. It
// is the same value Offset returns.
func (l *Lexeme) RuneOffset() int {
	return l.offset
}

// Line returns the 1-based line on which the lexeme starts.
func (l *Lexeme) Line() int {
	return l.span.Start().Line()
}

// Column returns the 0-based rune column at which the lexeme starts.
func (l *Lexeme) Column() int {
	return l.span.Start().Column()
}
