// Package textlexer provides a flexible, rule-based engine for lexical analysis.
//
// A Rule is a state-transition function that processes input one Symbol at a
// time. The lexer runs all defined rules in parallel, reading ahead until all
// rules have either rejected the input or terminated. It then selects the
// longest valid match as the next lexeme.
// If multiple rules match the same longest text, the one added first is chosen.
//
// # Layering
//
// The lexer owns lexical policy: rule state transitions, parallel rule
// evaluation, maximal-munch selection, registration-order tie breaking, the
// UNKNOWN fallback, and lexeme construction. It owns no text-stream mechanics.
// Reading, buffering, positions, and replay come from a positioned cursor
// (github.com/xiam/textreader): the cursor decodes UTF-8, tracks the logical
// position, retains the input a checkpoint needs, and restores a marked
// position exactly.
//
// A token is assembled speculatively and then committed. The lexer marks a
// checkpoint at the start of a token, reads located runes until the rules
// decide, and then steps the cursor back over the speculative overshoot so it
// sits at the end of the winning match. Token positions come from that
// committed range — the checkpoint's position and the cursor after the step
// back — never from where speculation stopped.
package textlexer

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/xiam/textreader"
)

const (
	// RuneEOF represents the end-of-file marker as a rune.
	RuneEOF = rune(-1)
)

// Cursor is the text-stream surface the lexer needs from a positioned cursor.
// *textreader.TextReader implements it. The interface is exported so a caller
// can supply its own cursor and so the lexer's dependency on the cursor stays
// explicit.
type Cursor interface {
	// ReadLocatedRune reads the next rune and reports the position it started
	// at.
	ReadLocatedRune() (textreader.LocatedRune, error)
	// Cursor returns the logical position of the next read.
	Cursor() textreader.Pos
	// Checkpoint marks the current logical position for replay.
	Checkpoint() *textreader.Checkpoint
	// Seek moves the logical cursor inside the retained input.
	Seek(offset int64, whence int) (int64, error)
}

// contextualCursor is implemented by cursors that can expose the text they
// retain. It backs Lexer.Context and is optional: everything else works with a
// plain Cursor.
type contextualCursor interface {
	Context(s textreader.Span, before, after int) (string, error)
}

// Option configures a lexer created by New.
type Option func(*config)

type config struct {
	maxTokenBytes int
}

// WithMaxTokenBytes bounds how many source bytes the lexer may retain while it
// assembles one token. Zero, the default, means no bound.
//
// The bound covers maximal-munch look-ahead: a rule that keeps matching forces
// the lexer to retain everything it has read for the token so far. When a token
// would exceed the bound, Next returns an error wrapping
// textreader.ErrRetentionExceeded, and the lexer makes no partial progress.
func WithMaxTokenBytes(n int) Option {
	return func(c *config) {
		if n < 0 {
			n = 0
		}
		c.maxTokenBytes = n
	}
}

// markWindow is how many committed lexemes keep their retained input. It is
// what lets Context read a lexeme together with the input that precedes it: a
// single mark would floor the retained region at the lexeme itself, so `before`
// could never return anything.
const markWindow = 3

// TextLexer orchestrates the tokenization of an input stream according to a
// set of user-defined rules. It reads through a positioned cursor and runs the
// rule processing engine over the located runes it reads.
type TextLexer struct {
	cursor Cursor

	// runes holds the symbols read speculatively for the token currently being
	// assembled. It is truncated at the start of every token and reused, so
	// tokenizing does not allocate per token.
	runes []Symbol

	// emitted counts the runes committed since the start of the stream.
	emitted int

	// marks keeps the committed bytes of the most recently returned lexemes
	// retained, so Context can read the text around one of them — including the
	// input that precedes it. It is a ring of markWindow entries; a mark is
	// released only when a later token commits and overwrites its slot, so a
	// failed Next or an EOF probe leaves the previous lexeme's context readable.
	marks     [markWindow]*textreader.Checkpoint
	marksNext int

	// maxTokenBytes is the per-token byte bound passed to WithMaxTokenBytes, or
	// zero for unbounded. It is enforced by the lexer here rather than delegated
	// to the cursor's retention budget, because the cursor must also retain the
	// previous lexeme while a token is assembled.
	maxTokenBytes int

	mu sync.Mutex

	rules    []LexemeType
	rulesMap map[LexemeType]Rule
	rulesMu  sync.RWMutex

	processor *rulesProcessor
}

// New creates a new TextLexer that reads from the provided io.RuneReader.
//
// A source that also implements io.Reader is read as a byte stream by the
// cursor, so no extra UTF-8 decoding layer is introduced for callers that
// already hold one. A source that only implements io.RuneReader is read rune by
// rune, and the cursor stores the UTF-8 encoding of each rune so positions stay
// meaningful.
//
// New keeps the constructor's original shape, including as a function value.
// Use NewWithOptions to configure the lexer or its cursor.
func New(rr io.RuneReader) *TextLexer {
	return NewWithOptions(rr)
}

// NewWithOptions is New with configuration options.
func NewWithOptions(rr io.RuneReader, opts ...Option) *TextLexer {
	cfg := newConfig(opts)

	// The cursor must retain the committed lexemes in the context window plus the
	// token being assembled, so its budget covers markWindow+1 tokens. The
	// per-token bound itself is enforced by the lexer.
	retained := 0
	if cfg.maxTokenBytes > 0 {
		retained = (markWindow + 1) * cfg.maxTokenBytes
	}

	var cur Cursor
	if r, ok := rr.(io.Reader); ok {
		cur = textreader.NewReader(r, textreader.WithMaxRetained(retained))
	} else {
		cur = textreader.NewRuneReader(rr, textreader.WithMaxRetained(retained))
	}

	lx := NewWithCursor(cur)
	lx.maxTokenBytes = cfg.maxTokenBytes

	return lx
}

// NewWithCursor creates a new TextLexer over a positioned cursor the caller
// already owns. Use it to share one cursor between several consumers, or to
// configure the cursor's retention policy directly.
func NewWithCursor(c Cursor) *TextLexer {
	return &TextLexer{
		cursor:    c,
		runes:     make([]Symbol, 0, 256),
		rules:     []LexemeType{},
		rulesMap:  map[LexemeType]Rule{},
		processor: nil,
	}
}

func newConfig(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

// AddRule registers a new tokenizing rule with the lexer.
// This method is not safe for concurrent use with Next(). All rules should be
// added before tokenization begins.
func (lx *TextLexer) AddRule(lexType LexemeType, lexRule Rule) error {
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	if _, ok := lx.rulesMap[lexType]; ok {
		return fmt.Errorf("rule %q already exists", lexType)
	}
	if lexType == "" {
		return fmt.Errorf("rule type cannot be empty")
	}
	if lexRule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	lx.rulesMap[lexType] = lexRule
	lx.rules = append(lx.rules, lexType)
	lx.processor = nil // Invalidate processor so it's rebuilt with the new rule.
	return nil
}

// MustAddRule is like AddRule but panics if the rule cannot be added.
func (lx *TextLexer) MustAddRule(lexType LexemeType, lexRule Rule) {
	if err := lx.AddRule(lexType, lexRule); err != nil {
		panic(fmt.Sprintf("MustAddRule: %v", err))
	}
}

// Next reads from the input and returns the next recognized Lexeme.
//
// It returns an io.EOF error only when the stream is fully consumed and no more
// lexemes can be produced. Any other error indicates a problem with the
// underlying reader or an unrecoverable state (e.g., a rule that requires more
// input at EOF, or a token larger than the configured retention bound).
//
// This method is safe for concurrent use by multiple goroutines.
func (lx *TextLexer) Next() (*Lexeme, error) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	return lx.nextLexeme()
}

// Context returns the source text around lex, extended by before bytes on the
// left and after bytes on the right and clamped to the retained input.
//
// Context is a diagnostic aid. The lexer retains the input of the lexemes in
// its context window, so `before` can reach back across the lexemes that
// precede lex; anything beyond the window is clamped, not reported as an error.
// A lexeme is released once enough later tokens have committed to push it out
// of the window, and a call that commits nothing — a failed Next, or the EOF
// probe — releases nothing. Once lex is no longer retained, Context returns
// textreader.ErrPositionOutOfBuffer. It returns an error for a cursor that does
// not expose retained text.
func (lx *TextLexer) Context(lex *Lexeme, before, after int) (string, error) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	if lex == nil {
		return "", errors.New("textlexer: nil lexeme")
	}

	contextual, ok := lx.cursor.(contextualCursor)
	if !ok {
		return "", errors.New("textlexer: cursor does not expose retained text")
	}

	return contextual.Context(lex.span, before, after)
}

// Release drops the retained input this lexer holds for its context window,
// including the checkpoint of the most recently committed lexeme. It does not
// close the underlying reader, and the lexer remains usable.
//
// Call it before handing a shared cursor to another consumer: while the lexer
// holds checkpoints, a cursor configured with a finite retention budget keeps
// that input reserved, and the other consumer sees ErrRetentionExceeded sooner
// than it expects. After Release, Context returns ErrPositionOutOfBuffer until
// the next token commits.
func (lx *TextLexer) Release() {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	for i, c := range lx.marks {
		if c != nil {
			_ = c.Release()
			lx.marks[i] = nil
		}
	}

	lx.marksNext = 0
}

func (lx *TextLexer) getProcessor() (*rulesProcessor, error) {
	// Fast path: Check for existing processor with a read lock.
	lx.rulesMu.RLock()
	p := lx.processor
	lx.rulesMu.RUnlock()

	if p != nil {
		p.Reset()
		return p, nil
	}

	// Slow path: Acquire a write lock to create the processor.
	lx.rulesMu.Lock()
	defer lx.rulesMu.Unlock()

	// Double-check in case another goroutine created it while we were waiting for the lock.
	if lx.processor != nil {
		lx.processor.Reset()
		return lx.processor, nil
	}

	if len(lx.rules) == 0 {
		return nil, fmt.Errorf("no rules defined")
	}
	lx.processor = newRulesProcessor(lx.rules, lx.rulesMap)
	return lx.processor, nil
}

// readSymbol reads the next symbol from the cursor.
//
// It reports isEOF for the synthetic end-of-input symbol, which carries RuneEOF
// and the FlagEOF flag and is not part of the input. The positional flags come
// from the located rune and the cursor position, so a symbol read again during
// replay gets the same flags.
func (lx *TextLexer) readSymbol() (sym Symbol, isEOF bool, err error) {
	lr, err := lx.cursor.ReadLocatedRune()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return Symbol{}, false, err
		}

		at := lx.cursor.Cursor()

		return NewSymbol(RuneEOF, symbolFlags(at, RuneEOF, true)), true, nil
	}

	return Symbol{r: lr.Rune(), flags: symbolFlags(lr.Pos(), lr.Rune(), false), size: lr.Size()}, false, nil
}

// symbolFlags derives a symbol's positional flags from the position it was read
// at. BOF is the start of the stream, BOL is rune column zero, and EOL is a
// newline; EOF is supplied by the caller.
func symbolFlags(at textreader.Pos, r rune, eof bool) uint {
	flags := uint(FlagNone)

	if at.IsStart() {
		flags |= FlagBOF
	}
	if at.Column() == 0 {
		flags |= FlagBOL
	}
	if r == '\n' {
		flags |= FlagEOL
	}
	if eof {
		flags |= FlagEOF
	}

	return flags
}

func (lx *TextLexer) nextLexeme() (*Lexeme, error) {
	processor, err := lx.getProcessor()
	if err != nil {
		return nil, fmt.Errorf("processor: %w", err)
	}

	// Mark the start of the token: the cursor retains every byte from here until
	// the token is committed, which is what makes the rewind below exact and
	// what WithMaxTokenBytes bounds.
	mark := lx.cursor.Checkpoint()

	symbols := lx.runes[:0]
	tokenBytes := 0

	// abandon ends the token without committing it: the cursor is rewound to the
	// token start so a failed Next consumes nothing and a retry starts over. The
	// previous lexeme's checkpoint is deliberately left active, so its context
	// survives a failed call and an EOF probe.
	abandon := func(err error) (*Lexeme, error) {
		_ = mark.Reset()
		_ = mark.Release()
		lx.runes = symbols

		return nil, err
	}

	for {
		sym, isEOF, err := lx.readSymbol()
		if err != nil {
			return abandon(fmt.Errorf("ReadRune: %w", err))
		}
		if !isEOF {
			symbols = append(symbols, sym)
			tokenBytes += sym.Size()

			if lx.maxTokenBytes > 0 && tokenBytes > lx.maxTokenBytes {
				return abandon(fmt.Errorf("lexer: token exceeds %d bytes: %w", lx.maxTokenBytes, textreader.ErrRetentionExceeded))
			}
		}

		typ, textLen := processor.Process(sym)

		// textLen < 0 means the rule needs more input to decide.
		if textLen < 0 {
			if isEOF {
				// Processor wants more input but we hit EOF.
				return abandon(fmt.Errorf("lexer: rule remained inconclusive at EOF"))
			}
			continue
		}

		if textLen == 0 {
			if len(symbols) > 0 {
				typ = LexemeTypeUnknown
				textLen = 1 // Force consumption of one unknown symbol.
			} else if isEOF {
				return abandon(io.EOF)
			} else {
				return abandon(fmt.Errorf("lexer: rule returned zero-length token"))
			}
		}

		if isEOF && len(symbols) == 0 {
			// The synthetic EOF symbol is fed to the rules, so a rule may
			// report a match for it. Nothing was consumed, so the stream is
			// simply done.
			return abandon(io.EOF)
		}

		if textLen > len(symbols) {
			return abandon(fmt.Errorf("lexer: rule matched %d symbols, only %d were read", textLen, len(symbols)))
		}

		winner := symbols[:textLen]

		runes := make([]rune, len(winner))
		spanBytes := 0
		for i, s := range winner {
			runes[i] = s.Rune()
			spanBytes += s.Size()
		}

		start := mark.Pos()

		// Commit the cursor over exactly the matched bytes. The token is the
		// prefix of what was read, so committing is stepping back over the
		// speculative overshoot rather than replaying the token.
		end := lx.cursor.Cursor()
		if overshoot := (end.ByteOffset() - start.ByteOffset()) - spanBytes; overshoot > 0 {
			if _, err := lx.cursor.Seek(int64(-overshoot), io.SeekCurrent); err != nil {
				return abandon(fmt.Errorf("lexer: commit cursor: %w", err))
			}
			end = lx.cursor.Cursor()
		}

		span := textreader.NewSpan(start, end)

		// The token is committed: keep its checkpoint so Context can read around it,
		// and release only the one whose slot it takes — a mark is never released
		// by a call that did not commit a token.
		if old := lx.marks[lx.marksNext]; old != nil {
			_ = old.Release()
		}
		lx.marks[lx.marksNext] = mark
		lx.marksNext = (lx.marksNext + 1) % markWindow

		lx.emitted += len(winner)
		lx.runes = symbols

		return NewLexemeWithSpan(typ, runes, span), nil
	}
}
