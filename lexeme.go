package textlexer

type LexemeType string

const LexemeTypeUnknown LexemeType = "UNKNOWN"

type Lexeme struct {
	Type LexemeType

	text   []rune
	offset int
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
