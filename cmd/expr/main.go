// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/expr: evaluate expressions.
// Implements srd066-expr R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "expr"

// helpText is printed when --help is given as the sole argument.
const helpText = `Usage: expr EXPRESSION
  or:  expr OPTION

      --help        display this help and exit
      --version     output version information and exit

Print the value of EXPRESSION to standard output.  A blank line below
separates increasing precedence groups.  EXPRESSION may be:

  ARG1 | ARG2       ARG1 if it is neither null nor 0, otherwise ARG2

  ARG1 & ARG2       ARG1 if neither argument is null or 0, otherwise 0

  ARG1 < ARG2       ARG1 is less than ARG2
  ARG1 <= ARG2      ARG1 is less than or equal to ARG2
  ARG1 = ARG2       ARG1 is equal to ARG2
  ARG1 != ARG2      ARG1 is not equal to ARG2
  ARG1 >= ARG2      ARG1 is greater than or equal to ARG2
  ARG1 > ARG2       ARG1 is greater than ARG2

  ARG1 + ARG2       arithmetic sum of ARG1 and ARG2
  ARG1 - ARG2       arithmetic difference of ARG1 and ARG2

  ARG1 * ARG2       arithmetic product of ARG1 and ARG2
  ARG1 / ARG2       arithmetic quotient of ARG1 divided by ARG2
  ARG1 %% ARG2       arithmetic remainder of ARG1 divided by ARG2

  ( EXPRESSION )    value of EXPRESSION
`

// versionText is printed when --version is given as the sole argument.
const versionText = "expr (go-unix-utils) 1.0\n"

// R1: Token types covering all expr operators and value types.
// R3.2: Precedence from lowest to highest: |, &, comparisons, +/-, */%.
type tokenType int

const (
	tokNumber tokenType = iota
	tokString
	tokOr  // |
	tokAnd // &
	tokLT  // <
	tokLE  // <=
	tokEQ  // =
	tokNE  // !=
	tokGE  // >=
	tokGT  // >
	tokAdd // +
	tokSub // -
	tokMul // *
	tokDiv // /
	tokMod // %
	tokMatch  // match
	tokSubstr // substr
	tokIndex  // index
	tokLength // length
	tokColon  // :
	tokLParen // (
	tokRParen // )
	tokEOF
)

// token holds a classified token type and its original string value.
type token struct {
	typ tokenType
	val string
}

// parser holds the token stream and current read position.
type parser struct {
	tokens []token
	pos    int
}

// R1.1-R1.4: tokenize converts command-line arguments into typed tokens.
func tokenize(args []string) []token {
	tokens := make([]token, 0, len(args)+1)
	for _, arg := range args {
		tokens = append(tokens, classifyToken(arg))
	}
	tokens = append(tokens, token{typ: tokEOF})
	return tokens
}

// classifyToken maps a single argument string to its token type.
func classifyToken(arg string) token {
	switch arg {
	case "|":
		return token{tokOr, arg}
	case "&":
		return token{tokAnd, arg}
	case "<":
		return token{tokLT, arg}
	case "<=":
		return token{tokLE, arg}
	case "=":
		return token{tokEQ, arg}
	case "!=":
		return token{tokNE, arg}
	case ">=":
		return token{tokGE, arg}
	case ">":
		return token{tokGT, arg}
	case "+":
		return token{tokAdd, arg}
	case "-":
		return token{tokSub, arg}
	case "*":
		return token{tokMul, arg}
	case "/":
		return token{tokDiv, arg}
	case "%":
		return token{tokMod, arg}
	case "(":
		return token{tokLParen, arg}
	case ")":
		return token{tokRParen, arg}
	case ":":
		return token{tokColon, arg}
	case "match":
		return token{tokMatch, arg}
	case "substr":
		return token{tokSubstr, arg}
	case "index":
		return token{tokIndex, arg}
	case "length":
		return token{tokLength, arg}
	}
	if isIntegerLiteral(arg) {
		return token{tokNumber, arg}
	}
	return token{tokString, arg}
}

// isIntegerLiteral returns true if s represents an integer (digits with
// optional leading minus).
func isIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// toInt converts a string to int64. Returns (0, false) if not an integer.
func toInt(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

func (p *parser) peek() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{typ: tokEOF}
}

func (p *parser) advance() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

// R1.3: | returns first arg if non-null/non-zero, else second.
func (p *parser) parseOr() (string, error) {
	left, err := p.parseAnd()
	if err != nil {
		return "", err
	}
	for p.peek().typ == tokOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return "", err
		}
		if isNullOrZero(left) {
			left = right
		}
	}
	return left, nil
}

// R1.3: & returns first arg if both non-null/non-zero, else 0.
func (p *parser) parseAnd() (string, error) {
	left, err := p.parseComparison()
	if err != nil {
		return "", err
	}
	for p.peek().typ == tokAnd {
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return "", err
		}
		if isNullOrZero(left) || isNullOrZero(right) {
			left = "0"
		}
	}
	return left, nil
}

func isComparisonOp(t tokenType) bool {
	return t >= tokLT && t <= tokGT
}

// R1.2: comparison operators. Numeric if both are integers, else lexicographic.
func (p *parser) parseComparison() (string, error) {
	left, err := p.parseAddSub()
	if err != nil {
		return "", err
	}
	for isComparisonOp(p.peek().typ) {
		op := p.advance()
		right, err := p.parseAddSub()
		if err != nil {
			return "", err
		}
		left = evalCompare(left, op.typ, right)
	}
	return left, nil
}

// R1.1: additive arithmetic operators (+ -).
func (p *parser) parseAddSub() (string, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return "", err
	}
	for p.peek().typ == tokAdd || p.peek().typ == tokSub {
		op := p.advance()
		right, err := p.parseMulDiv()
		if err != nil {
			return "", err
		}
		left, err = evalArithmetic(left, op.typ, right)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

// R1.1: multiplicative arithmetic operators (* / %).
func (p *parser) parseMulDiv() (string, error) {
	left, err := p.parseColonMatch()
	if err != nil {
		return "", err
	}
	for isMulDivOp(p.peek().typ) {
		op := p.advance()
		right, err := p.parseColonMatch()
		if err != nil {
			return "", err
		}
		left, err = evalArithmetic(left, op.typ, right)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

func isMulDivOp(t tokenType) bool {
	return t == tokMul || t == tokDiv || t == tokMod
}

// R2.1: STRING : REGEXP — infix match operator, higher precedence than * / %.
func (p *parser) parseColonMatch() (string, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	for p.peek().typ == tokColon {
		p.advance()
		right, err := p.parsePrimary()
		if err != nil {
			return "", err
		}
		left, err = evalMatch(left, right)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

// R1.4, R2.1-R2.4, R3.1: parse primary values — numbers, strings,
// parenthesized expressions, + escaping, and string keyword operations.
func (p *parser) parsePrimary() (string, error) {
	tok := p.peek()
	switch tok.typ {
	case tokLParen:
		return p.parseParenExpr()
	case tokAdd:
		return p.parsePlusEscape()
	case tokNumber, tokString:
		p.advance()
		return tok.val, nil
	case tokMatch:
		return p.parseMatchKeyword()
	case tokSubstr:
		return p.parseSubstr()
	case tokIndex:
		return p.parseIndex()
	case tokLength:
		return p.parseLength()
	case tokEOF:
		return "", fmt.Errorf("syntax error")
	default:
		return "", fmt.Errorf("syntax error")
	}
}

// R3.1: + TOKEN treats TOKEN as a string literal, consuming the +.
func (p *parser) parsePlusEscape() (string, error) {
	p.advance() // consume +
	tok := p.peek()
	if tok.typ == tokEOF {
		return "", fmt.Errorf("syntax error")
	}
	p.advance()
	return tok.val, nil
}

// R1.4: parse a parenthesized sub-expression.
func (p *parser) parseParenExpr() (string, error) {
	p.advance() // consume (
	result, err := p.parseOr()
	if err != nil {
		return "", err
	}
	if p.peek().typ != tokRParen {
		return "", fmt.Errorf("syntax error")
	}
	p.advance() // consume )
	return result, nil
}

// R2.1: match STRING REGEXP — keyword form of match operator.
func (p *parser) parseMatchKeyword() (string, error) {
	p.advance() // consume "match"
	str, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	pattern, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	return evalMatch(str, pattern)
}

// R2.1: evalMatch anchors REGEXP at start, converts BRE groups to RE2,
// and returns matched group or character count.
func evalMatch(str, pattern string) (string, error) {
	goPattern, hasGroup := breToRE2(pattern)
	anchored := "^(?:" + goPattern + ")"
	re, err := regexp.Compile(anchored)
	if err != nil {
		if hasGroup {
			return "", nil
		}
		return "0", nil
	}
	m := re.FindStringSubmatch(str)
	if m == nil {
		if hasGroup {
			return "", nil
		}
		return "0", nil
	}
	if hasGroup {
		return m[1], nil
	}
	return strconv.Itoa(len(m[0])), nil
}

// breToRE2 converts POSIX BRE \( \) groups to RE2 ( ) groups.
// Returns the converted pattern and whether any group was found.
func breToRE2(pattern string) (string, bool) {
	var b strings.Builder
	hasGroup := false
	for i := 0; i < len(pattern); i++ {
		if i+1 < len(pattern) && pattern[i] == '\\' {
			next := pattern[i+1]
			if next == '(' || next == ')' {
				b.WriteByte(next)
				if next == '(' {
					hasGroup = true
				}
				i++
				continue
			}
		}
		b.WriteByte(pattern[i])
	}
	return b.String(), hasGroup
}

// R2.2: substr STRING POS LENGTH — extract substring with 1-based indexing.
func (p *parser) parseSubstr() (string, error) {
	p.advance() // consume "substr"
	str, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	posStr, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	lenStr, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	return evalSubstr(str, posStr, lenStr), nil
}

// evalSubstr returns the substring at 1-based position with given length.
// Returns empty string if pos or length is non-positive or pos exceeds length.
func evalSubstr(str, posStr, lenStr string) string {
	pos, ok := toInt(posStr)
	if !ok || pos <= 0 {
		return ""
	}
	length, ok := toInt(lenStr)
	if !ok || length <= 0 {
		return ""
	}
	if int(pos) > len(str) {
		return ""
	}
	start := int(pos) - 1
	end := min(start+int(length), len(str))
	return str[start:end]
}

// R2.3: index STRING CHARS — return 1-based position of first match.
func (p *parser) parseIndex() (string, error) {
	p.advance() // consume "index"
	str, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	chars, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	return evalIndex(str, chars), nil
}

// evalIndex finds the first character in str that appears in chars.
// Returns 1-based position or "0" if not found.
func evalIndex(str, chars string) string {
	for i, ch := range str {
		if strings.ContainsRune(chars, ch) {
			return strconv.Itoa(i + 1)
		}
	}
	return "0"
}

// R2.4: length STRING — return character count.
func (p *parser) parseLength() (string, error) {
	p.advance() // consume "length"
	str, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	return strconv.Itoa(len(str)), nil
}

// R1.2: evalCompare compares two values. Uses numeric comparison when
// both are integers, otherwise lexicographic comparison.
func evalCompare(left string, op tokenType, right string) string {
	var result bool
	li, lok := toInt(left)
	ri, rok := toInt(right)
	if lok && rok {
		result = compareInts(li, op, ri)
	} else {
		result = compareStrings(left, op, right)
	}
	if result {
		return "1"
	}
	return "0"
}

func compareInts(l int64, op tokenType, r int64) bool {
	switch op {
	case tokLT:
		return l < r
	case tokLE:
		return l <= r
	case tokEQ:
		return l == r
	case tokNE:
		return l != r
	case tokGE:
		return l >= r
	case tokGT:
		return l > r
	}
	return false
}

func compareStrings(l string, op tokenType, r string) bool {
	switch op {
	case tokLT:
		return l < r
	case tokLE:
		return l <= r
	case tokEQ:
		return l == r
	case tokNE:
		return l != r
	case tokGE:
		return l >= r
	case tokGT:
		return l > r
	}
	return false
}

// R1.1: evalArithmetic evaluates a binary arithmetic operation.
// Returns error for non-integer operands or division by zero.
func evalArithmetic(left string, op tokenType, right string) (string, error) {
	l, ok := toInt(left)
	if !ok {
		return "", fmt.Errorf("non-integer argument")
	}
	r, ok := toInt(right)
	if !ok {
		return "", fmt.Errorf("non-integer argument")
	}
	return computeArithmetic(l, op, r)
}

// computeArithmetic performs the actual integer operation.
// R3.4: division by zero prints error and exits 2.
func computeArithmetic(l int64, op tokenType, r int64) (string, error) {
	switch op {
	case tokAdd:
		return strconv.FormatInt(l+r, 10), nil
	case tokSub:
		return strconv.FormatInt(l-r, 10), nil
	case tokMul:
		return strconv.FormatInt(l*r, 10), nil
	case tokDiv:
		if r == 0 {
			return "", fmt.Errorf("division by zero")
		}
		return strconv.FormatInt(l/r, 10), nil
	case tokMod:
		if r == 0 {
			return "", fmt.Errorf("division by zero")
		}
		return strconv.FormatInt(l%r, 10), nil
	}
	return "", fmt.Errorf("syntax error")
}

// evaluate tokenizes and parses the argument list, returning the result.
func evaluate(args []string) (string, error) {
	tokens := tokenize(args)
	p := &parser{tokens: tokens}
	result, err := p.parseOr()
	if err != nil {
		return "", err
	}
	if p.peek().typ != tokEOF {
		return "", fmt.Errorf("syntax error")
	}
	return result, nil
}

// isNullOrZero returns true if the result is null (empty string) or "0".
// R4.1, R4.2: used to determine exit code 0 vs 1.
func isNullOrZero(result string) bool {
	return result == "" || result == "0"
}

// D3: Call sys.InstallSIGPIPEHandler() at the start of main per shared protocol.
// R4: Exit 0 for non-null/non-zero, 1 for null/zero, 2 for syntax error.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the expr logic and returns the exit code.
func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr,
			"Try '%s --help' for more information.\n", progName)
		return 2
	}
	if code := handleOption(args); code >= 0 {
		return code
	}
	result, err := evaluate(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 2
	}
	fmt.Println(result)
	if isNullOrZero(result) {
		return 1
	}
	return 0
}

// handleOption checks for --help/--version as the sole argument.
// Returns exit code >= 0 if handled, -1 to continue normal evaluation.
func handleOption(args []string) int {
	if len(args) != 1 {
		return -1
	}
	switch args[0] {
	case "--help":
		fmt.Print(helpText)
		return 0
	case "--version":
		fmt.Print(versionText)
		return 0
	}
	return -1
}
