package processor

import "fmt"

// State represents an instruction for the state processor VM.
// Each state modifies the internal start and offset values differently,
// enabling flexible sequential processing with lookahead and backtracking.
//
// The processor maintains:
//   - start: the committed position (beginning of next potential token)
//   - offset: the lookahead distance from start
//   - current position = start + offset
type State uint

const (
	// StateContinue advances to examine the next symbol.
	// Operation: offset = offset + 1
	// Position after: start + (offset+1)
	StateContinue State = iota

	// StateAccept commits all symbols up to and including the current position.
	// The +1 moves start past the last accepted symbol to begin the next token.
	// Operation: start = start + offset + 1, offset = 0
	// Position after: (start + offset + 1) + 0
	StateAccept

	// StateReject abandons the current lookahead and returns to the last accepted position.
	// Does not advance start, allowing different rules to be tried from the same position.
	// Operation: offset = 0
	// Position after: start + 0
	StateReject

	// StatePushBack moves back one symbol for reconsideration.
	// Used for backtracking when a symbol should not be part of the current match.
	// Operation: offset = offset - 1
	// Position after: start + (offset-1)
	StatePushBack

	// StateMatch matches the current position without consuming input.
	// Used for zero-length matches (e.g., EOF, anchors).
	// Operation: start = start + offset, offset = 0
	// Position after: (start + offset) + 0
	StateMatch
)

// stateNames maps State values to their human-readable string representations.
var stateNames = map[State]string{
	StateContinue: "CONTINUE",
	StateAccept:   "ACCEPT",
	StateReject:   "REJECT",
	StatePushBack: "PUSHBACK",
	StateMatch:    "MATCH",
}

// String returns the string representation of the State.
// Returns "UnknownState(n)" for undefined state values.
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}

	return fmt.Sprintf("UnknownState(%d)", s)
}
