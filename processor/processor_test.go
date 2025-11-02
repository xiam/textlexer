package processor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecBoundaryConditions(t *testing.T) {
	tests := []struct {
		name           string
		start          uint64
		offset         uint64
		state          State
		expectedStart  uint64
		expectedOffset uint64
		expectedError  error
	}{
		{
			name:           "continue at max offset",
			start:          0,
			offset:         math.MaxUint64,
			state:          StateContinue,
			expectedStart:  0,
			expectedOffset: math.MaxUint64,
			expectedError:  ErrOffsetOverflow,
		},
		{
			name:           "continue at max offset minus 1",
			start:          0,
			offset:         math.MaxUint64 - 1,
			state:          StateContinue,
			expectedStart:  0,
			expectedOffset: math.MaxUint64,
			expectedError:  nil,
		},
		{
			name:           "pushback at zero offset",
			start:          100,
			offset:         0,
			state:          StatePushBack,
			expectedStart:  100,
			expectedOffset: 0,
			expectedError:  ErrOffsetUnderflow,
		},
		{
			name:           "pushback at offset 1",
			start:          100,
			offset:         1,
			state:          StatePushBack,
			expectedStart:  100,
			expectedOffset: 0,
			expectedError:  nil,
		},
		{
			name:           "accept at max start",
			start:          math.MaxUint64,
			offset:         0,
			state:          StateAccept,
			expectedStart:  math.MaxUint64,
			expectedOffset: 0,
			expectedError:  ErrStartOverflow,
		},
		{
			name:           "accept near max start",
			start:          math.MaxUint64 - 1,
			offset:         0,
			state:          StateAccept,
			expectedStart:  math.MaxUint64,
			expectedOffset: 0,
			expectedError:  nil,
		},
		{
			name:           "accept with overflow: start + offset + 1 > max",
			start:          math.MaxUint64 - 5,
			offset:         5,
			state:          StateAccept,
			expectedStart:  math.MaxUint64 - 5,
			expectedOffset: 5,
			expectedError:  ErrStartOverflow,
		},
		{
			name:           "accept at boundary: start + offset + 1 == max",
			start:          math.MaxUint64 - 10,
			offset:         9,
			state:          StateAccept,
			expectedStart:  math.MaxUint64,
			expectedOffset: 0,
			expectedError:  nil,
		},
		{
			name:           "accept with max offset",
			start:          math.MaxUint64,
			offset:         math.MaxUint64,
			state:          StateAccept,
			expectedStart:  math.MaxUint64,
			expectedOffset: math.MaxUint64,
			expectedError:  ErrStartOverflow,
		},
		{
			name:           "reject at max values",
			start:          math.MaxUint64,
			offset:         math.MaxUint64,
			state:          StateReject,
			expectedStart:  math.MaxUint64,
			expectedOffset: 0,
			expectedError:  nil,
		},
		{
			name:           "match at max start",
			start:          math.MaxUint64,
			offset:         0,
			state:          StateMatch,
			expectedStart:  math.MaxUint64,
			expectedOffset: 0,
			expectedError:  nil,
		},
		{
			name:           "match with overflow: start + offset > max",
			start:          math.MaxUint64 - 5,
			offset:         6,
			state:          StateMatch,
			expectedStart:  math.MaxUint64 - 5,
			expectedOffset: 6,
			expectedError:  ErrStartOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, offset, err := exec(tt.start, tt.offset, tt.state)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedStart, start, "start mismatch")
			assert.Equal(t, tt.expectedOffset, offset, "offset mismatch")
		})
	}
}

func TestExecInvalidState(t *testing.T) {
	assert.Panics(t, func() {
		exec(0, 0, State(999))
	}, "should panic on invalid state")
}

func TestExecBasicSequences(t *testing.T) {
	t.Run("continue then accept", func(t *testing.T) {
		start, offset := uint64(0), uint64(0)
		var err error

		start, offset, err = exec(start, offset, StateContinue)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), start)
		assert.Equal(t, uint64(1), offset)

		start, offset, err = exec(start, offset, StateAccept)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), start)
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("continue then pushback then accept", func(t *testing.T) {
		start, offset := uint64(0), uint64(0)
		var err error

		start, offset, err = exec(start, offset, StateContinue)
		require.NoError(t, err)
		start, offset, err = exec(start, offset, StateContinue)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), offset)

		start, offset, err = exec(start, offset, StatePushBack)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), offset)

		start, offset, err = exec(start, offset, StateAccept)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), start)
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("continue then reject", func(t *testing.T) {
		start, offset := uint64(10), uint64(0)
		var err error

		for i := 0; i < 5; i++ {
			start, offset, err = exec(start, offset, StateContinue)
			require.NoError(t, err)
		}
		assert.Equal(t, uint64(5), offset)

		start, offset, err = exec(start, offset, StateReject)
		require.NoError(t, err)
		assert.Equal(t, uint64(10), start)
		assert.Equal(t, uint64(0), offset)
	})
}

func TestExecStateMatch(t *testing.T) {
	t.Run("basic match operation", func(t *testing.T) {
		// Match at current position (zero-length)
		start, offset, err := exec(0, 0, StateMatch)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), start) // No advancement
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("match with offset", func(t *testing.T) {
		// Match after lookahead
		start, offset, err := exec(10, 5, StateMatch)
		require.NoError(t, err)
		assert.Equal(t, uint64(15), start) // start becomes start + offset
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("match vs accept difference", func(t *testing.T) {
		// Same starting conditions, different results
		start1, offset1, err1 := exec(10, 5, StateAccept)
		require.NoError(t, err1)
		start2, offset2, err2 := exec(10, 5, StateMatch)
		require.NoError(t, err2)

		assert.Equal(t, uint64(16), start1) // Accept: start + offset + 1
		assert.Equal(t, uint64(15), start2) // Match: start + offset
		assert.Equal(t, uint64(0), offset1)
		assert.Equal(t, uint64(0), offset2)
	})
}

func TestExecArithmeticBoundaries(t *testing.T) {
	t.Run("accept boundary cases", func(t *testing.T) {
		tests := []struct {
			name   string
			start  uint64
			offset uint64
			hasErr bool
		}{
			{"safe: 0 + 0 + 1", 0, 0, false},
			{"safe: max-1 + 0 + 1", math.MaxUint64 - 1, 0, false},
			{"overflow: max + 0 + 1", math.MaxUint64, 0, true},
			{"safe: max-10 + 9 + 1", math.MaxUint64 - 10, 9, false},
			{"overflow: max-10 + 10 + 1", math.MaxUint64 - 10, 10, true},
			{"overflow: 1 + max-1 + 1", 1, math.MaxUint64 - 1, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := exec(tt.start, tt.offset, StateAccept)
				if tt.hasErr {
					assert.ErrorIs(t, err, ErrStartOverflow)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("match boundary cases", func(t *testing.T) {
		tests := []struct {
			name   string
			start  uint64
			offset uint64
			hasErr bool
		}{
			{"safe: 0 + 0", 0, 0, false},
			{"safe: max-10 + 10", math.MaxUint64 - 10, 10, false},
			{"overflow: max-10 + 11", math.MaxUint64 - 10, 11, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := exec(tt.start, tt.offset, StateMatch)
				if tt.hasErr {
					assert.ErrorIs(t, err, ErrStartOverflow)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestExecErrorIdempotency(t *testing.T) {
	tests := []struct {
		name   string
		start  uint64
		offset uint64
		state  State
		err    error
	}{
		{
			"continue overflow",
			0,
			math.MaxUint64,
			StateContinue,
			ErrOffsetOverflow,
		},
		{
			"pushback underflow",
			100,
			0,
			StatePushBack,
			ErrOffsetUnderflow,
		},
		{
			"accept overflow",
			math.MaxUint64,
			0,
			StateAccept,
			ErrStartOverflow,
		},
		{
			"match overflow",
			math.MaxUint64,
			1,
			StateMatch,
			ErrStartOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Repeated calls should not change state on error
			for i := 0; i < 3; i++ {
				start, offset, err := exec(tt.start, tt.offset, tt.state)
				assert.ErrorIs(t, err, tt.err)
				assert.Equal(t, tt.start, start)
				assert.Equal(t, tt.offset, offset)
			}
		})
	}
}

func TestStateProcessor(t *testing.T) {
	t.Run("basic state progression", func(t *testing.T) {
		processor := NewStateProcessor()

		err := processor.Execute(StateContinue)
		require.NoError(t, err)
		start, offset := processor.Position()
		assert.Equal(t, uint64(0), start)
		assert.Equal(t, uint64(1), offset)

		err = processor.Execute(StateAccept)
		require.NoError(t, err)
		start, offset = processor.Position()
		assert.Equal(t, uint64(2), start)
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("multiple processors are independent", func(t *testing.T) {
		p1 := NewStateProcessor()
		p2 := NewStateProcessor()

		p1.Execute(StateContinue)
		p1.Execute(StateContinue)
		p1.Execute(StateAccept)

		p2.Execute(StateContinue)
		start, offset := p2.Position()
		assert.Equal(t, uint64(0), start)
		assert.Equal(t, uint64(1), offset)
	})

	t.Run("error recovery", func(t *testing.T) {
		processor := NewStateProcessor()

		// Should error but leave state unchanged
		err := processor.Execute(StatePushBack)
		assert.ErrorIs(t, err, ErrOffsetUnderflow)
		start, offset := processor.Position()
		assert.Equal(t, uint64(0), start)
		assert.Equal(t, uint64(0), offset)

		// Should work after error
		err = processor.Execute(StateContinue)
		require.NoError(t, err)
		start, offset = processor.Position()
		assert.Equal(t, uint64(0), start)
		assert.Equal(t, uint64(1), offset)
	})

	t.Run("complex sequence", func(t *testing.T) {
		processor := NewStateProcessor()

		for i := 0; i < 10; i++ {
			err := processor.Execute(StateContinue)
			require.NoError(t, err)
		}

		for i := 0; i < 5; i++ {
			err := processor.Execute(StatePushBack)
			require.NoError(t, err)
		}

		_, offset := processor.Position()
		assert.Equal(t, uint64(5), offset)

		err := processor.Execute(StateAccept)
		require.NoError(t, err)
		start, offset := processor.Position()
		assert.Equal(t, uint64(6), start)
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("match sequences", func(t *testing.T) {
		processor := NewStateProcessor()

		// Consume one char
		processor.Execute(StateContinue)
		processor.Execute(StateAccept)
		start, _ := processor.Position()
		assert.Equal(t, uint64(2), start)

		// Match at current position (zero-length)
		processor.Execute(StateMatch)
		start, _ = processor.Position()
		assert.Equal(t, uint64(2), start) // Start does not advance

		// Match again, should have no effect on position
		processor.Execute(StateMatch)
		start, _ = processor.Position()
		assert.Equal(t, uint64(2), start)
	})
}

func TestStateProcessorStress(t *testing.T) {
	t.Run("many continues and accepts", func(t *testing.T) {
		processor := NewStateProcessor()

		iterations := 10000
		for i := 0; i < iterations; i++ {
			err := processor.Execute(StateContinue)
			require.NoError(t, err)

			if (i+1)%100 == 0 {
				err = processor.Execute(StateAccept)
				require.NoError(t, err)
			}
		}

		start, offset := processor.Position()
		expectedStart := uint64((iterations / 100) * 101)
		assert.Equal(t, expectedStart, start)
		assert.Equal(t, uint64(0), offset)
	})

	t.Run("continues with pushbacks", func(t *testing.T) {
		processor := NewStateProcessor()

		for i := 0; i < 1000; i++ {
			// Continue 10, pushback 5, then accept
			for j := 0; j < 10; j++ {
				err := processor.Execute(StateContinue)
				require.NoError(t, err)
			}

			for j := 0; j < 5; j++ {
				err := processor.Execute(StatePushBack)
				require.NoError(t, err)
			}

			err := processor.Execute(StateAccept)
			require.NoError(t, err)
		}

		start, offset := processor.Position()
		expectedStart := uint64(1000 * 6)
		assert.Equal(t, expectedStart, start)
		assert.Equal(t, uint64(0), offset)
	})
}

func BenchmarkExec(b *testing.B) {
	b.Run("continue", func(b *testing.B) {
		start := uint64(0)
		offset := uint64(0)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			start, offset, _ = exec(start, offset, StateContinue)
			if offset > 1000 {
				offset = 0
			}
		}
	})

	b.Run("accept", func(b *testing.B) {
		start := uint64(0)
		offset := uint64(10)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			start, offset, _ = exec(start, offset, StateAccept)
			offset = 10
		}
	})

	b.Run("mixed_operations", func(b *testing.B) {
		start := uint64(0)
		offset := uint64(5)
		operations := []State{
			StateContinue,
			StateContinue,
			StateContinue,
			StatePushBack,
			StatePushBack,
			StateAccept,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			op := operations[i%len(operations)]
			if op == StatePushBack && offset == 0 {
				offset = 5
			}
			start, offset, _ = exec(start, offset, op)
		}
	})

	b.Run("overflow_check", func(b *testing.B) {
		start := uint64(math.MaxUint64 - 100)
		offset := uint64(50)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _, _ = exec(start, offset, StateAccept)
		}
	})
}

func BenchmarkStateProcessor(b *testing.B) {
	b.Run("continue_sequence", func(b *testing.B) {
		processor := NewStateProcessor()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			processor.Execute(StateContinue)
			if i%1000 == 0 {
				processor.Execute(StateAccept)
			}
		}
	})

	b.Run("mixed_operations", func(b *testing.B) {
		processor := NewStateProcessor()
		operations := []State{
			StateContinue,
			StateContinue,
			StateContinue,
			StatePushBack,
			StatePushBack,
			StateAccept,
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			op := operations[i%len(operations)]
			_, offset := processor.Position()
			if op == StatePushBack && offset == 0 {
				processor.Execute(StateContinue)
				processor.Execute(StateContinue)
			}
			processor.Execute(op)
		}
	})
}
