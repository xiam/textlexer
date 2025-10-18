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

// TextLexer orchestrates the tokenization of an input stream according to a
// set of user-defined rules. It manages input buffering, state tracking (line
// and column numbers), and the rule processing engine.
type TextLexer struct {
	rules    []LexemeType
	rulesMap map[LexemeType]Rule
	rulesMu  sync.Mutex

	symbolReader SymbolReader
	processor    *RulesProcessor
	mu           sync.Mutex

	runesRead     uint64 // Total number of read runes.
	runesAccepted uint64 // Total number of accepted runes.
}

// New creates a new TextLexer that reads from the provided io.RuneReader.
func New(rr io.RuneReader) *TextLexer {
	return &TextLexer{
		symbolReader: NewSymbolReader(rr),
		rules:        []LexemeType{},
		rulesMap:     map[LexemeType]Rule{},
	}
}

// AddRule registers a new tokenizing rule with the lexer.
// This method is not safe for concurrent use with Next(). All rules should be
// added before tokenization begins.
func (lx *TextLexer) AddRule(lexType LexemeType, lexRule Rule) error {
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	if lx.processor != nil {
		return fmt.Errorf("cannot add rule %q after tokenization has started", lexType)
	}

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
// This method is safe for concurrent use by multiple goroutines.
func (lx *TextLexer) Next() (*Lexeme, error) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	if err := lx.initProcessor(); err != nil {
		return nil, err
	}

	for {

		if lx.runesAccepted <= lx.runesRead {
			if err := lx.feedProcessor(); err != nil {
				if lx.runesRead == lx.runesAccepted {
					return nil, err
				}
			}
		}

		typ, runes, accepted, err := lx.nextLexeme()
		if typ == LexemeTypeUnspecified {
			// Need more input
			continue
		}
		if err != nil {
			return nil, err
		}

		lex := NewLexeme(typ, runes, lx.runesAccepted)
		lx.runesAccepted += accepted
		return lex, nil
	}

	return nil, io.EOF
}

func (lx *TextLexer) feedProcessor() error {
	// Read next symbol
	sym, err := lx.symbolReader.ReadSymbol()

	if err != nil {
		return err
	}

	// Feed the symbol to the processor.
	lx.runesRead++
	lx.processor.Feed(sym)

	return nil
}

func (lx *TextLexer) nextLexeme() (LexemeType, []rune, uint64, error) {
	typ, match, matchLen := lx.processor.Process()
	if typ == LexemeTypeUnspecified {
		// Needs more input to decide.
		return typ, nil, 0, nil
	}

	// Convert matched symbols to runes.
	runes := make([]rune, len(match))
	for i := uint64(0); i < matchLen; i++ {
		runes[i] = match[i].Rune()
	}

	return typ, runes, matchLen, nil
}

func (lx *TextLexer) initProcessor() error {
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	if lx.processor != nil {
		// Already initialized
		return nil
	}

	if len(lx.rules) == 0 {
		return fmt.Errorf("no rules defined")
	}

	lx.processor = NewRulesProcessor(lx.rules, lx.rulesMap)

	return nil
}
