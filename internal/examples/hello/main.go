package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/xiam/textlexer"
)

// isLetter checks if a symbol's rune is an alphabet character.
func isLetter(s textlexer.Symbol) bool {
	r := s.Rune()
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// newWordRule creates a rule that matches any sequence of one or more letters.
func newWordRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isLetter(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		if isLetter(s) {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

// newWhitespaceRule creates a rule that matches any sequence of whitespace.
func newWhitespaceRule() textlexer.Rule {
	var loop textlexer.Rule
	loop = func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
	return func(s textlexer.Symbol) (textlexer.Rule, textlexer.State) {
		r := s.Rune()
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return loop, textlexer.StateAccept
		}
		return nil, textlexer.StateReject
	}
}

// matchString creates a rule to match an exact string.
func matchString(s string) textlexer.Rule {
	var ruleFor func(sub string) textlexer.Rule
	ruleFor = func(sub string) textlexer.Rule {
		return func(sym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			runes := []rune(sub)
			if sym.Rune() != runes[0] {
				return nil, textlexer.StateReject
			}
			if len(runes) == 1 {
				return nil, textlexer.StateAccept
			}
			return ruleFor(string(runes[1:])), textlexer.StateContinue
		}
	}
	if s == "" {
		return func(sym textlexer.Symbol) (textlexer.Rule, textlexer.State) {
			return nil, textlexer.StateAccept
		}
	}
	return ruleFor(s)
}

func main() {
	// The text we want to analyze.
	myText := "say hello to the world"

	// 1. Create a new lexer that reads our text.
	lx := textlexer.New(strings.NewReader(myText))

	// 2. Add our rules. The order matters! We want to find specific
	//    keywords before falling back to the general "WORD" rule.
	lx.MustAddRule("KEYWORD", matchString("say"))
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
