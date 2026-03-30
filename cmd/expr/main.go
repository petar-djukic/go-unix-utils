// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expr implements the GNU expr utility for evaluating expressions.
// Implements prd066-expr R1.1 (arithmetic), R1.2 (comparisons),
// R1.3 (logical operators), R1.4 (parentheses).
package main

import (
	"fmt"
	"os"
	"strconv"

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

// parsePrimary parses atoms and parenthesized expressions.
// R1.4: ( and ) override default precedence.
func (p *parser) parsePrimary() (string, error) {
	if p.pos >= len(p.args) {
		return "", fmt.Errorf("syntax error: missing operand")
	}
	if p.peek() == "(" {
		p.next()
		result, err := p.parseOr()
		if err != nil {
			return "", err
		}
		if p.peek() != ")" {
			return "", fmt.Errorf("syntax error: expecting ')'")
		}
		p.next()
		return result, nil
	}
	return p.next(), nil
}
