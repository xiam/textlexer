package textlexer

const (
	FlagNone uint = 0
	FlagEOF  uint = 1 << iota // Symbol is at end of file (last rune)
	FlagBOF                   // Symbol is at beginning of file (first rune)
	FlagEOL                   // Symbol is at end of line
	FlagBOL                   // Symbol is at beginning of line
)

// Symbol represents a rune with contextual information about its position in
// the text stream. This allows the lexer to make decisions based not only on
// the character itself but also its position within the file or line.
//
// For example, a '#' character might only indicate a comment when it appears
// at the beginning of a line (BOL flag set), or a preprocessor directive might
// only be valid when starting at column 0.
type Symbol struct {
	r     rune
	flags uint
}

// IsEOF returns true if this symbol is at the end of the file.
func IsEOF(s Symbol) bool {
	return (s.flags & FlagEOF) != 0
}

// IsBOF returns true if this symbol is at the beginning of the file.
func IsBOF(s Symbol) bool {
	return (s.flags & FlagBOF) != 0
}

// IsEOL returns true if this symbol is at the end of a line.
func IsEOL(s Symbol) bool {
	return (s.flags & FlagEOL) != 0
}

// IsBOL returns true if this symbol is at the beginning of a line.
func IsBOL(s Symbol) bool {
	return (s.flags & FlagBOL) != 0
}

// IsPrintable returns true if the symbol's rune is a printable ASCII character
// (between space and tilde, inclusive).
func IsPrintable(s Symbol) bool {
	return s.r >= 32 && s.r <= 126
}

// Rune returns the underlying rune value of this symbol.
func (s Symbol) Rune() rune {
	return s.r
}

// NewSymbol creates a new Symbol with the specified rune and position flags.
// Multiple flags can be combined using bitwise OR operations.
//
// Example:
//
//	// Create a symbol for 'H' at the beginning of both file and line
//	sym := NewSymbol('H', FlagBOF | FlagBOL)
func NewSymbol(r rune, flags uint) Symbol {
	return Symbol{
		r:     r,
		flags: flags,
	}
}
