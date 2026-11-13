package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/xiam/textlexer"
)

// The rules below follow docs/rule-authoring.md: a constructor returns the
// start state, every state tests the EOF symbol first and ends with an explicit
// reject, and character classes are named predicates.

// isASCIILetter reports whether r is in [A-Za-z].
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isASCIIWhitespace reports whether r is in [ \t\n\r].
func isASCIIWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// newWordRule returns a rule for [A-Za-z]+. It accepts after each letter. At
// end of input it rejects the EOF symbol, so the letters read so far stay the
// match.
func newWordRule() textlexer.Rule {
	var letters textlexer.Rule
	letters = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if isASCIILetter(s.Rune()) {
			return letters, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return letters
}

// newWhitespaceRule returns a rule for [ \t\n\r]+. It accepts after each
// whitespace rune. At end of input it rejects the EOF symbol, so the run read
// so far stays the match.
func newWhitespaceRule() textlexer.Rule {
	var blanks textlexer.Rule
	blanks = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if isASCIIWhitespace(s.Rune()) {
			return blanks, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return blanks
}

// newLiteralRule returns a rule for the literal lit, which must not be empty.
// It accepts at the last rune of lit and stops. At end of input before that
// rune it rejects the EOF symbol, so it matches nothing.
func newLiteralRule(lit string) textlexer.Rule {
	if lit == "" {
		panic("newLiteralRule: lit must not be empty")
	}
	return literalAt([]rune(lit), 0)
}

// literalAt is the state of newLiteralRule that reads rune i of lit.
func literalAt(lit []rune, i int) textlexer.Rule {
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if s.IsEOF() {
			return nil, textlexer.StateReject
		}
		if s.Rune() == lit[i] {
			if i == len(lit)-1 {
				return nil, textlexer.StateAccept
			}
			return literalAt(lit, i+1), textlexer.StateContinue
		}
		return nil, textlexer.StateReject
	}
}

func main() {
	// The text we want to analyze.
	myText := "say hello to the world"

	// 1. Create a new lexer that reads our text.
	lx := textlexer.New(strings.NewReader(myText))

	// 2. Add our rules. The order matters! We want to find specific
	//    keywords before falling back to the general "WORD" rule.
	lx.MustAddRule("KEYWORD", newLiteralRule("say"))
	lx.MustAddRule("WORD", newWordRule())
	lx.MustAddRule("WHITESPACE", newWhitespaceRule()) // To handle spaces.

	// 3. Loop and get the next labeled piece until we reach the end.
	for {
		lexeme, err := lx.Next()

		// Stop if we're at the end of the text.
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err) // Should not happen in this example.
		}

		// Skip over the whitespace we found.
		if lexeme.Type() == "WHITESPACE" {
			continue
		}

		// Print the piece we found!
		fmt.Printf("Found: [%s] \"%s\"\n", lexeme.Type(), lexeme.Text())
	}
}
