// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd066-expr R1.1–R1.4: expr basic expression evaluation
// with arithmetic, comparison, logical operators, and parentheses.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "expr"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run evaluates the expression given as args and writes the result.
// R1.1–R1.4: arithmetic, comparison, logical operators, parentheses.
// R4.1–R4.3: exit code semantics.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", progName)
		return 2
	}
	// TODO: --help/--version skipped — conflicts with non_goals:
	// "cmd/expr does not implement GNU expr's --help and --version"
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
// R3.2: precedence from low to high: |, &, comparisons, +/-, */%
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

// parseOr handles the | operator (lowest precedence).
// R1.3: returns first arg if non-null/non-zero, else second.
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

// parseAnd handles the & operator.
// R1.3: returns first arg if both non-null/non-zero, else 0.
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

// parseCmp handles comparison operators: < <= = != >= >.
// R1.2: numeric comparison if both integers, else lexicographic.
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

// evalCmp evaluates a comparison. Returns "1" for true, "0" for false.
// R1.2: uses numeric comparison when both operands are integers.
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
	left, err := p.parsePrimary()
	if err != nil {
		return "", err
	}
	for p.peek() == "*" || p.peek() == "/" || p.peek() == "%" {
		op := p.next()
		right, err := p.parsePrimary()
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

// evalArith evaluates an arithmetic operation on two integer operands.
// R1.1: +, -, *, /, %. R3.4: division by zero returns error.
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

// parsePrimary handles atoms and parenthesized expressions.
// R1.4: ( expr ) groups for precedence override.
func (p *parser) parsePrimary() (string, error) {
	if p.pos >= len(p.args) {
		return "", fmt.Errorf("syntax error")
	}
	if p.peek() == "(" {
		return p.parseParenExpr()
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
