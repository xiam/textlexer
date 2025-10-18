package textlexer

import (
	"github.com/xiam/textlexer/processor"
)

type State = processor.State

// A Rule is a state transition function for a finite automaton.  It accepts a
// Symbol and returns the next Rule (the next state) and a State value
// indicating the result of the transition (e.g., continue, accept, reject,
// pushback).
type Rule func(s Symbol) (nextRule Rule, state State)

type RuleProcessor interface {
	Process(symbol Symbol) (nextRule Rule, state State, err error)
}
