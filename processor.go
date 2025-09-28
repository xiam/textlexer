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

// rulesProcessor processes input symbols through multiple rules and returns the
// best match based on the "maximal munch" principle (longest accepted lexeme).
type rulesProcessor struct {
	rules []LexemeType

	states map[LexemeType]*ruleState
	buf    []Symbol
}

func newRulesProcessor(rules []LexemeType, initialStates map[LexemeType]Rule) *rulesProcessor {
	if len(rules) != len(initialStates) {
		panic("rules and initialStates length mismatch")
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
	// Process the symbol through all active scanners
	activeScanners := rp.processSymbol(s)

	// If any scanners are still active, we need more input
	if activeScanners > 0 {
		return LexemeTypeUnknown, -1
	}

	// All scanners are done; pick the best match
	bestMatch, bestMatchLen := rp.pickBestMatch()

	// If no match was found, return UNKNOWN with length 1 to consume at least
	if bestMatch == LexemeTypeUnknown {
		return LexemeTypeUnknown, 1
	}

	// Remove the matched symbols from the buffer
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
