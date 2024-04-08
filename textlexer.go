package textlexer

import (
	"fmt"
	"io"
	"sync"
)

type Reader interface {
	io.RuneReader
	io.Seeker
}

type TextLexer struct {
	mu sync.Mutex

	r Reader

	offset int

	rules    []LexemeType
	rulesMap map[LexemeType]Rule
}

func New(r Reader) *TextLexer {
	return &TextLexer{
		r:        r,
		rules:    []LexemeType{},
		rulesMap: map[LexemeType]Rule{},
	}
}

func (lx *TextLexer) AddRule(lexType LexemeType, lexRule Rule) error {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	if _, ok := lx.rulesMap[lexType]; ok {
		return fmt.Errorf("rule for %q already exists", lexType)
	}
	lx.rulesMap[lexType] = lexRule
	lx.rules = append(lx.rules, lexType)
	return nil
}

func (lx *TextLexer) MustAddRule(lexType LexemeType, lexRule Rule) {
	if err := lx.AddRule(lexType, lexRule); err != nil {
		panic(fmt.Sprintf("MustAddRule: %v", err))
	}
}

func (lx *TextLexer) Next() (*Lexeme, error) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	scanners := map[LexemeType]Rule{}
	for _, lexType := range lx.rules {
		scanners[lexType] = lx.rulesMap[lexType]
	}

	var lastLexeme *Lexeme
	var isEOF bool

	var buf []rune

	offset := 0
	for {

		r, _, err := lx.r.ReadRune()
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read error: %v", err)
		}

		isEOF = err == io.EOF
		if isEOF {
			r = RuneEOF
		}

		if len(buf) == 0 && r == RuneEOF {
			return nil, io.EOF
		}

		for _, lexType := range lx.rules {
			scanner := scanners[lexType]
			if scanner == nil {
				continue
			}

			next, state := scanner(r)
			scanners[lexType] = next

			if state == StateReject {
				delete(scanners, lexType)
			}

			if state == StateAccept {
				delete(scanners, lexType)

				if offset > 0 {
					lastLexeme = &Lexeme{
						Type:   lexType,
						text:   buf,
						offset: lx.offset + offset,
					}
				} else {
					lastLexeme = &Lexeme{
						Type:   lexType,
						text:   []rune{r},
						offset: lx.offset + 1,
					}
				}
			}
		}

		buf = append(buf, r)
		offset++

		if len(scanners) == 0 || isEOF {
			// no scanners left
			break
		}
	}

	if lastLexeme != nil {
		lx.offset = lastLexeme.offset

		if _, err := lx.r.Seek(int64(lx.offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek: %v", err)
		}

		return lastLexeme, nil
	}

	if !isEOF {
		lastLexeme = &Lexeme{
			Type:   LexemeTypeUnknown,
			text:   buf,
			offset: lx.offset + offset,
		}

		lx.offset = lastLexeme.offset

		if _, err := lx.r.Seek(int64(lx.offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek: %v", err)
		}

		return lastLexeme, nil
	}

	return nil, io.EOF
}
