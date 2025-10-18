package rules

import (
	stdlog "log"

	"github.com/xiam/textlexer"
)

// AcceptCurrentandStop signals the current match as successful and stops.
func AcceptCurrentAndStop(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

// RejectCurrentAndStop signals the current match as failed and stops.
func RejectCurrentAndStop(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateReject
}

// PushBackCurrentAndAccept signals that the last read rune is not part of the
// current match, but the match is still valid up to that point. The scanner
// might move back one position.
func PushBackCurrentAndAccept(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	return Backtrack(1, textlexer.StateAccept)(s)
}

// Backtrack returns a rule that asks the scanner to move back `n` positions in
// the input stream, and then continue with the specified state.
func Backtrack(n int, endState textlexer.State) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if n > 0 {
			stdlog.Printf("backtrack: Backtracking %d positions...", n)
			return Backtrack(n-1, endState), textlexer.StatePushBack
		}
		return nil, endState
	}
}

func BacktrackAndContinue(n int, nextRule textlexer.Rule) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if n > 0 {
			stdlog.Printf("backtrack: Backtracking %d positions...", n)
			return BacktrackAndContinue(n-1, nextRule), textlexer.StatePushBack
		}
		return nextRule, textlexer.StateContinue
	}
}
