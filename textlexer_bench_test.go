package textlexer_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/xiam/textlexer"
)

// sink is used to prevent the compiler from optimizing away the benchmarked code.
var sink *textlexer.Lexeme

// BenchmarkSimpleTokens measures performance on a stream of common, simple tokens
// like identifiers, integers, and whitespace. This represents a typical
// programming language scenario.
func BenchmarkSimpleTokens(b *testing.B) {
	input := strings.Repeat("identifier 12345 \n", 1000)
	idRule := newIdentifierRule()
	intRule := newUnsignedIntegerRule()
	wsRule := newWhitespaceRule()

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("ID", idRule)
		lx.MustAddRule("INT", intRule)
		lx.MustAddRule("WS", wsRule)

		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			sink = lex
		}
	}
}

// BenchmarkComplexTokens measures performance with more complex, multi-state rules
// like floating-point numbers, strings, and comments. This stresses the state
// machine logic more heavily.
func BenchmarkComplexTokens(b *testing.B) {
	input := strings.Repeat("SELECT 'user' FROM users WHERE id = 123.456 /* comment */; ", 500)
	floatRule := newUnsignedFloatRule()
	stringRule := newSingleQuotedStringRule()
	commentRule := newSlashStarCommentRule()
	idRule := newIdentifierRule()
	wsRule := newWhitespaceRule()
	symbolRule := newSymbolRule()

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("FLOAT", floatRule)
		lx.MustAddRule("STRING", stringRule)
		lx.MustAddRule("COMMENT", commentRule)
		lx.MustAddRule("ID", idRule)
		lx.MustAddRule("WS", wsRule)
		lx.MustAddRule("SYMBOL", symbolRule)
		lx.MustAddRule("KEYWORD", matchAnyOf("SELECT", "FROM", "WHERE"))

		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			sink = lex
		}
	}
}

// BenchmarkLongestMatchContention measures performance when multiple rules could
// potentially match a prefix, forcing the lexer to read ahead to resolve the
// ambiguity based on the longest match.
func BenchmarkLongestMatchContention(b *testing.B) {
	input := strings.Repeat("if iffy ifelse for fore ", 1000)
	idRule := newIdentifierRule()
	wsRule := newWhitespaceRule()

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("IF", matchString("if"))
		lx.MustAddRule("IFELSE", matchString("ifelse"))
		lx.MustAddRule("FOR", matchString("for"))
		lx.MustAddRule("ID", idRule) // Will match "iffy" and "fore"
		lx.MustAddRule("WS", wsRule)

		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			sink = lex
		}
	}
}

// BenchmarkManyRules tests the overhead of the rulesProcessor when a large
// number of rules are defined, most of which will not match the input. This
// measures the cost of iterating through the list of active scanners.
func BenchmarkManyRules(b *testing.B) {
	input := strings.Repeat("find_me ", 1000)
	wsRule := newWhitespaceRule()
	targetRule := matchString("find_me")

	const numRules = 500
	rules := make(map[textlexer.LexemeType]textlexer.Rule, numRules)
	for i := 0; i < numRules; i++ {
		ruleType := textlexer.LexemeType(fmt.Sprintf("RULE_%d", i))
		ruleText := fmt.Sprintf("keyword%d", i)
		rules[ruleType] = matchString(ruleText)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		for typ, rule := range rules {
			lx.MustAddRule(typ, rule)
		}
		lx.MustAddRule("TARGET", targetRule)
		lx.MustAddRule("WS", wsRule)

		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			sink = lex
		}
	}
}

// BenchmarkLargeToken measures the performance of processing a single, very large
// token. This tests the efficiency of buffer growth and management.
func BenchmarkLargeToken(b *testing.B) {
	input := strings.Repeat("a", 1024*1024) // 1MB token
	idRule := newIdentifierRule()

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("ID", idRule)

		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			sink = lex
		}
	}
}

// BenchmarkNoMatch tests the worst-case scenario where no rules match the input,
// forcing the lexer to emit single-character UNKNOWN tokens. This measures the
// maximum overhead of the rule processing engine.
func BenchmarkNoMatch(b *testing.B) {
	input := strings.Repeat("!@#$%^&*()", 1000)
	idRule := newIdentifierRule()
	intRule := newUnsignedIntegerRule()

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lx := textlexer.New(strings.NewReader(input))
		lx.MustAddRule("ID", idRule)
		lx.MustAddRule("INT", intRule)

		for {
			lex, err := lx.Next()
			if err == io.EOF {
				break
			}
			sink = lex
		}
	}
}
