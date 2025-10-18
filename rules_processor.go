package textlexer

import (
	"fmt"
	"github.com/xiam/textlexer/processor"
)

type ruleScanner struct {
	sp processor.StateProcessor

	initialRule Rule
	rule        Rule

	acceptedLen uint64
	currentPos  uint64
}

func (rs *ruleScanner) AcceptedLength() uint64 {
	return rs.acceptedLen
}

func (rs *ruleScanner) CurrentPosition() uint64 {
	return rs.currentPos
}

func (rs *ruleScanner) Scan(sym Symbol) (bool, error) {
	if rs.rule == nil {
		return false, fmt.Errorf("scanner is inactive")
	}

	nextRule, state := rs.rule(sym)

	if err := rs.sp.Execute(state); err != nil {
		rs.rule = nil // deactivate on error
		return false, fmt.Errorf("error processing state: %w", err)
	}

	var offset uint64
	rs.acceptedLen, offset = rs.sp.Position()
	rs.currentPos = rs.acceptedLen + offset
	rs.rule = nextRule

	return rs.rule != nil, nil
}

func (rs *ruleScanner) IsActive() bool {
	return rs.rule != nil
}

func (rs *ruleScanner) Reset() {
	rs.rule = rs.initialRule
	rs.sp = processor.NewStateProcessor()
	rs.acceptedLen = 0
	rs.currentPos = 0
}

func NewRuleScanner(initial Rule) *ruleScanner {
	return &ruleScanner{
		initialRule: initial,
		rule:        initial,
		sp:          processor.NewStateProcessor(),
	}
}

// RulesProcessor processes input symbols through multiple rules and returns the
// best match based on the "maximal munch" principle (longest accepted lexeme).
type RulesProcessor struct {
	rules    []LexemeType
	scanners map[LexemeType]*ruleScanner

	buf    []Symbol
	bufLen uint64

	atEOF bool

	processed uint64
}

func NewRulesProcessor(rules []LexemeType, initialStates map[LexemeType]Rule) *RulesProcessor {
	if len(rules) != len(initialStates) {
		panic("rules and initialStates length mismatch")
	}

	scanners := map[LexemeType]*ruleScanner{}
	for _, typ := range rules {
		if initialStates[typ] == nil {
			panic(fmt.Sprintf("scanner for rule %q is nil", typ))
		}
		scanners[typ] = NewRuleScanner(initialStates[typ])
	}

	return &RulesProcessor{
		rules:    rules,
		scanners: scanners,
		buf:      []Symbol{},
	}
}

func (rp *RulesProcessor) Feed(sym Symbol) {
	rp.buf = append(rp.buf, sym)
	rp.bufLen = uint64(len(rp.buf))
	rp.atEOF = sym.IsEOF()
}

func (rp *RulesProcessor) Process() (LexemeType, []Symbol, uint64) {
	activeScanners := rp.runScanners()
	rp.processed++

	// If any scanners are still active and we haven't hit EOF, we need more input.
	// This will be handled by the caller.
	if activeScanners > 0 && !rp.atEOF {
		return LexemeTypeUnspecified, nil, 1
	}

	// All scanners are done (or we hit EOF); pick the best match (if any).
	bestMatchTyp, bestMatchLen := rp.pickBestMatch()

	bestMatch := make([]Symbol, bestMatchLen)
	copy(bestMatch, rp.buf[:bestMatchLen])

	// Remove processed symbols from the buffer
	rp.buf = rp.buf[bestMatchLen:]
	rp.bufLen = uint64(len(rp.buf))

	// Reset all scanners for the next round
	for typ := range rp.scanners {
		scanner := rp.scanners[typ]
		scanner.Reset()
	}

	return bestMatchTyp, bestMatch, bestMatchLen
}

func (rp *RulesProcessor) runScanners() int {
	activeScanners := 0

	for _, typ := range rp.rules {
		scanner := rp.scanners[typ]

		isActive := scanner.IsActive()

		for isActive {
			currentPos := scanner.CurrentPosition()

			if currentPos >= rp.bufLen {
				// No more symbols to process for this scanner
				break
			}

			sym := rp.buf[currentPos]

			isActive, _ = scanner.Scan(sym)
		}

		if isActive {
			// Still active after processing symbols
			activeScanners++
		}
	}

	return activeScanners
}

func (rp *RulesProcessor) pickBestMatch() (LexemeType, uint64) {
	bestMatch := LexemeTypeUnknown
	bestMatchLen := uint64(0)

	for _, typ := range rp.rules {
		scanner := rp.scanners[typ]
		acceptedLen := scanner.AcceptedLength()
		if acceptedLen > bestMatchLen {
			bestMatch = typ
			bestMatchLen = acceptedLen
		}
	}

	if bestMatch == LexemeTypeUnknown {
		// No matches found; consume one symbol to avoid stalling
		bestMatchLen = 1
	}

	return bestMatch, bestMatchLen
}
