package textlexer

import (
	"github.com/xiam/textlexer/processor"
)

const (
	// StateContinue signals a possible match. The rule processor needs to read
	// more input to determine if a full match is successful. The rule must
	// return a non-nil next Rule.
	StateContinue processor.State = processor.StateContinue

	// StateAccept signals a successful match. The current input sequence is a
	// valid token. The rule may optionally return a next Rule to continue
	// scanning for an even longer match.
	StateAccept processor.State = processor.StateAccept

	// StateReject signals a failed match: the current input sequence is
	// discarded. The rule may optionally return a next Rule to continue scanning
	// for a different match. If the rule returns nil as the next Rule, the
	// scanner will accept the immediate previous accepted match, if any. If there
	// was no previous accepted match, the scanner will reject the entire input.
	StateReject processor.State = processor.StateReject

	// StatePushBack indicates that the last-read symbol is not part of the
	// current match. The scanner should move back one position in the input
	// stream and continue processing with the next Rule returned by the current
	// rule. The rule must return a non-nil next Rule.
	StatePushBack processor.State = processor.StatePushBack
)
