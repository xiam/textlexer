package textlexer

type Rule func(r rune) (next Rule, state State)
