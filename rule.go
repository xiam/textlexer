package textlexer

type Rule func(s Symbol) (next Rule, state State)
