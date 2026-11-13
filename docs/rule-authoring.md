# Rule-authoring standard

This document says how to write a `textlexer` rule and how to check one. It is
**normative for every rule in this repository**: the test rules, the shared
rule helpers, and the runnable example. Consumers can use it as guidance for
their own rules.

The [core contract](specs/core-contract.md) states what the VM does with each
verdict a rule returns. This document does not change that contract. It states
which of those verdicts a rule in this repository uses, and in which shape, so
that every rule reads the same way and gets its intended match.

- **Enforcement.** Review against the [reviewer checklist](#reviewer-checklist)
  at the end of this document. No automated checker exists yet.
- **Line ends.** `\r\n` handling is out of scope. It is an open design question
  in the core contract.
- **Engine probes.** A rule that misbehaves on purpose to test the VM is exempt.
  See [Exempt engine probes](#exempt-engine-probes).

The examples use the shared helpers that `textlexer_test.go` defines:
`excludeLast`, `newLiteralRule`, `newLiteralThenRule`, `newRunePlusRule`, and
the predicates `isASCIIDigit`, `isASCIILetter` and `isASCIIWhitespace`.

## Terms

- **Rule value.** A value of type `textlexer.Rule`, the function
  `func(s textlexer.Symbol) (next textlexer.Rule, state textlexer.State)`.
- **Constructor.** A function `newXxxRule(...) textlexer.Rule` that returns the
  start state of a rule.
- **State.** One rule value that a rule returns as `next`. A state is either a
  scanning state or a pushback-chain callback.
- **Scanning state.** A state that the VM calls with the next input symbol.
- **Pushback-chain callback.** A state that a `StatePushBack` verdict hands
  control to. The VM calls it, in the same step, with the symbol that the
  pushback just excluded.
- **Delegating return.** A `return` statement that returns the result of a
  helper call, for example `return excludeLast(1, textlexer.StateAccept)`.
- **The EOF symbol.** The synthetic symbol that the VM shows every active rule
  at end of input. It carries `RuneEOF` and `FlagEOF`, and `s.IsEOF()` reports
  it. It is not part of the input.
- **Engine probe.** A rule that breaks a practice on purpose, so that a test can
  check how the VM reacts.

## Practice 1: one shape

**Rule.** A constructor `newXxxRule(...) textlexer.Rule` builds the rule and
returns its start state. Every state has a name. A state without a parameter is
a local variable, and the constructor declares all of them together
(`var start, body, end textlexer.Rule`). A state with a parameter is a named
constructor at file level (`afterDigits(n int) textlexer.Rule`). An anonymous
closure is never nested more than one level: the closure that a constructor or
a state constructor returns is the only level. A constructor of a rule with one
state, such as `newSymbolRule`, may return that state directly; the
constructor's name is the state's name.

**Rationale.** A reader finds every state of a rule in one place and can follow
a transition by name. Deeply nested closures hide the states and make a
transition hard to follow.

**Example.**

```go
// newSlashStarCommentRule returns a rule for (?s)/\*(?:[^*]|\*[^/])*\*/ ...
func newSlashStarCommentRule() textlexer.Rule {
	var start, afterSlash, body, afterStar textlexer.Rule
	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == '/' {
			return afterSlash, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	// afterSlash, body and afterStar follow the same pattern.
	...
	return start
}
```

A state with a parameter is a named constructor:

```go
// literalAt is the state of newLiteralRule that reads rune i of lit.
func literalAt(lit []rune, i int) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) { ... }
}
```

**Counter-example.** The states are anonymous and nested four levels deep:

```go
forKeywordRule := func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	if s.Rune() == 'f' {
		return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			if s.Rune() == 'o' {
				return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
					...
```

## Practice 2: one state, one decision

**Rule.** The practice has a separate rule for each of the three kinds of
function.

- **Scanning states.**
  1. The first statement handles the EOF symbol: `if s.IsEOF() { ... }`.
  2. Then the state decides with an `if` or a `switch` on named predicates
     (practice 3).
  3. The last statement is the fallback for the symbols that no branch
     handles: `return nil, textlexer.StateReject`. One exception: when every
     remaining symbol completes a lookahead decision, as in `keywordEnd`, the
     last statement may be that documented delegating verdict, for example
     `return excludeLast(1, textlexer.StateAccept) // the symbol after the keyword`.
     A reject there would change the match or be unreachable.
  4. Every `return` names its `State` constant, or is a delegating return.
  5. A scanning state never returns `StateAccept` for the EOF symbol. At end of
     input it rejects, or it excludes the EOF symbol with `excludeLast`
     (practice 5).
- **Pushback-chain callbacks.** The VM calls a callback with the symbol that
  the pushback just excluded. At end of input, the first callback receives the
  EOF symbol. A callback does not test for the EOF symbol and does not reject
  it. The last callback of the chain returns `(nil, StateAccept)` or
  `(nil, StateReject)` for any symbol, because the accepted span already
  excludes that symbol. A rule never builds a callback by hand. It calls the
  shared helper `excludeLast` (practice 5).
- **Delegating returns.** A scanning state may return the result of a helper
  call, for example `return excludeLast(1, textlexer.StateAccept)` or
  `return literalAt(lit, i+1), textlexer.StateContinue`. The comment of the
  helper states what it returns.

The invariant behind the three rules: **an accepted span never counts the EOF
symbol.**

**Rationale.** The VM shows the EOF symbol to every active rule, so each
scanning state must decide what end of input means for it. A state that tests
the EOF symbol first makes that decision visible. An accept that counts the EOF
symbol is longer than the input that was read, and the VM discards it
([EOF handling](specs/core-contract.md#eof-handling)). The explicit final reject
makes it clear that every other symbol fails the rule. A callback is different:
the pushback has already removed the symbol it receives from the match, so a
test there would reject a valid match at end of input.

**Example.** A scanning state of `[A-Za-z_][A-Za-z0-9_]*`:

```go
rest = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	if s.IsEOF() {
		return nil, textlexer.StateReject // keeps the identifier read so far
	}
	if isASCIIWord(s.Rune()) {
		return rest, textlexer.StateAccept
	}
	return nil, textlexer.StateReject
}
```

**Counter-example.** The state never tests for the EOF symbol, so the reader
must know that `RuneEOF` is `-1` and that `-1` fails the comparison. A state
that continues on every other symbol also continues on the EOF symbol:

```go
inComment = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	if s.Rune() == '*' {
		return afterStar, textlexer.StateContinue
	}
	return inComment, textlexer.StateContinue
}
```

A callback that rejects the EOF symbol loses the match at end of input:

```go
// Wrong: on "a" at end of input this emits UNKNOWN "a", not A "a".
return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	if s.IsEOF() {
		return nil, textlexer.StateReject
	}
	return nil, textlexer.StateAccept
}, textlexer.StatePushBack
```

## Practice 3: named predicates

**Rule.** A character class is a named predicate `isXxx(r rune) bool`. The name
says whether the class is ASCII-only or Unicode: `isASCIIDigit`,
`isASCIILetter`, `isUnicodeLetter`. The comment of the predicate gives the class
in regular-expression notation. A test for one literal rune
(`s.Rune() == '/'`) is written inline. A positional flag is tested with the
`Symbol` method (`s.IsBOL()`).

**Rationale.** A name states the intent, and the comment states the exact set.
A reader does not have to decode a range expression in every state. ASCII and
Unicode classes differ for most inputs outside ASCII, so the name must say
which one the rule uses.

**Example.**

```go
// isASCIILetter reports whether r is in [A-Za-z].
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isUnicodeLetter reports whether r is in \pL.
func isUnicodeLetter(r rune) bool { return unicode.IsLetter(r) }
```

**Counter-example.** The class is written inline, and its name (`letter`) does
not say that it is ASCII-only:

```go
if r := s.Rune(); (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
```

## Practice 4: state lives in returned rules

**Rule.** A rule keeps its state only in the rule values it returns and in
immutable constructor parameters. No captured variable is written after the
constructor returns.

**Rationale.** The VM restarts every rule at its registered start state for
each token ([principle 1](specs/core-contract.md#principle-1-stateless-stack-model)).
A rule that writes a captured variable carries that value into the next token,
and into every lexer that registered the same rule value. A rule that keeps its
state in returned values can be registered in several lexers and reused for
every token.

**Example.** The count lives in the returned rule value:

```go
// countedDigits is the state of [0-9]{lo,hi} after n digits.
func countedDigits(lo, hi, n int) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		...
		return countedDigits(lo, hi, n+1), textlexer.StateContinue
	}
}
```

**Counter-example.** `depth` is written after construction. The second token
starts with the depth that the first token left:

```go
depth := 0
var loop textlexer.Rule
loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	depth++
	...
}
```

## Practice 5: accept discipline and lookahead

**Rule.**

- **Eager accept.** Within a token, the match that a rule records only grows
  ([Rule states and pushback](specs/core-contract.md#rule-states-and-pushback)).
  So a rule accepts a span only when that span is a valid match on its own and
  no later decision can need to withdraw it or shorten it. A pending lookahead
  decision that can only grow the match does not stop the accept: `ab|abcd(?=ef)`
  accepts `ab` and then decides whether `abcd` is valid (`newGrowingAcceptRule`),
  and `ab(?=c)|ab.*z` accepts at each `z` (`newUnboundedLongerRule`). A
  lookahead decision that can need a span shorter than what the rule has read
  does stop it: the rule returns `StateContinue` past that span until it has
  decided.
- **Lookahead.** When the rule settles on a span shorter than what it has
  read, it ends with exactly one pushback chain, built by the shared helper
  `excludeLast(n, state)`. `state` is `StateAccept` or `StateReject`. The call
  site has a comment that names the `n` excluded symbols. At end of input, the
  first of them is the EOF symbol. A rule that settles in any other way needs
  no chain: it accepts everything it has read, or it rejects and keeps the
  accept it recorded earlier, if any.
- **Lookahead combined with a longer match.** Use delayed acceptance, the
  supported construction
  ([Lookahead combined with a longer match](specs/core-contract.md#lookahead-combined-with-a-longer-match)).
  The rule keeps scanning while the longer branch can still match, and it
  records no accept that is shorter than what it has read. It either accepts
  the longer match, or it ends with one terminal pushback chain back to the
  validated shorter match. A chain never ends in `StateContinue` or in a
  non-nil `next` with `StateAccept`.

The helper:

```go
// excludeLast returns the verdict of a rule that excludes the last n symbols
// (n >= 1) from its match and then ends the pushback chain with (nil, s).
func excludeLast(n int, s textlexer.State) (textlexer.Rule, textlexer.State)
```

**Rationale.** Pushback cannot shorten an accept that the rule already
recorded, so an early accept of a candidate that lookahead later rejects stays
in the result. One helper builds every chain, so every chain ends the rule and
no chain counts the EOF symbol by mistake. The comment on `n` is where the
off-by-one error happens, in particular the extra symbol at end of input.

**Example: lookahead with one excluded symbol.** A keyword matches only before
a symbol that cannot continue a word:

```go
// keywordEnd is the state of newKeywordRule after the keyword. It decides by
// one symbol of lookahead.
func keywordEnd(continuesWord func(rune) bool) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return excludeLast(1, textlexer.StateAccept) // the EOF symbol
		}
		if continuesWord(s.Rune()) {
			return nil, textlexer.StateReject
		}
		return excludeLast(1, textlexer.StateAccept) // the symbol after the keyword
	}
}
```

**Worked example: a rule that waits for the EOF symbol.** The rule matches
`[A-Za-z]+\z`, a run of ASCII letters that ends the input. It must see the EOF
symbol before it can accept, so it never accepts a letter. On the EOF symbol it
excludes that one symbol and accepts the letters. The token is not empty,
because the start state requires one letter. The committed test is
`TestRuleAuthoringEOFPushbackExample` in `textlexer_test.go`: on `ab` it emits
`WORD "ab"`, and on `ab cd` it emits `UNKNOWN "a"`, `UNKNOWN "b"`,
`UNKNOWN " "` and `WORD "cd"`.

```go
// newLettersAtEndRule returns a rule for [A-Za-z]+\z: a run of ASCII letters
// that ends the input. At end of input it excludes the EOF symbol and accepts
// the letters; any other symbol after a letter rejects.
func newLettersAtEndRule() textlexer.Rule {
	var start, letters textlexer.Rule
	start = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if isASCIILetter(s.Rune()) {
			return letters, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	letters = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return excludeLast(1, textlexer.StateAccept) // the EOF symbol
		}
		if isASCIILetter(s.Rune()) {
			return letters, textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
	return start
}
```

**Example: lookahead combined with a longer match.** The rule matches
`abcd(?=ef)|abcdefgh` with delayed acceptance. `abcd` is validated when `ef`
has been read, but the rule does not accept it then, because `abcdefgh` can
still match. If `gh` follows, it accepts all eight symbols. If the longer
branch fails, it excludes everything after `abcd`, the symbol that failed
included. The example gives the expected lexeme stream for
`abcdefgh`, `abcdefxx`, `abcdef`, `abcdefg`, `abcdefgX`, `abcde`, `abcdX` and
`abcdefghabcdef`. `TestLookaheadDelayedAcceptance` in `textlexer_test.go`
pins the same construction with `newLookaheadOrLongerRule`.

```go
// newABCDBeforeEFOrABCDEFGHRule returns a rule for abcd(?=ef)|abcdefgh, in
// Perl notation, with delayed acceptance. At end of input after "abcdef" it
// excludes the runes after "abcd" and the EOF symbol, and accepts "abcd".
func newABCDBeforeEFOrABCDEFGHRule() textlexer.Rule {
	return abcdefghAt(0)
}

// abcdefghAt is the state of newABCDBeforeEFOrABCDEFGHRule that reads rune i
// of "abcdefgh". From i = 6 on, "abcd" is validated by "ef".
func abcdefghAt(i int) textlexer.Rule {
	const long = "abcdefgh"
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			if i >= 6 {
				return excludeLast(i+1-4, textlexer.StateAccept) // the runes after "abcd", and the EOF symbol
			}
			return nil, textlexer.StateReject
		}
		if s.Rune() == rune(long[i]) {
			if i == len(long)-1 {
				return nil, textlexer.StateAccept
			}
			return abcdefghAt(i + 1), textlexer.StateContinue
		}
		if i >= 6 {
			return excludeLast(i+1-4, textlexer.StateAccept) // the runes after "abcd", and the symbol just read
		}
		return nil, textlexer.StateReject
	}
}
```

**Counter-example.** The rule accepts `abcd` early and then tries to continue
with the longer branch. The chain ends in a non-nil `next` with `StateAccept`,
which is unsupported ([KD-5](specs/core-contract.md#known-deviations)), and the
continuation expects to see `e` and `f` again. On `abcdefgh` the rule emits
`abcd` and four `UNKNOWN` runes:

```go
case 5: // the 'f' of "abcdef": exclude "ef", record "abcd", continue at 'e'
	return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		return func(textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return abcdefghAt(4), textlexer.StateAccept // unsupported: non-nil next
		}, textlexer.StatePushBack
	}, textlexer.StatePushBack
```

The construction also breaks practice 2: it builds the pushback-chain
callbacks by hand.

## Practice 6: progress and purity

**Rule.** A rule:

- never produces an empty match;
- always settles by the end of input, with an accept or a reject;
- does not panic;
- performs no side effect: it writes no variable outside its returned rule
  values, performs no I/O, and does not call `t.Fatal`;
- depends only on the symbols it receives: their runes and their flags.

A constructor may panic on an invalid argument, for example an empty literal,
because that panic happens at construction time, not while the VM runs.

**Rationale.** The VM contains a rule that breaks these points
([Custom-rule responsibilities](specs/core-contract.md#custom-rule-responsibilities)),
but the rule then loses its intended match. A rule that does not settle makes
every token read to the end of input. A side effect is permanent: the VM calls
losing candidates too, and it does not roll back their effects. A rule that
depends on anything other than its symbols gives a different token stream for
the same input.

**Example.** `newLiteralRule` rejects an empty literal at construction time and
consumes one rune per state:

```go
func newLiteralRule(lit string) textlexer.Rule {
	if lit == "" {
		panic("newLiteralRule: lit must not be empty")
	}
	return literalAt([]rune(lit), 0)
}
```

**Counter-example.** A rule that counts its calls in a captured variable
depends on how often the VM called it before:

```go
callCount := 0
rule = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
	callCount++
	if callCount > 4 {
		return nil, textlexer.StateReject
	}
	...
}
```

## Practice 7: documentation

**Rule.** The comment of the constructor:

- gives the regular expression that the rule implements, in Go `regexp` syntax
  where the syntax can express it, and in Perl notation (`(?=...)`, `(?!...)`)
  for lookahead. When no regular expression exists, it gives a prose
  definition;
- states the behaviour at end of input: what the rule matches when the input
  ends in each of its states, and whether it waits for the EOF symbol.

A named state constructor and a predicate also have a comment. A comment says
what the code does, not why it was changed.

**Rationale.** The regular expression is the specification that the
conformance test checks the rule against (practice 9). The behaviour at end of
input is the part of a rule that a reader cannot see from the input alone.

**Example.**

```go
// newUnsignedIntegerRule returns a rule for [0-9]+. It accepts after each
// digit. At end of input it rejects the EOF symbol, so the digits read so far
// stay the match.
```

**Counter-example.** `// matchString is a test helper that creates a simple
Rule to match an exact string.` The comment gives neither the regular
expression nor the end-of-input behaviour.

## Practice 8: naming

**Rule.** A rule constructor is `newXxxRule`, where `Xxx` names what the rule
matches. A named state constructor is a noun phrase for the position in the
rule (`literalAt`, `afterDigits`, `keywordEnd`). A predicate is `isXxx`. A
declared lexeme type constant is `lexTypeXxx`. Initialisms keep one case
(`newASCIILettersRule`, `isASCIIDigit`).

**Rationale.** A reader recognizes the start of a rule by its name and finds
all rules with one search.

**Example.** `newIdentifierRule`, `newLiteralAlternationRule`,
`lexTypeIdentifier`.

**Counter-example.** `matchString`, `matchAnyOf`: the names do not say that
they return a rule, and they do not start with `new`.

## Practice 9: tests

**Rule.** Each rule has conformance cases in the style of
`TestRegexpConformance` (`textlexer_regexp_test.go`):

- an input that matches;
- an input that does not match;
- an input where the match stops before the end of input;
- the end of input in each scanning state: an input that ends while the rule is
  in that state;
- the empty input.

A rule whose regular expression Go's `regexp` can check is a row of
`TestRegexpConformance`. A rule with lookahead, a positional flag or a
back-reference has hand-written cases with complete expected lexeme streams,
as in `TestRegexpWithoutOracle` and the `TestLookahead...` tests. A rule that
ends by lookahead has a case in which the end of input completes the lookahead
and the token is emitted.

**Rationale.** Go's `regexp` is an independent oracle for the matched text. End
of input is where most rule defects are, and each scanning state reacts to the
EOF symbol on its own.

**Example.**

```go
{
	expr:  `[0-9]+`,
	rule:  newUnsignedIntegerRule(),
	input: []string{"7", "123", "12a", "a1"},
},
```

**Counter-example.** A rule that only appears in a stream test for one
sentence of input, with no input that ends inside the rule.

## Exempt engine probes

A test that checks how the VM reacts to a bad verdict needs a rule that returns
that verdict: a rule that panics, returns an invalid `State`, never settles,
counts the EOF symbol, ends a chain in an unsupported way, or records the
symbols it is shown. Such an **engine probe** is exempt from the practices it
breaks. The exemption rule:

- The probe breaks a practice **on purpose**, and the purpose is the VM
  reaction that its test checks. A rule that only happens to break a practice is
  not a probe; bring it into line.
- The probe has a comment that starts with `Engine probe:` and names each
  practice it breaks on purpose, for example
  `// Engine probe: never settles on purpose (breaks practice 6).`
- The probe follows every practice that its purpose does not need it to break.
- A probe is local to its test. Other tests do not reuse it as a rule.

## Reviewer checklist

Check each rule against each line. For an engine probe, check that its
`Engine probe:` comment names each practice it breaks, and check the other
lines.

- [ ] **1. One shape.** A `newXxxRule` constructor returns the start state.
      Every state is a named local variable declared together, or a named
      constructor. No anonymous closure is nested more than one level.
- [ ] **2a. Scanning states.** Each one tests `s.IsEOF()` first, decides with
      named predicates, and ends with `return nil, textlexer.StateReject` for
      the unhandled symbols, or with a commented terminal `excludeLast`
      verdict when every remaining symbol completes the lookahead.
      Every return names its `State` constant or is a delegating return. No
      scanning state returns `StateAccept` for the EOF symbol.
- [ ] **2b. Pushback-chain callbacks.** The rule builds no callback by hand;
      every chain comes from `excludeLast`. No callback tests for or rejects the
      EOF symbol.
- [ ] **2c. Delegating returns.** Each one calls a helper whose comment states
      what it returns.
- [ ] **3. Named predicates.** Every character class is an `isXxx(r rune) bool`
      predicate whose name says ASCII or Unicode and whose comment gives the
      class.
- [ ] **4. State in returned rules.** No captured variable is written after the
      constructor returns.
- [ ] **5. Accept discipline and lookahead.** Every accept is a valid match
      that no later decision needs to withdraw or shorten; while a decision
      can need a shorter span, the rule continues. A rule that settles on a
      span shorter than what it has read ends with one `excludeLast` chain,
      and the call site comments the `n` excluded symbols, the EOF symbol
      included. A rule with lookahead and a longer branch uses delayed
      acceptance. No chain ends in `StateContinue` or in a non-nil `next`
      with `StateAccept`.
- [ ] **6. Progress and purity.** No empty match, settles by the end of input,
      no panic, no side effect, depends only on its symbols.
- [ ] **7. Documentation.** The constructor comment gives the regular
      expression (or a prose definition) and the behaviour at end of input.
- [ ] **8. Naming.** `newXxxRule`, `isXxx`, `lexTypeXxx`; initialisms in one
      case.
- [ ] **9. Tests.** Conformance cases: a match, a non-match, a match that
      stops early, the end of input in each scanning state, and the empty
      input. A lookahead rule has a case in which the end of input completes
      the lookahead.
