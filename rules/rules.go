package rules

import (
	"github.com/xiam/textlexer"
)

// Whitespace matches one or more whitespace characters.
// Example: ` `, ` \t`, `\n\r\n`, `\t\t `
func Whitespace(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return newCharacterClassMatcher(
		isCommonWhitespace,
		1,
		-1,
	)(s)
}
