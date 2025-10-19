package textlexer

// LexemeType is a string identifier for a class of lexeme, e.g., "IDENTIFIER".
type LexemeType string

const (
	// LexemeTypeUnspecified is a special type that indicates the lexeme type has
	// not been set.
	LexemeTypeUnspecified LexemeType = ""

	// LexemeTypeUnknown is the default type for a lexeme that does not match
	// any defined rule.
	LexemeTypeUnknown LexemeType = "UNKNOWN"
)

func (lt LexemeType) String() string {
	if lt == LexemeTypeUnspecified {
		return "UNSPECIFIED"
	}

	return string(lt)
}

// Lexeme represents a token identified by the lexer. It contains the token's
// type, its textual content, and its starting position in the input source.
type Lexeme struct {
	typ LexemeType

	symbols []Symbol
	offset  uint64
}

// NewLexeme creates and returns a new Lexeme.
func NewLexeme(typ LexemeType, symbols []Symbol, offset uint64) *Lexeme {
	return &Lexeme{
		typ:     typ,
		symbols: symbols,
		offset:  offset,
	}
}

// NewLexemeFromString creates and returns a new Lexeme from a string.
func NewLexemeFromString(typ LexemeType, text string, offset uint64) *Lexeme {
	runes := []rune(text)
	symbols := make([]Symbol, len(runes))
	for i, r := range runes {
		symbols[i] = NewSymbol(r, FlagNone)
	}
	return &Lexeme{
		typ:     typ,
		symbols: symbols,
		offset:  offset,
	}
}

// Type returns the type of the lexeme.
func (l *Lexeme) Type() LexemeType {
	return l.typ
}

// Text returns the textual content of the lexeme as a string.
func (l *Lexeme) Text() string {
	runes := make([]rune, len(l.symbols))
	for i, sym := range l.symbols {
		runes[i] = sym.Rune()
	}
	return string(runes)
}

// Offset returns the zero-based starting position of the lexeme in the
// original input source.
func (l *Lexeme) Offset() uint64 {
	return l.offset
}

// Len returns the length of the lexeme's text in runes. This is useful for
// calculating the end position (Offset + Len).
func (l *Lexeme) Len() int {
	return len(l.symbols)
}

// Symbols returns the underlying symbols that make up this lexeme.
// This provides access to positional information (BOF, EOL, etc.) if needed.
func (l *Lexeme) Symbols() []Symbol {
	return l.symbols
}
