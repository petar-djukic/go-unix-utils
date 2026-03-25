// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expr evaluates arithmetic, comparison, logical, and string expressions.
// Implements prd066-expr R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// exprErr is the panic type for expression evaluation errors.
type exprErr struct {
	msg  string
	code int
}

// parser holds the argument list and current parse position.
type parser struct {
	args []string
	pos  int
}

func main() {
	sys.InstallSIGPIPEHandler()
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "expr: missing operand")
		os.Exit(2)
	}
	p := &parser{args: os.Args[1:]}
	os.Exit(run(p))
}

// run evaluates the expression, recovering from exprErr panics.
func run(p *parser) (code int) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*exprErr); ok {
				fmt.Fprintln(os.Stderr, e.msg)
				code = e.code
				return
			}
			panic(r)
		}
	}()
	result := p.parseOr()
	if p.pos < len(p.args) {
		fail(fmt.Sprintf("syntax error: unexpected argument '%s'",
			p.args[p.pos]))
	}
	fmt.Println(result)
	return exitCode(result)
}

// exitCode returns 0 for non-null/non-zero, 1 otherwise.
// R4.1: exit 0 when non-null and non-zero.
// R4.2: exit 1 when null or "0".
func exitCode(result string) int {
	if result == "" || result == "0" {
		return 1
	}
	return 0
}

// fail panics with an exprErr causing exit code 2. R4.3.
func fail(msg string) {
	panic(&exprErr{msg: "expr: " + msg, code: 2})
}

// peek returns the current token without consuming it, or "" at end.
func (p *parser) peek() string {
	if p.pos >= len(p.args) {
		return ""
	}
	return p.args[p.pos]
}

// next consumes and returns the current token.
func (p *parser) next() string {
	if p.pos >= len(p.args) {
		fail("syntax error")
	}
	tok := p.args[p.pos]
	p.pos++
	return tok
}

// R1.3: | returns first arg if non-null/non-zero, else second.
func (p *parser) parseOr() string {
	left := p.parseAnd()
	for p.peek() == "|" {
		p.next()
		right := p.parseAnd()
		if isNullOrZero(left) {
			left = right
		}
	}
	return left
}

// R1.3: & returns first arg if neither is null/zero, else "0".
func (p *parser) parseAnd() string {
	left := p.parseCompare()
	for p.peek() == "&" {
		p.next()
		right := p.parseCompare()
		if isNullOrZero(left) || isNullOrZero(right) {
			left = "0"
		}
	}
	return left
}

// R1.2: comparison operators with numeric or lexicographic semantics.
func (p *parser) parseCompare() string {
	left := p.parseAdd()
	for isCompareOp(p.peek()) {
		op := p.next()
		right := p.parseAdd()
		left = compareValues(left, op, right)
	}
	return left
}

// R1.1: addition and subtraction.
func (p *parser) parseAdd() string {
	left := p.parseMul()
	for p.peek() == "+" || p.peek() == "-" {
		op := p.next()
		right := p.parseMul()
		l, r := requireInt(left), requireInt(right)
		if op == "+" {
			left = strconv.FormatInt(l+r, 10)
		} else {
			left = strconv.FormatInt(l-r, 10)
		}
	}
	return left
}

// R1.1: multiplication, division, modulo.
func (p *parser) parseMul() string {
	left := p.parseColon()
	for p.peek() == "*" || p.peek() == "/" || p.peek() == "%" {
		op := p.next()
		right := p.parseColon()
		l, r := requireInt(left), requireInt(right)
		left = evalMulOp(l, r, op)
	}
	return left
}

// evalMulOp performs a single *, /, or % operation. R3.4: div-by-zero check.
func evalMulOp(l, r int64, op string) string {
	switch op {
	case "*":
		return strconv.FormatInt(l*r, 10)
	case "/":
		if r == 0 {
			fail("division by zero")
		}
		return strconv.FormatInt(l/r, 10)
	default: // %
		if r == 0 {
			fail("division by zero")
		}
		return strconv.FormatInt(l%r, 10)
	}
}

// R2.1: the : (colon) operator for regex matching.
func (p *parser) parseColon() string {
	left := p.parseAtom()
	for p.peek() == ":" {
		p.next()
		right := p.parseAtom()
		left = doMatch(left, right)
	}
	return left
}

// parseAtom handles atoms: parens, keywords, + escape, literals.
// R1.4: parentheses. R2.1-R2.4: match/substr/index/length. R3.1: + escape.
func (p *parser) parseAtom() string {
	tok := p.next()
	switch tok {
	case "(":
		return p.parseParenExpr()
	case "+":
		return p.next()
	case "match":
		s, pattern := p.parseAtom(), p.parseAtom()
		return doMatch(s, pattern)
	case "substr":
		s, pos, length := p.parseAtom(), p.parseAtom(), p.parseAtom()
		return doSubstr(s, pos, length)
	case "index":
		s, chars := p.parseAtom(), p.parseAtom()
		return doIndex(s, chars)
	case "length":
		return strconv.Itoa(len(p.parseAtom()))
	default:
		return tok
	}
}

// parseParenExpr evaluates a parenthesized sub-expression. R1.4.
func (p *parser) parseParenExpr() string {
	result := p.parseOr()
	if p.next() != ")" {
		fail("syntax error: expected ')'")
	}
	return result
}

// isCompareOp returns true for comparison operator tokens.
func isCompareOp(s string) bool {
	switch s {
	case "<", "<=", "=", "!=", ">=", ">":
		return true
	}
	return false
}

// compareValues performs numeric or lexicographic comparison. R1.2.
func compareValues(left, op, right string) string {
	cmp := strings.Compare(left, right)
	if isInteger(left) && isInteger(right) {
		cmp = intCmp(toInt(left), toInt(right))
	}
	return boolToResult(applyCmp(cmp, op))
}

// intCmp returns -1, 0, or 1 for two int64 values.
func intCmp(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// applyCmp applies a comparison operator to a cmp result (-1, 0, or 1).
func applyCmp(cmp int, op string) bool {
	switch op {
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "=":
		return cmp == 0
	case "!=":
		return cmp != 0
	case ">=":
		return cmp >= 0
	default: // >
		return cmp > 0
	}
}

// boolToResult converts a boolean comparison result to "1" or "0".
func boolToResult(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// isNullOrZero returns true if the value is empty or "0".
func isNullOrZero(s string) bool {
	return s == "" || s == "0"
}

// isInteger returns true if s matches an optional minus followed by digits.
func isInteger(s string) bool {
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

// toInt parses a validated integer string to int64.
func toInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// requireInt parses s as an integer, failing on non-integer input.
func requireInt(s string) int64 {
	if !isInteger(s) {
		fail("non-integer argument")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		fail("non-integer argument")
	}
	return n
}

// doMatch performs anchored BRE matching. R2.1.
func doMatch(s, pattern string) string {
	grouped := hasGroups(pattern)
	ere := "^" + breToERE(pattern)
	re, err := regexp.Compile(ere)
	if err != nil {
		fail(fmt.Sprintf("invalid regular expression '%s'", pattern))
	}
	m := re.FindStringSubmatch(s)
	if m == nil {
		return matchDefault(grouped)
	}
	if grouped && len(m) > 1 {
		return m[1]
	}
	return strconv.Itoa(len(m[0]))
}

// matchDefault returns the default for a failed match: "" with groups, "0" without.
func matchDefault(grouped bool) string {
	if grouped {
		return ""
	}
	return "0"
}

// hasGroups checks if a BRE pattern contains \( grouping.
func hasGroups(pattern string) bool {
	for i := 0; i+1 < len(pattern); i++ {
		if pattern[i] == '\\' {
			if pattern[i+1] == '(' {
				return true
			}
			i++ // skip escaped char
		}
	}
	return false
}

// breToERE converts a POSIX basic regex to Go (ERE-like) syntax.
func breToERE(pattern string) string {
	var b strings.Builder
	b.Grow(len(pattern))
	for i := 0; i < len(pattern); i++ {
		if i+1 < len(pattern) && pattern[i] == '\\' {
			i++
			writeEscaped(&b, pattern[i])
		} else {
			writeUnescaped(&b, pattern[i])
		}
	}
	return b.String()
}

// writeEscaped handles a character after backslash in BRE-to-ERE conversion.
func writeEscaped(b *strings.Builder, ch byte) {
	switch ch {
	case '(', ')', '+', '?', '|', '{', '}':
		b.WriteByte(ch) // BRE special → ERE metachar
	default:
		b.WriteByte('\\')
		b.WriteByte(ch)
	}
}

// writeUnescaped handles an unescaped character in BRE-to-ERE conversion.
func writeUnescaped(b *strings.Builder, ch byte) {
	switch ch {
	case '(', ')', '+', '?', '|', '{', '}':
		b.WriteByte('\\')
		b.WriteByte(ch) // literal in BRE → escaped in ERE
	default:
		b.WriteByte(ch)
	}
}

// doSubstr returns a substring starting at pos (1-based) with given length. R2.2.
func doSubstr(s, posStr, lenStr string) string {
	pos := requireInt(posStr)
	length := requireInt(lenStr)
	if pos < 1 || length <= 0 || int(pos) > len(s) {
		return ""
	}
	start := int(pos) - 1
	end := start + int(length)
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

// doIndex returns 1-based position of first char in s found in chars. R2.3.
func doIndex(s, chars string) string {
	for i, ch := range s {
		if strings.ContainsRune(chars, ch) {
			return strconv.Itoa(i + 1)
		}
	}
	return "0"
}
