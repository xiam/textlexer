package textlexer

const (
	FlagNone uint = 0         // FlagNone indicates no special positional flags.
	FlagEOF  uint = 1 << iota // FlagEOF indicates the symbol is the last in the input stream.
	FlagBOF                   // FlagBOF indicates the symbol is at the beginning of the input stream.
	FlagEOL                   // FlagEOL indicates the symbol is at the end of a line.
	FlagBOL                   // FlagBOL indicates the symbol is at the beginning of a line.
)

// Symbol represents a rune with contextual information about its position in
// the text stream. This allows lexical rules to make decisions based not only
// on the character itself but also on its positional context.
type Symbol struct {
	r     rune
	flags uint
}

// NewSymbol creates a new Symbol with the specified rune and position flags.
// Multiple flags can be combined using a bitwise OR.
func NewSymbol(r rune, flags uint) Symbol {
	return Symbol{
		r:     r,
		flags: flags,
	}
}

// IsEOF returns true if this symbol is at the end of the input stream.
func (s Symbol) IsEOF() bool {
	return (s.flags & FlagEOF) != 0
}

// IsBOF returns true if this symbol is at the beginning of the input stream.
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

// Rune returns the underlying rune value of this symbol.
func (s Symbol) Rune() rune {
	return s.r
}

// String returns a string representation of the Symbol's rune.
func (s Symbol) String() string {
	return string(s.r)
}
