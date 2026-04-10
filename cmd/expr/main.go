// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/expr: evaluate expressions.
// Implements srd066-expr R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "expr"

// R1: Token types covering all expr operators and value types.
// R3.2: Precedence from lowest to highest: |, &, comparisons, +/-, */%.
type tokenType int

const (
	// Values
	tokNumber tokenType = iota
	tokString

	// Logical operators (lowest precedence)
	tokOr  // |
	tokAnd // &

	// Comparison operators
	tokLT  // <
	tokLE  // <=
	tokEQ  // =
	tokNE  // !=
	tokGE  // >=
	tokGT  // >

	// Additive operators
	tokAdd // +
	tokSub // -

	// Multiplicative operators
	tokMul // *
	tokDiv // /
	tokMod // %

	// String operation keywords
	tokMatch  // match
	tokSubstr // substr
	tokIndex  // index
	tokLength // length
	tokColon  // :

	// Grouping
	tokLParen // (
	tokRParen // )

	// R3.1: + TOKEN escaping
	tokPlus // + (escape prefix, consumed by tokenizer)

	tokEOF
)

// R2: token holds a token type and its string value.
type token struct {
	typ tokenType
	val string
}

// R3: tokenize converts command-line arguments into a token slice.
// D2: All values are strings internally, coerced to integers only when needed.
func tokenize(args []string) []token {
	panic("not implemented")
}

// parseExpr is the recursive-descent parser entry point.
// D1: Uses recursive descent with operator precedence levels matching GNU expr.
func parseExpr(tokens []token, pos int) (string, int) {
	panic("not implemented")
}

// evaluate dispatches evaluation of the full expression from args.
// Returns the result string and an error if the expression is invalid.
func evaluate(args []string) (string, error) {
	panic("not implemented")
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

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(2)
	}

	result, err := evaluate(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(2)
	}

	fmt.Println(result)
	if isNullOrZero(result) {
		os.Exit(1)
	}
	os.Exit(0)
}
