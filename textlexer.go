// Package textlexer provides a flexible, rule-based engine for lexical analysis.
//
// A Rule is a state-transition function that processes input one Symbol at a
// time. The lexer runs all defined rules in parallel, buffering input until
// all rules have either rejected the input or terminated. It then selects the
// longest valid match as the next lexeme.
// If multiple rules match the same longest text, the one added first is chosen.
package textlexer

import (
	"fmt"
	"io"
	"sync"
)

const (
	// RuneEOF represents the end-of-file marker as a rune.
	RuneEOF = rune(-1)
)

// TextLexer orchestrates the tokenization of an input stream according to a
// set of user-defined rules. It manages input buffering, state tracking (line
// and column numbers), and the rule processing engine.
type TextLexer struct {
	reader io.RuneReader

	offset  int
	lineNum int
	colNum  int

	isNewLine bool

	buf []Symbol
	w   int
	r   int

	mu sync.Mutex

	rules    []LexemeType
	rulesMap map[LexemeType]Rule
	rulesMu  sync.RWMutex

	processor *rulesProcessor
}

// New creates a new TextLexer that reads from the provided io.RuneReader.
func New(rr io.RuneReader) *TextLexer {
	return &TextLexer{
		buf:       make([]Symbol, 0, 4096),
		reader:    rr,
		rules:     []LexemeType{},
		rulesMap:  map[LexemeType]Rule{},
		lineNum:   1,
		colNum:    0,
		isNewLine: true,
	}
}

// AddRule registers a new tokenizing rule with the lexer.
// This method is not safe for concurrent use with Next(). All rules should be
// added before tokenization begins.
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

// MustAddRule is like AddRule but panics if the rule cannot be added.
func (lx *TextLexer) MustAddRule(lexType LexemeType, lexRule Rule) {
	if err := lx.AddRule(lexType, lexRule); err != nil {
		panic(fmt.Sprintf("MustAddRule: %v", err))
	}
}

// Next reads from the input and returns the next recognized Lexeme.
//
// It returns an io.EOF error only when the stream is fully consumed and no more
// lexemes can be produced. Any other error indicates a problem with the
// underlying reader or an unrecoverable state (e.g., a rule that requires more
// input at EOF).
//
// This method is safe for concurrent use by multiple goroutines.
func (lx *TextLexer) Next() (*Lexeme, error) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	typ, runes, n, err := lx.nextLexeme()
	if err != nil {
		return nil, err
	}

	lex := NewLexeme(typ, runes, lx.offset)

	lx.offset += n

	lx.buf = lx.buf[n:]
	lx.w = lx.w - n
	lx.r = 0

	return lex, nil
}

func (lx *TextLexer) getProcessor() (*rulesProcessor, error) {
	// Fast path: Check for existing processor with a read lock.
	lx.rulesMu.RLock()
	p := lx.processor
	lx.rulesMu.RUnlock()

	if p != nil {
		p.Reset()
		return p, nil
	}

	// Slow path: Acquire a write lock to create the processor.
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	// Double-check in case another goroutine created it while we were waiting for the lock.
	if lx.processor != nil {
		lx.processor.Reset()
		return lx.processor, nil
	}

	if len(lx.rules) == 0 {
		return nil, fmt.Errorf("no rules defined")
	}
	lx.processor = newRulesProcessor(lx.rules, lx.rulesMap)
	return lx.processor, nil
}

func (lx *TextLexer) createSymbol(r rune, isEOF bool) Symbol {
	flags := uint(FlagNone)

	// Set BOF flag only for the very first symbol.
	if lx.offset == 0 && lx.r == 0 && lx.w == 0 {
		flags |= FlagBOF
	}

	// Set BOL flag if the previous symbol was a newline.
	if lx.isNewLine {
		flags |= FlagBOL
		lx.isNewLine = false
	}

	// Set EOL flag if the current symbol is a newline.
	if r == '\n' {
		flags |= FlagEOL
		lx.isNewLine = true
	}

	// Set EOF flag if this is the end of the input stream.
	if isEOF {
		flags |= FlagEOF
	}

	return NewSymbol(r, flags)
}

func (lx *TextLexer) nextLexeme() (LexemeType, []rune, int, error) {
	processor, err := lx.getProcessor()
	if err != nil {
		return LexemeTypeUnknown, nil, 0, fmt.Errorf("processor: %w", err)
	}

	startPos := lx.r

	for {
		var sym Symbol
		var isEOF bool

		if lx.r < lx.w {
			// Read from existing buffer.
			sym = lx.buf[lx.r]
		} else {
			// Buffer is exhausted, read a new rune.
			var r rune
			r, _, err = lx.reader.ReadRune()
			if err != nil {
				if err != io.EOF {
					return LexemeTypeUnknown, nil, 0, fmt.Errorf("ReadRune: %w", err)
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

		typ, textLen := processor.Process(sym)

		// textLen < 0 means the rule needs more input to decide.
		if textLen < 0 {
			lx.r++
			if isEOF {
				// Processor wants more input but we hit EOF.
				return LexemeTypeUnknown, nil, 0, fmt.Errorf("lexer: rule remained inconclusive at EOF")
			}
			continue
		}

		if textLen == 0 {
			if startPos < lx.w {
				typ = LexemeTypeUnknown
				textLen = 1 // Force consumption of one unknown symbol.
			} else if isEOF {
				return LexemeTypeUnknown, nil, 0, io.EOF
			} else {
				return LexemeTypeUnknown, nil, 0, fmt.Errorf("lexer: rule returned zero-length token")
			}
		}

		if isEOF && lx.r == startPos {
			return LexemeTypeUnknown, nil, 0, io.EOF
		}

		symbols := lx.buf[startPos : startPos+textLen]
		runes := make([]rune, len(symbols))
		for i, s := range symbols {
			runes[i] = s.Rune()
		}

		lx.r = startPos + textLen

		for _, s := range symbols {
			if s.Rune() == '\n' {
				lx.lineNum++
				lx.colNum = 0
			} else {
				lx.colNum++
			}
		}

		return typ, runes, textLen, nil
	}
}
