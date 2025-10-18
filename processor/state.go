package processor

import "fmt"

// State represents an instruction for the state processor VM.
// Each state modifies the internal start and offset values differently,
// enabling flexible sequential processing with lookahead and backtracking.
type State uint

const (
	// StateContinue advances the offset by one position.
	// Used to continue scanning/processing forward through input.
	// Operation: offset = offset + 1
	StateContinue State = iota

	// StateAccept commits the current position and resets the offset.
	// Used to mark a successful match/parse and advance the start position.
	// Operation: start = start + offset + 1, offset = 0
	StateAccept

	// StateReject abandons the current position and resets the offset.
	// Used to mark a failed match/parse without advancing the start position.
	// Operation: offset = 0
	StateReject

	// StatePushBack moves the offset back by one position.
	// Used for backtracking or undoing a speculative forward move.
	// Operation: offset = offset - 1
	StatePushBack
)

// stateNames maps State values to their human-readable string representations.
var stateNames = map[State]string{
	StateContinue: "CONTINUE",
	StateAccept:   "ACCEPT",
	StateReject:   "REJECT",
	StatePushBack: "PUSHBACK",
}

// String returns the string representation of the State.
// Returns "UnknownState(n)" for undefined state values.
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}

	return fmt.Sprintf("UnknownState(%d)", s)
}
