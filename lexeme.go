package textlexer

type LexemeType string

const (
	LexemeTypeUnknown LexemeType = "UNKNOWN"
)

// Lexeme represents a token identified by the lexer.
type Lexeme struct {
	typ LexemeType

	text   []rune
	offset int
}

// NewLexeme creates and returns a new Lexeme.
func NewLexeme(typ LexemeType, text string, offset int) *Lexeme {
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

// Offset returns the zero-based starting position of the lexeme
// in the original input source.
func (l *Lexeme) Offset() int {
	return l.offset
}

// Len returns the length of the lexeme's text in runes.
// Useful for calculating the end position (Offset + Len).
func (l *Lexeme) Len() int {
	return len(l.text)
}
