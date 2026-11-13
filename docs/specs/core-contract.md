# textlexer core contract

This document is the canonical prose specification for the `textlexer` core:
what the engine does today, what it is intended to become, and where the design
is still open. It is paired with the [documentation entrypoint](../README.md)
and the [root README](../../README.md). Where this document and code disagree,
code wins and this document is wrong — file a change against this document
rather than against behavior.

## Status convention

Every substantive statement in this document carries one of four statuses.
Read the last three as a promise ladder, not as a to-do list. The first one
sits above the ladder and binds it.

- **Design principle (non-negotiable)** — a rule the owner has fixed for the
  design of the engine. It binds every engine change: a change that breaks a
  principle is rejected, and a principle is not traded for a feature. This
  status does **not** by itself promise that the current engine conforms. Each
  principle records its current conformance separately, either as Current
  contract with evidence, or as a **Known deviation**: current behavior that
  breaks the principle, with a code reference and its fix status (for example
  "not fixed yet"). A consumer must not rely on a Known deviation, because the
  principle requires it to change. A statement elsewhere in this document that
  disagrees with a principle is reconciled with it and marked **Reconciled with
  principle N**, or marked **Conflict with principle N** until the owner
  resolves it. See
  [VM design principles](#vm-design-principles).
- **Current contract** — the behavior exists in code at `master`. Consumers may
  rely on it. Citing a named test or source location is required, because this
  is the part of the document that is load-bearing. A cited test must execute
  the code path behind the sentence it supports. Where no committed test
  executes that path, the statement cites the source location and carries the
  marker **no test yet**. The statement is still Current contract, because the
  code at `master` implements it; the marker records the gap so that a later
  change adds the test. Replacing a "no test yet" marker with a test that
  executes the path does not change the statement.
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
**Reconciled with principles 1 and 4.** A lexical mode needs state across
tokens, and a post-match action is an action, so both live outside the VM, in a
consumer or builder layer above `Next()`. The VM itself stays as the
[VM design principles](#vm-design-principles) describe it.

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
  (can no longer match), or `StatePushBack` (exclude the top symbol of the
  token's stack, first the just-read symbol, from this match and show it to
  `next`; a chain of pushbacks excludes more; see
  [Rule states and pushback](#rule-states-and-pushback)). **Current contract**
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
  to rewind a token that is abandoned. The engine reuses a fixed pool of
  checkpoint slots (the context window plus the in-flight token) when the
  cursor can re-arm a released checkpoint, instead of allocating one per token.
  **Current contract** (`textlexer.go`).
- **Context window** — a fixed-size ring of the most recent committed lexemes
  (three today) whose retained input the engine keeps so `Context` can read the
  text around one of them. **Current contract** (`textlexer.go`, `markWindow`).
- **VM** — the rule engine: `rulesProcessor` (`processor.go`) together with the
  token loop in `TextLexer.nextLexeme` (`textlexer.go:428-549`) that feeds it
  symbols and commits its result. The cursor is not part of the VM. In this
  document "the engine" and "the VM" mean the same thing. **Current contract**
  (`processor.go`, `textlexer.go`).

## VM design principles

**Status: Design principle (non-negotiable).** The owner stated these five
principles on 2026-11-12. The stateless design is deliberate, not accidental.
Every change to the engine must keep all five. A change that would break one is
rejected; changing a principle is an owner decision, not part of an engine
change.

Each principle is followed by its current conformance at `master`: Current
contract statements with their evidence, and pointers to the
[Known deviations](#known-deviations), where the engine does not yet meet the
principle. This section only records deviations; it does not fix them.

### Principle 1: stateless stack model

**Design principle (non-negotiable).** The VM is based on a simple stack of the
symbols read for the token in progress. The VM keeps no state on a rule's
behalf. For one token it holds only:

- the stack: the symbols read for this token;
- the read position;
- the boundaries;
- per-rule bookkeeping: each rule's current `Rule`, its longest accepted
  length, and whether it is still active.

The VM resets all of it before the next token starts, so nothing carries over
from one token to the next. The VM never holds marks, modes, or any other state
on a rule's behalf, and it keeps no count for a rule across symbol steps. A
rule's own state is the `Rule` value it returns.

**Current contract — conformance.** The engine conforms.

- **The per-token holdings.** The stack is `rulesProcessor.buf`, which also
  receives the synthetic EOF symbol (`processor.go:95`). The per-rule
  bookkeeping is `ruleState.rule`, `ruleState.lastAccept`, and
  `ruleState.isActive` (`processor.go:7-12`). `TextLexer.runes` holds the same
  token's real symbols only, without the EOF symbol (`textlexer.go:459-461`); it
  is what the commit slices the winning text from. The read position is the
  cursor together with the in-flight checkpoint that marks the token start
  (`textlexer.go:437`). The boundaries are the length of the stack, the number
  of real symbols read, and the per-token byte count `tokenBytes`, a local of
  one `nextLexeme` call (`textlexer.go:440`, `463`).
- **Reset timing.** `Reset` (`processor.go:48-55`) runs from `getProcessor`
  (`textlexer.go:333-346`) when the next token starts, and the symbol buffer is
  truncated at the same point (`lx.runes[:0]`, `textlexer.go:439`). Between two
  tokens the processor still holds the last token's symbols and `Rule` values,
  but nothing reads them before the reset. So the accurate statement is "reset
  before the next token starts; nothing carries over", not "cleared when the
  token ends". `Reset` returns every rule to the `Rule` it was registered with.
  Evidence: `TestLexerLexemeSpans` (a multi-token stream in which every byte is
  covered by exactly one lexeme, in order; it would fail if one token's rule
  state carried into the next).
- **The per-step pushback count.** Inside one `processSymbol` call the VM
  counts the pushbacks of one rule's chain. `pushbackCount` is declared inside
  the per-rule loop (`processor.go:100`), so there is one count per rule per call,
  not one per call. The VM uses it to choose the popped symbol
  (`processor.go:111`), to bound the chain (`processor.go:110`), and to compute the
  match length (`processor.go:124`). The count ends with that rule's turn. So the
  VM does count within one symbol step, and it keeps no count across symbol
  steps.
- **The stack is a model.** A pushback does not remove anything from
  `rp.buf`. It reads `rp.buf[len(rp.buf)-pushbackCount]` (`processor.go:108-116`).
  So one rule's pushback cannot change what any other rule is shown, and the
  stack is the same for every rule within a step. Source: `processor.go:108-116`;
  `TestPushBackIsPerRule` for the isolation between two rules.
- **Fields that persist across tokens.** These fields outlive a token. None of
  them holds state on a rule's behalf, and no rule can read any of them, so they
  do not break the principle:
  - `ruleState.initialRule` and `rulesProcessor.rules`: registration data, the
    `Rule` each type was registered with and the registration order.
  - `rulesProcessor.states`: the map that holds each rule's `ruleState`. The
    map itself persists; the bookkeeping in each entry is reset as described
    above.
  - `TextLexer.rules` and `TextLexer.rulesMap`: the registration table.
  - `TextLexer.processor`: a cache of the processor, rebuilt when a rule is
    added (`textlexer.go:251`); its per-token contents are reset as described
    above.
  - `TextLexer.cursor`: the stream position, owned by `textreader`.
  - `TextLexer.marks` and `TextLexer.marksNext`: the context-window ring of
    checkpoints for committed lexemes. They mark source positions for
    `Context`, not rule state. The context window is not a deviation.
  - `TextLexer.canRemark` (a cursor capability) and `TextLexer.maxTokenBytes`
    (a configured boundary).
  - `TextLexer.mu` and `TextLexer.rulesMu`: the locks.
  - `TextLexer.emitted`: a count of committed runes, written at
    `textlexer.go:544` and never read.
- **State a rule keeps itself.** A rule is an ordinary Go function, and it may
  keep its own state in the closures of the `Rule` values it returns. That
  state is the rule's, not the VM's: the VM neither holds it nor resets it, and
  [returned-error rollback](#returned-error-rollback) does not undo it.

### Principle 2: the VM checks boundaries

**Design principle (non-negotiable).** Boundary checks are the VM's
responsibility, not the rule's. The VM validates every verdict a rule returns
against what it holds, including:

- a pushback past the bottom of the stack;
- a nil `next` where the verdict needs one;
- a match longer than the symbols read;
- a zero-length match;
- a `State` value outside the four defined constants.

`WithMaxTokenBytes` is unbounded by default (`textlexer.go:91-92`), so a rule
that never settles makes the VM retain input until the end of input.

**Current contract — conformance.** The engine conforms. The VM detects every
case in the list. Each reaction to a bad verdict retires only the rule that
returned it, or refuses only the bad match, so principle 3 holds for these cases
as well. The per-token byte bound below is a configured limit, not a verdict,
and fails only the token that exceeds it.

- **Pushback past the bottom of the stack.** The chain stops and the rule is
  retired as a reject (`processor.go:110`, `113-115`). Evidence:
  `TestLexerProcessor` "Pushback Infinite Loop" (a rule that always pushes back
  with a non-nil `next` yields one `UNKNOWN` lexeme per rune) and "Excessive
  Pushback (Index Out of Bounds)".
- **Nil `next` with `StatePushBack`.** Same branch: the rule is retired as a
  reject (`processor.go:110`, `113-115`). Evidence: `TestLexerPathologicalRules`
  "Rule Returns nil with StatePushBack".
- **Nil `next` with `StateContinue`.** The rule is retired
  (`processor.go:130-131`, `140-142`). Evidence: `TestLexerProcessor` "Rule
  returns nil, StateContinue". A nil `next` with `StateAccept` is not a boundary
  violation: it records the acceptance and stops the rule (`processor.go:120-129`,
  `141`).
- **A match longer than the symbols read.** Before the EOF symbol the stack
  holds only real symbols, so only an accept of the synthetic EOF symbol itself
  can be longer than the symbols read. The VM discards that verdict and retires
  the rule; an accept that the rule recorded earlier stays
  (`processor.go:120-129`, `137-142`). An accept whose pushback chain excludes
  the EOF symbol is valid. Evidence: `TestAcceptOfEOFSymbolIsDiscarded`;
  `TestProcessDecidesOnEOFSymbol`. `nextLexeme` keeps a defensive length check
  that this makes unreachable (`textlexer.go:500-506`). See
  [KD-3](#known-deviations).
- **A zero-length match.** The VM commits no empty token. It emits a one-rune
  `UNKNOWN` lexeme instead (`textlexer.go:482-485`). Evidence:
  `TestProcessZeroLengthMatch`.
- **A `State` outside the four constants.** The VM retires the rule as
  `StateReject` does, whether the value is returned directly or at the end of a
  pushback chain (`processor.go:132-135`, `140-142`). It does not panic.
  Evidence: `TestInvalidStateRetiresRule`; `TestProcessDecidesOnEOFSymbol`. See
  [KD-1](#known-deviations).
- **A rule still active at the end of input.** No input follows the EOF symbol,
  so after it the VM retires every rule that is still active
  (`processor.go:137-142`), and `Process` always decides. The winner is chosen
  from the recorded accepts; with none, the token is one `UNKNOWN` rune.
  Evidence: `TestStillActiveAtEOFRetiresRule`; `TestLexerProcessor` "Rule Never
  Settles (UNKNOWN per rune)"; `TestProcessDecidesOnEOFSymbol`. `nextLexeme`
  keeps a defensive check for an undecided processor after the EOF symbol that
  this makes unreachable (`textlexer.go:471-478`). See
  [KD-4](#known-deviations).
- **The per-token byte bound.** When `WithMaxTokenBytes` is set, a token that
  would exceed it fails with an error wrapping `textreader.ErrRetentionExceeded`
  (`textlexer.go:463-465`). Evidence: `TestLexerMaxTokenBytes`.

### Principle 3: rules are free

**Design principle (non-negotiable).** A rule may do anything, including return
bad values. The VM reacts only to what it can do within its boundaries. A bad
value from one rule must not corrupt the VM, the stack, or another rule.

**Current contract — conformance.** The engine conforms.

- **Conforms.** A bad pushback or a nil `next` retires only the rule that
  returned it, and the other rules continue on the same stack
  (`processor.go:110-115`, `130-131`, `140-142`). Evidence:
  `TestLexerPathologicalRules` "Rule Returns nil with StatePushBack" (the bad
  rule is retired, and a sibling rule still wins `"a"`). A rule that never
  advances is contained by the pushback bound and the zero-length check under
  principle 2.
- **Conforms.** An invalid `State`, an accept of the synthetic EOF symbol, and a
  rule still active at the end of input each retire only that rule. Its
  earlier accept stays, and a sibling rule's valid match is emitted. Evidence:
  `TestInvalidStateRetiresRule`, `TestAcceptOfEOFSymbolIsDiscarded`, and
  `TestStillActiveAtEOFRetiresRule`; see [KD-1, KD-3, and
  KD-4](#known-deviations).
- **Conforms.** A rule that panics, on a direct call or as the next rule of a
  pushback chain, is retired for the current token as `StateReject` does. Its
  earlier accept stays, a sibling rule's valid match is emitted, and lexing
  continues. The panic does not reach the caller (`processor.go:107`, `112`,
  `173-185`). Evidence: `TestPanicRetiresRule`, `TestPanicKeepsEarlierAccept`,
  and `TestPanicLeavesLexerUsable`; see [KD-2](#known-deviations) and
  [Panics](#panics).

### Principle 4: the VM only reports matches

**Design principle (non-negotiable).** The VM emits which rule matched and what
it matched: the type, the text, and the span. It performs no action and
interprets nothing. A consumer acts on that information.

**Current contract — conformance.** The engine conforms.

- `Next()` returns a lexeme built from the winning type, the matched runes, and
  the committed span (`textlexer.go:547`). The engine runs no hook and decodes
  nothing; see
  [Matched content](#matched-content-versus-transformed-or-decoded-values).
- **A rule's panic is not reported.** The VM retires the rule and discards the
  panic value; `Next()` returns the lexeme chosen from the recorded accepts and
  no error (`processor.go:173-185`). Evidence: `TestLexerErrorConditions` "Rule
  Panic Types Are Recovered"; `TestCallRule`. See [Panics](#panics).
- **`UNKNOWN` when no rule matches** (`processor.go:69-71`). Verdict: conforms.
  `UNKNOWN` is the VM's report that no rule matched one symbol; it is not an
  action and not an interpretation of the input. Evidence: `TestLexerProcessor`
  "No Matching Rules (produces UNKNOWN)".
- **`UNKNOWN` in place of the winning rule's type on a zero-length match**
  (`textlexer.go:484`). Verdict: conforms. This is the principle-2 boundary
  reaction: the VM refuses the zero-length match, so no rule matched a
  non-empty text, and the report says so. Evidence:
  `TestProcessZeroLengthMatch`.

### Principle 5: lookahead belongs to the rule

**Design principle (non-negotiable).** A rule decides how far to read and how
much to give back with `StatePushBack`. The VM applies the stack operation and
the boundary check.

**Current contract — conformance.** The engine conforms.

- The VM reads the next symbol while any rule is still active
  (`processor.go:61-64`), so the rules decide how far a token reads. The VM has
  no lookahead mechanism of its own.
- Each `StatePushBack` pops one more symbol from the model stack and shows it to
  the returned `next`: the current symbol first, then earlier symbols in reverse
  order (`processor.go:108-116`). The boundary check is principle 2's. Evidence:
  `TestLexerProcessor` "Context-Aware Rules (BOL)" (the comment rule pushes back
  its terminator, so the terminator is not part of the comment).
- A pushback chain that ends in `StateContinue` or in a non-nil `next` with
  `StateAccept` is not remembered after the step; see
  [KD-5](#known-deviations).

### Known deviations

**Status: Known deviation** for each entry marked "deviates"; **Current
contract** for each entry marked "conforms". This list audits `master` against
the five principles. It was first written against v0.2.3, where KD-1, KD-2,
KD-3, and KD-4 deviated. Release v0.2.4 fixed them, and their entries now
record the conforming behavior.
Probe results are from scratch programs run outside the repository.

- **KD-1. A `State` outside the four constants.** Conforms to principles 2
  and 3. `processSymbol` retires the rule as `StateReject` does, whether the
  value is returned directly or at the end of a pushback chain
  (`processor.go:132-135`, `140-142`). The VM does not panic. An accept that the
  rule recorded earlier stays, and the other rules continue on the same stack.
  Evidence: `TestInvalidStateRetiresRule` (with a sibling, `ab` yields
  `WORD "ab"`; as the only rule, one `UNKNOWN` per rune; both for a direct
  return and for the end of a pushback chain); `TestProcessDecidesOnEOFSymbol`.
  In v0.2.3 the VM panicked here, with the consequences that KD-2 records for
  a rule's own panic in v0.2.3.
- **KD-2. A rule that panics.** Conforms to principles 3 and 4. The VM calls
  every rule through `callRule` (`processor.go:107`, `112`, `173-185`), which
  recovers a panic raised during that one call and returns `(nil,
  StateReject)`. So a rule that panics, directly or as the next rule of a
  pushback chain, is retired for the current token as `StateReject` does. An
  accept that the rule recorded earlier stays, the other rules continue on the
  same stack, and the winner is chosen from the recorded accepts as usual. A
  panic with any value is recovered, `panic(nil)` included: `callRule` detects
  the panic with a completion flag, not with the result of `recover`, so this
  holds under `GODEBUG=panicnil=1` as well. The panic does not reach the
  caller, `Next()` returns no error for it, and the VM neither keeps nor
  reports the panic value. The recovery covers only the rule call, not the VM's
  own code. Evidence: `TestPanicRetiresRule` (with a sibling, `ab` yields
  `WORD "ab"`; as the only rule, one `UNKNOWN` per rune; both for a direct
  panic and for a panic in a pushback chain); `TestPanicKeepsEarlierAccept`
  (`aba` with a rule that accepts `a` and then panics: `A "a"`, `UNKNOWN "b"`,
  `A "a"`); `TestLexerPanickingRuleIsRecoveredAndRetired`;
  `TestLexerPanicRetiresRuleAndLexingContinues`; `TestLexerErrorConditions`
  "Rule Panic Types Are Recovered" (string, error, integer, and nil values);
  `TestCallRule` (the same values, also under `GODEBUG=panicnil=1`);
  `TestPanickingRuleConcurrentDeterminism`. The former probe, inverted:
  `TestPanicLeavesLexerUsable` (input `abcd ` followed by 40 copies of `ab `,
  `WithMaxTokenBytes(8)`, and a word rule that panics once on its third
  symbol: every later `Next()` succeeds, and every input byte appears in
  exactly one lexeme, starting with `WORD "ab"` and `WORD "cd"`). In v0.2.3
  the panic propagated out of `Next()` without running `abandon`, so a caller
  that recovered found the symbols read before the panic lost and the in-flight
  checkpoint orphaned on the next `Next()`.
- **KD-3. A rule that accepts the synthetic EOF symbol while a sibling rule has
  a valid match.** Conforms to principles 2 and 3. Before the EOF symbol the
  stack holds only real symbols, so an accept with no pushback on the EOF symbol
  is the only verdict whose length counts more than the symbols read.
  `processSymbol` discards that verdict and retires the rule
  (`processor.go:120-129`, `137-142`). An accept that the rule recorded earlier
  stays, and the winner is chosen from the recorded accepts as usual. An accept
  whose pushback chain excludes the EOF symbol is valid and recorded. Evidence:
  `TestAcceptOfEOFSymbolIsDiscarded` ("sibling": a letters `WORD` rule and a
  sibling that accepts the EOF symbol, on `ab`, yield `WORD "ab"` on the first
  call; "earlier accept by the same rule": the rule's accept of `ab` wins; the
  two "pushback excludes the EOF symbol" cases guard the valid path);
  `TestProcessDecidesOnEOFSymbol`. In v0.2.3 the token failed for every rule,
  on every retry.
- **KD-4. A rule still active at the end of input while a sibling rule has a
  valid match.** Conforms to principles 2 and 3. No input follows the EOF
  symbol, so after it `processSymbol` retires every rule that is still active,
  whether through `StateContinue` or a non-nil `next` with `StateAccept`
  (`processor.go:137-142`). `Process` therefore always decides on the EOF
  symbol, and the winner is chosen from the recorded accepts. With no recorded
  accept the token is one single-rune `UNKNOWN` lexeme, or `io.EOF` when no
  real symbol was read. Evidence: `TestStillActiveAtEOFRetiresRule` (a sibling
  that always continues, or always returns itself with `StateAccept`, on `ab`:
  `WORD "ab"`); `TestLexerProcessor` "Rule Never Settles (UNKNOWN per rune)"
  (the only rule never settles: one `UNKNOWN` per rune, then `io.EOF`);
  `TestProcessDecidesOnEOFSymbol`. A rule that never settles costs one read to
  the end of input per emitted lexeme; see
  [Unmatched input, EOF, and errors](#unmatched-input-eof-and-errors). In
  v0.2.3 the token failed for every rule, on every retry.
- **KD-5. A pushback chain that ends in `StateContinue` or in a non-nil `next`
  with `StateAccept`.** Conforms to principles 1 and 5; the ending is
  unsupported. The VM forgets the pushback count at the end of the step
  (`processor.go:100`), so the symbols pushed back are shown to the rule again
  and later match lengths do not account for them. Probe: on `abcX`, a rule that
  pushes back `c` and then continues sees `c` twice and yields the lexeme
  `abcX`. Remembering the count across steps would itself break principle 1.
  Evidence: `TestPushBackChainEndingContinue` and
  `TestPushBackChainEndingAcceptWithNext`.
- **KD-6. `(nil, StateContinue)`.** Conforms to principles 2 and 3. The VM
  retires the rule (`processor.go:130-131`, `140-142`). Evidence:
  `TestLexerProcessor` "Rule returns nil, StateContinue".
- **KD-7. The fields of `rulesProcessor`, `ruleState`, and `TextLexer`.**
  Conform to principle 1. Each field is either per-token stack or bookkeeping,
  which `Reset` restores before the next token, or a field that persists across
  tokens and holds nothing on a rule's behalf. See
  [principle 1](#principle-1-stateless-stack-model).
- **KD-8. The two places where the VM chooses a type itself.** Conform to
  principle 4: `UNKNOWN` when no rule matches (`processor.go:69-71`) and
  `UNKNOWN` on a zero-length match (`textlexer.go:484`). See
  [principle 4](#principle-4-the-vm-only-reports-matches).

## Lexer/consumer boundary

**Current contract.** The lexer owns lexical policy and nothing else: rule state
transitions, competing-rule evaluation, maximal-munch selection,
registration-order tie breaking, the `UNKNOWN` fallback, and lexeme
construction. It owns **no** text-stream mechanics. **Reconciled with
principle 1.** "Rule state transitions" means that the VM applies the
transitions a rule returns. The state itself is the rule's: it is the `Rule`
value the rule returns, and the VM holds it only as per-token bookkeeping.

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
lexeme carries matched content only. **Reconciled with principle 4.** The VM
performs no action, so such a hook would live in a consumer or builder layer
above `Next()` and act on the lexeme the VM reports. The open question is the
shape of that layer, not whether the VM runs actions.

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

**Mark ownership.** The engine keeps a fixed pool of checkpoint slots — one per
lexeme the window retains, plus the token currently being assembled — rather
than allocating a fresh checkpoint for every token. When the cursor can re-arm
a released checkpoint (the `Remark` capability shipped in `textreader`), a
committed token's slot is released and the next token re-arms that same slot in
place, so the number of live checkpoints is bounded by the window and
steady-state tokenization allocates no checkpoint. A failed `Next` and the EOF
probe reset and release only their own in-flight slot, so earlier committed
context is left intact and a mark is never re-armed while it is still active. A
cursor that does not expose the capability (see `NewWithCursor`) keeps the
older behavior of allocating a checkpoint per token. The window's width,
eviction order, and every behavioral promise above are unchanged by the reuse.

**Evidence.** `TestLexerContext` (a lexeme stays readable across one more token,
then falls out and returns `ErrPositionOutOfBuffer`);
`TestLexerContextReachesPrecedingLexemes` (`before` reaches back across preceding
lexemes and clamps rather than errors when asked for more than is retained);
`TestLexerContextSurvivesFailedNext` and `TestLexerContextSurvivesEOFProbe`
(a non-committing call releases nothing);
`TestLexerMarkReuseLongSequence` (a long stream lexes under a tight finite
retention budget, so no mark is leaked and registrations stay bounded);
`TestLexerMarkEvictionExact` (exactly the three most recent lexemes remain
readable, every earlier one is evicted, and the retained region is the window);
`TestLexerMarkReuseAfterFailures` (repeated failed `Next` calls commit nothing
and the prior context survives, then the stream lexes on);
`TestLexerMarkReuseAfterRelease` (`Release` drops the window and later tokens
re-arm it); `TestLexerMarkFallbackWithoutRemark` (an injected cursor without the
capability lexes identically but still allocates a checkpoint per token).

### Unmatched input, EOF, and errors

**Current contract.**

- **Unmatched input** is never silently dropped. If no rule accepts a symbol, the
  engine emits a **single-rune `UNKNOWN` lexeme** for that symbol and advances by
  exactly one rune. Tokenization always makes progress: every successful `Next()`
  consumes at least one rune, and the `UNKNOWN` fallback guarantees that.
- **EOF** is returned as `io.EOF` from `Next()` once the stream is fully consumed
  and no further lexeme can be produced. A rule that still needs input at EOF
  (an unterminated construct) is retired after the EOF symbol, like every rule
  that is still active then; only the accepts it recorded earlier count. If no
  rule recorded an accept, the token is unmatched input: a single-rune `UNKNOWN`
  lexeme, and `io.EOF` once no real symbol is left. See
  [EOF handling](#eof-handling).
- **Cost of a rule that never settles.** Such a rule keeps each token open to
  the end of input, so every lexeme it leaves unmatched costs one read to the
  end of input: lexing n runes costs O(n²) symbol steps. The token start is
  marked, so the cursor retains every byte from that start; under the default
  unbounded `WithMaxTokenBytes` all input is retained. `WithMaxTokenBytes`
  bounds both: a token that grows past the bound fails with an error wrapping
  `textreader.ErrRetentionExceeded` (see
  [Speculative lookahead limits](#speculative-lookahead-limits)).
- **Errors other than EOF** indicate a problem with the underlying reader or an
  unrecoverable engine state (for example a token larger than the configured
  retention bound). On a returned error, `Next` has made **no partial progress**:
  the cursor is rewound to the token start, so a retry starts over rather than
  resuming mid-token. See
  [Returned-error rollback](#returned-error-rollback).

**Evidence.** `TestLexerProcessor` "No Matching Rules (produces UNKNOWN)" (each
unmatched rune is its own `UNKNOWN` lexeme); `TestLexerEdgeCasesAtEOF` (a
settled rule yields `io.EOF`; empty input yields `io.EOF` immediately);
`TestLexerProcessor` "Rule Never Settles (UNKNOWN per rune)" (a rule that is
still active at EOF and never accepts yields one `UNKNOWN` rune per input rune,
then `io.EOF`); `TestStillActiveAtEOFRetiresRule` (a still-active sibling does
not stop another rule's match);
`TestLexerZeroLengthMatchPrevention` (a rule that never advances still yields one
`UNKNOWN` rune per input rune, never an infinite loop; it reaches the
zero-length branch at `textlexer.go:482-485`);
`TestLexerRetentionFailureConsumesNothing` (a rejected token consumes nothing and
a retry is identical).

**Open design question.** Whether the `UNKNOWN` fallback should be
configurable or replaceable (for example, a consumer-supplied "on no match"
handler that can emit a different token or raise) is undecided. Today it is
fixed to one-rune `UNKNOWN`. **Reconciled with principle 4.** The VM keeps
reporting `UNKNOWN` when no rule matches. A handler that emits a different token
or raises acts on that report, so it would live in a layer above `Next()`, not
in the VM.

## Execution contract

This is how the engine decides, from code and tests. Each item is load-bearing.

### Competing rules, evaluated sequentially

**Current contract.** A rule is a state machine; the engine runs **all** active
rules over the same input, one symbol at a time. "Parallel" means *competing
state machines*, not concurrency: within a single `Next()`, the rules are stepped
**sequentially in registration order** for each symbol. The engine tracks each
rule's furthest acceptance separately, as per-rule bookkeeping
([principle 1](#principle-1-stateless-stack-model)). The engine reads ahead
until every rule has either rejected or terminated, then selects the winner.

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

- `StateContinue` — possible match, needs more input; a rule that wants more
  input returns a non-nil `next`. **Reconciled with principle 3.** This is not a
  duty the VM depends on: the VM retires a rule that returns
  `(nil, StateContinue)` (`processor.go:130-131`, `140-142`; evidence:
  `TestLexerProcessor` "Rule returns nil, StateContinue").
- `StateAccept` — a valid match so far; the rule may return a non-nil `next` to
  keep scanning for a longer match, or nil to stop. An accept of the synthetic
  EOF symbol itself is discarded; see [EOF handling](#eof-handling).
- `StateReject` — this rule can no longer match; it is deactivated for the
  current token.
- `StatePushBack` — exclude the top symbol of the token's stack from this
  match and show it to the returned `next` rule, in the same step. This is the
  mechanism for lookahead and backtracking. The mechanics follow.

Any other `State` value, returned directly or at the end of a pushback chain,
retires the rule as `StateReject` does (`processor.go:132-135`, `140-142`;
evidence: `TestInvalidStateRetiresRule`). This is the boundary check of
[principle 2](#principle-2-the-vm-checks-boundaries); see
[KD-1](#known-deviations).

**Current contract — pushback mechanics** (`processor.go:78-146`).

- **The stack.** The VM keeps the symbols read for the current token as a
  stack, `rulesProcessor.buf`. At end of input the synthetic EOF symbol is
  pushed on that stack too (`processor.go:95`).
- **One pushback.** `(next, StatePushBack)` removes the top symbol of the stack
  from this rule's match and calls `next` with the removed symbol, in the same
  step (`processor.go:108-112`). The first pushback removes the symbol that was
  just read. At end of input, the first pushback removes the EOF symbol.
  Evidence: `TestLookaheadKeywordBoundary` (one pushback excludes the symbol
  after `for`, or the EOF symbol); `TestLexerProcessor` "Context-Aware Rules
  (BOL)" (the comment rule pushes back its terminator so it is not included in
  the comment).
- **Chained pushback.** `next` then decides:
  - `(nil, StateAccept)`: the match is every symbol below the removed ones
    (`processor.go:120-128`), subject to the only-grows rule below.
  - `StatePushBack` again: remove one more symbol, toward the token start, and
    call the new `next` with that symbol.
  - `StateReject`: the rule is retired (`processor.go:132-135`, `141`). An
    accept that the rule recorded earlier stays.

  Evidence: `TestPushBackSymbolOrder` (a chain on `abcd` is shown `d`, `c`,
  `b`, in that order); `TestLookaheadTwoCandidateEnds` (after `abcd` and any
  number of `.`, one rule yields `ab` on `X` and `abcd` on `Y`, so it chooses
  between two ends after it has seen what follows); `TestLexerPathologicalRules`
  "Rule Rejects After Full Buffer Consumption (Veto Rule)" (a chain that ends
  in the rule's own `StateReject` keeps its earlier accept).
- **Only-grows rule.** Within a token, a rule's recorded match only grows. An
  accept at the end of a pushback chain that is shorter than an accept the
  same rule already recorded is ignored (`processor.go:124-127`). Pushback
  therefore cannot withdraw or shorten a match that the rule already accepted.
  A rule that decides by lookahead must return `StateContinue`, not
  `StateAccept`, until it has decided. Evidence: `TestLexerPathologicalRules`
  "Bait and Switch Rule" (the chain's accept of length 1 is ignored, and the
  rule's earlier accept of `token` stays);
  `TestLexerPathologicalRuleWithDeepBacktracking` (the same for chains of up to
  101 pushbacks); `TestLookaheadFloatVersusRange` (`newUnsignedFloatRule`
  continues until it reads a symbol that ends the float, so `1.` is not a float
  in `1..5`).
- **Count rule.** A rule that excludes `n` trailing symbols returns `n`
  pushbacks and then an accept: the match length is the stack length minus the
  chain's pushback count (`processor.go:124`). A helper that itself issues `k`
  pushbacks, entered through one more `StatePushBack`, excludes `k + 1`
  symbols. Evidence: `TestLookaheadMultiSymbol` (four pushbacks exclude `://`
  and a letter, which leaves `http`); `TestLookaheadVariableLength` (the rule
  scans a whole page name and carries its count in the `Rule` values it
  returns, so the count depends on the input); `TestPushBackSymbolOrder` (a
  helper that issues two pushbacks, entered through one more, excludes three
  symbols and leaves `a`).
- **Bounds rule.** More pushbacks than symbols on the stack, the EOF symbol
  included, or a nil `next`, retire the rule as a reject (`processor.go:110`,
  `113-115`). This is the boundary check of
  [principle 2](#principle-2-the-vm-checks-boundaries). An accept that the rule
  recorded earlier stays. On input `a`, the stack at end of input holds `a` and
  the EOF symbol: one pushback matches `a`, two are a zero-length match (see
  [Progress and zero-length behavior](#progress-and-zero-length-behavior)), and
  three retire the rule. Evidence: `TestProcessPushBackAtEOF` (`Process`
  returns the rule's type with length 1, then the rule's type with length 0,
  then `UNKNOWN` with length 1); `TestPushBackAtEOF` (the same three cases
  through `Next`); `TestPushBackRetiresRuleKeepsEarlierAccept` (both reject
  causes keep the earlier accept of `a`); `TestLexerMaximumLimits` "Maximum
  pushback depth" (a chain of 1001 pushbacks runs to its accept, and one more
  pushback than the stack holds retires the rule); `TestLexerProcessor`
  "Pushback Infinite Loop" and "Excessive Pushback (Index Out of Bounds)";
  `TestLexerPathologicalRules` "Rule Returns nil with StatePushBack".
- **Per rule.** Each rule's pushback is its own. A pushback only reads the
  stack (`processor.go:111`), so other rules are not affected; see
  [principle 1](#principle-1-stateless-stack-model). Evidence:
  `TestPushBackIsPerRule`.
- **Per step.** The exclusion count lasts for the current symbol step only.
  Within a step the VM counts the pushbacks of the chain (`processor.go:100`);
  it keeps no count across steps.

**Chain endings.** A pushback chain must end in `(nil, StateAccept)` or
`StateReject`. Because the exclusion count is dropped at the end of the step, a
chain that keeps the rule active gives a result that later symbols overwrite.
These two endings are unsupported ([KD-5](#known-deviations)):

- A chain that ends in `StateContinue`: the pushback is forgotten. Evidence:
  `TestPushBackChainEndingContinue` (a rule that reads `ab`, pushes back `c`,
  and continues lexes `abcde` as one token).
- A chain that ends in `(non-nil next, StateAccept)`: the shorter length is
  recorded, the rule stays active, and a later accept counts the excluded
  symbols back in (`processor.go:120-129`, `137-139`). Evidence:
  `TestPushBackChainEndingAcceptWithNext` (`aXbc` lexes as one token; the same
  chain with a nil `next` matches `a`).

Both pins push back on a real symbol. On the EOF symbol no later symbol can
overwrite the result, because the VM retires every rule after the EOF symbol: a
chain that excludes the EOF symbol and ends in `(non-nil next, StateAccept)`
records its accept as `(nil, StateAccept)` would, and a chain that ends in
`StateContinue` records nothing. Evidence: `TestAcceptOfEOFSymbolIsDiscarded`
"pushback excludes the EOF symbol, non-nil next". See
[EOF handling](#eof-handling).

### Progress and zero-length behavior

**Current contract.** The engine guarantees forward progress: a token is never
committed with zero length. If a rule's net effect would be a zero-length match
(a pushback that consumes nothing, or an accept of an empty sequence), the engine
does **not** emit an empty token and does **not** loop; it falls back to a
single-rune `UNKNOWN` lexeme and advances by one rune. This makes a
pathologically non-advancing rule terminate rather than spin.

**Evidence.** The zero-length branch in `nextLexeme` (`textlexer.go:482-485`).
`TestProcessZeroLengthMatch` proves the path: one pushback to an acceptor on
the first symbol makes `Process` return the rule's type with length 0, while
two pushbacks on the first symbol pass the bottom of the stack and return
`UNKNOWN` with length 1. Through `Next` both give the same single-rune
`UNKNOWN` lexeme. `TestLexerZeroLengthMatchPrevention` and the
`TestLexerProcessor` case "Zero-Length Match Causes Infinite Loop" use the
one-pushback rule and assert one `UNKNOWN` rune per input rune; the
`TestLexerInfiniteLoopProtection` case "Zero Length Accept Loop" uses it and
asserts that lexing terminates. This is the zero-length check of
[principle 2](#principle-2-the-vm-checks-boundaries).

### EOF handling

**Current contract.** The engine feeds a **synthetic EOF symbol** (carrying
`RuneEOF` and `FlagEOF`, not part of the input) to the rules once the underlying
reader is exhausted. This lets a rule observe end-of-input. No input follows
the EOF symbol, so after it the VM retires every rule that is still active —
through `StateContinue`, or through a non-nil `next` with `StateAccept`
(`processor.go:137-142`). `Process` therefore always decides on the EOF symbol.
Each rule contributes the longest accept it recorded, and the winner is chosen
as usual. If no rule recorded an accept, the token is unmatched input: one
single-rune `UNKNOWN` lexeme. If no real symbol was consumed and the stream is
done, `Next` returns `io.EOF`. A rule that never settles therefore yields one
`UNKNOWN` lexeme per rune, and each of those tokens reads to the end of input
first; see the cost under
[Unmatched input, EOF, and errors](#unmatched-input-eof-and-errors).

**Reactions to the EOF symbol.** A rule that wants its match to survive end of
input rejects the EOF symbol (or pushes it back) and does not accept it. The EOF
symbol is on the token's stack, so a pushback at end of input removes it first,
and it counts toward the pushback bound; see
[Rule states and pushback](#rule-states-and-pushback) (evidence:
`TestPushBackAtEOF`, `TestProcessPushBackAtEOF`, and `TestLexerProcessor`
"Pushback at EOF"). A rule
that returns `StateAccept` for the EOF symbol with no pushback produces a match
whose length counts the synthetic symbol, which is longer than the symbols
read. The VM discards that verdict and retires the rule; an accept the rule
recorded earlier stays (`processor.go:120-129`, `137-142`). An accept at the end
of a pushback chain that excludes the EOF symbol is valid and recorded.
**Reconciled with principle 3.** These are not duties the VM depends on: a rule
is free, and the VM checks the result (principle 2). Each reaction retires only
the offending rule, so another rule's valid match is still emitted; see
[KD-3 and KD-4](#known-deviations).

**Evidence.** `TestLexerEdgeCasesAtEOF` (a settled rule at EOF → `io.EOF`; a rule
in a different state at EOF → the longer real acceptance wins; empty input →
`io.EOF` immediately); `TestLexerProcessor` "Tokens immediately at EOF";
`TestStillActiveAtEOFRetiresRule` (a sibling still active at EOF does not stop
`WORD "ab"`); `TestLexerProcessor` "Rule Never Settles (UNKNOWN per rune)" (the
only rule never settles: one `UNKNOWN` per rune, then `io.EOF`);
`TestAcceptOfEOFSymbolIsDiscarded` (an accept of the EOF symbol is discarded, an
earlier accept stays, and a pushback that excludes the EOF symbol accepts the
real symbols); `TestProcessDecidesOnEOFSymbol` (`Process` decides on the EOF
symbol in each case). The `textLen < 0 && isEOF` branch (`textlexer.go:471-478`)
and the `textLen > len(symbols)` branch (`textlexer.go:500-506`) in
`nextLexeme` stay as defensive checks; the reactions above make them
unreachable.

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

**Current contract — rule panics.** A rule's panic is not an error and does
not take this path. The VM recovers the panic within the rule's call and
retires that rule for the current token, as `StateReject` does; the token is
then decided from the recorded accepts and committed as usual, so `Next`
returns no error for it. See [Panics](#panics). **Reconciled with
principle 3.** The owner chose recovery and retirement on 2026-11-12, which
closes open design question 7. An invalid `State` does not panic: the VM
retires the rule ([KD-1](#known-deviations)).

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
deterministic stream); `TestPanickingRuleConcurrentDeterminism` (the same
with a rule that panics in every identifier token);
`TestLexerConcurrentAddRuleAndNext` (concurrent `AddRule`+`Next` is
memory-race-clean, run under `-race`);
`TestLexerReleaseHandsOffSharedCursor` (after `Release`, another consumer reads
the remainder through the same finite-retention cursor).

### Panics

**Current contract.** A panic raised by a rule does not reach the caller. The
VM calls each rule through `callRule` (`processor.go:107`, `112`, `173-185`),
which wraps only that one call in a deferred `recover`:

- A rule that panics while it processes a symbol, on a direct call or as the
  next rule of a pushback chain, is retired for the current token as
  `StateReject` does. An accept that the rule recorded earlier in the token
  stays.
- The other rules in the same step and in later steps are evaluated normally,
  and the winner is chosen from the recorded accepts as usual. Lexing
  continues with the next token, in which every rule, the one that panicked
  included, starts again from the `Rule` it was registered with.
- A panic with any value is recovered, a `nil` value included. On Go 1.21 and
  later, `panic(nil)` raises a `*runtime.PanicNilError`; with
  `GODEBUG=panicnil=1`, `recover` returns `nil` for it. `callRule` detects the
  panic with a completion flag, so both cases retire the rule.
- `Next` returns no error and does not panic because of a rule's panic. The
  returned lexeme is the one chosen from the recorded accepts, so retiring a
  rule can change which lexeme wins, for example to `UNKNOWN` when no rule
  remains.
- The VM keeps no state for a panic and does not report the panic value
  (principles 1 and 4). Retiring the rule uses the existing `isActive`
  bookkeeping.

The recovery covers only the rule call (`processor.go:173-185`). A panic raised
by the engine itself, outside a rule, is out of its scope and is not
recovered. Side effects that a rule performed before it panicked are not undone
(see [Custom-rule responsibilities](#custom-rule-responsibilities)).

**Reconciled with principle 3.** The owner chose this behavior on 2026-11-12. It
replaces the earlier contract, under which the panic propagated out of `Next`
and a consumer had to `recover` around its own call; that consumer no longer
receives the panic as a signal. See [KD-2](#known-deviations).

**Evidence.** `TestLexerPanickingRuleIsRecoveredAndRetired` (the only rule
panics on every symbol: one `UNKNOWN` per rune, then `io.EOF`);
`TestLexerPanicRetiresRuleAndLexingContinues` (a rule panics once on the first
token; a sibling's match wins, and later tokens lex normally);
`TestPanicRetiresRule` (a sibling's match survives a direct panic and a panic
in a pushback chain); `TestPanicKeepsEarlierAccept` (an earlier accept stands);
`TestLexerErrorConditions` "Rule Panic Types Are Recovered" (string, error,
integer, and nil values); `TestCallRule` (the same values, also under
`GODEBUG=panicnil=1`); `TestPanicLeavesLexerUsable` (no later `Next` fails under
`WithMaxTokenBytes(8)`, and every input byte appears in exactly one lexeme);
`TestPanickingRuleConcurrentDeterminism` (concurrent `Next`, run with `-race`).

### Custom-rule responsibilities

**Current contract.** The engine is a general automaton runner.
**Reconciled with principles 2 and 3.** The VM validates every verdict a rule
returns (principle 2), and a rule is free to return anything (principle 3). So
the items below are not duties the VM depends on. They describe how to write a
rule that gets its intended match, and what the VM does today when a rule does
otherwise. Where today's reaction breaks a principle, the item names the Known
deviation.

- **Handle EOF.** To keep its match at end of input, a rule rejects (or pushes
  back) the EOF symbol and does not accept it. A pushback at end of input
  removes the EOF symbol first, and the EOF symbol counts toward the pushback
  bound ([Rule states and pushback](#rule-states-and-pushback)). An accept that
  counts the EOF symbol is longer than the symbols read: the VM discards it and
  retires the rule, and only the rule's earlier accepts count
  (`processor.go:120-129`, `137-142`; evidence:
  `TestAcceptOfEOFSymbolIsDiscarded`). Other rules are not affected:
  [KD-3](#known-deviations).
- **Settle.** To produce a match, a rule records its accept before the end of
  input, or at the end of a pushback chain that excludes the EOF symbol. The VM
  retires every rule that is still active after the EOF symbol
  (`processor.go:137-142`), so a rule that never settles contributes only the
  accepts it recorded earlier, and with none it produces `UNKNOWN` lexemes, each
  of which reads to the end of input first (evidence:
  `TestStillActiveAtEOFRetiresRule`; `TestLexerProcessor` "Rule Never Settles
  (UNKNOWN per rune)"). Other rules are not affected: [KD-4](#known-deviations).
- **Decide before accepting.** A rule that decides by lookahead returns
  `StateContinue` until it has decided, and ends its pushback chain in
  `(nil, StateAccept)` or `StateReject`. An earlier accept cannot be withdrawn
  or shortened by a later pushback, and the other chain endings are
  unsupported; see [Rule states and pushback](#rule-states-and-pushback).
- **Advance.** To produce its own match, a rule's net effect consumes input. A
  rule that never advances is contained by the zero-length/`UNKNOWN` fallback
  and the bounded pushback chain, but it produces `UNKNOWN` tokens rather than
  the rule's intended match.
- **Return a valid `State`.** The four defined states are the ones the VM acts
  on. Any other value, returned directly or at the end of a pushback chain,
  retires the rule as `StateReject` does, and an accept the rule recorded
  earlier stays (`processor.go:132-135`, `140-142`; evidence:
  `TestInvalidStateRetiresRule`). Other rules are not affected:
  [KD-1](#known-deviations).
- **Return rather than panic.** A rule that panics is retired for the current
  token as `StateReject` does, and an accept it recorded earlier stays
  (`processor.go:173-185`; evidence: `TestPanicRetiresRule`,
  `TestPanicKeepsEarlierAccept`). The VM does not report the panic value. Other
  rules are not affected: [KD-2](#known-deviations).
- **Side effects are the rule's.** The engine offers no transactionality around a
  rule; any side effect a rule performs is permanent and not rolled back, also
  when the rule panics.

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
  **Reconciled with principle 1.** The VM never holds a mode, and `Reset`
  returns every rule to the `Rule` it was registered with before each token
  (`processor.go:48-55`), so a mode that depends on a prior token cannot live in
  the `Rule` chain. A mode lives above `Next()`. The mechanism already exists:
  one lexer per mode over a shared cursor (`NewWithCursor`), with `Release`
  called before the cursor is handed to the next mode's lexer (see
  [Concurrency boundaries](#concurrency-boundaries)). After a commit the cursor
  sits exactly at the end of the token (`textlexer.go:520-529`). No new API is
  needed.
- **Actions applied only after a winning match commits.** The ability to attach a
  post-match action (decode a number, unescape a string, compute a value) that
  runs **only** for the lexeme the engine actually commits — not for the losing
  candidates the maximal-munch process rejected. This is the intended home for
  "transformed/decoded values", and it is the resolution path for the
  [matched-content question](#matched-content-versus-transformed-or-decoded-values).
  **Reconciled with principle 4.** The VM performs no action. An action runs in
  a consumer or builder layer above `Next()`, on the lexeme the VM reports. A
  lexeme returned by `Next()` is already the committed winner, so the "only for
  the committed lexeme" property holds by construction in that layer.

**Open design questions (recorded, not decided).**

- **Composition and ambiguity.** How composed rules interact with maximal munch
  and registration-order ties is undecided. In particular: when a composed rule
  and a flat rule accept the same length, and how a composed rule's internal
  alternatives participate in the global longest-match selection.
- **State ownership and reset — closed by principle 1.** **Reconciled with
  principle 1.** A composed rule owns its internal state, and it carries that
  state in the `Rule` values it returns. The VM holds none of it and resets every
  rule to its registered `Rule` before each token. A composed rule must fit that
  model; the option of changing the model is closed. State that must survive
  from one token to the next lives above `Next()`, as for
  lexical modes.
- **Recovery.** Whether and how the toolkit offers structured recovery (a
  consumer-facing "no match" hook, a recoverable token stream) instead of the
  fixed one-rune `UNKNOWN` fallback is undecided. **Reconciled with
  principle 4.** The VM keeps reporting `UNKNOWN`; a recovery hook acts on that
  report in a layer above `Next()`. The open question is that layer's shape.
- **Synthetic-token and progress semantics.** Whether composed rules may emit
  zero-width or synthetic tokens (a token not backed by a source span) is
  undecided, and conflicts with the current progress guarantee that every
  committed lexeme covers a non-empty source range. Any such capability must be
  reconciled with that guarantee explicitly. **Reconciled with principles 2
  and 4.** The VM refuses a zero-length match (principle 2) and reports only
  what a rule matched (principle 4), so a synthetic token can only come from a
  layer above `Next()`.

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
consumes through end of line (pushing back the EOF symbol, or the `\n` when a
newline follows, so a newline stays its own token). Result: `WHITESPACE`,
`HASH "#"`, identifiers `not`, `a`, `comment`, `WHITESPACE "\n"`, then
`COMMENT "# a comment"`.

This is the concrete, shipped demonstration of **context sensitivity**: the same
symbol `#` classifies differently depending on its positional context (BOL or
not), using the `Symbol` flags rather than any mode machinery.

**Mapped to:** `TestLexerProcessor` "Context-Aware Rules (BOL)".

**Aspirational.** A full **lexical mode** (e.g., switching to a
"string-interpolation" rule set inside `"...${ ... }..."` and back out) is an
intended capability, not a current one. The BOL example is position-based
sensitivity; mode-based sensitivity (state carried across tokens, not just within
one) is recorded under
[Construction capabilities](#intended-construction-capabilities) as intended.
**Reconciled with principle 1.** The VM carries no state across tokens. The
switch happens above `Next()`: a consumer moves the shared cursor from one
mode's lexer to another's, as described under
[Construction capabilities](#intended-construction-capabilities).

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
3. **Decoded/transformed values** — an optional post-match action hook, above
   `Next()` (principle 4); shape undecided. ([Matched content](#matched-content-versus-transformed-or-decoded-values))
4. **Grapheme-cluster coordinates** — rune columns today; grapheme columns
   undecided. ([Byte versus rune units](#byte-versus-rune-units))
5. **`\r\n` line-break convention** — single unit or `\r` + `\n` for column
   purposes; not pinned by a test. ([Line and column conventions](#line-and-column-conventions))
6. **Configurable no-match behavior** — fixed one-rune `UNKNOWN` today; a
   replaceable hook above `Next()` (principle 4) undecided. ([Unmatched input](#unmatched-input-eof-and-errors))
7. **Panic isolation** — closed by the owner decision of 2026-11-12: the VM
   recovers a rule's panic and retires that rule, as for `StateReject`, and
   does not report the panic value. ([Panics](#panics))
8. **Composition and ambiguity** — how composed rules interact with maximal
   munch and registration-order ties. ([Construction capabilities](#intended-construction-capabilities))
9. **State ownership and reset** — closed by principle 1: a composed rule owns
   its state in the `Rule` values it returns, and the VM resets every rule per
   token. ([Construction capabilities](#intended-construction-capabilities))
10. **Recovery** — structured recovery, above `Next()` (principle 4), vs. the
    fixed `UNKNOWN` fallback.
    ([Construction capabilities](#intended-construction-capabilities))
11. **Synthetic-token / progress semantics** — zero-width or synthetic tokens
    conflict with the non-empty-span progress guarantee; under principles 2
    and 4 they can only come from a layer above `Next()`.
    ([Construction capabilities](#intended-construction-capabilities))
12. **Core/extension seam** — the exact interface for language-specific
    policies (indentation, formatting, error/recovery tied to a grammar).
    ([Construction capabilities](#intended-construction-capabilities))
13. **`\r\n` post-sequence line/column** — no committed test. (Scenario 2.)
14. **Trailing incomplete UTF-8 at EOF** — no committed test. (Scenario 4.)
