package textlexer

const (
	FlagNone uint = 0
	FlagEOF  uint = 1 << iota // Symbol is at end of file (last rune)
	FlagBOF                   // Symbol is at beginning of file (first rune)
	FlagEOL                   // Symbol is at end of line
	FlagBOL                   // Symbol is at beginning of line
)

// Symbol represents a rune with contextual information about its position
// in the text stream. This allows lexical analyzers to make decisions based
// not only on the character itself but also its position within the file
// or line.
//
// For example, a '#' character might only indicate a comment when it appears
// at the beginning of a line (s.IsBOL() is true), or a preprocessor directive
// might only be valid when starting at column 0.
type Symbol struct {
	r     rune
	flags uint
}

// IsEOF returns true if this symbol is at the end of the file.
func (s Symbol) IsEOF() bool {
	return (s.flags & FlagEOF) != 0
}

// IsBOF returns true if this symbol is at the beginning of the file.
func (s Symbol) IsBOF() bool {
	return (s.flags & FlagBOF) != 0
}

// IsEOL returns true if this symbol is at the end of a line.
func (s Symbol) IsEOL() bool {
	return (s.flags & FlagEOL) != 0
}

// IsBOL returns true if this symbol is at the beginning of a line.
func (s Symbol) IsBOL() bool {
	return (s.flags & FlagBOL) != 0
}

// IsPrintable returns true if the symbol's rune is a printable ASCII character
// (between space and tilde, inclusive).
func (s Symbol) IsPrintable() bool {
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

// String returns a string representation of the Symbol's rune.
func (s Symbol) String() string {
	return string(s.r)
}

// Equal compares two symbols for equality (both rune and flags must match).
func (s Symbol) Equal(other Symbol) bool {
	return s.r == other.r && s.flags == other.flags
}

// RuneEqual checks if the symbol's rune matches the given rune.
func (s Symbol) RuneEqual(r rune) bool {
	return s.r == r
}
