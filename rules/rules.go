package rules

import (
	stdlog "log"

	"github.com/xiam/textlexer"
)

// common symbols and tokens
const (
	doubleQuote = '"'
	singleQuote = '\''

	plus  = '+'
	minus = '-'

	slash = '/'
	star  = '*'

	backslash = '\\'

	ampersand = '&'
	pipe      = '|'
	colon     = ':'
	semicolon = ';'
	comma     = ','
	period    = '.'
	exponent  = '^'
	tilde     = '~'
	backtick  = '`'

	lparen = '('
	rparen = ')'

	lbracket = '['
	rbracket = ']'

	lbrace = '{'
	rbrace = '}'

	langle = '<'
	rangle = '>'

	hash = '#'

	newLine = '\n'

	exclamation       = '!'
	percent           = '%'
	question          = '?'
	equals            = '='
	doubleEquals      = "=="
	tripleEquals      = "==="
	addAndAssign      = "+="
	subtractAndAssign = "-="
	multiplyAndAssign = "*="
	divideAndAssign   = "/="
	modulusAndAssign  = "%="
	doubleStar        = "**"
	notEquals         = "!="
	differentFrom     = "<>"
	increment         = "++"
	decrement         = "--"

	nullishCoalescing = "??"
	optionalChaining  = "?."
	shortDeclaration  = ":="
	spreadOperator    = "..."
	scopeOperator     = "::"
	tripleBacktick    = "```"

	leftShift  = "<<"
	rightShift = ">>"
)

// common character classes
var (
	asciiLetters    = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'}
	asciiDigits     = []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}
	asciiWhitespace = []rune{' ', '\t', '\r', '\n', '\f'}
	hexDigits       = []rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F'}
	octalDigits     = []rune{'0', '1', '2', '3', '4', '5', '6', '7'}
	binaryDigits    = []rune{'0', '1'}
)

// common character-class based matchers
var (
	// matches a sequence of [0-9]
	asciiDigitsMatcher = NewGreedyCharacterMatcher(asciiDigits)

	// matches a sequence of [a-zA-Z]
	asciiLettersMatcher = NewGreedyCharacterMatcher(asciiLetters)

	// matches: \n
	nlMatcher = NewCharacterMatcher(newLine)

	// matches: #
	hashMatcher = NewCharacterMatcher(hash)

	// matches parens: ( or )
	parensMatcher = newAnyOfCharactersMatcher(
		[]rune{lparen, rparen},
	)

	// matches: (
	lparenMatcher = NewCharacterMatcher(lparen)

	// matches: )
	rparenMatcher = NewCharacterMatcher(rparen)

	// matches brackets: [ or ]
	bracketsMatcher = newAnyOfCharactersMatcher(
		[]rune{lbracket, rbracket},
	)

	// matches: [
	lbracketMatcher = NewCharacterMatcher(lbracket)

	// matches: ]
	rbracketMatcher = NewCharacterMatcher(rbracket)

	// matches braces: { or }
	bracesMatcher = newAnyOfCharactersMatcher(
		[]rune{lbrace, rbrace},
	)

	// matches: {
	lbraceMatcher = NewCharacterMatcher(lbrace)

	// matches: }
	rbraceMatcher = NewCharacterMatcher(rbrace)

	// matches angles: < or >
	anglesMatcher = newAnyOfCharactersMatcher(
		[]rune{langle, rangle},
	)

	// matches: <
	langleMatcher = NewCharacterMatcher(langle)

	// matches: >
	rangleMatcher = NewCharacterMatcher(rangle)

	// matches: ,
	commaMatcher = NewCharacterMatcher(comma)

	// matches: :
	colonMatcher = NewCharacterMatcher(colon)

	// matches: ;
	semicolonMatcher = NewCharacterMatcher(semicolon)

	// matches: .
	periodMatcher = NewCharacterMatcher(period)

	// matches: ^
	exponentMatcher = NewCharacterMatcher(exponent)

	// matches: ~
	tildeMatcher = NewCharacterMatcher(tilde)

	// matches: `
	backtickMatcher = NewCharacterMatcher(backtick)

	// matches: +
	plusMatcher = NewCharacterMatcher(plus)

	// matches: -
	minusMatcher = NewCharacterMatcher(minus)

	// matches: *
	starMatcher = NewCharacterMatcher(star)

	// matches: /
	slashMatcher = NewCharacterMatcher(slash)

	// matches: \
	backslashMatcher = NewCharacterMatcher(backslash)

	// matches: |
	pipeMatcher = NewCharacterMatcher(pipe)

	// matches: &
	ampersandMatcher = NewCharacterMatcher(ampersand)

	// matches: =
	equalsMatcher = NewCharacterMatcher(equals)

	// matches: ==
	doubleEqualsMatcher = NewMatchString(doubleEquals)

	// matches: ===
	tripleEqualsMatcher = NewMatchString(tripleEquals)

	// matches: +=
	addAndAssignMatcher = NewMatchString(addAndAssign)

	// matches: -=
	subtractAndAssignMatcher = NewMatchString(subtractAndAssign)

	// matches: *=
	multiplyAndAssignMatcher = NewMatchString(multiplyAndAssign)

	// matches: /=
	divideAndAssignMatcher = NewMatchString(divideAndAssign)

	// matches: %=
	modulusAndAssignMatcher = NewMatchString(modulusAndAssign)

	// matches: **
	doubleStarMatcher = NewMatchString(doubleStar)

	// matches: !=
	notEqualsMatcher = NewMatchString(notEquals)

	// matches: <>
	differentFromMatcher = NewMatchString(differentFrom)

	// matches: ++
	incrementMatcher = NewMatchString(increment)

	// matches: --
	decrementMatcher = NewMatchString(decrement)

	// matches: ??
	nullishCoalescingMatcher = NewMatchString(nullishCoalescing)

	// matches: ?.
	optionalChainingMatcher = NewMatchString(optionalChaining)

	// matches: :=
	shortDeclarationMatcher = NewMatchString(shortDeclaration)

	// matches: ...
	spreadOperatorMatcher = NewMatchString(spreadOperator)

	// matches: ::
	scopeOperatorMatcher = NewMatchString(scopeOperator)

	// matches: <<
	leftShiftMatcher = NewMatchString(leftShift)

	// matches: >>
	rightShiftMatcher = NewMatchString(rightShift)

	// matches: ```
	tripleBacktickMatcher = NewMatchString(tripleBacktick)

	// matches: !
	exclamationMatcher = NewCharacterMatcher(exclamation)

	// matches: %
	percentMatcher = NewCharacterMatcher(percent)

	// matches: ?
	questionMarkMatcher = NewCharacterMatcher(question)

	// matches "
	doubleQuoteMatcher = NewCharacterMatcher(doubleQuote)

	// matches a sequence of [0-9a-fA-F]
	hexDigitsMatcher = NewGreedyCharacterMatcher(hexDigits)

	// matches a sequence of [0-1]
	binaryDigitsMatcher = NewGreedyCharacterMatcher(binaryDigits)

	// mathematical operators
	mathOperatorsMatcher = newAnyOfCharactersMatcher(
		[]rune{plus, minus, star, slash},
	)

	// matches a sequence of [0-7]
	octalDigitsMatcher = NewGreedyCharacterMatcher(octalDigits)

	// matches '
	singleQuoteMatcher = NewCharacterMatcher(singleQuote)

	// matches common whitespace characters
	commonWhitespaceMatcher = NewGreedyCharacterMatcher(asciiWhitespace)

	// matches a fat arrow: =>
	fatArrowMatcher = NewMatchString("=>")

	// matches an arrow: ->
	arrowMatcher = NewMatchString("->")

	// matches a logical OR: ||
	logicalOrMatcher = NewMatchString("||")

	// matches a logical AND: &&
	logicalAndMatcher = NewMatchString("&&")

	// matches a less than or equal operator: <=
	lessThanOrEqualMatcher = NewMatchString("<=")

	// matches a greater than or equal operator: >=
	greaterThanOrEqualMatcher = NewMatchString(">=")
)

var (
	// matches a numeric value
	unsignedNumericSwitchMatcher = NewSwitchMatcher(
		UnsignedInteger,
		UnsignedFloat,
	)

	// matches a signed numeric value
	signedNumericSwitchMatcher = NewSwitchMatcher(
		SignedInteger,
		SignedFloat,
	)
)

// EOF matches the end of file (EOF) character.
func EOF(r rune) (textlexer.Rule, textlexer.State) {
	if IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nil, textlexer.StateReject
}

// UnsignedInteger matches one or more digits (0-9).
// Example: `123`, `0`, `9876543210`
func UnsignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	return asciiDigitsMatcher(r)
}

// EOL matches the end of line (EOL) character or EOF.
func EOL(r rune) (textlexer.Rule, textlexer.State) {
	if r == '\r' {
		return nlMatcher, textlexer.StateContinue
	}

	if IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nlMatcher(r)
}

// OptionalWhitespace matches zero or more whitespace characters.
// Example: ` `, `\t`, `\n\r\n`, `\t\t `, “
func OptionalWhitespace(r rune) (textlexer.Rule, textlexer.State) {
	if isCommonWhitespace(r) {
		return OptionalWhitespace, textlexer.StateContinue
	}

	return PushBackCurrentAndAccept(r)
}

// SignedInteger matches an optional sign (+/-) and digits. It allows
// whitespace between the sign and the number.
// Example: `123`, `+45`, `-0`, `+ 99`, `- 1`
func SignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	if r == '-' || r == '+' {
		// match might start with a sign, in this case, we continue
		// matching whitespace, and expect the numeric part after that
		return newWhitespaceConsumer(
			UnsignedInteger,
		), textlexer.StateContinue
	}

	return UnsignedInteger(r)
}

// UnsignedFloat matches a floating-point number without sign.
// Example: `123.45`, `0.1`, `.56`, `123.`, `0.`
func UnsignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var integerPartMatcher textlexer.Rule
	var fractionalPartMatcher textlexer.Rule

	var radixPointMatcher func(bool) func(rune) (textlexer.Rule, textlexer.State)

	fractionalPartMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return fractionalPartMatcher, textlexer.StateContinue
		}

		return PushBackCurrentAndAccept(r)
	}

	radixPointMatcher = func(requireFractional bool) func(r rune) (textlexer.Rule, textlexer.State) {
		return func(r rune) (textlexer.Rule, textlexer.State) {
			if r != '.' {
				return nil, textlexer.StateReject
			}

			return func(r rune) (textlexer.Rule, textlexer.State) {
				// if any digit follows the radix point, we can continue matching the
				// fractional part
				if isASCIIDigit(r) {
					return fractionalPartMatcher, textlexer.StateContinue
				}

				if requireFractional {
					// we require a fractional part, but none was found
					return nil, textlexer.StateReject
				}

				// if no digit follows the radix point, we can accept the radix point and
				// stop matching
				return PushBackCurrentAndAccept(r)
			}, textlexer.StateContinue
		}
	}

	integerPartMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return integerPartMatcher, textlexer.StateContinue
		}

		if r == '.' {
			return radixPointMatcher(false)(r)
		}

		return nil, textlexer.StateReject
	}

	if isASCIIDigit(r) {
		return integerPartMatcher, textlexer.StateContinue
	}

	if r == '.' {
		return radixPointMatcher(true)(r)
	}

	return nil, textlexer.StateReject
}

// SignedFloat matches a float with optional sign (+/-). Whitespace is
// allowed after the sign and before the number.
// Example: `123.45`, `+0.1`, `-.56`, `+ 123.45`, `- .5`
func SignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	if r == '-' || r == '+' {
		return newWhitespaceConsumer(
			UnsignedFloat,
		), textlexer.StateContinue
	}

	return UnsignedFloat(r)
}

// UnsignedNumeric matches integers or floating-point numbers.
// Example: `123`, `0`, `123.45`, `0.5`, `.5`, `5.0`
func UnsignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	return unsignedNumericSwitchMatcher(r)
}

// SignedNumeric matches numbers with optional sign (+/-). Whitespace is
// allowed after the sign and before the number.
// Example: `123`, `+45`, `-0.5`, `+ 99`, `- .5`, `+ 1.`, `- 123.45`
func SignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	return signedNumericSwitchMatcher(r)
}

// Whitespace matches one or more whitespace characters.
// Example: ` `, ` \t`, `\n\r\n`, `\t\t `
func Whitespace(r rune) (textlexer.Rule, textlexer.State) {
	return commonWhitespaceMatcher(r)
}

// Identifier matches programming identifiers (letter followed by letters/digits).
// Example: `variable`, `count1`, `isValid`, `i`, `MyClass`
func Identifier(r rune) (next textlexer.Rule, state textlexer.State) {
	var matcher textlexer.Rule

	matcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIILetter(r) || isASCIIDigit(r) || r == '_' {
			return matcher, textlexer.StateContinue
		}

		return PushBackCurrentAndAccept(r)
	}

	if isASCIILetter(r) || r == '_' {
		return matcher, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// DoubleQuotedString matches text in double quotes.
// Example: `"hello world"`, `""`, `"a"`, `"quote"`
func DoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	doubleQuotedStringMatcher := newChainedRuleMatcher(
		doubleQuoteMatcher,
		func(r rune) (textlexer.Rule, textlexer.State) {
			if r == doubleQuote {
				return nil, textlexer.StateAccept
			}

			if IsEOF(r) {
				return nil, textlexer.StateReject
			}

			return nil, textlexer.StateContinue
		},
	)

	return doubleQuotedStringMatcher(r)
}

// SingleQuotedString matches text in single quotes.
// Example: `'hello world'`, `"`, `'a'`, `'quote'`
func SingleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
	singleQuoteStringMatcher := newChainedRuleMatcher(
		singleQuoteMatcher,
		func(r rune) (textlexer.Rule, textlexer.State) {
			if r == singleQuote {
				return nil, textlexer.StateAccept
			}

			if IsEOF(r) {
				return nil, textlexer.StateReject
			}

			return nil, textlexer.StateContinue
		},
	)

	return singleQuoteStringMatcher(r)
}

// DoubleQuotedEscapedString matches text in double quotes with escape
// sequences. A scape sequence is a backslash followed by any character.  The
// validity of the escape sequence is not checked.
// Example: `"hello \"world\""`, `"a\\b"`, `"\\"`, `"escaped"`
func DoubleQuotedEscapedString(r rune) (textlexer.Rule, textlexer.State) {
	escapedDoubleQuotedStringMatcher := newChainedRuleMatcher(
		doubleQuoteMatcher,
		func(r rune) (textlexer.Rule, textlexer.State) {
			if r == doubleQuote {
				return nil, textlexer.StateAccept
			}

			if IsEOF(r) {
				return nil, textlexer.StateReject
			}

			if r == '\\' { // escape sequence
				return func(r rune) (textlexer.Rule, textlexer.State) {
					if IsEOF(r) {
						return nil, textlexer.StateReject
					}

					return nil, textlexer.StateContinue
				}, textlexer.StateContinue
			}

			return nil, textlexer.StateContinue
		},
	)

	return escapedDoubleQuotedStringMatcher(r)
}

// DoubleSlashComment matches single-line comments starting with //.
// Example: `// This is a comment`, `// Another comment<EOF>`
func DoubleSlashComment(r rune) (textlexer.Rule, textlexer.State) {
	return newChainedRuleMatcher(
		newLiteralSequenceMatcher([]rune{slash, slash}),
		AnyUntilEOL,
	)(r)
}

// AnyUntilEOF consumes all characters until EOF.
func AnyUntilEOF(r rune) (textlexer.Rule, textlexer.State) {
	var matcher textlexer.Rule

	matcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if IsEOF(r) {
			stdlog.Println("EOF")
			return PushBackCurrentAndAccept(r)
		}

		return matcher, textlexer.StateContinue
	}

	return matcher(r)
}

// AnyUntilEOL consumes characters until newline or EOF.
func AnyUntilEOL(r rune) (textlexer.Rule, textlexer.State) {
	var matcher textlexer.Rule

	matcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if r == newLine {
			return nil, textlexer.StateAccept
		}

		if IsEOF(r) {
			return PushBackCurrentAndAccept(r)
		}

		return matcher, textlexer.StateContinue
	}

	return matcher(r)
}

// SlashStarComment matches C-style /* */ comments.
// Example: `/* comment */`, `/* multi\nline */`, `/**/`
func SlashStarComment(r rune) (textlexer.Rule, textlexer.State) {
	return newChainedRuleMatcher(
		newLiteralSequenceMatcher([]rune{slash, star}),
		func(r rune) (textlexer.Rule, textlexer.State) {
			var matcher textlexer.Rule

			matcher = func(r rune) (textlexer.Rule, textlexer.State) {
				if r == star {
					return func(r rune) (textlexer.Rule, textlexer.State) {
						if r == slash {
							return nil, textlexer.StateAccept
						}

						return matcher(r)
					}, textlexer.StateContinue
				}

				if IsEOF(r) {
					return nil, textlexer.StateReject
				}

				return nil, textlexer.StateContinue
			}

			return matcher(r)
		},
	)(r)
}

// BasicMathOperator matches +, -, *, or /.
func BasicMathOperator(r rune) (textlexer.Rule, textlexer.State) {
	return mathOperatorsMatcher(r)
}

// LessEqual matches the less than or equal operator `<=`.
func LessEqual(r rune) (textlexer.Rule, textlexer.State) {
	return lessThanOrEqualMatcher(r)
}

// GreaterEqual matches the greater than or equal operator `>=`.
func GreaterEqual(r rune) (textlexer.Rule, textlexer.State) {
	return greaterThanOrEqualMatcher(r)
}

// LogicalAnd matches the logical AND operator `&&`.
func LogicalAnd(r rune) (textlexer.Rule, textlexer.State) {
	return logicalAndMatcher(r)
}

// LogicalOr matches the logical OR operator `||`.
func LogicalOr(r rune) (textlexer.Rule, textlexer.State) {
	return logicalOrMatcher(r)
}

// Arrow matches the arrow operator `->`.
func Arrow(r rune) (textlexer.Rule, textlexer.State) {
	return arrowMatcher(r)
}

// FatArrow matches the fat arrow operator `=>`.
func FatArrow(r rune) (textlexer.Rule, textlexer.State) {
	return fatArrowMatcher(r)
}

// HexInteger matches hexadecimal integers prefixed with `0x`
// or `0X`.
// Example: `0x1A`, `0XFF`, `0x0`, `0xabcdef`
func HexInteger(r rune) (textlexer.Rule, textlexer.State) {
	zeroXMatcher := func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '0' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if r == 'x' || r == 'X' {
					return nil, textlexer.StateAccept
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	return newChainedRuleMatcher(
		zeroXMatcher,
		hexDigitsMatcher,
	)(r)
}

// BinaryInteger matches binary integers prefixed with `0b` or `0B`.
// Example: `0b1010`, `0B11`, `0b0`
func BinaryInteger(r rune) (textlexer.Rule, textlexer.State) {
	zeroBMatcher := func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '0' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if r == 'b' || r == 'B' {
					return nil, textlexer.StateAccept
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	return newChainedRuleMatcher(
		zeroBMatcher,
		binaryDigitsMatcher,
	)(r)
}

// OctalInteger matches octal integers prefixed with `0o` or `0O`.
// Example: `0o12`, `0O7`, `0o0`
func OctalInteger(r rune) (textlexer.Rule, textlexer.State) {
	zeroOMatcher := func(r rune) (textlexer.Rule, textlexer.State) {
		if r == '0' {
			return func(r rune) (textlexer.Rule, textlexer.State) {
				if r == 'o' || r == 'O' {
					return nil, textlexer.StateAccept
				}
				return nil, textlexer.StateReject
			}, textlexer.StateContinue
		}

		return nil, textlexer.StateReject
	}

	return newChainedRuleMatcher(
		zeroOMatcher,
		octalDigitsMatcher,
	)(r)
}

// HashComment matches single-line comments starting with #.
// Example: `# This is a comment\n`, `# Another comment<EOF>`
func HashComment(r rune) (textlexer.Rule, textlexer.State) {
	return newChainedRuleMatcher(
		hashMatcher,
		AnyUntilEOL,
	)(r)
}

// AnyUntilWhitespaceOrEOF matches any sequence of characters until a common
// whitespace or EOF.
func AnyUntilWhitespaceOrEOF(r rune) (textlexer.Rule, textlexer.State) {
	// starts with a non-whitespace character
	start := func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) || IsEOF(r) {
			return nil, textlexer.StateReject
		}

		return nil, textlexer.StateAccept
	}

	// continues until a whitespace or EOF
	next := func(r rune) (textlexer.Rule, textlexer.State) {
		if isCommonWhitespace(r) || IsEOF(r) {
			return PushBackCurrentAndAccept(r)
		}

		return nil, textlexer.StateContinue
	}

	return newChainedRuleMatcher(start, next)(r)
}

// Paren matches either ( or ).
// Example: `(`, `)`
func Paren(r rune) (textlexer.Rule, textlexer.State) {
	return parensMatcher(r)
}

// LParen matches an opening parenthesis (.
func LParen(r rune) (textlexer.Rule, textlexer.State) {
	return lparenMatcher(r)
}

// RParen matches a closing parenthesis ).
func RParen(r rune) (textlexer.Rule, textlexer.State) {
	return rparenMatcher(r)
}

// Brace matches either { or }.
// Example: `{`, `}`
func Brace(r rune) (textlexer.Rule, textlexer.State) {
	return bracesMatcher(r)
}

// LBrace matches an opening brace {.
func LBrace(r rune) (textlexer.Rule, textlexer.State) {
	return lbraceMatcher(r)
}

// RBrace matches a closing brace }.
func RBrace(r rune) (textlexer.Rule, textlexer.State) {
	return rbraceMatcher(r)
}

// Bracket matches either [ or ].
// Example: `[`, `]`
func Bracket(r rune) (textlexer.Rule, textlexer.State) {
	return bracketsMatcher(r)
}

// LBracket matches an opening bracket [.
func LBracket(r rune) (textlexer.Rule, textlexer.State) {
	return lbracketMatcher(r)
}

// RBracket matches a closing bracket ].
func RBracket(r rune) (textlexer.Rule, textlexer.State) {
	return rbracketMatcher(r)
}

// Angle matches either < or >.
// Example: `<`, `>`
func Angle(r rune) (textlexer.Rule, textlexer.State) {
	return anglesMatcher(r)
}

// LAngle matches a left angle bracket <.
func LAngle(r rune) (textlexer.Rule, textlexer.State) {
	return langleMatcher(r)
}

// RAngle matches a right angle bracket >.
func RAngle(r rune) (textlexer.Rule, textlexer.State) {
	return rangleMatcher(r)
}

// Comma matches a comma.
func Comma(r rune) (textlexer.Rule, textlexer.State) {
	return commaMatcher(r)
}

// Colon matches a colon.
func Colon(r rune) (textlexer.Rule, textlexer.State) {
	return colonMatcher(r)
}

// Semicolon matches a semicolon.
func Semicolon(r rune) (textlexer.Rule, textlexer.State) {
	return semicolonMatcher(r)
}

// Exponent matches an exponent sign (^).
func Exponent(r rune) (textlexer.Rule, textlexer.State) {
	return exponentMatcher(r)
}

// Tilde matches a tilde (~).
func Tilde(r rune) (textlexer.Rule, textlexer.State) {
	return tildeMatcher(r)
}

// Backtick matches a backtick (`).
func Backtick(r rune) (textlexer.Rule, textlexer.State) {
	return backtickMatcher(r)
}

// Period matches a period.
func Period(r rune) (textlexer.Rule, textlexer.State) {
	return periodMatcher(r)
}

// Plus matches a plus sign.
func Plus(r rune) (textlexer.Rule, textlexer.State) {
	return plusMatcher(r)
}

// Minus matches a minus sign.
func Minus(r rune) (textlexer.Rule, textlexer.State) {
	return minusMatcher(r)
}

// Star matches an asterisk.
func Star(r rune) (textlexer.Rule, textlexer.State) {
	return starMatcher(r)
}

// Slash matches a forward slash.
func Slash(r rune) (textlexer.Rule, textlexer.State) {
	return slashMatcher(r)
}

// Backslash matches a backslash.
func Backslash(r rune) (textlexer.Rule, textlexer.State) {
	return backslashMatcher(r)
}

// Percent matches a percent sign.
func Percent(r rune) (textlexer.Rule, textlexer.State) {
	return percentMatcher(r)
}

// Equal matches an equals sign (=).
func Equals(r rune) (textlexer.Rule, textlexer.State) {
	return equalsMatcher(r)
}

// DoubleEquals matches a double equals sign (==).
func DoubleEquals(r rune) (textlexer.Rule, textlexer.State) {
	return doubleEqualsMatcher(r)
}

// TripleEquals matches a triple equals sign (===).
func TripleEquals(r rune) (textlexer.Rule, textlexer.State) {
	return tripleEqualsMatcher(r)
}

// AddAndAssign matches a plus equals sign (+=).
func AddAndAssign(r rune) (textlexer.Rule, textlexer.State) {
	return addAndAssignMatcher(r)
}

// SubtractAndAssign matches a minus equals sign (-=).
func SubtractAndAssign(r rune) (textlexer.Rule, textlexer.State) {
	return subtractAndAssignMatcher(r)
}

// MultiplyAndAssign matches a star equals sign (*=).
func MultiplyAndAssign(r rune) (textlexer.Rule, textlexer.State) {
	return multiplyAndAssignMatcher(r)
}

// DivideAndAssign matches a slash equals sign (/=).
func DivideAndAssign(r rune) (textlexer.Rule, textlexer.State) {
	return divideAndAssignMatcher(r)
}

// ModulusAndAssign matches a percent equals sign (%=).
func ModulusAndAssign(r rune) (textlexer.Rule, textlexer.State) {
	return modulusAndAssignMatcher(r)
}

// DoubleStar matches a double star (**).
func DoubleStar(r rune) (textlexer.Rule, textlexer.State) {
	return doubleStarMatcher(r)
}

// NotEquals matches a not equals sign (!=).
func NotEquals(r rune) (textlexer.Rule, textlexer.State) {
	return notEqualsMatcher(r)
}

// DifferentFrom matches a different from sign (<>).
func DifferentFrom(r rune) (textlexer.Rule, textlexer.State) {
	return differentFromMatcher(r)
}

// Increment matches an increment operator (++).
func Increment(r rune) (textlexer.Rule, textlexer.State) {
	return incrementMatcher(r)
}

// Decrement matches a decrement operator (--).
func Decrement(r rune) (textlexer.Rule, textlexer.State) {
	return decrementMatcher(r)
}

// NullishCoalescing matches a nullish coalescing operator (??).
func NullishCoalescing(r rune) (textlexer.Rule, textlexer.State) {
	return nullishCoalescingMatcher(r)
}

// OptionalChaining matches an optional chaining operator (?.)
func OptionalChaining(r rune) (textlexer.Rule, textlexer.State) {
	return optionalChainingMatcher(r)
}

// ShortDeclaration matches a short declaration operator (:=).
func ShortDeclaration(r rune) (textlexer.Rule, textlexer.State) {
	return shortDeclarationMatcher(r)
}

// SpreadOperator matches a spread operator (...).
func SpreadOperator(r rune) (textlexer.Rule, textlexer.State) {
	return spreadOperatorMatcher(r)
}

// ScopeOperator matches a scope operator (::).
func ScopeOperator(r rune) (textlexer.Rule, textlexer.State) {
	return scopeOperatorMatcher(r)
}

// TripleBacktick matches a triple backtick (```).
func TripleBacktick(r rune) (textlexer.Rule, textlexer.State) {
	return tripleBacktickMatcher(r)
}

// DoubleQuote matches a double quote.
func DoubleQuote(r rune) (textlexer.Rule, textlexer.State) {
	return doubleQuoteMatcher(r)
}

// LeftShift matches a left shift operator (<<).
func LeftShift(r rune) (textlexer.Rule, textlexer.State) {
	return leftShiftMatcher(r)
}

// RightShift matches a right shift operator (>>).
func RightShift(r rune) (textlexer.Rule, textlexer.State) {
	return rightShiftMatcher(r)
}

// Exclamation matches an exclamation mark.
func Exclamation(r rune) (textlexer.Rule, textlexer.State) {
	return exclamationMatcher(r)
}

// Pipe matches a pipe symbol.
func Pipe(r rune) (textlexer.Rule, textlexer.State) {
	return pipeMatcher(r)
}

// Ampersand matches an ampersand.
func Ampersand(r rune) (textlexer.Rule, textlexer.State) {
	return ampersandMatcher(r)
}

// QuestionMark matches a question mark.
func QuestionMark(r rune) (textlexer.Rule, textlexer.State) {
	return questionMarkMatcher(r)
}
