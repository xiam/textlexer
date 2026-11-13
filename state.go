package textlexer

import "fmt"

// State represents the outcome of processing a symbol within a Rule.
//
// A value outside the four constants below retires the rule for the current
// token, as StateReject does; an accept the rule recorded earlier stays.
type State uint

const (
	// StateContinue signals a possible match. The rule processor needs to read
	// more input to determine if a full match is successful. The returned next
	// Rule is called with the next symbol; a nil next retires the rule. No
	// input follows the EOF symbol, so a rule that continues on it is retired.
	StateContinue State = iota

	// StateAccept signals a successful match. The current input sequence is a
	// valid token. The rule may optionally return a next Rule to continue
	// scanning for an even longer match. The synthetic EOF symbol is not part
	// of the input, so an accept that counts it is discarded; to keep a match
	// at end of input, reject the EOF symbol or exclude it with StatePushBack.
	// After the EOF symbol the rule is retired whatever next it returns.
	StateAccept

	// StateReject signals a failed match. This rule can no longer match the
	// current input sequence and will be deactivated for the current token.
	StateReject

	// StatePushBack excludes the top symbol of the token's stack (first the
	// symbol just read) from the current match and calls the returned 'next'
	// Rule with that symbol in the same step. 'next' may return StatePushBack
	// again to exclude one more symbol toward the token start; such a chain
	// gives a rule multi-symbol lookahead and must end in (nil, StateAccept) or
	// StateReject. See "Rule states and pushback" in docs/specs/core-contract.md.
	StatePushBack
)

var stateNames = map[State]string{
	StateContinue: "CONTINUE",
	StateAccept:   "ACCEPT",
	StateReject:   "REJECT",
	StatePushBack: "PUSHBACK",
}

func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("UnknownState(%d)", s)
}
