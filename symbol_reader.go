package textlexer

import (
	"errors"
	"fmt"
	"io"
)

const (
	symbolReaderBufSize = 128
)

var (
	eofSymbol = NewSymbol(0, FlagBOF|FlagEOF|FlagEOL|FlagBOL)
)

// SymbolReader reads runes from an underlying io.RuneReader and converts them
// to Symbols with appropriate positional flags (BOF, BOL, EOL, EOF).
type SymbolReader interface {
	// ReadSymbol reads and returns the next Symbol from the input stream.
	// It returns io.EOF when the end of the stream is reached.
	ReadSymbol() (Symbol, error)
}

// bufferedSymbolReader implements SymbolReader with efficient buffering and
// clean state management for production use.
type bufferedSymbolReader struct {
	rr  io.RuneReader // underlying rune reader
	buf []Symbol      // buffer of Symbols

	r int // read index relative to the start of buf
	w int // write index relative to the start of buf

	isNewLine  bool // track if we're at the start of a new line
	eofReached bool // track if we've hit EOF on the underlying reader
}

// NewSymbolReader creates a new SymbolReader that reads from the provided
// io.RuneReader.
func NewSymbolReader(rr io.RuneReader) SymbolReader {
	return &bufferedSymbolReader{
		rr:  rr,
		buf: make([]Symbol, 0, symbolReaderBufSize),

		r: 0,
		w: 0,

		isNewLine:  true, // start at beginning of line
		eofReached: false,
	}
}

// ReadSymbol reads and returns the next Symbol from the input stream.
func (sr *bufferedSymbolReader) ReadSymbol() (Symbol, error) {
	err := sr.ensureAvailable()
	if err != nil && !errors.Is(err, io.EOF) {
		return eofSymbol, err
	}

	if sr.r >= sr.w {
		return eofSymbol, io.EOF
	}

	sym := sr.buf[sr.r]
	sr.r++

	return sym, nil
}

func (sr *bufferedSymbolReader) ensureAvailable() error {

	// Compact the buffer
	needsCompaction := sr.r > 0 && (sr.w-sr.r < symbolReaderBufSize/2)
	if needsCompaction {
		copy(sr.buf, sr.buf[sr.r:sr.w])
		sr.w -= sr.r
		sr.r = 0
		sr.buf = sr.buf[:sr.w]
	}

	// If we've consumed a lot of symbols, shift the buffer to free space.
	for sr.w < symbolReaderBufSize && !sr.eofReached {
		r, _, err := sr.rr.ReadRune()
		if err != nil {
			sr.eofReached = errors.Is(err, io.EOF)
			if !sr.eofReached {
				return fmt.Errorf("ReadRune: %w", err)
			}

			// If EOF reached and we have symbols, set EOF and EOL flags on the last
			// symbol.
			if sr.w > 0 {
				sr.buf[sr.w-1].flags |= FlagEOF | FlagEOL
			}
			return nil
		}

		sr.buf = append(sr.buf, sr.createSymbol(r))
		sr.w++
	}

	return nil
}

func (sr *bufferedSymbolReader) createSymbol(r rune) Symbol {
	var flags uint

	// Set BOF flag if this is the first symbol read.
	if sr.r == 0 && sr.w == 0 {
		flags |= FlagBOF
	}

	// Set BOL flag if the previous symbol was a newline.
	if sr.isNewLine {
		flags |= FlagBOL
		sr.isNewLine = false
	}

	// Set EOL flag if the current symbol is a newline.
	if r == '\n' {
		flags |= FlagEOL
		sr.isNewLine = true
	}

	return NewSymbol(r, flags)
}
