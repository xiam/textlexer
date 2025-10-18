package processor

import (
	"errors"
	"math"
)

var (
	// ErrOffsetOverflow indicates the offset would exceed MaxUint64 on a CONTINUE operation.
	// This prevents the VM from wrapping around to 0 unexpectedly.
	ErrOffsetOverflow = errors.New("offset overflow on continue")

	// ErrOffsetUnderflow indicates a PUSHBACK was attempted when offset is already 0.
	// This prevents backing up beyond the start position.
	ErrOffsetUnderflow = errors.New("offset underflow on pushback")

	// ErrStartOverflow indicates the start position would exceed MaxUint64 on an ACCEPT operation.
	// This prevents the committed position from wrapping around.
	ErrStartOverflow = errors.New("start overflow on accept")
)

// exec performs a single state transition in the VM.
// It takes the current start position, offset, and a state instruction,
// then returns the new start, offset, and any error that occurred.
//
// Parameters:
// - start: the committed position (or last accepted position)
// - offset: the distance from start we're currently exploring (lookahead/backtrack)
// - current position = start + offset
func exec(start uint64, offset uint64, state State) (uint64, uint64, error) {
	switch state {

	case StateContinue:
		// Advance offset by one, ensuring we don't overflow

		if offset == math.MaxUint64 {
			return start, offset, ErrOffsetOverflow
		}
		return start, offset + 1, nil

	case StatePushBack:
		// Move offset back by one, ensuring we don't go below zero

		if offset < 1 {
			return start, offset, ErrOffsetUnderflow
		}
		return start, offset - 1, nil

	case StateAccept:
		// Commit the current position and reset for next sequence

		if offset == math.MaxUint64 {
			return start, offset, ErrStartOverflow
		}

		if start > math.MaxUint64-(offset+1) {
			return start, offset, ErrStartOverflow
		}
		return start + offset + 1, 0, nil

	case StateReject:
		// Abort current sequence and reset offset

		return start, 0, nil
	}

	panic("unreachable: unknown state")
}

// StateProcessor defines the interface for a state processing VM.
type StateProcessor interface {
	// Execute executes the given state instruction, updating internal start and
	// offset. Returns an error if the operation would cause overflow/underflow.
	Execute(state State) (err error)

	// Position returns the current absolute position (start + offset).
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
