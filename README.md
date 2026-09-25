# textlexer

`textlexer` is a simple tool for reading text and identifying and labeling its
different parts, like a smart highlighter.

Imagine you have a block of text, like a command or a log entry. You want to
break it down into meaningful pieces: this is a command, this is a number, this
is a word, etc. `textlexer` lets you define simple rules for what each piece
looks like, and it will scan your text and hand you back the labeled pieces one
by one.

### How It Works

1.  **You Define the Rules:** You tell the tool what you're looking for. For
    example, a "KEYWORD" is the exact word `say`, and a "WORD" is any sequence
    of letters.
2.  **It Finds the Longest Match:** If one rule could match "send" and another
    could match "send_message", the tool is smart enough to choose the longer
    one ("send_message").
3.  **It Returns Labeled Pieces:** The tool gives you a stream of the pieces it
    found, each with the label (e.g., "KEYWORD") and the text (e.g., "say")
    that it matched.

## Example

Let's find all the keywords and words in the phrase "say hello to the world".

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

MIT License. See the [LICENSE.md](LICENSE) file for details.
