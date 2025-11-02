package textlexer

import (
	"fmt"
	"log/slog"

	"github.com/xiam/textlexer/processor"
)

type ruleScanner struct {
	sp processor.StateProcessor

	initialRule Rule

	rule Rule

	maybeAccept bool

	// lastAcceptedLength tracks how many symbols have been accepted for the
	// current token. Updated each time StateAccept is executed.
	lastAcceptedLength uint64

	// explorationPos tracks the current position being explored (start +
	// offset).  This is where the next symbol will be read from.
	explorationPos uint64
}

func (rs *ruleScanner) AcceptedLength() uint64 {
	return rs.lastAcceptedLength
}

func (rs *ruleScanner) CurrentPosition() uint64 {
	return rs.explorationPos
}

func (rs *ruleScanner) Scan(sym Symbol) (bool, error) {
	if !rs.IsActive() {
		slog.Info("***** Rule skipped (inactive) *****", "symbol", sym.String())
		return false, fmt.Errorf("scanner is inactive")
	}

	nextRule, state := rs.rule(sym)
	slog.Info(
		"***** Rule executed *****",
		"symbol", sym.String(),
		"nextRule", nextRule != nil,
		"state", state,
	)

	if err := rs.sp.Execute(state); err != nil {
		slog.Error("Error executing state processor", "error", err)
		// deactivate on error
		rs.rule = nil
		rs.sp = nil
		return false, fmt.Errorf("error processing state: %w", err)
	}

	// Determine if this could be a valid acceptance point
	switch state {
	case processor.StateMatch, processor.StateAccept:
		slog.Info("State processor indicates possible acceptance")
		rs.maybeAccept = true
	}

	// After execution, get the current position:
	// - nextTokenStart: length of accepted token so far (from position 0)
	// - lookAheadOffset: current offset from that position
	nextTokenStart, lookAheadOffset := rs.sp.Position()
	slog.Info(
		"State processor position",
		"nextTokenStart", nextTokenStart,
		"lookAheadOffset", lookAheadOffset,
	)

	// Update last accepted length if we just accepted more symbols
	rs.lastAcceptedLength = nextTokenStart

	// Update exploration position for the next scan
	rs.explorationPos = nextTokenStart + lookAheadOffset

	rs.rule = nextRule

	return rs.IsActive(), nil
}

func (rs *ruleScanner) IsActive() bool {
	return rs.rule != nil && rs.sp != nil
}

func (rs *ruleScanner) Reset() {
	rs.rule = rs.initialRule
	rs.sp = processor.NewStateProcessor()
	rs.lastAcceptedLength = 0
	rs.maybeAccept = false
	rs.explorationPos = 0
}

func NewRuleScanner(initial Rule) *ruleScanner {
	return &ruleScanner{
		initialRule: initial,
		rule:        initial,
		maybeAccept: false,
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
	slog.Info("")
	slog.Info("Processing buffer", "bufLen", rp.bufLen, "atEOF", rp.atEOF)

	activeScanners := rp.runScanners()
	rp.processed++

	// If any scanners are still active and we haven't hit EOF, we need more input.
	// This will be handled by the caller.
	if activeScanners > 0 && !rp.atEOF {
		slog.Info(
			"More input needed",
			"activeScanners", activeScanners,
			"processed", rp.processed,
		)
		return LexemeTypeUnspecified, nil, 1
	}
	slog.Info("All scanners done", "processed", rp.processed)

	// All scanners are done (or we hit EOF); pick the best match (if any).
	bestMatchTyp, bestMatchLen := rp.pickBestMatch()

	bestMatch := make([]Symbol, bestMatchLen)
	copy(bestMatch, rp.buf[:bestMatchLen])

	// Remove processed symbols from the buffer
	rp.buf = rp.buf[bestMatchLen:]
	rp.bufLen = uint64(len(rp.buf))

	// We have a non-zero match; reset all scanners for the next round
	if bestMatchLen > 0 {
		for typ := range rp.scanners {
			scanner := rp.scanners[typ]
			scanner.Reset()
		}
	}

	slog.Info("Best match selected",
		"type", bestMatchTyp,
		"symbols", bestMatch,
		"length", bestMatchLen,
		"remainingBufLen", rp.bufLen,
	)

	return bestMatchTyp, bestMatch, bestMatchLen
}

func (rp *RulesProcessor) runScanners() int {
	activeScanners := 0

	for _, typ := range rp.rules {
		scanner := rp.scanners[typ]

		isActive := scanner.IsActive()
		slog.Info("Starting scanner", "type", typ, "isActive", isActive)

		for isActive {
			currentPos := scanner.CurrentPosition()

			if currentPos >= rp.bufLen {
				slog.Info("Scanner reached end of buffer", "type", typ)
				// No more symbols to process for this scanner
				break
			}
			slog.Info("***** Scanner processing symbol *****", "type", typ, "position", currentPos)

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
	slog.Info("")
	slog.Info("===== Picking best match among scanners =====")

	bestMatch := LexemeTypeUnknown
	bestMatchLen := uint64(0)

	bestMatchIdx := -1
	for idx := range rp.rules {
		typ := rp.rules[idx]
		scanner := rp.scanners[typ]

		if scanner.sp == nil {
			slog.Info("Scanner state processor is nil; skipping", "type", typ)
			continue
		}

		if !scanner.maybeAccept {
			slog.Info("Scanner state processor is nil; skipping", "type", typ)
			continue
		}

		acceptedLen := scanner.AcceptedLength()
		if bestMatch == LexemeTypeUnknown || acceptedLen > bestMatchLen {
			bestMatchIdx = idx

			bestMatch = typ
			bestMatchLen = acceptedLen
		}
	}

	if bestMatchIdx == -1 {
		// No matches found; consume one symbol to avoid stalling
		bestMatchLen = 1
	} else {
		// Disable state processor to avoid re-matching on the same input
		scanner := rp.scanners[bestMatch]
		slog.Info("Disabling scanner state processor", "type", bestMatch)
		scanner.sp = nil
	}

	return bestMatch, bestMatchLen
}
