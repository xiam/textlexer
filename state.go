package textlexer

import "fmt"

// State represents the outcome of processing a symbol within a Rule.
type State uint

const (
	// StateContinue signals a possible match. The rule processor needs to read
	// more input to determine if a full match is successful. The rule must
	// return a non-nil next Rule.
	StateContinue State = iota

	// StateAccept signals a successful match. The current input sequence is a
	// valid token. The rule may optionally return a next Rule to continue
	// scanning for an even longer match.
	StateAccept

	// StateReject signals a failed match. This rule can no longer match the
	// current input sequence and will be deactivated for the current token.
	StateReject

	// StatePushBack indicates that the last-read symbol is not part of the
	// current match. The symbol is "pushed back" to be re-evaluated by the
	// returned 'next' Rule. This allows for lookahead and backtracking.
	StatePushBack
)

var stateNames = map[State]string{
	StateContinue: "CONTINUE",
	StateAccept:   "ACCEPT",
	StateReject:   "REJECT",
	StatePushBack: "PUSHBACK",
}

func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("UnknownState(%d)", s)
}
