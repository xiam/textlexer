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

## Installation

```sh
go get github.com/xiam/textlexer
```

## License

MIT License. See the [LICENSE.md](LICENSE) file for details.
