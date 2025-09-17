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

	offset int

	buf []rune
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
		buf:      make([]rune, 0, 4096),
		reader:   r,
		rules:    []LexemeType{},
		rulesMap: map[LexemeType]Rule{},
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

	// The number of runes consumed (n) is the single source of truth for all state updates.
	typ, text, n, err := lx.nextLexeme()
	if err != nil {
		return nil, err
	}

	lex := NewLexeme(typ, text, lx.offset)

	// Update our global offset by the number of runes consumed.
	lx.offset += n

	// compact the buffer by removing consumed runes.
	lx.buf = lx.buf[n:]
	lx.w = lx.w - n
	lx.r = 0

	return lex, nil
}

func (lx *TextLexer) nextLexeme() (LexemeType, string, int, error) {
	processor, err := lx.getProcessor()
	if err != nil {
		return LexemeTypeUnknown, "", 0, fmt.Errorf("processor: %w", err)
	}

	startPos := lx.r

	for {
		var r rune
		var isEOF bool

		if lx.r < lx.w {
			// Read from the existing buffer.
			r = lx.buf[lx.r]
		} else {
			// Buffer is exhausted, read from the underlying io.Reader.
			var readErr error
			r, _, readErr = lx.reader.ReadRune()
			if readErr != nil {
				if readErr != io.EOF {
					return LexemeTypeUnknown, "", 0, fmt.Errorf("ReadRune: %w", readErr)
				}
				isEOF = true
				r = RuneEOF
			} else {
				lx.buf = append(lx.buf, r)
				lx.w++
			}
		}

		// Process the rune we just got.
		typ, textLen := processor.Process(r)

		// `textLen` < 0 signals that the processor needs more runes to decide.
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
				textLen = 1 // Force consumption of one rune.
			} else if isEOF {
				return LexemeTypeUnknown, "", 0, io.EOF
			} else {
				return LexemeTypeUnknown, "", 0, fmt.Errorf("lexer: rule returned zero-length token")
			}
		}

		if isEOF && lx.r == startPos {
			// We are at EOF and the processor did not consume any runes.
			return LexemeTypeUnknown, "", 0, io.EOF
		}

		// The number of runes to consume is the length returned by the processor.
		text := string(lx.buf[startPos : startPos+textLen])

		// The read pointer is now positioned at the end of the consumed token.
		lx.r = startPos + textLen

		return typ, text, textLen, nil
	}
}
