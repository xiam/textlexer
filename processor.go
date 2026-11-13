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

// processSymbol pushes s on the token's stack and steps every active rule over
// it. It returns the number of rules that are still active.
//
// The VM validates each verdict and retires only the rule that returned a bad
// one; the other rules continue on the same stack:
//
//   - A State outside the four constants, returned directly or at the end of a
//     pushback chain, retires the rule as StateReject does.
//   - s is the synthetic EOF symbol when it carries FlagEOF; only nextLexeme's
//     end-of-input symbol does. An accept that counts it (an accept with no
//     pushback) is longer than the symbols read, so the VM discards it. An
//     accept whose pushback chain excludes it is recorded as usual.
//   - No input follows the EOF symbol, so every rule is retired after it,
//     whatever its verdict. Process then always decides.
//
// An accept that the rule recorded earlier stays in each case.
func (rp *rulesProcessor) processSymbol(s Symbol) int {
	rp.buf = append(rp.buf, s)
	atEOF := s.IsEOF()

	activeScanners := 0
	for _, typ := range rp.rules {
		var pushbackCount int

		state := rp.states[typ]
		if !state.isActive {
			continue
		}

		nextRule, nextState := callRule(state.rule, s)
		for nextState == StatePushBack {
			pushbackCount++
			if nextRule != nil && pushbackCount <= len(rp.buf) {
				pushedBackSymbol := rp.buf[len(rp.buf)-pushbackCount]
				nextRule, nextState = callRule(nextRule, pushedBackSymbol)
			} else {
				nextRule, nextState = nil, StateReject
			}
		}

		keepActive := false
		switch nextState {
		case StateAccept:
			// At EOF, an accept with no pushback counts the EOF symbol. It is
			// discarded, and the rule is retired below.
			if !atEOF || pushbackCount > 0 {
				matchLen := len(rp.buf) - pushbackCount
				if matchLen > state.lastAccept {
					state.lastAccept = matchLen
				}
			}
			keepActive = nextRule != nil
		case StateContinue:
			keepActive = nextRule != nil
		default:
			// StateReject, and any State outside the four constants, retire
			// the rule.
		}

		if keepActive && !atEOF {
			state.rule = nextRule
			activeScanners++
		} else {
			state.isActive = false
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

// callRule calls r with s and returns its verdict. If r does not return
// normally, callRule recovers the panic and returns (nil, StateReject), so the
// VM retires that rule for the current token as for a reject (principle 3,
// KD-2 in docs/specs/core-contract.md). The VM neither keeps nor reports the
// panic value.
//
// The completion flag, not the result of recover, detects the panic: with
// GODEBUG panicnil=1 (a main module that declares a go version before 1.21),
// recover returns nil for panic(nil), and the panic must still retire the rule.
// The deferred recover covers only the call to r, never the VM's own code.
func callRule(r Rule, s Symbol) (next Rule, state State) {
	returned := false
	defer func() {
		if !returned {
			_ = recover()
			next, state = nil, StateReject
		}
	}()

	next, state = r(s)
	returned = true
	return next, state
}
