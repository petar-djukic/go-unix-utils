// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expr.
// Covers prd066-expr R1.1 (arithmetic), R1.2 (comparisons),
// R1.3 (logical operators), R1.4 (parentheses).
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpr")
	if err != nil {
		t.Skip("reference binary gexpr not in PATH")
	}
	tests := []testutils.DiffTest{
		// R1.1: Arithmetic operators
		{Name: "addition", Args: []string{"2", "+", "3"}, ExitCode: 0},
		{Name: "subtraction", Args: []string{"10", "-", "3"}, ExitCode: 0},
		{Name: "multiplication", Args: []string{"4", "*", "5"}, ExitCode: 0},
		{Name: "division", Args: []string{"20", "/", "4"}, ExitCode: 0},
		{Name: "modulo", Args: []string{"17", "%", "5"}, ExitCode: 0},
		{Name: "negative_result", Args: []string{"3", "-", "10"}, ExitCode: 0},
		{Name: "zero_result", Args: []string{"0", "+", "0"}, ExitCode: 1},

		// R1.1: Division/modulo by zero (discard stderr, binary names differ).
		{
			Name:      "division_by_zero",
			Args:      []string{"1", "/", "0"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "modulo_by_zero",
			Args:      []string{"1", "%", "0"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},

		// R1.2: Comparison operators (numeric)
		{Name: "less_true", Args: []string{"5", "<", "10"}, ExitCode: 0},
		{Name: "less_false", Args: []string{"10", "<", "5"}, ExitCode: 1},
		{Name: "less_equal_true", Args: []string{"5", "<=", "5"}, ExitCode: 0},
		{Name: "equal_true", Args: []string{"42", "=", "42"}, ExitCode: 0},
		{Name: "equal_false", Args: []string{"42", "=", "43"}, ExitCode: 1},
		{Name: "not_equal_true", Args: []string{"1", "!=", "2"}, ExitCode: 0},
		{Name: "greater_true", Args: []string{"10", ">", "5"}, ExitCode: 0},
		{Name: "greater_equal_true", Args: []string{"5", ">=", "5"}, ExitCode: 0},

		// R1.2: Comparison operators (lexicographic)
		{Name: "string_less", Args: []string{"abc", "<", "def"}, ExitCode: 0},
		{Name: "string_equal", Args: []string{"hello", "=", "hello"}, ExitCode: 0},
		{Name: "string_not_equal", Args: []string{"abc", "!=", "xyz"}, ExitCode: 0},

		// R1.3: Logical operators
		{Name: "or_first_nonzero", Args: []string{"5", "|", "3"}, ExitCode: 0},
		{Name: "or_first_zero", Args: []string{"0", "|", "3"}, ExitCode: 0},
		{Name: "or_both_zero", Args: []string{"0", "|", "0"}, ExitCode: 1},
		{Name: "and_both_nonzero", Args: []string{"5", "&", "3"}, ExitCode: 0},
		{Name: "and_first_zero", Args: []string{"0", "&", "3"}, ExitCode: 1},
		{Name: "and_second_zero", Args: []string{"3", "&", "0"}, ExitCode: 1},

		// R1.4: Parentheses
		{Name: "parentheses_override", Args: []string{"(", "2", "+", "3", ")", "*", "4"}, ExitCode: 0},
		{Name: "nested_parentheses", Args: []string{"(", "(", "3", "+", "2", ")", "*", "4", ")"}, ExitCode: 0},
		{Name: "parens_in_comparison", Args: []string{"(", "1", "+", "2", ")", "=", "3"}, ExitCode: 0},

		// Precedence: * binds tighter than +
		{Name: "precedence_mul_add", Args: []string{"2", "+", "3", "*", "4"}, ExitCode: 0},

		// Syntax errors (discard stderr, binary names differ).
		{
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "missing_right_operand",
			Args:      []string{"1", "+"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
