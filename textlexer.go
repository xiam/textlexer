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
	io.Seeker
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
}

func New(r Reader) *TextLexer {
	return &TextLexer{
		reader:   r,
		rules:    []LexemeType{},
		rulesMap: map[LexemeType]Rule{},
	}
}

func (lx *TextLexer) AddRule(lexType LexemeType, lexRule Rule) error {
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	if _, ok := lx.rulesMap[lexType]; ok {
		return fmt.Errorf("rule %q already exists", lexType)
	}

	lx.rulesMap[lexType] = lexRule
	lx.rules = append(lx.rules, lexType)
	return nil
}

func (lx *TextLexer) MustAddRule(lexType LexemeType, lexRule Rule) {
	if err := lx.AddRule(lexType, lexRule); err != nil {
		panic(fmt.Sprintf("MustAddRule: %v", err))
	}
}

func (lx *TextLexer) Next() (*Lexeme, error) {
	typ, text, err := lx.nextLexeme()
	if err != nil {
		return nil, err
	}

	lex := NewLexeme(typ, text, lx.offset)

	// update offset
	lx.offset += lx.r

	// compact buffer
	lx.buf = lx.buf[lx.r:]
	lx.w -= lx.r
	lx.r = 0

	return lex, nil
}

func (lx *TextLexer) nextLexeme() (LexemeType, string, error) {
	scanners := map[LexemeType]Rule{}
	rules := []LexemeType{}

	lx.rulesMu.Lock()
	for _, lexType := range lx.rules {
		rules = append(rules, lexType) // it's important to preserve order
		scanners[lexType] = lx.rulesMap[lexType]
	}
	lx.rulesMu.Unlock()

	scanner := newRuleScanner(rules, scanners)

	for {

		var r rune
		if lx.r < lx.w {
			r = lx.buf[lx.r]
			lx.r++
		} else {
			var err error
			r, _, err = lx.reader.ReadRune()
			if err != nil {
				if err != io.EOF {
					return LexemeTypeUnknown, "", fmt.Errorf("read rune: %v", err)
				}
				r = RuneEOF
			}
			lx.buf = append(lx.buf, r)
			lx.w++
			lx.r++
		}

		matchType, matchLen := scanner.Scan(r)
		if matchLen < 0 {
			continue
		}

		lx.r = matchLen

		if matchType == LexemeTypeUnknown && r == RuneEOF {
			return LexemeTypeUnknown, "", io.EOF
		}

		return matchType, string(lx.buf[:lx.r]), nil
	}
}

type ruleState struct {
	rule       Rule
	lastAccept int
	isActive   bool
}

type ruleScanner struct {
	rules  []LexemeType
	states map[LexemeType]*ruleState

	buf []rune
}

func newRuleScanner(rules []LexemeType, scanners map[LexemeType]Rule) *ruleScanner {
	if len(scanners) != len(rules) {
		panic("scanners and rules length mismatch")
	}

	states := map[LexemeType]*ruleState{}
	for _, typ := range rules {
		if scanners[typ] == nil {
			panic(fmt.Sprintf("scanner for rule %q is nil", typ))
		}
		states[typ] = &ruleState{
			rule:       scanners[typ],
			lastAccept: -1,
			isActive:   true,
		}
	}

	return &ruleScanner{
		rules:  rules,
		states: states,

		buf: []rune{},
	}
}

func (rs *ruleScanner) Scan(r rune) (LexemeType, int) {
	rs.buf = append(rs.buf, r)

	activeScanners := 0

	for _, typ := range rs.rules {
		var pushbacks int

		scanner := rs.states[typ]
		if !scanner.isActive {
			continue
		}

		next, state := scanner.rule(r)
		for state == StatePushBack {
			pushbacks++
			if next != nil && pushbacks <= len(rs.buf) {
				next, state = next(rs.buf[len(rs.buf)-pushbacks])
			} else {
				next, state = nil, StateReject
			}
		}

		// update state
		if next == nil {
			scanner.isActive = false
		} else {
			scanner.rule = next
			activeScanners++
		}

		switch state {
		case StateReject:
			scanner.isActive = false
		case StateContinue:
			// keep going
		case StateAccept:
			// keep going, but remember last accept position
			scanner.lastAccept = len(rs.buf) - pushbacks
		default:
			panic(fmt.Sprintf("unknown state: %v", state))
		}
	}

	if activeScanners > 0 {
		return LexemeTypeUnknown, -1
	}

	bestMatch := LexemeTypeUnknown
	bestMatchLen := -1

	for _, typ := range rs.rules {
		scanner := rs.states[typ]
		if scanner.lastAccept > bestMatchLen {
			bestMatch = typ
			bestMatchLen = scanner.lastAccept
		}
	}

	if bestMatch == LexemeTypeUnknown {
		return LexemeTypeUnknown, len(rs.buf)
	}

	return bestMatch, bestMatchLen
}
