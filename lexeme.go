package textlexer

// LexemeType is a string identifier for a class of lexeme, e.g., "IDENTIFIER".
type LexemeType string

const (
	// LexemeTypeUnknown is the default type for a lexeme that does not match
	// any defined rule.
	LexemeTypeUnknown LexemeType = "UNKNOWN"
)

// Lexeme represents a token identified by the lexer. It contains the token's
// type, its textual content, and its starting position in the input source.
type Lexeme struct {
	typ LexemeType

	text   []rune
	offset int
}

// NewLexeme creates and returns a new Lexeme.
func NewLexeme(typ LexemeType, text []rune, offset int) *Lexeme {
	return &Lexeme{
		typ:    typ,
		text:   text,
		offset: offset,
	}
}

// NewLexemeFromString creates and returns a new Lexeme from a string.
func NewLexemeFromString(typ LexemeType, text string, offset int) *Lexeme {
	return &Lexeme{
		typ:    typ,
		text:   []rune(text),
		offset: offset,
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

// Offset returns the zero-based starting position of the lexeme in the
// original input source.
func (l *Lexeme) Offset() int {
	return l.offset
}

// Len returns the length of the lexeme's text in runes. This is useful for
// calculating the end position (Offset + Len).
func (l *Lexeme) Len() int {
	return len(l.text)
}
