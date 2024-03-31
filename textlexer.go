package textlexer

import (
	"errors"
	"fmt"
	"io"

	"github.com/xiam/textreader"
)

type TextLexer struct {
	tr *textreader.TextReader

	rules    []TokenType
	rulesMap map[TokenType]Rule
}

func NewTextLexer(tr io.Reader) *TextLexer {
	return &TextLexer{
		tr:       textreader.NewReader(tr),
		rules:    []TokenType{},
		rulesMap: map[TokenType]Rule{},
	}
}

func (lx *TextLexer) AddRule(tokenType TokenType, tokRule Rule) error {
	if _, ok := lx.rulesMap[tokenType]; ok {
		return fmt.Errorf("Token rule for %q already exists", tokenType)
	}
	lx.rulesMap[tokenType] = tokRule
	lx.rules = append(lx.rules, tokenType)
	return nil
}

func (lx *TextLexer) MustAddRule(tokenType TokenType, tokRule Rule) {
	if err := lx.AddRule(tokenType, tokRule); err != nil {
		panic(fmt.Sprintf("MustAddRule: %v", err))
	}
}

func (lx *TextLexer) readNext() (rune, error) {
	next, _, err := lx.tr.ReadRune()
	if err != nil {
		if err == io.EOF {
			return RuneEOF, err
		}
		return RuneEOF, fmt.Errorf("read error: %v", err)
	}
	return next, nil
}

func (lx *TextLexer) Next() (*Token, error) {
	var tok *Token

	buf := []rune{}

	// initial scanner states
	scanners := map[TokenType]Rule{}
	for _, tokenType := range lx.rules {
		scanners[tokenType] = lx.rulesMap[tokenType]
	}

	var atEOF bool

	for {
		next, err := lx.readNext()
		if err != nil {
			return nil, err
		}

		atEOF = next == RuneEOF

		for _, tokenType := range lx.rules {

			if scanners[tokenType] == nil {
				continue
			}

			scanner := scanners[tokenType]

			rule, state := scanner(next)

			if state == StateAccept {
				tok = &Token{
					Type: tokenType,
					text: buf,
				}

				if len(scanners) == 1 || atEOF {

					if !atEOF {
						err := lx.tr.UnreadRune()
						if err != nil {
							return nil, fmt.Errorf("UnreadRune error: %v", err)
						}
					}

					return tok, nil
				}

				rule = nil
			}

			if rule == nil {
				delete(scanners, tokenType)
			} else {
				scanners[tokenType] = rule
			}
		}

		if atEOF {
			return nil, io.EOF
		}

		buf = append(buf, next)
	}

	return nil, errors.New("Not implemented")
}
