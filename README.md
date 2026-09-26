# textlexer

`textlexer` is a toolkit for building lexers in Go. You describe the lexical
rules for a text — what a keyword is, what a word is, what a number is, where a
comment starts and ends — and the engine scans your text and hands you back a
stream of **lexemes**: the matched content, a label you assigned, and the exact
source position (byte and rune offset, line and column) of each piece.

What you do with the lexemes is up to you. `textlexer` tokenizes; it does not
parse. Grammar validation, AST construction, and interpretation belong to you,
the consumer. The same engine is intended to serve many different lexers — a
highlighter, a SQL tokenizer, a log parser — each defined by its own rules.

The canonical prose specification — the contract for what the engine does today,
what it is intended to become, and what is still an open design question — lives
in [docs/specs/core-contract.md](docs/specs/core-contract.md). This README
presents the capabilities and limitations and a runnable example; the spec is
the reference for exact behavior.

## Capabilities today

- **Rules you write in Go.** A rule is a state-transition function, so any
  matching logic expressible in Go is expressible as a rule: literals,
  character predicates, Unicode-aware matching, and positional decisions
  (beginning of line, end of line, end of input) via the symbol's flags.
- **Longest match, with deterministic tie breaking.** If two rules both match,
  the longer match wins; if they match the same length, the rule registered
  first wins.
- **Full source positions.** Every lexeme carries a half-open span with byte
  offset, rune offset, 1-based line, and 0-based rune column at both ends.
- **Safe to drive concurrently.** Multiple goroutines can call `Next()` on one
  lexer; the token stream stays deterministic and complete.
- **Bounded memory.** An optional per-token byte bound caps how much source a
  single in-flight token may retain, and a diagnostic `Context` accessor reads
  the text around a recent lexeme.

## Limitations today

- **No rule builders yet.** Rules are hand-written Go functions. Reusable,
  composable building blocks — literal and predicate combinators, Unicode
  categories, sequence/alternation/repetition, named rules, lexical modes, and
  post-match actions — are intended capabilities, not shipped APIs. The spec
  records them at the semantic level; nothing here is available today.
- **One flat label.** A lexeme carries a single string-valued type. There is no
  kind/metadata API yet; expressing a broad category plus a specific kind is an
  intended capability with an open design question.
- **No parsing.** The output is a stream of descriptive lexemes, nothing more.
  Anything structural is the consumer's job.

**Future work.** On top of the current engine: the reusable rule tools above,
and eventually declarative frontends (for example, a schema- or table-driven
way of declaring rules) that compile down to the same engine. These are
direction, not commitments — the spec marks each as intended or open.

## Example

Find all the keywords and words in the phrase "say hello to the world".

Here is a snippet showing the core logic.

```go
package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/xiam/textlexer"
)

func main() {
	// The text we want to analyze.
	myText := "say hello to the world"

	// 1. Create a new lexer that reads our text.
	lx := textlexer.New(strings.NewReader(myText))

	// 2. Add rules for what we want to find. The order matters!
	//    (Rule helper functions like matchString and newWordRule are
	//    defined in the full example.)
	lx.MustAddRule("KEYWORD", matchString("say"))
	lx.MustAddRule("WORD", newWordRule())
	lx.MustAddRule("WHITESPACE", newWhitespaceRule())

	// 3. Loop and get the next labeled piece until we reach the end.
	for {
		lexeme, err := lx.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		// Skip over whitespace.
		if lexeme.Type() == "WHITESPACE" {
			continue
		}

		// Print the piece we found!
		fmt.Printf("Found: [%s] \"%s\"\n", lexeme.Type(), lexeme.Text())
	}
}
```

For a complete, runnable program including the rule helper functions, please
see the file at
[internal/examples/hello/main.go](./internal/examples/hello/main.go).

#### Output:

```
Found: [KEYWORD] "say"
Found: [WORD] "hello"
Found: [WORD] "to"
Found: [WORD] "the"
Found: [WORD] "world"
```

## Layering

`textlexer` owns lexical policy: rule state transitions, parallel rule
evaluation, maximal-munch selection, registration-order tie breaking, the
`UNKNOWN` fallback, and lexeme construction. It owns no text-stream mechanics.

Reading, buffering, positions, and replay come from a positioned cursor —
[`github.com/xiam/textreader`](https://github.com/xiam/textreader). The cursor
decodes UTF-8, tracks the logical position, retains the input a checkpoint needs,
and restores a marked position exactly.

`New` accepts the same `io.RuneReader` it always did. A source that also
implements `io.Reader` is read as a byte stream by the cursor, so no extra UTF-8
decoder is introduced; a source that only implements `io.RuneReader` is read rune
by rune. `NewWithCursor` takes an already-built cursor instead.

### Token spans

Every lexeme reports the source span it covers:

```go
lex, err := lx.Next()
span := lex.Span()

span.Start().ByteOffset()  // byte offset of the first byte
span.Start().RuneOffset()  // rune offset of the first rune
span.Start().Line()        // 1-based line
span.Start().Column()      // 0-based rune column
span.End()                 // position just past the token
span.Bytes()               // width in bytes
span.Runes()               // width in runes
```

`lex.ByteOffset()`, `lex.RuneOffset()`, `lex.Line()`, and `lex.Column()` are
shorthands for the start position. `Offset()` is unchanged and still counts
runes, so it equals `RuneOffset()`; use `ByteOffset()` when bytes are what you
need.

Positions come from the committed token, not from where the lexer stopped
reading: a rule that looks ahead past its match does not shift the next token's
span.

While a token is being assembled the lexer holds a checkpoint, so
`WithMaxTokenBytes(n)` bounds how many source bytes one token may retain. A token
that exceeds the bound fails with an error wrapping
`textreader.ErrRetentionExceeded` instead of growing without limit, and the
cursor is rewound to the token start, so a failed `Next` consumes nothing. The
default is unbounded.

`lexer.Context(lex, before, after)` returns the source text around a lexeme,
including the input that precedes it. The lexer keeps a window of the most
recent lexemes retained, so the context stays available while `lex` is inside
that window, and `before` is clamped to it. A `Next` that commits nothing — one
that fails, or the final EOF probe — releases nothing. Once later tokens push
`lex` out of the window, its retained input is released and calls return
`textreader.ErrPositionOutOfBuffer`.

## Installation

```sh
go get github.com/xiam/textlexer
```

## License

MIT License. See the [LICENSE.md](LICENSE.md) file for details.
