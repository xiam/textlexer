package textlexer

import (
	"fmt"
)

type ruleState struct {
	initialRule Rule
	rule        Rule
	lastAccept  int
	isActive    bool
}

// rulesProcessor processes input symbols through multiple rules (scanners) and
// returns the best match (the longest accepted lexeme).
type rulesProcessor struct {
	rules []LexemeType

	states map[LexemeType]*ruleState
	buf    []Symbol
}

func newRulesProcessor(rules []LexemeType, initialStates map[LexemeType]Rule) *rulesProcessor {
	if len(rules) != len(initialStates) {
		panic("rules and rules length mismatch")
	}

	states := map[LexemeType]*ruleState{}
	for _, typ := range rules {
		if initialStates[typ] == nil {
			panic(fmt.Sprintf("scanner for rule %q is nil", typ))
		}
		states[typ] = &ruleState{
			initialRule: initialStates[typ],
			rule:        initialStates[typ],
			lastAccept:  -1,
			isActive:    true,
		}
	}

	return &rulesProcessor{
		rules:  rules,
		states: states,
		buf:    make([]Symbol, 0, 4096),
	}
}

func (rp *rulesProcessor) Reset() {
	rp.buf = rp.buf[:0]
	for _, state := range rp.states {
		state.rule = state.initialRule
		state.lastAccept = -1
		state.isActive = true
	}
}

func (rp *rulesProcessor) Process(s Symbol) (LexemeType, int) {
	// Process the symbol through all active scanners.
	activeScanners := rp.processSymbol(s)

	// If there are still active scanners, we need more input.
	if activeScanners > 0 {
		return LexemeTypeUnknown, -1
	}

	// No active scanners left, find the best match.
	bestMatch, bestMatchLen := rp.pickBestMatch()

	// No rule matched, consume one symbol to advance the input.
	if bestMatch == LexemeTypeUnknown {
		return LexemeTypeUnknown, 1
	}

	return bestMatch, bestMatchLen
}

func (rp *rulesProcessor) processSymbol(s Symbol) int {
	rp.buf = append(rp.buf, s)

	activeScanners := 0
	for _, typ := range rp.rules {
		var pushbackCount int

		state := rp.states[typ]
		if !state.isActive {
			continue
		}

		nextRule, nextState := state.rule(s)
		for nextState == StatePushBack {
			pushbackCount++
			if nextRule != nil && pushbackCount <= len(rp.buf) {
				pushedBackSymbol := rp.buf[len(rp.buf)-pushbackCount]
				nextRule, nextState = nextRule(pushedBackSymbol)
			} else {
				// Can't push back anymore, reject the input.
				nextRule, nextState = nil, StateReject
			}
		}

		switch nextState {
		case StateReject:
			state.isActive = false
		case StateAccept:
			matchLen := len(rp.buf) - pushbackCount
			if matchLen > state.lastAccept {
				state.lastAccept = matchLen
			}
			if nextRule != nil {
				state.rule = nextRule
				activeScanners++
			} else {
				state.isActive = false
			}
		case StateContinue:
			if nextRule != nil {
				state.rule = nextRule
				activeScanners++
			} else {
				state.isActive = false
			}
		default:
			panic(fmt.Sprintf("unknown nextState: %v", nextState))
		}
	}

	return activeScanners
}

func (rp *rulesProcessor) pickBestMatch() (LexemeType, int) {
	bestMatch := LexemeTypeUnknown
	bestMatchLen := -1

	for _, typ := range rp.rules {
		state := rp.states[typ]
		if state.lastAccept > bestMatchLen {
			bestMatch = typ
			bestMatchLen = state.lastAccept
		}
	}

	return bestMatch, bestMatchLen
}

// GetBuffer returns the symbols currently in the buffer as a string.
func (rp *rulesProcessor) GetBuffer() string {
	runes := make([]rune, len(rp.buf))
	for i, sym := range rp.buf {
		runes[i] = sym.Rune()
	}
	return string(runes)
}
