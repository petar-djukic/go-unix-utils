// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expr implements the GNU expr utility for evaluating expressions.
// Implements prd066-expr R1.1 (arithmetic), R1.2 (comparisons),
// R1.3 (logical operators), R1.4 (parentheses), R2.1 (match/:),
// R2.2 (substr), R2.3 (index), R2.4 (length).
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

type parser struct {
	args []string
	pos  int
}

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "expr: missing operand\n")
		os.Exit(2)
	}
	p := &parser{args: args}
	result, err := p.parseOr()
	if err != nil {
		fmt.Fprintf(os.Stderr, "expr: %s\n", err)
		os.Exit(2)
	}
	if p.pos < len(p.args) {
		fmt.Fprintf(os.Stderr, "expr: syntax error: unexpected argument '%s'\n", p.args[p.pos])
		os.Exit(2)
	}
	fmt.Println(result)
	if isNullOrZero(result) {
		os.Exit(1)
	}
}

// isNullOrZero returns true if the value is empty or "0".
// R4.1, R4.2: determines exit code based on expression result.
func isNullOrZero(s string) bool {
	return s == "" || s == "0"
}

func (p *parser) peek() string {
	if p.pos >= len(p.args) {
		return ""
	}
	return p.args[p.pos]
}

func (p *parser) next() string {
	s := p.args[p.pos]
	p.pos++
	return s
}

// parseOr parses or-expressions (lowest precedence).
// R1.3: | returns first if nonzero/non-empty, else second.
func (p *parser) parseOr() (string, error) {
	left, err := p.parseAnd()
	if err != nil {
		return "", err
	}
	for p.peek() == "|" {
		p.next()
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

// parseAnd parses and-expressions.
// R1.3: & returns first if both nonzero/non-empty, else 0.
func (p *parser) parseAnd() (string, error) {
	left, err := p.parseComp()
	if err != nil {
		return "", err
	}
	for p.peek() == "&" {
		p.next()
		right, err := p.parseComp()
		if err != nil {
			return "", err
		}
		if isNullOrZero(left) || isNullOrZero(right) {
			left = "0"
		}
	}
	return left, nil
}

// parseComp parses comparison expressions.
// R1.2: comparisons use numeric order for two integers, else lexicographic.
func (p *parser) parseComp() (string, error) {
	left, err := p.parseAdd()
	if err != nil {
		return "", err
	}
	for isCompOp(p.peek()) {
		op := p.next()
		right, err := p.parseAdd()
		if err != nil {
			return "", err
		}
		left = evalComp(left, op, right)
	}
	return left, nil
}

func isCompOp(s string) bool {
	switch s {
	case "<", "<=", "=", "!=", ">=", ">":
		return true
	}
	return false
}

// evalComp evaluates a comparison, returning "1" or "0".
// R1.2: numeric comparison if both parse as integers, else lexicographic.
func evalComp(a, op, b string) string {
	ai, errA := strconv.ParseInt(a, 10, 64)
	bi, errB := strconv.ParseInt(b, 10, 64)
	var result bool
	if errA == nil && errB == nil {
		result = compareInts(ai, op, bi)
	} else {
		result = compareStrings(a, op, b)
	}
	if result {
		return "1"
	}
	return "0"
}

func compareInts(a int64, op string, b int64) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "=":
		return a == b
	case "!=":
		return a != b
	case ">=":
		return a >= b
	case ">":
		return a > b
	}
	return false
}

func compareStrings(a, op, b string) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "=":
		return a == b
	case "!=":
		return a != b
	case ">=":
		return a >= b
	case ">":
		return a > b
	}
	return false
}

// parseAdd parses addition and subtraction.
// R1.1: + (add), - (subtract).
func (p *parser) parseAdd() (string, error) {
	left, err := p.parseMul()
	if err != nil {
		return "", err
	}
	for p.peek() == "+" || p.peek() == "-" {
		op := p.next()
		right, err := p.parseMul()
		if err != nil {
			return "", err
		}
		left, err = evalArith(left, op, right)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

// parseMul parses multiplication, division, and modulo.
// R1.1: * (multiply), / (integer divide), % (modulo).
func (p *parser) parseMul() (string, error) {
	left, err := p.parseColon()
	if err != nil {
		return "", err
	}
	for p.peek() == "*" || p.peek() == "/" || p.peek() == "%" {
		op := p.next()
		right, err := p.parseColon()
		if err != nil {
			return "", err
		}
		left, err = evalArith(left, op, right)
		if err != nil {
			return "", err
		}
	}
	return left, nil
}

// evalArith evaluates an arithmetic operation on two string operands.
func evalArith(a, op, b string) (string, error) {
	ai, errA := strconv.ParseInt(a, 10, 64)
	if errA != nil {
		return "", fmt.Errorf("non-integer argument '%s'", a)
	}
	bi, errB := strconv.ParseInt(b, 10, 64)
	if errB != nil {
		return "", fmt.Errorf("non-integer argument '%s'", b)
	}
	result, err := computeArith(ai, op, bi)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(result, 10), nil
}

// computeArith performs the arithmetic operation on two int64 values.
func computeArith(a int64, op string, b int64) (int64, error) {
	switch op {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	case "%":
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a % b, nil
	}
	return 0, fmt.Errorf("unknown operator '%s'", op)
}

// parseColon parses the : (match) operator (highest binary precedence).
// R2.1: STRING : REGEXP anchors pattern at beginning of STRING.
func (p *parser) parseColon() (string, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	for p.peek() == ":" {
		p.next()
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

// parsePrimary parses atoms, parenthesized expressions, and keyword operations.
// R1.4: parentheses. R2.1-R2.4: keyword string operations.
func (p *parser) parsePrimary() (string, error) {
	if p.pos >= len(p.args) {
		return "", fmt.Errorf("syntax error: missing operand")
	}
	switch p.peek() {
	case "(":
		return p.parseGrouped()
	case "match":
		return p.parseKeywordMatch()
	case "substr":
		return p.parseKeywordSubstr()
	case "index":
		return p.parseKeywordIndex()
	case "length":
		return p.parseKeywordLength()
	}
	return p.next(), nil
}

// parseGrouped parses a parenthesized sub-expression.
// R1.4: ( and ) override default precedence.
func (p *parser) parseGrouped() (string, error) {
	p.next() // consume "("
	result, err := p.parseOr()
	if err != nil {
		return "", err
	}
	if p.peek() != ")" {
		return "", fmt.Errorf("syntax error: expecting ')'")
	}
	p.next() // consume ")"
	return result, nil
}

// parseKeywordMatch parses: match STRING REGEXP
// R2.1: anchors pattern at beginning of STRING.
func (p *parser) parseKeywordMatch() (string, error) {
	p.next() // consume "match"
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

// parseKeywordSubstr parses: substr STRING POS LENGTH
// R2.2: returns substring starting at 1-based POS with LENGTH.
func (p *parser) parseKeywordSubstr() (string, error) {
	p.next() // consume "substr"
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

// parseKeywordIndex parses: index STRING CHARS
// R2.3: returns 1-based position of first matching character.
func (p *parser) parseKeywordIndex() (string, error) {
	p.next() // consume "index"
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

// parseKeywordLength parses: length STRING
// R2.4: returns the number of characters in STRING.
func (p *parser) parseKeywordLength() (string, error) {
	p.next() // consume "length"
	str, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	return strconv.Itoa(len(str)), nil
}

// evalMatch performs regex matching for : and match operations.
// R2.1: anchors at beginning, returns captured group or match length.
func evalMatch(str, pattern string) (string, error) {
	goPattern := "^(?:" + breToGoRegex(pattern) + ")"
	re, err := regexp.Compile(goPattern)
	if err != nil {
		return "", fmt.Errorf("syntax error: invalid regex")
	}
	hasCap := hasCapture(pattern)
	m := re.FindStringSubmatch(str)
	if m == nil {
		if hasCap {
			return "", nil
		}
		return "0", nil
	}
	if hasCap && len(m) > 1 {
		return m[1], nil
	}
	return strconv.Itoa(len(m[0])), nil
}

// hasCapture reports whether the BRE pattern contains \( \) groups.
func hasCapture(pattern string) bool {
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '\\' && pattern[i+1] == '(' {
			return true
		}
	}
	return false
}

// breToGoRegex converts a POSIX BRE pattern to Go regexp syntax.
// BRE \( \) become capturing groups; literal ( ) + ? | { } are escaped.
func breToGoRegex(bre string) string {
	var b strings.Builder
	for i := 0; i < len(bre); i++ {
		if bre[i] == '\\' && i+1 < len(bre) {
			i++
			switch bre[i] {
			case '(':
				b.WriteByte('(')
			case ')':
				b.WriteByte(')')
			case '{':
				b.WriteByte('{')
			case '}':
				b.WriteByte('}')
			case '+':
				b.WriteByte('+')
			case '?':
				b.WriteByte('?')
			case '|':
				b.WriteByte('|')
			default:
				b.WriteByte('\\')
				b.WriteByte(bre[i])
			}
			continue
		}
		switch bre[i] {
		case '(':
			b.WriteString("\\(")
		case ')':
			b.WriteString("\\)")
		case '{':
			b.WriteString("\\{")
		case '}':
			b.WriteString("\\}")
		case '+':
			b.WriteString("\\+")
		case '?':
			b.WriteString("\\?")
		case '|':
			b.WriteString("\\|")
		default:
			b.WriteByte(bre[i])
		}
	}
	return b.String()
}

// evalSubstr returns a substring of str starting at 1-based pos.
// R2.2: returns "" for out-of-range or non-integer arguments.
func evalSubstr(str, posStr, lenStr string) string {
	pos, err := strconv.Atoi(posStr)
	if err != nil || pos < 1 {
		return ""
	}
	length, err := strconv.Atoi(lenStr)
	if err != nil || length < 1 {
		return ""
	}
	if pos > len(str) {
		return ""
	}
	end := min(pos-1+length, len(str))
	return str[pos-1 : end]
}

// evalIndex returns the 1-based position of the first byte in str found in chars.
// R2.3: returns "0" if no character matches.
func evalIndex(str, chars string) string {
	for i := 0; i < len(str); i++ {
		if strings.IndexByte(chars, str[i]) >= 0 {
			return strconv.Itoa(i + 1)
		}
	}
	return "0"
}
