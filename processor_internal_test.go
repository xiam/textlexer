package textlexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// These tests call rulesProcessor.Process directly. Through Next, a zero-length
// match and a rule that matched nothing both become the same single-rune
// UNKNOWN lexeme (textlexer.go:482-485 and processor.go:69-71), so only Process
// shows which path a token took.

// Every rule in this file is an engine probe: it exists to drive one path of
// rulesProcessor, and each one says which practice of docs/rule-authoring.md it
// breaks on purpose.

// excludeLast returns the verdict of a rule that excludes the last n symbols
// (n >= 1) from its match and then ends the pushback chain with (nil, s). It is
// the internal-package copy of the shared helper in textlexer_test.go, which
// this package cannot import.
func excludeLast(n int, s State) (Rule, State) {
	if n < 1 {
		panic("excludeLast: n must be at least 1")
	}
	return pushBackChain(n-1, s), StatePushBack
}

// pushBackChain returns the pushback-chain callback that issues n more
// StatePushBack verdicts and then returns (nil, s). It ignores the symbol it is
// shown, which is already excluded from the match.
func pushBackChain(n int, s State) Rule {
	return func(Symbol) (Rule, State) {
		if n > 0 {
			return pushBackChain(n-1, s), StatePushBack
		}
		return nil, s
	}
}

// processInput feeds input and then the EOF symbol to a processor with the one
// rule typ, and returns the first settled Process result.
func processInput(t *testing.T, typ LexemeType, rule Rule, input string) (LexemeType, int) {
	t.Helper()

	rp := newRulesProcessor([]LexemeType{typ}, map[LexemeType]Rule{typ: rule})
	for _, r := range input {
		if got, n := rp.Process(NewSymbol(r, FlagNone)); n >= 0 {
			return got, n
		}
	}
	got, n := rp.Process(NewSymbol(RuneEOF, FlagEOF))
	if n < 0 {
		t.Fatalf("rule %q remained active at EOF", typ)
	}
	return got, n
}

// TestProcessPushBackAtEOF: the stack holds the EOF symbol, and it counts
// toward the bound. On input "a", one pushback at EOF matches "a"; two match
// zero symbols; three pass the bottom of the two-symbol stack and retire the
// rule, so no rule matches.
func TestProcessPushBackAtEOF(t *testing.T) {
	for _, tc := range []struct {
		pushbacks int
		wantType  LexemeType
		wantLen   int
	}{
		{1, "R", 1},
		{2, "R", 0},
		{3, LexemeTypeUnknown, 1},
	} {
		// Engine probe: two and three pushbacks exclude more than the match
		// on purpose (breaks practices 1, 2 and 6).
		rule := func(s Symbol) (Rule, State) {
			if s.Rune() != 'a' {
				return nil, StateReject
			}
			return func(s Symbol) (Rule, State) {
				if !s.IsEOF() {
					return nil, StateReject
				}
				return excludeLast(tc.pushbacks, StateAccept)
			}, StateContinue
		}

		gotType, gotLen := processInput(t, "R", rule, "a")
		assert.Equal(t, tc.wantType, gotType, "%d pushbacks", tc.pushbacks)
		assert.Equal(t, tc.wantLen, gotLen, "%d pushbacks", tc.pushbacks)
	}
}

// TestProcessZeroLengthMatch: one pushback to an acceptor on the first symbol is
// a zero-length match of the rule's own type, which nextLexeme turns into one
// UNKNOWN rune (textlexer.go:482-485). Two pushbacks on the first symbol pass
// the bottom of the stack instead, and Process reports no match.
func TestProcessZeroLengthMatch(t *testing.T) {
	// Engine probes: an empty match, and a chain past the bottom of the
	// stack, on purpose (breaks practices 2 and 6).
	zeroLength := func(Symbol) (Rule, State) { return excludeLast(1, StateAccept) }
	gotType, gotLen := processInput(t, "Z", zeroLength, "a")
	assert.Equal(t, LexemeType("Z"), gotType)
	assert.Equal(t, 0, gotLen)

	overPush := func(Symbol) (Rule, State) { return excludeLast(2, StateAccept) }
	gotType, gotLen = processInput(t, "Z", overPush, "a")
	assert.Equal(t, LexemeTypeUnknown, gotType)
	assert.Equal(t, 1, gotLen)
}

// TestProcessDecidesOnEOFSymbol: Process always decides on the EOF symbol. A
// rule still active after it is retired, an accept that counts it is
// discarded, and an invalid State retires the rule without a panic. With no
// recorded accept, Process reports UNKNOWN with length 1; an accept recorded
// before the EOF symbol stays.
func TestProcessDecidesOnEOFSymbol(t *testing.T) {
	// Engine probes: never settles, accepts the EOF symbol, and returns an
	// invalid State, on purpose (breaks practices 1, 2 and 6).
	var alwaysContinue Rule
	alwaysContinue = func(Symbol) (Rule, State) { return alwaysContinue, StateContinue }

	var alwaysAccept Rule
	alwaysAccept = func(Symbol) (Rule, State) { return alwaysAccept, StateAccept }

	invalidOnEOF := func(Symbol) (Rule, State) {
		return func(Symbol) (Rule, State) { return nil, State(9) }, StateAccept
	}

	for _, tc := range []struct {
		name     string
		rule     Rule
		wantType LexemeType
		wantLen  int
	}{
		{"still active, no accept", alwaysContinue, LexemeTypeUnknown, 1},
		{"accepts the EOF symbol after accepting a", alwaysAccept, "R", 1},
		{"invalid State on the EOF symbol after accepting a", invalidOnEOF, "R", 1},
	} {
		gotType, gotLen := processInput(t, "R", tc.rule, "a")
		assert.Equal(t, tc.wantType, gotType, tc.name)
		assert.Equal(t, tc.wantLen, gotLen, tc.name)
	}
}

// TestCallRule: callRule passes a returned verdict through unchanged, an
// invalid State and a non-nil next included, and turns a panic with any value
// into (nil, StateReject). The VM's reaction to these verdicts is tested
// through Next in textlexer_test.go.
func TestCallRule(t *testing.T) {
	sym := NewSymbol('a', FlagNone)

	// Engine probes: the rules below return each verdict, an invalid State
	// included, or panic, on purpose (breaks practices 2 and 6).

	t.Run("returned verdicts pass through", func(t *testing.T) {
		var self Rule
		self = func(Symbol) (Rule, State) { return self, StateAccept }

		next, state := callRule(self, sym)
		assert.NotNil(t, next)
		assert.Equal(t, StateAccept, state)

		for _, want := range []State{StateContinue, StateAccept, StateReject, StatePushBack, State(9)} {
			want := want
			next, state := callRule(func(Symbol) (Rule, State) { return nil, want }, sym)
			assert.Nil(t, next)
			assert.Equal(t, want, state)
		}
	})

	panicValues := []struct {
		name  string
		value interface{}
	}{
		{"string", "boom"},
		{"error", assert.AnError},
		{"integer", 42},
		{"nil", nil},
	}

	check := func(t *testing.T) {
		for _, pv := range panicValues {
			pv := pv
			t.Run(pv.name, func(t *testing.T) {
				var next Rule
				var state State
				assert.NotPanics(t, func() {
					next, state = callRule(func(Symbol) (Rule, State) { panic(pv.value) }, sym)
				})
				assert.Nil(t, next)
				assert.Equal(t, StateReject, state)
			})
		}
	}

	t.Run("panics become a reject", check)

	// With panicnil=1, recover returns nil for panic(nil). callRule detects the
	// panic with a completion flag, so the rule is still retired.
	t.Run("panics become a reject with GODEBUG panicnil=1", func(t *testing.T) {
		t.Setenv("GODEBUG", "panicnil=1")
		check(t)
	})
}
