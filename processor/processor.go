// Package processor provides a state machine VM for tracking position during
// lexical analysis with support for lookahead and backtracking.
//
// # State Machine Overview
//
// The processor implements a simple VM that tracks position using two
// counters:
//   - start: the committed position (where accepted tokens begin)
//   - offset: the lookahead distance from start
//   - current position = start + offset
//
// # The Four States
//
// The VM supports four operations that modify these counters:
//
//  1. StateContinue: Advance forward to examine the next position
//     Operation: offset = offset + 1
//     Use case: Moving forward through input during pattern matching
//
//  2. StateAccept: Commit the current position as accepted
//     Operation: start = start + offset + 1, offset = 0
//     Use case: Marking a valid token match, can be called multiple times
//     to progressively extend a match
//
//  3. StateReject: Abandon lookahead and return to last accepted position
//     Operation: offset = 0
//     Use case: Pattern match failed, backtrack to last good position
//
//  4. StatePushBack: Step back one position
//     Operation: offset = offset - 1
//     Use case: Undo a speculative forward move, usually before accepting
//
// # The Accept-then-Continue Pattern
//
// A key feature is the ability to call StateAccept multiple times while
// continuing to explore input. This enables "maximal munch" lexing where
// you want the longest possible match.
//
// Example: Matching an integer "1234"
//
//	Initial state: start=0, offset=0
//
//	Read '1':
//	  StateContinue → start=0, offset=1 (examining '1')
//	  StateAccept   → start=1, offset=0 (accepted "1")
//
//	Read '2':
//	  StateContinue → start=1, offset=1 (examining '2')
//	  StateAccept   → start=2, offset=0 (accepted "12")
//
//	Read '3':
//	  StateContinue → start=2, offset=1 (examining '3')
//	  StateAccept   → start=3, offset=0 (accepted "123")
//
//	Read '4':
//	  StateContinue → start=3, offset=1 (examining '4')
//	  StateAccept   → start=4, offset=0 (accepted "1234")
//
//	Read 'x' (not a digit):
//	  StateContinue → start=4, offset=1 (examining 'x')
//	  StateReject   → start=4, offset=0 (rejected, final match is "1234")
//
// After each StateAccept, 'start' represents:
//   - The length of the accepted token so far (from position 0)
//   - Where the next token would begin if we stopped here
//
// # Error Handling
//
// The VM protects against:
//   - Offset overflow (StateContinue beyond MaxUint64)
//   - Offset underflow (StatePushBack when offset is 0)
//   - Start overflow (StateAccept would exceed MaxUint64)
//
// These errors prevent silent wraparound bugs that could cause incorrect
// position tracking.
package processor

import (
	"errors"
	"math"
)

var (
	// ErrOffsetOverflow indicates the offset would exceed MaxUint64 on a
	// CONTINUE operation.
	ErrOffsetOverflow = errors.New("offset overflow on continue")

	// ErrOffsetUnderflow indicates a PUSHBACK was attempted when offset is
	// already 0.
	ErrOffsetUnderflow = errors.New("offset underflow on pushback")

	// ErrStartOverflow indicates the start position would exceed MaxUint64 on an
	// ACCEPT operation.
	ErrStartOverflow = errors.New("start overflow on accept")
)

// `exec` performs a single state transition in the VM.
// It takes the current start position, offset, and a state instruction,
// then returns the new start, offset, and any error that occurred.
func exec(
	// The committed position (last accepted position)
	start uint64,

	// The lookahead distance from start
	offset uint64,

	// The state instruction to execute
	state State,
) (uint64, uint64, error) {
	switch state {

	case StateContinue:
		// Advance to examine the next symbol.
		// Position after: start + (offset+1)

		if offset == math.MaxUint64 {
			return start, offset, ErrOffsetOverflow
		}
		return start, offset + 1, nil

	case StatePushBack:
		// Move back one symbol for reconsideration.
		// Position after: start + (offset-1)

		if offset < 1 {
			return start, offset, ErrOffsetUnderflow
		}
		return start, offset - 1, nil

	case StateAccept:
		// Commit all symbols up to and including the current position.
		// The +1 moves start past the last accepted symbol.
		// New token starts at: start + offset + 1, offset resets to 0
		//
		// After StateAccept, 'start' represents both:
		// - The length of the accepted token (from position 0)
		// - The starting position for the next token

		if offset == math.MaxUint64 {
			return start, offset, ErrStartOverflow
		}

		if start > math.MaxUint64-(offset+1) {
			return start, offset, ErrStartOverflow
		}
		return start + offset + 1, 0, nil

	case StateReject:
		// Abandon current lookahead, return to last accepted position.
		// Position after: start + 0 (ready to try different rules)

		return start, 0, nil
	}

	panic("unreachable: unknown state")
}

// StateProcessor defines the interface for a state processing VM.
//
// After StateAccept executes, Position() returns (start, offset) where:
//   - 'start' is the length of symbols accepted for the current token, and is
//     also where the next token would begin
//   - 'offset' is reset to 0 and can grow again with StateContinue
//
// This allows rules to progressively extend matches:
//
//	Initial: start=0, offset=0
//	Process '1': Continue → Accept → start=1, offset=0 (accepted "1")
//	Process '2': Continue → Accept → start=2, offset=0 (accepted "12")
//	Process '3': Continue → Accept → start=3, offset=0 (accepted "123")
//	Process 'x': Continue → Reject → start=3, offset=0 (final: "123")
type StateProcessor interface {
	// Execute executes the given state instruction, updating internal start and
	// offset. Returns an error if the operation would cause overflow/underflow.
	Execute(state State) (err error)

	// Position returns the current start (committed position) and offset
	// (lookahead distance).  The current absolute position is start + offset.
	//
	// After a StateAccept, 'start' represents the length of the accepted token.
	Position() (start uint64, offset uint64)
}

type stateProcessor struct {
	start  uint64
	offset uint64
}

func NewStateProcessor() StateProcessor {
	return &stateProcessor{}
}

func (sp *stateProcessor) Execute(state State) error {
	var err error
	sp.start, sp.offset, err = exec(sp.start, sp.offset, state)
	return err
}

func (sp *stateProcessor) Position() (uint64, uint64) {
	return sp.start, sp.offset
}
