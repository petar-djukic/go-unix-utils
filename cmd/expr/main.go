// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd066-expr R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4: expr
// expression evaluation with arithmetic, comparison, logical, string, and
// pattern matching operators, exit code handling, and differential testing.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "expr"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run evaluates the expression given as args and writes the result.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
		return 2
	}
	p := &parser{args: args}
	result, err := p.parseExpr()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 2
	}
	if p.pos < len(p.args) {
		fmt.Fprintf(stderr, "%s: syntax error\n", progName)
		return 2
	}
	fmt.Fprintln(stdout, result)
	return exitCode(result)
}

// exitCode returns 0 if result is non-null and non-zero, 1 otherwise.
// Implements R4.1–R4.2.
func exitCode(result string) int {
	if result == "" || result == "0" {
		return 1
	}
	return 0
}

// parser holds state for recursive descent expression parsing.
// R3.2: precedence from low to high: |, &, comparisons, +/-, */%, :, primary
// R3.3: all binary operators are left-associative.
type parser struct {
	args []string
	pos  int
}

// peek returns the current token without consuming it.
func (p *parser) peek() string {
	if p.pos >= len(p.args) {
		return ""
	}
	return p.args[p.pos]
}

// next consumes and returns the current token.
func (p *parser) next() string {
	tok := p.peek()
	p.pos++
	return tok
}

// parseExpr is the entry point for expression parsing.
func (p *parser) parseExpr() (string, error) {
	return p.parseOr()
}

// parseOr handles the | operator (lowest precedence). R1.3.
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
		left = evalOr(left, right)
	}
	return left, nil
}

// evalOr returns left if non-null/non-zero, else right. R1.3.
func evalOr(left, right string) string {
	if !isNullOrZero(left) {
		return left
	}
	return right
}

// parseAnd handles the & operator. R1.3.
func (p *parser) parseAnd() (string, error) {
	left, err := p.parseCmp()
	if err != nil {
		return "", err
	}
	for p.peek() == "&" {
		p.next()
		right, err := p.parseCmp()
		if err != nil {
			return "", err
		}
		left = evalAnd(left, right)
	}
	return left, nil
}

// evalAnd returns left if both non-null/non-zero, else "0". R1.3.
func evalAnd(left, right string) string {
	if !isNullOrZero(left) && !isNullOrZero(right) {
		return left
	}
	return "0"
}

// isNullOrZero returns true if s is empty or "0".
func isNullOrZero(s string) bool {
	return s == "" || s == "0"
}

// parseCmp handles comparison operators: < <= = != >= >. R1.2.
func (p *parser) parseCmp() (string, error) {
	left, err := p.parseAdd()
	if err != nil {
		return "", err
	}
	for isCmpOp(p.peek()) {
		op := p.next()
		right, err := p.parseAdd()
		if err != nil {
			return "", err
		}
		left = evalCmp(left, op, right)
	}
	return left, nil
}

// isCmpOp returns true if tok is a comparison operator.
func isCmpOp(tok string) bool {
	switch tok {
	case "<", "<=", "=", "!=", ">=", ">":
		return true
	}
	return false
}

// evalCmp evaluates a comparison. Returns "1" for true, "0" for false. R1.2.
func evalCmp(left, op, right string) string {
	li, lok := parseInteger(left)
	ri, rok := parseInteger(right)
	var result bool
	if lok && rok {
		result = cmpInt(li, op, ri)
	} else {
		result = cmpStr(left, op, right)
	}
	if result {
		return "1"
	}
	return "0"
}

// cmpInt compares two integers with the given operator.
func cmpInt(a int64, op string, b int64) bool {
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

// cmpStr compares two strings lexicographically with the given operator.
func cmpStr(a, op, b string) bool {
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

// parseAdd handles + and - arithmetic operators. R1.1.
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
		result, err := evalArith(left, op, right)
		if err != nil {
			return "", err
		}
		left = result
	}
	return left, nil
}

// parseMul handles *, /, % operators. R1.1.
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
		result, err := evalArith(left, op, right)
		if err != nil {
			return "", err
		}
		left = result
	}
	return left, nil
}

// evalArith evaluates an arithmetic operation. R1.1, R3.4.
func evalArith(left, op, right string) (string, error) {
	a, ok := parseInteger(left)
	if !ok {
		return "", fmt.Errorf("non-integer argument")
	}
	b, ok := parseInteger(right)
	if !ok {
		return "", fmt.Errorf("non-integer argument")
	}
	return applyArithOp(a, op, b)
}

// applyArithOp applies the arithmetic operator and returns the result.
func applyArithOp(a int64, op string, b int64) (string, error) {
	switch op {
	case "+":
		return strconv.FormatInt(a+b, 10), nil
	case "-":
		return strconv.FormatInt(a-b, 10), nil
	case "*":
		return strconv.FormatInt(a*b, 10), nil
	case "/":
		if b == 0 {
			return "", fmt.Errorf("division by zero")
		}
		return strconv.FormatInt(a/b, 10), nil
	case "%":
		if b == 0 {
			return "", fmt.Errorf("division by zero")
		}
		return strconv.FormatInt(a%b, 10), nil
	}
	return "", fmt.Errorf("unknown operator '%s'", op)
}

// parseColon handles the : pattern matching operator. R2.1.
// Precedence: higher than * / %, lower than primary.
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
		result, err := evalMatchOp(left, right)
		if err != nil {
			return "", err
		}
		left = result
	}
	return left, nil
}

// parsePrimary handles atoms, parenthesized expressions, and keywords.
// R1.4, R2.1–R2.4, R3.1.
func (p *parser) parsePrimary() (string, error) {
	if p.pos >= len(p.args) {
		return "", fmt.Errorf("syntax error")
	}
	switch p.peek() {
	case "(":
		return p.parseParenExpr()
	case "+":
		return p.parsePlusEscape()
	case "match":
		return p.parseMatchKw()
	case "substr":
		return p.parseSubstrKw()
	case "index":
		return p.parseIndexKw()
	case "length":
		return p.parseLengthKw()
	}
	return p.next(), nil
}

// parseParenExpr parses a parenthesized sub-expression. R1.4.
func (p *parser) parseParenExpr() (string, error) {
	p.next() // consume "("
	result, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	if p.peek() != ")" {
		return "", fmt.Errorf("syntax error")
	}
	p.next() // consume ")"
	return result, nil
}

// parsePlusEscape handles R3.1: + TOKEN treats next token as string.
func (p *parser) parsePlusEscape() (string, error) {
	p.next() // consume "+"
	if p.pos >= len(p.args) {
		return "", fmt.Errorf("syntax error")
	}
	return p.next(), nil
}

// parseMatchKw handles: match STRING REGEX. R2.1.
func (p *parser) parseMatchKw() (string, error) {
	p.next() // consume "match"
	str, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	pattern, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	return evalMatchOp(str, pattern)
}

// parseSubstrKw handles: substr STRING POS LENGTH. R2.2.
func (p *parser) parseSubstrKw() (string, error) {
	p.next() // consume "substr"
	str, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	posStr, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	lenStr, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	return evalSubstr(str, posStr, lenStr)
}

// parseIndexKw handles: index STRING CHARS. R2.3.
func (p *parser) parseIndexKw() (string, error) {
	p.next() // consume "index"
	str, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	chars, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	return evalIndex(str, chars), nil
}

// parseLengthKw handles: length STRING. R2.4.
func (p *parser) parseLengthKw() (string, error) {
	p.next() // consume "length"
	str, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	return evalLength(str), nil
}

// evalMatchOp performs pattern matching with BRE. R2.1.
// Anchors the pattern at the start of the string.
func evalMatchOp(str, pattern string) (string, error) {
	goPattern, hasGroups := breToGo(pattern)
	re, err := regexp.Compile("^" + goPattern)
	if err != nil {
		return "", fmt.Errorf("invalid regular expression")
	}
	return matchResult(re, str, hasGroups), nil
}

// matchResult extracts the result from a regex match.
// With groups: returns first group or "". Without: returns match length or "0".
func matchResult(re *regexp.Regexp, str string, hasGroups bool) string {
	match := re.FindStringSubmatch(str)
	if match == nil {
		if hasGroups {
			return ""
		}
		return "0"
	}
	if hasGroups && len(match) > 1 {
		return match[1]
	}
	return strconv.Itoa(len(match[0]))
}

// breToGo converts a POSIX BRE pattern to Go regexp syntax.
// Returns the converted pattern and whether it contains \(\) groups.
func breToGo(pattern string) (string, bool) {
	var result strings.Builder
	hasGroups := false
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			if convertBreEscape(pattern[i+1], &result) {
				hasGroups = true
			}
			i++
		} else {
			convertBreLiteral(pattern[i], &result)
		}
	}
	return result.String(), hasGroups
}

// convertBreEscape converts a BRE escape sequence to Go regexp.
// Returns true if the escape introduces a group.
func convertBreEscape(ch byte, w *strings.Builder) bool {
	switch ch {
	case '(':
		w.WriteByte('(')
		return true
	case ')':
		w.WriteByte(')')
	case '{':
		w.WriteByte('{')
	case '}':
		w.WriteByte('}')
	case '+', '?', '|':
		w.WriteByte(ch)
	default:
		w.WriteByte('\\')
		w.WriteByte(ch)
	}
	return false
}

// convertBreLiteral converts a BRE literal character to Go regexp.
func convertBreLiteral(ch byte, w *strings.Builder) {
	switch ch {
	case '(', ')', '+', '?', '|', '{', '}':
		w.WriteByte('\\')
		w.WriteByte(ch)
	default:
		w.WriteByte(ch)
	}
}

// evalSubstr extracts a substring. R2.2.
// POS is 1-based. Returns empty string for out-of-range values.
func evalSubstr(str, posStr, lenStr string) (string, error) {
	pos, ok := parseInteger(posStr)
	if !ok {
		return "", fmt.Errorf("non-integer argument")
	}
	length, ok := parseInteger(lenStr)
	if !ok {
		return "", fmt.Errorf("non-integer argument")
	}
	if pos < 1 || length < 1 || int(pos) > len(str) {
		return "", nil
	}
	return extractSubstr(str, int(pos), int(length)), nil
}

// extractSubstr performs substring extraction with bounds clamping.
func extractSubstr(str string, pos, length int) string {
	start := pos - 1
	end := min(start+length, len(str))
	return str[start:end]
}

// evalIndex returns the 1-based position of the first character in str
// that appears in chars, or "0" if none. R2.3.
func evalIndex(str, chars string) string {
	for i, ch := range str {
		if strings.ContainsRune(chars, ch) {
			return strconv.Itoa(i + 1)
		}
	}
	return "0"
}

// evalLength returns the byte length of str as a string. R2.4.
func evalLength(str string) string {
	return strconv.Itoa(len(str))
}

// parseInteger checks if s is a valid integer per GNU expr rules
// (optional leading minus, then one or more digits) and returns it.
func parseInteger(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	start := 0
	if s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return 0, false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
