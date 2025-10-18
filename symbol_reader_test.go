package textlexer

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readAllSymbols is a helper function to drain the SymbolReader and return all symbols.
func readAllSymbols(sr SymbolReader) ([]Symbol, error) {
	var symbols []Symbol
	for {
		sym, err := sr.ReadSymbol()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		symbols = append(symbols, sym)
	}
	return symbols, nil
}

// symbolsToString is a helper to convert a slice of Symbols back to a string for easy comparison.
func symbolsToString(symbols []Symbol) string {
	var b strings.Builder
	for _, s := range symbols {
		// Don't write the synthetic rune for the EOF-only symbol
		if s.Rune() != 0 || s.flags != (FlagBOF|FlagBOL|FlagEOL|FlagEOF) {
			b.WriteRune(s.Rune())
		}
	}
	return b.String()
}

func TestBufferedSymbolReader(t *testing.T) {

	t.Run("Empty Input", func(t *testing.T) {
		sr := NewSymbolReader(strings.NewReader(""))
		sym, err := sr.ReadSymbol()

		require.Error(t, err)
		require.Equal(t, int32(0), sym.Rune(), "Rune for empty input should be 0")

		assert.True(t, sym.IsEOF())
		assert.True(t, sym.IsBOF())
		assert.True(t, sym.IsEOL())
		assert.True(t, sym.IsBOL())
	})

	t.Run("Single Character Input", func(t *testing.T) {
		sr := NewSymbolReader(strings.NewReader("a"))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, 1)

		s := symbols[0]
		assert.Equal(t, 'a', s.Rune())
		// The single character should be BOF, BOL, EOL, and EOF
		expectedFlags := FlagBOF | FlagBOL | FlagEOL | FlagEOF
		assert.Equal(t, expectedFlags, s.flags, "Flags for a single character should be BOF|BOL|EOL|EOF")
	})

	t.Run("Simple Multiline Input", func(t *testing.T) {
		input := "a\nb"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, 3)
		assert.Equal(t, input, symbolsToString(symbols))

		// Check 'a'
		assert.Equal(t, FlagBOF|FlagBOL, symbols[0].flags)
		// Check '\n'
		assert.Equal(t, FlagEOL, symbols[1].flags)
		// Check 'b'
		assert.Equal(t, FlagBOL|FlagEOF|FlagEOL, symbols[2].flags)
	})

	t.Run("File Ending Without Newline", func(t *testing.T) {
		input := "abc"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, 3)

		// Check 'a'
		assert.Equal(t, FlagBOF|FlagBOL, symbols[0].flags)
		// Check 'b'
		assert.Equal(t, FlagNone, symbols[1].flags)
		// Check 'c' - last symbol should get EOF and EOL flags
		assert.Equal(t, FlagEOF|FlagEOL, symbols[2].flags)
	})

	t.Run("Multiple Consecutive Newlines", func(t *testing.T) {
		input := "a\n\nb"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, 4)
		assert.Equal(t, input, symbolsToString(symbols))

		// Check 'a'
		assert.Equal(t, FlagBOF|FlagBOL, symbols[0].flags)
		// Check first '\n'
		assert.Equal(t, FlagEOL, symbols[1].flags)
		// Check second '\n'
		assert.Equal(t, FlagBOL|FlagEOL, symbols[2].flags)
		// Check 'b'
		assert.Equal(t, FlagBOL|FlagEOF|FlagEOL, symbols[3].flags)
	})

	t.Run("Buffer Boundary Condition", func(t *testing.T) {
		// Create input that forces a buffer refill right after a newline.
		// `symbolReaderBufSize - 1` 'a's, then a newline, then 'b', 'c'.
		// The newline will be the last char in the first read batch.
		// 'b' will be the first char in the second read batch.
		input := strings.Repeat("a", symbolReaderBufSize-1) + "\nbc"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, symbolReaderBufSize+2)
		assert.Equal(t, input, symbolsToString(symbols))

		// Check symbol before boundary ('\n')
		newlineSymbol := symbols[symbolReaderBufSize-1]
		assert.Equal(t, '\n', newlineSymbol.Rune())
		assert.True(t, newlineSymbol.IsEOL())
		assert.False(t, newlineSymbol.IsBOL())

		// Check symbol at boundary ('b')
		boundarySymbol := symbols[symbolReaderBufSize]
		assert.Equal(t, 'b', boundarySymbol.Rune())
		assert.True(t, boundarySymbol.IsBOL(), "'b' after newline should be BOL")
		assert.False(t, boundarySymbol.IsEOL())

		// Check last symbol ('c')
		lastSymbol := symbols[symbolReaderBufSize+1]
		assert.Equal(t, 'c', lastSymbol.Rune())
		assert.True(t, lastSymbol.IsEOF(), "Last symbol should be EOF")
		assert.True(t, lastSymbol.IsEOL(), "Last symbol should also be EOL")
	})

	t.Run("Unicode Characters", func(t *testing.T) {
		input := "你好\n世界"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, 5) // 2 chars + newline + 2 chars
		assert.Equal(t, input, symbolsToString(symbols))

		// 你
		assert.Equal(t, FlagBOF|FlagBOL, symbols[0].flags)
		// 好
		assert.Equal(t, FlagNone, symbols[1].flags)
		// \n
		assert.Equal(t, FlagEOL, symbols[2].flags)
		// 世
		assert.Equal(t, FlagBOL, symbols[3].flags)
		// 界
		assert.Equal(t, FlagEOF|FlagEOL, symbols[4].flags)
	})
}

func TestBufferedSymbolReader_Stress(t *testing.T) {

	t.Run("Large Input Beyond Buffer Capacity", func(t *testing.T) {
		// Create input larger than symbolReaderBufSize (4096)
		size := symbolReaderBufSize * 3
		input := strings.Repeat("x", size)
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, size)
		assert.Equal(t, input, symbolsToString(symbols))

		// First symbol should be BOF|BOL
		assert.Equal(t, FlagBOF|FlagBOL, symbols[0].flags)
		// Last symbol should be EOF|EOL
		assert.Equal(t, FlagEOF|FlagEOL, symbols[size-1].flags)
		// Middle symbols should have no flags
		for i := 1; i < size-1; i++ {
			assert.Equal(t, FlagNone, symbols[i].flags, "Symbol at index %d should have no flags", i)
		}
	})

	t.Run("Very Long Line Without Newlines", func(t *testing.T) {
		// Simulates a minified JS file or CSV with huge field
		size := 10000
		input := strings.Repeat("a", size)
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, size)

		// Only first should have BOL, last should have EOF|EOL
		assert.True(t, symbols[0].IsBOL())
		assert.False(t, symbols[size-2].IsEOL(), "Second to last should not be EOL")
		assert.True(t, symbols[size-1].IsEOF())
	})

	t.Run("File With Only Newlines", func(t *testing.T) {
		size := 1000
		input := strings.Repeat("\n", size)
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, size)

		// All symbols should be newlines with EOL flag
		for i, sym := range symbols {
			assert.Equal(t, '\n', sym.Rune())
			assert.True(t, sym.IsEOL(), "Symbol %d should be EOL", i)

			// First newline should also be BOL
			if i == 0 {
				assert.True(t, sym.IsBOL())
			}
			// Every newline after another newline should be BOL
			if i > 0 {
				assert.True(t, sym.IsBOL(), "Newline at %d should be BOL", i)
			}
			// Last newline should be EOF
			if i == size-1 {
				assert.True(t, sym.IsEOF())
			}
		}
	})

	t.Run("Alternating Pattern Across Many Buffer Refills", func(t *testing.T) {
		// Pattern that spans multiple buffer refills
		pattern := "a\n"
		repetitions := symbolReaderBufSize * 10 // Force many refills
		input := strings.Repeat(pattern, repetitions)
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		require.Len(t, symbols, repetitions*2)

		// Verify pattern integrity across buffer boundaries
		for i := 0; i < len(symbols); i += 2 {
			// 'a' should have BOL flag
			assert.Equal(t, 'a', symbols[i].Rune())
			assert.True(t, symbols[i].IsBOL(), "Symbol at index %d should be BOL", i)

			// '\n' should have EOL flag
			if i+1 < len(symbols) {
				assert.Equal(t, '\n', symbols[i+1].Rune())
				assert.True(t, symbols[i+1].IsEOL(), "Symbol at index %d should be EOL", i+1)
			}
		}
	})

	t.Run("Multiple EOF Calls", func(t *testing.T) {
		input := "test"
		sr := NewSymbolReader(strings.NewReader(input))

		// Read all symbols
		symbols, err := readAllSymbols(sr)
		require.NoError(t, err)
		require.Len(t, symbols, 4)

		// Try reading more - should consistently return EOF
		for i := 0; i < 10; i++ {
			_, err := sr.ReadSymbol()
			assert.Equal(t, io.EOF, err, "Call %d after exhaustion should return EOF", i+1)
		}
	})

	t.Run("Newline at Exact ShortReadSize Boundary", func(t *testing.T) {
		// Force newline to be exactly at position symbolReaderBufSize
		input := strings.Repeat("a", symbolReaderBufSize-1) + "\n" + strings.Repeat("b", symbolReaderBufSize)
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)

		// The newline at position symbolReaderBufSize-1
		newlineIdx := symbolReaderBufSize - 1
		assert.Equal(t, '\n', symbols[newlineIdx].Rune())
		assert.True(t, symbols[newlineIdx].IsEOL())

		// First 'b' after newline should be BOL
		assert.Equal(t, 'b', symbols[newlineIdx+1].Rune())
		assert.True(t, symbols[newlineIdx+1].IsBOL(), "First 'b' after newline should be BOL")
	})

	t.Run("Mixed Whitespace Patterns", func(t *testing.T) {
		// Simulates source code with various indentation
		input := "func main() {\n\t\tif true {\n\t\t\treturn\n\t\t}\n}"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		assert.Equal(t, input, symbolsToString(symbols))

		// Verify all newlines have correct flags
		newlineCount := 0
		for i, sym := range symbols {
			if sym.Rune() == '\n' {
				newlineCount++
				assert.True(t, sym.IsEOL(), "Newline at index %d should be EOL", i)

				// Next non-EOF symbol should be BOL
				if i+1 < len(symbols) {
					assert.True(t, symbols[i+1].IsBOL(), "Symbol after newline at %d should be BOL", i+1)
				}
			}
		}
		assert.Equal(t, 4, newlineCount, "Should have 4 newlines")
	})

	t.Run("Windows Line Endings", func(t *testing.T) {
		input := "line1\r\nline2\r\nline3"
		sr := NewSymbolReader(strings.NewReader(input))
		symbols, err := readAllSymbols(sr)

		require.NoError(t, err)
		assert.Equal(t, input, symbolsToString(symbols))

		// Find all '\n' and verify '\r' before it doesn't have EOL
		for i, sym := range symbols {
			if sym.Rune() == '\n' {
				assert.True(t, sym.IsEOL(), "\\n at index %d should be EOL", i)

				// Previous '\r' should NOT be EOL
				if i > 0 && symbols[i-1].Rune() == '\r' {
					assert.False(t, symbols[i-1].IsEOL(), "\\r before \\n should not be EOL")
				}

				// Next symbol should be BOL
				if i+1 < len(symbols) && !symbols[i+1].IsEOF() {
					assert.True(t, symbols[i+1].IsBOL(), "Symbol after \\n should be BOL")
				}
			}
		}
	})

	t.Run("Pathological Case - Single Char Repeated", func(t *testing.T) {
		// Tests buffer management with simplest possible pattern
		size := symbolReaderBufSize * 2
		input := strings.Repeat("x", size)
		sr := NewSymbolReader(strings.NewReader(input))

		// Read one at a time to stress the reader
		for i := 0; i < size; i++ {
			sym, err := sr.ReadSymbol()
			require.NoError(t, err)
			assert.Equal(t, 'x', sym.Rune())

			if i == 0 {
				assert.True(t, sym.IsBOF())
				assert.True(t, sym.IsBOL())
			} else if i == size-1 {
				assert.True(t, sym.IsEOF())
			} else {
				assert.Equal(t, FlagNone, sym.flags)
			}
		}

		// Should be at EOF now
		_, err := sr.ReadSymbol()
		assert.Equal(t, io.EOF, err)
	})

	t.Run("Performance - Read 1MB of Text", func(t *testing.T) {
		// 1MB of text with realistic content
		line := "The quick brown fox jumps over the lazy dog.\n"
		repetitions := (1024 * 1024) / len(line) // Approximately 1MB
		input := strings.Repeat(line, repetitions)

		sr := NewSymbolReader(strings.NewReader(input))

		symbolCount := 0
		for {
			_, err := sr.ReadSymbol()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			symbolCount++
		}

		expectedCount := len(input)
		assert.Equal(t, expectedCount, symbolCount, "Should read exactly %d symbols", expectedCount)
	})
}
