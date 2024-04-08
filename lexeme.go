package textlexer

import (
	"fmt"
)

type LexemeType string

const LexemeTypeUnknown LexemeType = "UNKNOWN"

type Lexeme struct {
	Type LexemeType

	text   []rune
	offset int
}

func (t *Lexeme) String() string {
	return fmt.Sprintf("(%s %q)", t.Type, string(t.text))
}

func (t *Lexeme) Text() string {
	return string(t.text)
}

func NewLexeme(typ LexemeType, text string) *Lexeme {
	return &Lexeme{
		Type: typ,
		text: []rune(text),
	}
}
