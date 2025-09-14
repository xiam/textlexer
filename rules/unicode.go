package rules

const (
	emojiStart = 0x1F600 // Start of emoji range
	emojiEnd   = 0x1F64F // End of emoji range
)

// IsEmoji checks if the given rune is an emoji character.
func IsEmoji(r rune) bool {
	return r >= emojiStart && r <= emojiEnd
}

// IsASCII checks if the given rune is an ASCII character.
func IsASCII(r rune) bool {
	return r >= 0 && r <= 0x7F
}
