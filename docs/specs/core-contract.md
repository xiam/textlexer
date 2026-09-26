# textlexer core contract

This document is the canonical prose specification for the `textlexer` core:
what the engine does today, what it is intended to become, and where the design
is still open. It is paired with the [documentation entrypoint](../README.md)
and the [root README](../../README.md). Where this document and code disagree,
code wins and this document is wrong — file a change against this document
rather than against behavior.

## Status convention

Every substantive statement in this document carries one of three statuses.
Read them as a promise ladder, not as a to-do list.

- **Current contract** — the behavior exists in code at `master` and is
  exercised by committed tests. Consumers may rely on it. Citing a named test
  or source location is required, because this is the part of the document that
  is load-bearing.
- **Intended capability** — the owner intends the toolkit to provide this, and
  this document records the *semantic* intent so later work is decidable. It is
  **not** available API. No method signature, data structure, or construction
  order is approved by this statement.
- **Open design question** — the shape is genuinely undecided. This document
  names the question and records the constraint it must satisfy; it does not
  choose an answer, and it does not pretend an answer has owner approval.

A promise that has no status is a bug in this document.

## Purpose and scope

`textlexer` is intended to be a **reusable toolkit for constructing many
different lexers**. A consumer supplies the lexical policy for one language or
one text format and receives back a stream of **descriptive lexemes** — content,
classification, and source position — and then decides what to do with them.

**Current contract.** The engine available today is a single general-purpose
tokenization core (`TextLexer`) driven by user-written rule state machines. It
tokenizes; it does not parse. There is no public rule-builder API: every rule in
the shipped example and test suite is a hand-written Go function.

**Intended capability.** On top of that core, the toolkit is intended to offer
reusable, composable rule tools (literals, character predicates, Unicode
categories, sequence/alternation/repetition, named rules, lexical modes, and
post-match actions) so that a consumer can describe a lexer declaratively in
native Go instead of hand-rolling automata. See
[Construction capabilities](#intended-construction-capabilities).

**Out of scope for the core.** Parsing, grammar validation, AST construction,
and interpretation are downstream consumers' jobs. The core never produces a
syntax tree and never decides what a token *means* beyond assigning it a
classification and a span. See [Lexer/consumer boundary](#lexerconsumer-boundary).

## Terminology

- **Rule** — a state-transition function of the form
  `func(Symbol) (next Rule, state State)`. A rule is one state of a finite
  automaton; the returned `next` is the next state. **Current contract**
  (`rule.go`).
- **State** — the outcome of one transition: `StateContinue` (possibly matches,
  needs more input), `StateAccept` (a valid match so far), `StateReject`
  (can no longer match), or `StatePushBack` (the just-read symbol is not part
  of this match; re-evaluate it with `next`). **Current contract**
  (`state.go`).
- **Symbol** — a rune enriched with positional flags: `Rune()`, `Size()` (source
  bytes), and `IsEOF`/`IsBOF`/`IsEOL`/`IsBOL`. A rule decides on the rune and the
  flags together. **Current contract** (`symbol.go`).
- **Lexeme** — the output unit: a classification (`LexemeType`), its text
  (`[]rune`), its starting offset, and the source `Span` it covers.
  **Current contract** (`lexeme.go`).
- **Token** — the in-progress match the engine assembles speculatively before it
  commits it as a lexeme. A token becomes a lexeme only when the rules settle on
  a winning match and the engine commits the cursor over exactly that range.
  **Current contract** (`textlexer.go`).
- **Maximal munch** — the engine selects the *longest* accepted match as the next
  lexeme. **Current contract** (`processor.go`).
- **Cursor** — the positioned text stream the lexer reads through, provided by
  `github.com/xiam/textreader`. It owns reading, buffering, positions,
  checkpoints, and replay. The lexer owns no text-stream mechanics.
  **Current contract** (package doc, `textlexer.go`).
- **Checkpoint** — a marked logical position on the cursor that the engine uses
  to rewind a token that is abandoned. **Current contract** (`textlexer.go`).
- **Context window** — a fixed-size ring of the most recent committed lexemes
  (three today) whose retained input the engine keeps so `Context` can read the
  text around one of them. **Current contract** (`textlexer.go`, `markWindow`).

## Lexer/consumer boundary

**Current contract.** The lexer owns lexical policy and nothing else: rule state
transitions, competing-rule evaluation, maximal-munch selection,
registration-order tie breaking, the `UNKNOWN` fallback, and lexeme
construction. It owns **no** text-stream mechanics.

Reading, buffering, positions, and replay are owned by the positioned cursor
(`github.com/xiam/textreader`). The cursor decodes UTF-8, tracks the logical
position, retains the input a checkpoint needs, and restores a marked position
exactly.

Downstream of the lexer, the consumer owns everything else: deciding what a
`LexemeType` means, building a grammar or parser, constructing an AST, and
interpreting or executing. The core makes none of those decisions and exposes no
API for them. A consumer that wants parsing gets lexemes and writes a parser
over them.

**Intended capability.** The boundary is expected to stay fixed as the toolkit
grows: new rule tools change *how a consumer describes lexical policy*, not
*what the lexer returns*. The output unit remains a descriptive lexeme.

**Open design question.** Whether the core should ever expose a stable
"token stream" abstraction (e.g., an iterator interface) rather than the
imperative `Next()` loop is undecided. The current `Next()` API is the contract;
the question is recorded so a future change is a deliberate one.

## Consumer contract

This is what a consumer actually receives from `Next()`, and the guarantees that
hold for it.

### Classification

**Current contract.** A lexeme carries exactly one string-valued
classification, `LexemeType`. `Type()` returns it; it is whatever string the
consumer registered the rule under. The fallback for input no rule matched is
the constant `LexemeTypeUnknown` (`"UNKNOWN"`). There is **no** separate kind
API, no hierarchy, and no built-in set of types — the type space is the
consumer's.

**Intended capability.** The toolkit is intended to let a consumer express both a
broad category and a specific kind (for example, a *literal* that is
specifically a *string literal*), and to carry structured metadata alongside the
type. **This is intent only.** No broad/specific hierarchy, no metadata shape,
and no API for either has been approved. A future API must not be described as
existing.

**Open design question.** Whether classification stays a flat string, becomes a
typed value, or gains an orthogonal kind/metadata field is undecided. The
constraint: whatever the shape, a lexeme must remain descriptive and self-sufficient
(a consumer should not need the original source to interpret a lexeme's
classification).

### Matched content versus transformed or decoded values

**Current contract.** `Text()` returns the **matched content**: the runes the
winning rule accepted, exactly as read, as a `string` (from `[]rune`). The lexer
does **not** transform or decode the content. It does not unescape, it does not
parse a number into an `int`, it does not strip quotes, and it does not apply any
language-specific semantics. A `NUMBER` lexeme's `Text()` is the digit string,
not a numeric value. Any decoding is the consumer's job.

`Len()` returns the rune count of `Text()`. `Offset()`, `RuneOffset()`,
`ByteOffset()`, `Line()`, and `Column()` report the **start** position of the
match; `Span()` reports the full range.

**Open design question.** Whether the core should offer an optional
post-match *action* hook that can attach a decoded/transformed value to a
lexeme is recorded under
[Construction capabilities](#intended-construction-capabilities) as intended
capability with an open shape. As of this document, no such hook exists and the
lexeme carries matched content only.

### Source spans

**Current contract.** Every lexeme the lexer produces carries a `Span` — a
**half-open** range `[Start, End)` in the source's own coordinates. Both ends
expose byte offset, rune offset, line, and rune column:

- `Start().ByteOffset()` / `End().ByteOffset()` — zero-based byte offsets.
- `Start().RuneOffset()` / `End().RuneOffset()` — zero-based rune offsets.
- `Start().Line()` / `End().Line()` — **1-based** line numbers.
- `Start().Column()` / `End().Column()` — **0-based rune columns**.
- `Span().Bytes()` / `Span().Runes()` — widths in bytes and runes.

`Lexeme` shorthands `ByteOffset()`, `RuneOffset()`, `Line()`, `Column()` all
report the **start**. `Offset()` is the long-standing accessor that counts
**runes** and equals `RuneOffset()`; use `ByteOffset()` when bytes are what you
need. `Len()` equals `Span().Runes()` for a lexer-produced lexeme.

Spans are **committed, not speculative**: a span covers exactly the winning
match, never the furthest point the rules read ahead to. A rule that looks ahead
past its match does not shift the next token's span.

**Evidence.** `TestLexerLexemeSpans` (ASCII, multi-byte UTF-8, newline: every
byte of the input covered by exactly one lexeme, in order, with `Len ==
Span().Runes` and `Offset == RuneOffset`); `TestLexerSpanIsCommittedNotSpeculated`
(a rule that reads one symbol past its match still commits a 1-byte span per
token); `TestLexerRuneReaderSourceSpans` (rune-only source yields correct byte,
rune, line, and column coordinates). Source: the commit block in
`nextLexeme` (`textlexer.go`) and `Span`/`Pos` in `textreader`.

### Lexeme constructors and the manual-constructor exception

**Current contract.** A `Lexeme` is a plain value; nothing forces a lexeme to
carry a span. There are three exported constructors, and they split into two
kinds:

- `NewLexeme(typ, text []rune, offset int)` and
  `NewLexemeFromString(typ, text, offset)` are the **manual constructors**. They
  record a **rune offset only** and leave the span empty. For such a lexeme
  `Offset()` and `RuneOffset()` return the offset the caller passed, but
  `ByteOffset()`, `Line()`, and `Column()` all report **zero** (the empty span's
  start), and `Span()` is the zero `Span`. These are the "legacy" constructors a
  consumer or test can call to fabricate a lexeme without a cursor.
- `NewLexemeWithSpan(typ, text []rune, span)` carries a **full source span**; its
  `Offset()`/`RuneOffset()` are taken from the span's start, so the compatible
  accessor and the span agree.

A **lexer-produced** lexeme — one returned by `Next()` — always carries a real
span covering the committed match, so its byte, rune, line, and column
coordinates are all meaningful. The manual-constructor exception is only that a
consumer-built lexeme may legitimately have a rune offset with no span, and the
byte/line/column accessors then read zero rather than error. This is a
documented property of the value, not a bug.

**Evidence.** `NewLexeme`/`NewLexemeFromString`/`NewLexemeWithSpan` and the
accessor doc comments in `lexeme.go`; the committed-span path in `nextLexeme`
(`textlexer.go`).

### Byte versus rune units

**Current contract.** Both unit systems are first-class and always consistent
within a lexeme:

- **Byte offsets** address the source as a byte stream. They are what you need
  to slice the original `[]byte`, to feed a byte-oriented consumer, or to
  round-trip into the source.
- **Rune offsets** address the source as a sequence of decoded runes. `Offset()`
  and `RuneOffset()` are rune counts; `Len()` is a rune count; columns are rune
  columns.

A multi-byte rune occupies several bytes but one rune and one column. `héllo`
is 5 runes in 6 bytes and 5 columns. `Span().Bytes()` and `Span().Runes()` give
the two widths; they differ exactly by the extra bytes of multi-byte runes.

**Open design question.** Whether the core should also expose a
*grapheme-cluster* coordinate (a single user-perceived character may be several
rune code points) is undecided. Columns are currently rune columns, not
grapheme columns; a consumer that needs display-width or grapheme behavior must
derive it from the span itself.

### Line and column conventions

**Current contract.** Lines are **1-based** (the first line is line 1). Columns
are **0-based rune columns** (the first rune of a line is column 0). A `\n` ends
the current line; the next rune is on the next line at column 0. `\r` does not,
by itself, start a new line in the column accounting the way `\n` does — the
cursor's line/column tracking is defined by `textreader` and is the authority
here.

**Evidence.** `TestLexerLexemeSpans` asserts a token on the second line has
`Line()==2, Column()==0`. `TestLexerRuneReaderSourceSpans` asserts a token after
a newline has `Line()==2, Column()==0` and a mid-line token has the expected
column.

**Open design question.** Whether `\r\n` should be treated as a single
line-break unit for column purposes (so a token after `\r\n` reports column 0
on the new line) or as `\r` followed by `\n` is not pinned down by a committed
test. This is recorded as a question, not a promise; the cursor's behavior is
the current authority.

### Lexeme lifetime versus diagnostic-context lifetime

**Current contract.** There are two distinct lifetimes:

- **The lexeme value is durable.** A `Lexeme` is a self-contained value. Its
  `Span`, offsets, line/column, `Type()`, and `Text()` are immutable and remain
  correct no matter how many further tokens the consumer reads. The span is a
  value copied out of the cursor at commit time; it is not a reference into
  cursor state.
- **Diagnostic context is bounded.** `Context(lex, before, after)` returns the
  source text around `lex`, including input that *precedes* it. This is a
  diagnostic aid, and it is only available while `lex` is inside the engine's
  context window. Once enough later tokens commit and push `lex` out of that
  window, its retained input is released and `Context` returns
  `textreader.ErrPositionOutOfBuffer`. A call that commits nothing — a failed
  `Next`, or the final EOF probe — releases nothing, so a lexeme's context
  survives a failed call and an EOF probe.

So: the *facts about* a lexeme (its span, type, text) live forever; the *raw
text around* a lexeme lives only as long as the window retains it. A consumer
that needs surrounding text for a lexeme much later must have captured it with
`Context` while the lexeme was still in the window.

**Evidence.** `TestLexerContext` (a lexeme stays readable across one more token,
then falls out and returns `ErrPositionOutOfBuffer`);
`TestLexerContextReachesPrecedingLexemes` (`before` reaches back across preceding
lexemes and clamps rather than errors when asked for more than is retained);
`TestLexerContextSurvivesFailedNext` and `TestLexerContextSurvivesEOFProbe`
(a non-committing call releases nothing).

### Unmatched input, EOF, and errors

**Current contract.**

- **Unmatched input** is never silently dropped. If no rule accepts a symbol, the
  engine emits a **single-rune `UNKNOWN` lexeme** for that symbol and advances by
  exactly one rune. Tokenization always makes progress: every successful `Next()`
  consumes at least one rune, and the `UNKNOWN` fallback guarantees that.
- **EOF** is returned as `io.EOF` from `Next()` once the stream is fully consumed
  and no further lexeme can be produced. A rule that still needs input at EOF
  (an unterminated construct) is an error, not EOF: `Next` returns
  `lexer: rule remained inconclusive at EOF` and consumes nothing.
- **Errors other than EOF** indicate a problem with the underlying reader or an
  unrecoverable engine state (for example a token larger than the configured
  retention bound). On a returned error, `Next` has made **no partial progress**:
  the cursor is rewound to the token start, so a retry starts over rather than
  resuming mid-token. See
  [Returned-error rollback](#returned-error-rollback).

**Evidence.** `TestLexerProcessor` "No Matching Rules (produces UNKNOWN)" (each
unmatched rune is its own `UNKNOWN` lexeme); `TestLexerEdgeCasesAtEOF` (a
settled rule yields `io.EOF`; empty input yields `io.EOF` immediately);
`TestLexerZeroLengthMatchPrevention` (a rule that never advances still yields one
`UNKNOWN` rune per input rune, never an infinite loop);
`TestLexerRetentionFailureConsumesNothing` (a rejected token consumes nothing and
a retry is identical).

**Open design question.** Whether the `UNKNOWN` fallback should be
configurable or replaceable (for example, a consumer-supplied "on no match"
handler that can emit a different token or raise) is undecided. Today it is
fixed to one-rune `UNKNOWN`.

## Execution contract

This is how the engine decides, from code and tests. Each item is load-bearing.

### Competing rules, evaluated sequentially

**Current contract.** A rule is a state machine; the engine runs **all** active
rules over the same input, one symbol at a time. "Parallel" means *competing
state machines*, not concurrency: within a single `Next()`, the rules are stepped
**sequentially in registration order** for each symbol. Each rule independently
tracks its own furthest acceptance. The engine reads ahead until every rule has
either rejected or terminated, then selects the winner.

**Evidence.** `rulesProcessor.processSymbol` iterates `rp.rules` in order
(`processor.go`); `TestLexerConcurrentAccess` and `TestLexerConcurrentDeterminism`
exercise many goroutines calling `Next` on one lexer and assert the token stream
is deterministic and complete.

### Longest match

**Current contract.** The engine applies **maximal munch**: the rule with the
**longest** accepted match wins, regardless of registration order. A rule that
accepted 4 symbols beats a rule that accepted 3, even if the 3-rule was
registered first.

**Evidence.** `TestLexerProcessor` "Longest Match (Float vs Integer)" (input
`12.345`, the FLOAT rule's longer acceptance beats INT's `12`);
`TestLexerProcessor` "Float with trailing decimal at EOF" and "Fallback from
potential Float to Integer" (the float's accepted prefix wins where it is
longer).

### Tie priority

**Current contract.** When two rules accept the **same** longest length, the one
**registered earlier** wins. The engine walks rules in registration order with a
strict `>` comparison on accepted length, so a later rule cannot displace an
earlier rule at equal length.

**Evidence.** `TestLexerProcessor` "Rule Priority (Keyword vs Identifier)"
(`if` registered before `identifier` → `if` wins the tie at length 2) and
"Rule Priority Reversed (Identifier wins)" (registering `identifier` first flips
the outcome, proving registration order, not type, decides ties).

### Rule states and pushback

**Current contract.** A rule reports one of four states per symbol:

- `StateContinue` — possible match, needs more input; the rule must return a
  non-nil `next`.
- `StateAccept` — a valid match so far; the rule may return a non-nil `next` to
  keep scanning for a longer match, or nil to stop.
- `StateReject` — this rule can no longer match; it is deactivated for the
  current token.
- `StatePushBack` — the just-read symbol is **not** part of this match; the
  engine re-feeds that symbol to the returned `next` rule. This is the mechanism
  for lookahead and backtracking.

Pushback is **bounded by the symbols read so far**: a rule can push back only as
many symbols as are currently in the token buffer. Pushing back more than exists
deactivates the rule (it cannot re-consume input it never saw). A rule that
returns `(nil, StatePushBack)` has nowhere to push back to and is treated as
done.

**Evidence.** `TestLexerProcessor` "Context-Aware Rules (BOL)" (the comment rule
pushes back the EOL/EOF terminator so it is not included in the comment);
`newUnsignedFloatRule` in the test helpers (pushes back the first non-digit after
a radix so `12.3` does not swallow the next token); `TestLexerPathologicalRules`
"Rule Returns nil with StatePushBack" (a nil pushback target is handled, not
crashed on).

### Progress and zero-length behavior

**Current contract.** The engine guarantees forward progress: a token is never
committed with zero length. If a rule's net effect would be a zero-length match
(a pushback that consumes nothing, or an accept of an empty sequence), the engine
does **not** emit an empty token and does **not** loop; it falls back to a
single-rune `UNKNOWN` lexeme and advances by one rune. This makes a
pathologically non-advancing rule terminate rather than spin.

**Evidence.** `TestLexerZeroLengthMatchPrevention` (a rule that always
pushes-back-then-accepts yields exactly one `UNKNOWN` rune per input rune and
terminates within a bounded number of iterations); `TestLexerInfiniteLoopProtection`
(Always-Continue, Always-PushBack, and Zero-Length-Accept-Loop rules all
terminate).

### EOF handling

**Current contract.** The engine feeds a **synthetic EOF symbol** (carrying
`RuneEOF` and `FlagEOF`, not part of the input) to the rules once the underlying
reader is exhausted. This lets a rule observe end-of-input. The rules are
expected to **settle** on EOF: a rule that is still `StateContinue` (needing more
input) at EOF makes the token inconclusive, and `Next` returns
`lexer: rule remained inconclusive at EOF` with no progress. A rule that has
already accepted and is now inactive contributes its last acceptance. If no real
symbol was consumed and the stream is done, `Next` returns `io.EOF`.

**Rule-author responsibility.** A well-behaved rule treats the EOF symbol as
non-matchable — it rejects it (or pushes it back), never accepts it. A rule that
returns `StateAccept` for the EOF symbol produces a match whose length counts the
synthetic symbol, which the engine rejects as inconsistent (`lexer: rule matched
N symbols, only N-1 were read`) unless the EOF symbol was the only one, in which
case the clean `io.EOF` path applies. See
[Custom-rule responsibilities](#custom-rule-responsibilities).

**Evidence.** `TestLexerEdgeCasesAtEOF` (a settled rule at EOF → `io.EOF`; a rule
in a different state at EOF → the longer real acceptance wins; empty input →
`io.EOF` immediately); `TestLexerProcessor` "Tokens immediately at EOF". The
inconclusive-at-EOF path is the `textLen < 0 && isEOF` branch in `nextLexeme`.

### Registration validation

**Current contract.** `AddRule` validates before registering:

- A `LexemeType` that is already registered is rejected (one rule per type).
- An empty `LexemeType` (`""`) is rejected.
- A `nil` rule is rejected.

`MustAddRule` panics on any of these. Calling `Next` with **no rules registered**
returns an error (`processor: no rules defined`) rather than panicking.

**Evidence.** `TestLexerErrorConditions` "Duplicate Rule Types", "Nil Rule",
"Empty LexemeType". The no-rules path is `getProcessor` in `textlexer.go`.

### Returned-error rollback

**Current contract.** When `Next` returns a **non-EOF error**, the engine has
made **no partial progress**. It rewinds the cursor's logical position back to
the token's start (releasing the token's checkpoint), so the next `Next` sees
exactly the same input. A failed call consumes nothing, and a retry is
identical: it does not resume in the middle of the rejected token.

This rollback is a **logical** rewind of the cursor over already-read, retained
input. It is distinct from two related but separate things:

- It is **not** a re-read of the underlying source. The bytes were already read
  from the reader into the cursor's retained region; rollback repositions the
  logical cursor inside that region. It does not call the underlying
  `io.Reader` again for those bytes.
- It does **not** undo arbitrary side effects a rule may have performed. A rule
  is an ordinary Go function; if it wrote to a file, mutated shared state, or
  printed, the rollback does not and cannot reverse that. The guarantee is about
  the *lexer's* view of the input, not about the rule's effects on the world.

**Open design question.** Whether rollback should be extended to rule *panics*
is explicitly **not** a promise. Today a panicking rule propagates the panic and
bypasses the rollback path entirely; the engine does not recover from rule
panics. See [Panics](#panics).

**Evidence.** `TestLexerRetentionFailureConsumesNothing` (a token rejected by a
retention bound leaves the cursor at byte offset 0 and a retry is identical, for
both the cursor-level and token-level bounds); `TestLexerContextSurvivesFailedNext`
(a failed `Next` commits nothing and does not release the previous context).

### Speculative lookahead limits

**Current contract.** While a token is assembled the engine holds a checkpoint
covering everything read for that token, including maximal-munch **lookahead**.
`WithMaxTokenBytes(n)` bounds how many source bytes one token may retain. The
bound **includes** look-ahead: a rule that keeps matching forces the engine to
retain everything it has read for the token so far, and the counter advances with
every speculatively read symbol. A token that would exceed the bound makes `Next`
return an error wrapping `textreader.ErrRetentionExceeded`, with no partial
progress (the cursor is rewound). The default is unbounded. When the bound is set,
the cursor's retention budget is sized to cover the context window plus the
in-flight token, so a stream of tokens that each fit the bound lexes all the way
through.

**Evidence.** `TestLexerMaxTokenBytes` (within the bound lexes normally; over the
bound returns `ErrRetentionExceeded` with no lexeme);
`TestLexerMaxTokenBytesAcrossWindow` (consecutive 1-byte tokens under a 1-byte
bound lex fully); `TestLexerRetentionFailureConsumesNothing` (the bound fires with
no progress).

### Concurrency boundaries

**Current contract.**

- `Next` is **safe for concurrent use** by multiple goroutines. Concurrent
  callers observe a single, deterministic, complete token stream; no token is
  duplicated or lost.
- `AddRule` is **not** safe for concurrent use with `Next`. All rules should be
  added **before** tokenization begins. (The engine guards the rule table with a
  lock and rebuilds its processor when a rule is added, so a concurrent
  `AddRule`+`Next` does not race in memory, but the *policy* is: register first,
  then tokenize. The committed concurrency tests exercise `Next`-only
  concurrency as the supported pattern.)
- A lexer created over a **shared cursor** (via `NewWithCursor`) should call
  `Release` before handing the cursor to another consumer. While the lexer holds
  its context-window checkpoints, a finite-retention cursor keeps that input
  reserved, and the other consumer hits `ErrRetentionExceeded` sooner than
  expected. After `Release`, `Context` returns `ErrPositionOutOfBuffer` until the
  next token commits. `Release` does not close the underlying reader and the
  lexer remains usable.

**Evidence.** `TestLexerConcurrentAccess` (4 goroutines, 5000 tokens, exact
count, no error); `TestLexerConcurrentDeterminism` (concurrent `Next` yields a
deterministic stream); `TestLexerConcurrentAddRuleAndNext` (concurrent
`AddRule`+`Next` is memory-race-clean, run under `-race`);
`TestLexerReleaseHandsOffSharedCursor` (after `Release`, another consumer reads
the remainder through the same finite-retention cursor).

### Panics

**Current contract, stated to bound a promise.** The engine's
**returned-error** guarantees (rollback, no partial progress) apply to errors a
rule or the engine *returns*. They **do not extend to panics**. A rule is an
ordinary Go function; if it panics, the panic propagates out of `Next` and the
engine does **not** recover it, does **not** run the rollback path, and makes no
claim about the lexer's subsequent state. A consumer that wants panic isolation
must `recover` around its own `Next` call.

**Evidence.** `TestLexerPanickingRule` and `TestLexerPanicRecovery` (a panicking
rule propagates the panic; the engine does not recover it);
`TestLexerErrorConditions` "Rule Panic Types" (string, error, integer, and nil
panic values all propagate). The rollback path is the `abandon` closure in
`nextLexeme`, which is reached only on returned errors, not on a panic unwinding
the stack.

### Custom-rule responsibilities

**Current contract.** The engine is a general automaton runner; it does not
validate that a rule is *well-behaved*. The following are the consumer's
responsibilities, and violating them yields the documented outcomes rather than
an engine fix:

- **Handle EOF.** A rule should reject (or push back) the EOF symbol, never
  accept it. Accepting it makes the match length count a synthetic symbol and the
  engine rejects the token as inconsistent, unless it was the only symbol.
- **Settle.** A rule should not remain `StateContinue` (needing more input)
  forever. A rule that is still inconclusive at EOF makes the token fail with
  `rule remained inconclusive at EOF`.
- **Advance.** A rule's net effect should consume input. A rule that never
  advances is contained by the zero-length/`UNKNOWN` fallback and the
  infinite-loop protection, but it produces `UNKNOWN` tokens rather than the
  rule's intended match.
- **Return a valid `State`.** The four defined states are the contract. Returning
  any other value makes the engine panic (`unknown nextState`), because the
  processor treats an unrecognized state as a programming error.
- **Side effects are the rule's.** The engine offers no transactionality around a
  rule; any side effect a rule performs is permanent and not rolled back.

## Intended construction capabilities

**Status: Intended capability, throughout.** This section records the *semantic*
capabilities the owner intends the toolkit to provide so that later core work is
decidable. It deliberately contains **no** method signatures, **no** data
structures, and **no** implementation roadmap. Nothing here is available API.

The intent is that a consumer can **describe** a lexer from reusable parts rather
than hand-rolling automata, while the engine keeps the same execution contract
documented above (maximal munch, registration-order ties, committed spans,
progress guarantee).

- **Reusable literals and character predicates.** Building blocks that match an
  exact literal, a character in a set, or a Unicode category, so a consumer does
  not re-derive "is this a letter/digit/punctuation" per language.
- **Unicode categories.** Classification by Unicode property (letter, digit,
  mark, etc.) as a first-class predicate, so a rule can say "any letter" without
  an ASCII-only hand-written test.
- **Sequence, alternatives, repetition, optional, and named rules.**
  Composition so a consumer can express "A then B", "A or B", "one or more A",
  "optional A", and a rule that can be referenced by name and reused.
- **Context-sensitive lexical modes.** The ability to switch what the active rule
  set is based on position or a prior token (for example, inside a string versus
  outside one), so the same symbol can classify differently in different states.
- **Actions applied only after a winning match commits.** The ability to attach a
  post-match action (decode a number, unescape a string, compute a value) that
  runs **only** for the lexeme the engine actually commits — not for the losing
  candidates the maximal-munch process rejected. This is the intended home for
  "transformed/decoded values", and it is the resolution path for the
  [matched-content question](#matched-content-versus-transformed-or-decoded-values).

**Open design questions (recorded, not decided).**

- **Composition and ambiguity.** How composed rules interact with maximal munch
  and registration-order ties is undecided. In particular: when a composed rule
  and a flat rule accept the same length, and how a composed rule's internal
  alternatives participate in the global longest-match selection.
- **State ownership and reset.** Who owns a composed rule's internal state, and
  how it resets between tokens, is undecided. The current engine resets all rule
  state at the start of every token; a composed rule must fit that model or the
  model must change deliberately.
- **Recovery.** Whether and how the toolkit offers structured recovery (a
  consumer-facing "no match" hook, a recoverable token stream) instead of the
  fixed one-rune `UNKNOWN` fallback is undecided.
- **Synthetic-token and progress semantics.** Whether composed rules may emit
  zero-width or synthetic tokens (a token not backed by a source span) is
  undecided, and conflicts with the current progress guarantee that every
  committed lexeme covers a non-empty source range. Any such capability must be
  reconciled with that guarantee explicitly.

**Core/extension boundary.** Language-specific policies that are *not* about
recognizing text — indentation and formatting rules, language-specific error
messages, recovery strategies tied to a particular grammar — are intended to live
in an **extension** layer on top of the core, not in the core itself. The core
provides the recognition and classification machinery; an extension applies a
particular language's policies. The exact seam (an interface, a set of hooks, a
separate package) is an open design question.

## Illustrative scenarios

These are **prose examples** to make the contract concrete, not new derived lexer
packages and not shipped lexers. Each is labeled **current** (it is what the
engine does today, mapped to a named test or source location) or **aspirational**
(it is what an intended capability would make possible). Where a current scenario
has a coverage gap, the gap is recorded as a question.

### 1. Keyword versus a longer identifier — current

Input: `if identifier` with a keyword rule for the literal `if`, an identifier
rule for a letter followed by letters/digits, and a whitespace rule.

Output: `IF "if"`, then `WHITESPACE " "`, then `IDENTIFIER "identifier"`.

The `if` and `identifier` rules both accept the text `if` (the identifier rule
accepts it as a valid identifier), a **tie at length 2**. The keyword rule was
registered first, so registration-order tie priority gives it the token. A longer
identifier that the keyword rule cannot match (`identifier`) is won outright by
the identifier rule under maximal munch.

**Mapped to:** `TestLexerProcessor` "Rule Priority (Keyword vs Identifier)" and
"Rule Priority Reversed (Identifier wins)". This is the canonical demonstration
that ties are decided by registration order, not by type, and that a longer match
outranks a shorter one.

### 2. Multi-byte and multi-line spans — current

Input: `héllo 世界\n42` with a word rule (Unicode letters), a number rule, a
newline rule, and a space rule.

Output: the word `héllo` is **5 runes in 6 bytes** (the `é` is two bytes); it
starts at line 1, column 0, and its span is `Bytes()==6, Runes()==5`. The `42`
number starts on **line 2, column 0**, after the newline. Every byte of the input
is covered by exactly one lexeme, in order, with each lexeme starting exactly
where the previous one ended.

**Mapped to:** `TestLexerLexemeSpans` (the byte/runes/line/column assertions,
including `Len == Span().Runes` and the line-2 column-0 number) and
`TestLexerRuneReaderSourceSpans` (the same coordinates through a rune-only
source, where the cursor re-encodes each rune to UTF-8 so byte offsets stay
meaningful).

**Coverage gap (question).** There is no committed test asserting the exact
line/column a token reports **immediately after a `\r\n`** sequence (as opposed to
a bare `\n`). The `\r\n` column convention is therefore an open design question,
not a verified behavior.

### 3. Context-dependent delimiter — current

Input: `  # not a comment\n# a comment` with a comment rule that matches `#`
**only at the beginning of a line** (using `IsBOL`), a whitespace rule, a
literal `#` fallback rule, and an identifier rule.

Output: the first `#` (not at line start) is **not** a comment — it is matched by
the fallback literal `#` rule and the following words are identifiers. The second
`#` **is** at the beginning of a line, so the comment rule accepts it and
consumes through end of line (pushing back the terminating `\n` so the newline
stays its own token). Result: `WHITESPACE`, `HASH "#"`, identifiers `not`, `a`,
`comment`, `WHITESPACE "\n"`, then `COMMENT "# a comment"`.

This is the concrete, shipped demonstration of **context sensitivity**: the same
symbol `#` classifies differently depending on its positional context (BOL or
not), using the `Symbol` flags rather than any mode machinery.

**Mapped to:** `TestLexerProcessor` "Context-Aware Rules (BOL)".

**Aspirational.** A full **lexical mode** (e.g., the engine switching to a
"string-interpolation" rule set inside `"...${ ... }..."` and back out) is an
intended capability, not a current one. The BOL example is position-based
sensitivity; mode-based sensitivity (state carried across tokens, not just within
one) is recorded under
[Construction capabilities](#intended-construction-capabilities) as intended,
with state ownership/reset an open question.

### 4. Unmatched and malformed input — current

**Unmatched.** Input: `abc` with only an integer rule registered. No rule
accepts a letter, so the engine emits one `UNKNOWN` lexeme per rune: `UNKNOWN
"a"`, `UNKNOWN "b"`, `UNKNOWN "c"`. Nothing is dropped; progress is one rune per
token. **Mapped to:** `TestLexerProcessor` "No Matching Rules (produces UNKNOWN)".

**Malformed UTF-8.** Input containing a byte that is not valid UTF-8 (for example
a stray `0xFF`) is **not** an error. The cursor decodes each invalid byte as the
single replacement rune U+FFFD (size 1, one byte, one rune, one column) and
continues. The lexeme's **span remains accurate** — its byte offsets still point
into the original source — but `Text()` is **not** byte-for-byte: the original
invalid byte is represented in `Text()` as U+FFFD, which re-encodes to a
different (three-byte) sequence. In other words: **positions are faithful; text is
lossy** for malformed UTF-8. A consumer that must preserve the exact bytes of a
malformed region should work from the span (byte offsets into the source), not
from `Text()`.

**Rune-only source.** A source that implements only `io.RuneReader` (no
`io.Reader`) is read rune by rune, and the cursor re-encodes each rune to UTF-8
in its buffer. Byte offsets therefore describe the **UTF-8 encoding of the rune
stream the caller's reader represents**, not any physical byte layout the caller
might have had. Coordinates (byte, rune, line, column) are all still consistent
and correct. **Mapped to:** `TestLexerRuneReaderSourceSpans` and the
`NewRuneReader` path in `textreader`.

**Coverage gap (question).** There is no committed test asserting the exact
`Text()`/span relationship for a **trailing incomplete** multi-byte sequence at
EOF (a leading byte with no continuation bytes before the end of input). The
cursor's carry/flush logic handles it by crediting the incomplete bytes as a
replacement rune, but this specific case is not pinned down by a named test and
is recorded as a question rather than a promise.

### Motivating use cases — not shipped lexers

SQL, Python, Go, regular-expression syntax, and natural-language segmentation are
**motivating use cases** for the toolkit: each is a domain a consumer could build
a lexer for using the core (and, in time, the intended rule tools). None of them
is a shipped lexer, and none of them implies a claim of **universal linguistic
understanding**. A natural-language "lexer" built on this core would be a
consumer's choice of rules; the core provides recognition and classification
machinery and nothing about language semantics. This document makes no claim that
any of these is available out of the box.

## Open design questions (index)

Collected in one place so later work can close them one by one. Each is
referenced where it first arises.

1. **Classification shape** — flat string, typed value, or type + orthogonal
   kind/metadata. ([Classification](#classification))
2. **Token-stream abstraction** — keep the imperative `Next()` loop or expose a
   stable iterator. ([Lexer/consumer boundary](#lexerconsumer-boundary))
3. **Decoded/transformed values** — an optional post-match action hook; shape
   undecided. ([Matched content](#matched-content-versus-transformed-or-decoded-values))
4. **Grapheme-cluster coordinates** — rune columns today; grapheme columns
   undecided. ([Byte versus rune units](#byte-versus-rune-units))
5. **`\r\n` line-break convention** — single unit or `\r` + `\n` for column
   purposes; not pinned by a test. ([Line and column conventions](#line-and-column-conventions))
6. **Configurable no-match behavior** — fixed one-rune `UNKNOWN` today;
   replaceable hook undecided. ([Unmatched input](#unmatched-input-eof-and-errors))
7. **Panic isolation** — rollback explicitly does not extend to rule panics;
   whether it ever should is open. ([Returned-error rollback](#returned-error-rollback))
8. **Composition and ambiguity** — how composed rules interact with maximal
   munch and registration-order ties. ([Construction capabilities](#intended-construction-capabilities))
9. **State ownership and reset** — who owns composed-rule state; current model
   resets per token. ([Construction capabilities](#intended-construction-capabilities))
10. **Recovery** — structured recovery vs. the fixed `UNKNOWN` fallback.
    ([Construction capabilities](#intended-construction-capabilities))
11. **Synthetic-token / progress semantics** — zero-width or synthetic tokens
    conflict with the non-empty-span progress guarantee.
    ([Construction capabilities](#intended-construction-capabilities))
12. **Core/extension seam** — the exact interface for language-specific
    policies (indentation, formatting, error/recovery tied to a grammar).
    ([Construction capabilities](#intended-construction-capabilities))
13. **`\r\n` post-sequence line/column** — no committed test. (Scenario 2.)
14. **Trailing incomplete UTF-8 at EOF** — no committed test. (Scenario 4.)
