// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

var args []string
var pos int

func main() {
	sys.InstallSIGPIPEHandler()

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "expr: missing operand")
		os.Exit(2)
	}

	args = os.Args[1:]
	pos = 0

	result := parseOr()

	if pos != len(args) {
		fmt.Fprintf(os.Stderr, "expr: syntax error: unexpected argument '%s'\n", args[pos])
		os.Exit(2)
	}

	fmt.Println(result)

	if isNullOrZero(result) {
		os.Exit(1)
	}
	os.Exit(0)
}

func isNullOrZero(s string) bool {
	if s == "" {
		return true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n == 0
}

func peek() string {
	if pos >= len(args) {
		return ""
	}
	return args[pos]
}

func advance() string {
	tok := args[pos]
	pos++
	return tok
}

func expectArg() {
	if pos >= len(args) {
		fmt.Fprintf(os.Stderr, "expr: syntax error: missing argument after '%s'\n", args[pos-1])
		os.Exit(2)
	}
}

func parseOr() string {
	left := parseAnd()
	for peek() == "|" {
		advance()
		expectArg()
		right := parseAnd()
		if !isNullOrZero(left) {
			continue
		}
		left = right
	}
	return left
}

func parseAnd() string {
	left := parseComparison()
	for peek() == "&" {
		advance()
		expectArg()
		right := parseComparison()
		if !isNullOrZero(left) && !isNullOrZero(right) {
			continue
		}
		left = "0"
	}
	return left
}

func parseComparison() string {
	left := parseAddSub()
	for {
		op := peek()
		switch op {
		case "<", "<=", "=", "!=", ">=", ">":
			advance()
			expectArg()
			right := parseAddSub()
			left = evalComparison(left, op, right)
		default:
			return left
		}
	}
}

func evalComparison(left, op, right string) string {
	li, lErr := strconv.ParseInt(left, 10, 64)
	ri, rErr := strconv.ParseInt(right, 10, 64)

	var cmp int
	if lErr == nil && rErr == nil {
		switch {
		case li < ri:
			cmp = -1
		case li > ri:
			cmp = 1
		default:
			cmp = 0
		}
	} else {
		cmp = strings.Compare(left, right)
	}

	var result bool
	switch op {
	case "<":
		result = cmp < 0
	case "<=":
		result = cmp <= 0
	case "=":
		result = cmp == 0
	case "!=":
		result = cmp != 0
	case ">=":
		result = cmp >= 0
	case ">":
		result = cmp > 0
	}

	if result {
		return "1"
	}
	return "0"
}

func parseAddSub() string {
	left := parseMulDiv()
	for {
		op := peek()
		if op != "+" && op != "-" {
			return left
		}
		advance()
		expectArg()
		right := parseMulDiv()
		left = evalArithmetic(left, op, right)
	}
}

func parseMulDiv() string {
	left := parsePrimary()
	for {
		switch op := peek(); op {
		case ":":
			advance()
			expectArg()
			pattern := parsePrimary()
			left = evalMatch(left, pattern)
		case "*", "/", "%":
			advance()
			expectArg()
			right := parsePrimary()
			left = evalArithmetic(left, op, right)
		default:
			return left
		}
	}
}

func evalArithmetic(left, op, right string) string {
	l, lErr := strconv.ParseInt(left, 10, 64)
	r, rErr := strconv.ParseInt(right, 10, 64)

	if lErr != nil {
		fmt.Fprintf(os.Stderr, "expr: non-integer argument '%s'\n", left)
		os.Exit(2)
	}
	if rErr != nil {
		fmt.Fprintf(os.Stderr, "expr: non-integer argument '%s'\n", right)
		os.Exit(2)
	}

	switch op {
	case "+":
		return strconv.FormatInt(l+r, 10)
	case "-":
		return strconv.FormatInt(l-r, 10)
	case "*":
		return strconv.FormatInt(l*r, 10)
	case "/":
		if r == 0 {
			fmt.Fprintln(os.Stderr, "expr: division by zero")
			os.Exit(2)
		}
		return strconv.FormatInt(l/r, 10)
	case "%":
		if r == 0 {
			fmt.Fprintln(os.Stderr, "expr: division by zero")
			os.Exit(2)
		}
		return strconv.FormatInt(l%r, 10)
	}
	return "0"
}

func parsePrimary() string {
	if pos >= len(args) {
		fmt.Fprintln(os.Stderr, "expr: syntax error: missing argument")
		os.Exit(2)
	}

	tok := peek()

	if tok == "(" {
		advance()
		result := parseOr()
		if peek() != ")" {
			fmt.Fprintln(os.Stderr, "expr: syntax error: expecting ')'")
			os.Exit(2)
		}
		advance()
		return result
	}

	// R3.1: + TOKEN treats TOKEN as a literal string
	if tok == "+" {
		advance()
		if pos >= len(args) {
			return "+"
		}
		return advance()
	}

	if tok == "match" {
		return parseMatch()
	}
	if tok == "substr" {
		return parseSubstr()
	}
	if tok == "index" {
		return parseIndex()
	}
	if tok == "length" {
		return parseLength()
	}

	advance()
	return tok
}

func evalMatch(str, pattern string) string {
	fullPattern := "^(?:" + breToGoRegex(pattern) + ")"

	re, err := regexp.Compile(fullPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "expr: invalid regular expression '%s'\n", pattern)
		os.Exit(2)
	}

	m := re.FindStringSubmatch(str)
	if m == nil {
		if hasGroup(pattern) {
			return ""
		}
		return "0"
	}

	if len(m) > 1 {
		return m[1]
	}
	return strconv.Itoa(len(m[0]))
}

func hasGroup(pattern string) bool {
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] == '\\' && pattern[i+1] == '(' {
			return true
		}
	}
	return false
}

func breToGoRegex(pattern string) string {
	var b strings.Builder
	i := 0
	for i < len(pattern) {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
			switch next {
			case '(', ')':
				b.WriteByte(next)
			case '{', '}', '+', '?', '|':
				b.WriteByte(next)
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				b.WriteByte('\\')
				b.WriteByte(next)
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
			}
			i += 2
		} else {
			ch := pattern[i]
			switch ch {
			case '(', ')', '{', '}', '+', '?', '|':
				b.WriteByte('\\')
				b.WriteByte(ch)
			default:
				b.WriteByte(ch)
			}
			i++
		}
	}
	return b.String()
}

func readOperand() string {
	if pos >= len(args) {
		fmt.Fprintln(os.Stderr, "expr: syntax error: missing argument")
		os.Exit(2)
	}
	tok := peek()
	if tok == "+" {
		advance()
		if pos >= len(args) {
			return "+"
		}
		return advance()
	}
	return advance()
}

func parseMatch() string {
	advance()
	str := readOperand()
	pattern := readOperand()
	return evalMatch(str, pattern)
}

func parseSubstr() string {
	advance()
	str := readOperand()
	posStr := readOperand()
	lenStr := readOperand()

	p, err := strconv.Atoi(posStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "expr: non-integer argument '%s'\n", posStr)
		os.Exit(2)
	}
	l, err := strconv.Atoi(lenStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "expr: non-integer argument '%s'\n", lenStr)
		os.Exit(2)
	}

	runes := []rune(str)
	if p < 1 || l < 0 || p > len(runes) {
		return ""
	}
	end := min(p-1+l, len(runes))
	return string(runes[p-1 : end])
}

func parseIndex() string {
	advance()
	str := readOperand()
	chars := readOperand()

	for i, ch := range str {
		if strings.ContainsRune(chars, ch) {
			return strconv.Itoa(i + 1)
		}
	}
	return "0"
}

func parseLength() string {
	advance()
	str := readOperand()
	return strconv.Itoa(len([]rune(str)))
}
