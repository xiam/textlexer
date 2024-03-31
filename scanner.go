package textlexer

type State uint

const (
	StateContinue State = iota

	StateAccept
	StateReject
)

type Rule func(r rune) (next Rule, state State)

const RuneEOF = -1

func IsEOF(r rune) bool {
	return r == RuneEOF
}

func (s State) String() string {
	switch s {
	case StateAccept:
		return "ACCEPT"
	case StateReject:
		return "REJECT"
	case StateContinue:
		return "CONTINUE"
	}
	panic("unreachable")
}
