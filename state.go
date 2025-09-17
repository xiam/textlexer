package textlexer

import "fmt"

type State uint

const (
	// StateContinue signals a possible match. The rule matcher needs to read
	// more input to determine if the match is successful. The scanner should
	// advance one position.
	StateContinue State = iota

	// StateAccept signals a successful match, the scanner may consume the
	// current match.
	StateAccept

	// StateReject signals a failed match, the scanner may discard the current
	// position.
	StateReject

	// StatePushBack indicates that the last read rune is not part of the current
	// match. The scanner might move back one position.
	StatePushBack
)

var stateNames = map[State]string{
	StateContinue: "CONTINUE",
	StateAccept:   "ACCEPT",
	StateReject:   "REJECT",
	StatePushBack: "PUSHBACK",
}

func (s State) String() string {
	if _, ok := stateNames[s]; ok {
		return stateNames[s]
	}

	return fmt.Sprintf("UnknownState(%d)", s)
}
