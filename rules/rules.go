package rules

import (
	"github.com/xiam/textlexer"
)

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

	lparen = '('
	rparen = ')'

	lbracket = '['
	rbracket = ']'

	lbrace = '{'
	rbrace = '}'

	langle = '<'
	rangle = '>'

	hash = '#'

	exclamation = '!'
	percent     = '%'
	question    = '?'
	equal       = '='

	newLine = '\n'
)

// internal character-class based matchers
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

	// matches: [
	lbracketMatcher = NewCharacterMatcher(lbracket)

	// matches: ]
	rbracketMatcher = NewCharacterMatcher(rbracket)

	// matches: {
	lbraceMatcher = NewCharacterMatcher(lbrace)

	// matches: }
	rbraceMatcher = NewCharacterMatcher(rbrace)

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

	// matches: +
	plusMatcher = NewCharacterMatcher(plus)

	// matches: -
	minusMatcher = NewCharacterMatcher(minus)

	// matches: *
	starMatcher = NewCharacterMatcher(star)

	// matches: /
	slashMatcher = NewCharacterMatcher(slash)

	// matches: |
	pipeMatcher = NewCharacterMatcher(pipe)

	// matches: &
	ampersandMatcher = NewCharacterMatcher(ampersand)

	// matches: =
	equalMatcher = NewCharacterMatcher(equal)

	// matches: !
	exclamationMatcher = NewCharacterMatcher(exclamation)

	// matches: %
	percentMatcher = NewCharacterMatcher(percent)

	// matches: ?
	questionMatcher = NewCharacterMatcher(question)

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
)

// common patterns
var (
	lessThanOrEqualMatcher    = NewMatchString("<=")
	greaterThanOrEqualMatcher = NewMatchString(">=")
)

// TODO: remove
func MatchAnyCharacter(r rune) (textlexer.Rule, textlexer.State) {
	return nil, textlexer.StateContinue
}

// MatchEOF matches the end of file (EOF) character.
func MatchEOF(r rune) (textlexer.Rule, textlexer.State) {
	if IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nil, textlexer.StateReject
}

// MatchUnsignedInteger matches one or more digits (0-9).
// Example: `123`, `0`, `9876543210`
func MatchUnsignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	return asciiDigitsMatcher(r)
}

// MatchEOL matches the end of line (EOL) character or EOF.
func MatchEOL(r rune) (textlexer.Rule, textlexer.State) {
	if r == '\r' {
		return nlMatcher, textlexer.StateContinue
	}

	if IsEOF(r) {
		return nil, textlexer.StateAccept
	}

	return nlMatcher(r)
}

// MatchZeroOrMoreWhitespaces matches zero or more whitespace characters.
// Example: ` `, `\t`, `\n\r\n`, `\t\t `, “
func MatchZeroOrMoreWhitespaces(r rune) (textlexer.Rule, textlexer.State) {
	if isCommonWhitespace(r) {
		return MatchZeroOrMoreWhitespaces, textlexer.StateContinue
	}

	return PushBackCurrentAndAccept(r)
}

// MatchSignedInteger matches an optional sign (+/-) and digits. It allows
// whitespace between the sign and the number.
// Example: `123`, `+45`, `-0`, `+ 99`, `- 1`
func MatchSignedInteger(r rune) (textlexer.Rule, textlexer.State) {
	if r == '-' || r == '+' {
		// match might start with a sign, in this case, we continue
		// matching whitespace, and expect the numeric part after that
		return newWhitespaceConsumer(
			MatchUnsignedInteger,
		), textlexer.StateContinue
	}

	return MatchUnsignedInteger(r)
}

// MatchUnsignedFloat matches a floating-point number without sign.
// Example: `123.45`, `0.1`, `.56`
func MatchUnsignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	var integerPartMatcher textlexer.Rule
	var radixPointMatcher textlexer.Rule
	var fractionalPartMatcher textlexer.Rule

	fractionalPartMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return fractionalPartMatcher, textlexer.StateContinue
		}

		return PushBackCurrentAndAccept(r)
	}

	radixPointMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if r != '.' {
			return nil, textlexer.StateReject
		}

		// expects a digit immediately after the radix point
		return func(r rune) (textlexer.Rule, textlexer.State) {
			if isASCIIDigit(r) {
				return fractionalPartMatcher, textlexer.StateContinue
			}
			return nil, textlexer.StateReject
		}, textlexer.StateContinue
	}

	integerPartMatcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIIDigit(r) {
			return integerPartMatcher, textlexer.StateContinue
		}

		if r == '.' {
			return radixPointMatcher(r)
		}

		return nil, textlexer.StateReject
	}

	return integerPartMatcher(r)
}

// MatchSignedFloat matches a float with optional sign (+/-). Whitespace is
// allowed after the sign and before the number.
// Example: `123.45`, `+0.1`, `-.56`, `+ 123.45`, `- .5`
func MatchSignedFloat(r rune) (textlexer.Rule, textlexer.State) {
	if r == '-' || r == '+' {
		return newWhitespaceConsumer(
			MatchUnsignedFloat,
		), textlexer.StateContinue
	}

	return MatchUnsignedFloat(r)
}

var unsignedNumericSwitchMatcher = NewSwitchMatcher(
	MatchUnsignedInteger,
	MatchUnsignedFloat,
)

var signedNumericSwitchMatcher = NewSwitchMatcher(
	MatchSignedInteger,
	MatchSignedFloat,
)

// MatchUnsignedNumeric matches integers or floating-point numbers.
// Example: `123`, `0`, `123.45`, `0.5`, `.5`, `5.0`
func MatchUnsignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	return unsignedNumericSwitchMatcher(r)
}

// MatchSignedNumeric matches numbers with optional sign (+/-). Whitespace is
// allowed after the sign and before the number.
// Example: `123`, `+45`, `-0.5`, `+ 99`, `- .5`, `+ 1.`, `- 123.45`
func MatchSignedNumeric(r rune) (textlexer.Rule, textlexer.State) {
	return signedNumericSwitchMatcher(r)
}

// MatchWhitespace matches one or more whitespace characters.
// Example: ` `, ` \t`, `\n\r\n`, `\t\t `
func MatchWhitespace(r rune) (textlexer.Rule, textlexer.State) {
	return commonWhitespaceMatcher(r)
}

// MatchIdentifier matches programming identifiers (letter followed by letters/digits).
// Example: `variable`, `count1`, `isValid`, `i`, `MyClass`
func MatchIdentifier(r rune) (next textlexer.Rule, state textlexer.State) {
	var matcher textlexer.Rule

	matcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if isASCIILetter(r) || isASCIIDigit(r) {
			return matcher, textlexer.StateContinue
		}

		return PushBackCurrentAndAccept(r)
	}

	if isASCIILetter(r) {
		return matcher, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// MatchDoubleQuotedString matches text in double quotes.
// Example: `"hello world"`, `""`, `"a"`, `"quote"`
func MatchDoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchSingleQuotedString matches text in single quotes.
// Example: `'hello world'`, `"`, `'a'`, `'quote'`
func MatchSingleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchEscapedDoubleQuotedString matches text in double quotes with escape
// sequences. A scape sequence is a backslash followed by any character.  The
// validity of the escape sequence is not checked.
// Example: `"hello \"world\""`, `"a\\b"`, `"\\"`, `"escaped"`
func MatchEscapedDoubleQuotedString(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchInlineComment matches single-line comments starting with //.
// Example: `// This is a comment`, `// Another comment<EOF>`
func MatchInlineComment(r rune) (textlexer.Rule, textlexer.State) {
	return newChainedRuleMatcher(
		newLiteralSequenceMatcher([]rune{slash, slash}),
		MatchExceptEOL,
	)(r)
}

// MatchExceptEOF consumes all characters until EOF.
func MatchExceptEOF(r rune) (textlexer.Rule, textlexer.State) {
	var matcher textlexer.Rule

	matcher = func(r rune) (textlexer.Rule, textlexer.State) {
		if IsEOF(r) {
			return PushBackCurrentAndAccept(r)
		}

		return matcher, textlexer.StateContinue
	}

	return matcher(r)
}

// MatchExceptEOL consumes characters until newline or EOF.
func MatchExceptEOL(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchSlashStarComment matches C-style /* */ comments.
// Example: `/* comment */`, `/* multi\nline */`, `/**/`
func MatchSlashStarComment(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchBasicMathOperator matches +, -, *, or /.
func MatchBasicMathOperator(r rune) (textlexer.Rule, textlexer.State) {
	return mathOperatorsMatcher(r)
}

// MatchLessEqual matches the less than or equal operator `<=`.
func MatchLessEqual(r rune) (textlexer.Rule, textlexer.State) {
	return lessThanOrEqualMatcher(r)
}

// MatchGreaterEqual matches the greater than or equal operator `>=`.
func MatchGreaterEqual(r rune) (textlexer.Rule, textlexer.State) {
	return greaterThanOrEqualMatcher(r)
}

// MatchLogicalAnd matches the logical AND operator `&&`.
func MatchLogicalAnd(r rune) (textlexer.Rule, textlexer.State) {
	return NewMatchString("&&")(r)
}

// MatchLogicalOr matches the logical OR operator `||`.
func MatchLogicalOr(r rune) (textlexer.Rule, textlexer.State) {
	return NewMatchString("||")(r)
}

// MatchArrow matches the arrow operator `->`.
func MatchArrow(r rune) (textlexer.Rule, textlexer.State) {
	return NewMatchString("->")(r)
}

// MatchFatArrow matches the fat arrow operator `=>`.
func MatchFatArrow(r rune) (textlexer.Rule, textlexer.State) {
	return NewMatchString("=>")(r)
}

// MatchHexInteger matches hexadecimal integers prefixed with `0x`
// or `0X`.
// Example: `0x1A`, `0XFF`, `0x0`, `0xabcdef`
func MatchHexInteger(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchBinaryInteger matches binary integers prefixed with `0b` or `0B`.
// Example: `0b1010`, `0B11`, `0b0`
func MatchBinaryInteger(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchOctalInteger matches octal integers prefixed with `0o` or `0O`.
// Example: `0o12`, `0O7`, `0o0`
func MatchOctalInteger(r rune) (textlexer.Rule, textlexer.State) {
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

// MatchHashComment matches single-line comments starting with #.
// Example: `# This is a comment\n`, `# Another comment<EOF>`
func MatchHashComment(r rune) (textlexer.Rule, textlexer.State) {
	return newChainedRuleMatcher(
		hashMatcher,
		MatchExceptEOL,
	)(r)
}

// MatchIdentifierWithUnderscore matches programming identifiers
// (letter or underscore followed by letters/digits/underscores).
// Example: `variable`, `_private`, `count1`, `isValid`, `i`, `My_Class`
func MatchIdentifierWithUnderscore(r rune) (next textlexer.Rule, state textlexer.State) {
	var nextChar textlexer.Rule

	nextChar = func(r rune) (textlexer.Rule, textlexer.State) {
		// can be followed by more letters, digits, or underscores
		if isASCIILetter(r) || isASCIIDigit(r) || r == '_' {
			return nextChar, textlexer.StateContinue
		}

		// ends with any other character
		return PushBackCurrentAndAccept(r)
	}

	// starts with a letter or underscore
	if isASCIILetter(r) || r == '_' {
		return nextChar, textlexer.StateContinue
	}

	return nil, textlexer.StateReject
}

// AcceptAnyParen matches either ( or ).
// Example: `(`, `)`
func AcceptAnyParen(r rune) (textlexer.Rule, textlexer.State) {
	return parensMatcher(r)
}

// AcceptLParen matches an opening parenthesis (.
func AcceptLParen(r rune) (textlexer.Rule, textlexer.State) {
	return lparenMatcher(r)
}

// AcceptRParen matches a closing parenthesis ).
func AcceptRParen(r rune) (textlexer.Rule, textlexer.State) {
	return rparenMatcher(r)
}

// AcceptLBrace matches an opening brace {.
func AcceptLBrace(r rune) (textlexer.Rule, textlexer.State) {
	return lbraceMatcher(r)
}

// AcceptRBrace matches a closing brace }.
func AcceptRBrace(r rune) (textlexer.Rule, textlexer.State) {
	return rbraceMatcher(r)
}

// AcceptLBracket matches an opening bracket [.
func AcceptLBracket(r rune) (textlexer.Rule, textlexer.State) {
	return lbracketMatcher(r)
}

// AcceptRBracket matches a closing bracket ].
func AcceptRBracket(r rune) (textlexer.Rule, textlexer.State) {
	return rbracketMatcher(r)
}

// AcceptLAngle matches a left angle bracket <.
func AcceptLAngle(r rune) (textlexer.Rule, textlexer.State) {
	return langleMatcher(r)
}

// AcceptRAngle matches a right angle bracket >.
func AcceptRAngle(r rune) (textlexer.Rule, textlexer.State) {
	return rangleMatcher(r)
}

// AcceptComma matches a comma.
func AcceptComma(r rune) (textlexer.Rule, textlexer.State) {
	return commaMatcher(r)
}

// AcceptColon matches a colon.
func AcceptColon(r rune) (textlexer.Rule, textlexer.State) {
	return colonMatcher(r)
}

// AcceptSemicolon matches a semicolon.
func AcceptSemicolon(r rune) (textlexer.Rule, textlexer.State) {
	return semicolonMatcher(r)
}

// AcceptPeriod matches a period.
func AcceptPeriod(r rune) (textlexer.Rule, textlexer.State) {
	return periodMatcher(r)
}

// AcceptPlus matches a plus sign.
func AcceptPlus(r rune) (textlexer.Rule, textlexer.State) {
	return plusMatcher(r)
}

// AcceptMinus matches a minus sign.
func AcceptMinus(r rune) (textlexer.Rule, textlexer.State) {
	return minusMatcher(r)
}

// AcceptStar matches an asterisk.
func AcceptStar(r rune) (textlexer.Rule, textlexer.State) {
	return starMatcher(r)
}

// AcceptSlash matches a forward slash.
func AcceptSlash(r rune) (textlexer.Rule, textlexer.State) {
	return slashMatcher(r)
}

// AcceptPercent matches a percent sign.
func AcceptPercent(r rune) (textlexer.Rule, textlexer.State) {
	return percentMatcher(r)
}

// AcceptEqual matches an equals sign.
func AcceptEqual(r rune) (textlexer.Rule, textlexer.State) {
	return equalMatcher(r)
}

// AcceptExclamation matches an exclamation mark.
func AcceptExclamation(r rune) (textlexer.Rule, textlexer.State) {
	return exclamationMatcher(r)
}

// AcceptPipe matches a pipe symbol.
func AcceptPipe(r rune) (textlexer.Rule, textlexer.State) {
	return pipeMatcher(r)
}

// AcceptAmpersand matches an ampersand.
func AcceptAmpersand(r rune) (textlexer.Rule, textlexer.State) {
	return ampersandMatcher(r)
}

// AcceptQuestionMark matches a question mark.
func AcceptQuestionMark(r rune) (textlexer.Rule, textlexer.State) {
	return questionMatcher(r)
}

// MatchUntilCommonWhitespaceOrEOF matches characters until a common whitespace or EOF.
func MatchUntilCommonWhitespaceOrEOF(r rune) (textlexer.Rule, textlexer.State) {
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
