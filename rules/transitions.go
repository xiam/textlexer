package rules

import (
	"github.com/xiam/textlexer"
)

// AcceptCurrentandStop signals the current match as successful and stops.
func AcceptCurrentAndStop(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateAccept
}

// RejectCurrentAndStop signals the current match as failed and stops.
func RejectCurrentAndStop(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateReject
}

// PushBackCurrentAndAccept signals that the last read rune is not part of the
// current match, but the match is still valid up to that point. The scanner
// might move back one position.
func PushBackCurrentAndAccept(r rune) (textlexer.Rule, textlexer.State) {
	return AcceptCurrentAndStop, textlexer.StatePushBack
}

// BacktrackAndAccept is a rule that allows pushing-back by n positions, and
// accepting the final position after backtracking.
func BacktrackAndAccept(n int) textlexer.Rule {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		if n > 0 {
			return BacktrackAndAccept(n - 1), textlexer.StatePushBack
		}
		return AcceptCurrentAndStop, textlexer.StatePushBack
	}
}

// NewContinueWith returns a rule that signals the scanner to continue with the
// specified next rule.
func NewContinueWith(next textlexer.Rule) textlexer.Rule {
	return func(r rune) (textlexer.Rule, textlexer.State) {
		return next, textlexer.StateContinue
	}
}
