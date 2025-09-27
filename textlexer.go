package textlexer

import (
	"fmt"
	"io"
	"sync"
)

const (
	RuneEOF = rune(-1)
	RuneBOF = rune(-2)
)

type Reader interface {
	io.RuneReader
}

type TextLexer struct {
	reader Reader

	offset    int
	lineNum   int
	colNum    int
	isNewLine bool

	buf []Symbol
	w   int
	r   int

	mu sync.Mutex

	rules    []LexemeType
	rulesMap map[LexemeType]Rule
	rulesMu  sync.Mutex

	processor *rulesProcessor
}

func New(r Reader) *TextLexer {
	return &TextLexer{
		buf:       make([]Symbol, 0, 4096),
		reader:    r,
		rules:     []LexemeType{},
		rulesMap:  map[LexemeType]Rule{},
		lineNum:   1,
		colNum:    0,
		isNewLine: true, // First character is at beginning of line
	}
}

func (lx *TextLexer) getProcessor() (*rulesProcessor, error) {
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	if lx.processor == nil {
		if len(lx.rules) == 0 {
			return nil, fmt.Errorf("no rules defined")
		}
		lx.processor = newRulesProcessor(lx.rules, lx.rulesMap)
	}

	lx.processor.Reset()
	return lx.processor, nil
}

func (lx *TextLexer) AddRule(lexType LexemeType, lexRule Rule) error {
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	if _, ok := lx.rulesMap[lexType]; ok {
		return fmt.Errorf("rule %q already exists", lexType)
	}
	if lexType == "" {
		return fmt.Errorf("rule type cannot be empty")
	}

	if lexRule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	lx.rulesMap[lexType] = lexRule
	lx.rules = append(lx.rules, lexType)
	lx.processor = nil // Invalidate processor so it's rebuilt with the new rule.

	return nil
}

func (lx *TextLexer) MustAddRule(lexType LexemeType, lexRule Rule) {
	if err := lx.AddRule(lexType, lexRule); err != nil {
		panic(fmt.Sprintf("MustAddRule: %v", err))
	}
}

func (lx *TextLexer) Next() (*Lexeme, error) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	// The number of symbols consumed (n) is the single source of truth for all state updates.
	typ, text, n, err := lx.nextLexeme()
	if err != nil {
		return nil, err
	}

	lex := NewLexeme(typ, text, lx.offset)

	// Update our global offset by the number of symbols consumed.
	lx.offset += n

	// compact the buffer by removing consumed symbols.
	lx.buf = lx.buf[n:]
	lx.w = lx.w - n
	lx.r = 0

	return lex, nil
}

// createSymbol creates a Symbol with appropriate positional flags based on current lexer state.
func (lx *TextLexer) createSymbol(r rune, isEOF bool) Symbol {
	flags := uint(FlagNone)

	// Set BOF flag for the very first symbol
	if lx.offset == 0 && lx.r == 0 && lx.w == 0 {
		flags |= FlagBOF
	}

	// Set BOL flag if we're at the beginning of a line
	if lx.isNewLine {
		flags |= FlagBOL
		lx.isNewLine = false
	}

	// Set EOL flag if this is a newline character
	if r == '\n' {
		flags |= FlagEOL
		lx.isNewLine = true
	}

	// Set EOF flag if this is the last symbol
	if isEOF {
		flags |= FlagEOF
	}

	return NewSymbol(r, flags)
}

func (lx *TextLexer) nextLexeme() (LexemeType, string, int, error) {
	processor, err := lx.getProcessor()
	if err != nil {
		return LexemeTypeUnknown, "", 0, fmt.Errorf("processor: %w", err)
	}

	startPos := lx.r

	for {
		var sym Symbol
		var isEOF bool

		if lx.r < lx.w {
			// Read from the existing buffer.
			sym = lx.buf[lx.r]
		} else {
			// Buffer is exhausted, read from the underlying io.Reader.
			var readErr error
			var r rune
			r, _, readErr = lx.reader.ReadRune()
			if readErr != nil {
				if readErr != io.EOF {
					return LexemeTypeUnknown, "", 0, fmt.Errorf("ReadRune: %w", readErr)
				}
				isEOF = true
				r = RuneEOF
			}

			sym = lx.createSymbol(r, isEOF)
			if !isEOF {
				lx.buf = append(lx.buf, sym)
				lx.w++
			}
		}

		// Process the symbol we just got.
		typ, textLen := processor.Process(sym)

		// `textLen` < 0 signals that the processor needs more symbols to decide.
		if textLen < 0 {
			lx.r++
			if isEOF {
				// Processor wants more input, but we are at EOF. This means no token
				// can be formed from the remaining buffer.
				return LexemeTypeUnknown, "", 0, fmt.Errorf("lexer: rule remained inconclusive at EOF")
			}
			// We may continue processing with more input.
			continue
		}

		if textLen == 0 {
			if startPos < lx.w {
				typ = LexemeTypeUnknown
				textLen = 1 // Force consumption of one symbol.
			} else if isEOF {
				return LexemeTypeUnknown, "", 0, io.EOF
			} else {
				return LexemeTypeUnknown, "", 0, fmt.Errorf("lexer: rule returned zero-length token")
			}
		}

		if isEOF && lx.r == startPos {
			// We are at EOF and the processor did not consume any symbols.
			return LexemeTypeUnknown, "", 0, io.EOF
		}

		// Extract the text from the symbols
		symbols := lx.buf[startPos : startPos+textLen]
		runes := make([]rune, len(symbols))
		for i, s := range symbols {
			runes[i] = s.Rune()
		}
		text := string(runes)

		// The read pointer is now positioned at the end of the consumed token.
		lx.r = startPos + textLen

		// Update line and column tracking
		for _, s := range symbols {
			if s.Rune() == '\n' {
				lx.lineNum++
				lx.colNum = 0
			} else {
				lx.colNum++
			}
		}

		return typ, text, textLen, nil
	}
}
