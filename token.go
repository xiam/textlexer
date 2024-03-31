package textlexer

import (
	"fmt"
)

type TokenType string

const TokenTypeUnknown TokenType = ""

type Token struct {
	Type TokenType

	text []rune
}

func (t *Token) String() string {
	return fmt.Sprintf("%q::%s", string(t.text), t.Type)
}

func (t *Token) Text() string {
	return string(t.text)
}
