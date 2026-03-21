// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd066-expr R1.1–R1.4.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gexpr" with "expr" in output
// so differential tests pass despite different binary names.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gexpr"), []byte("expr"))
}

// normalizeTryHelp removes the "Try '...' --help" line that contains
// the full binary path, which differs between reference and Go binaries.
func normalizeTryHelp(data []byte) []byte {
	var result []byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.Contains(line, []byte("Try '")) && bytes.Contains(line, []byte("--help")) {
			continue
		}
		if len(result) > 0 {
			result = append(result, '\n')
		}
		result = append(result, line...)
	}
	return result
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpr")
	if err != nil {
		t.Skipf("reference binary gexpr not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: addition
		{Name: "add", Args: []string{"1", "+", "2"}},
		// R1.1: subtraction
		{Name: "sub", Args: []string{"10", "-", "3"}},
		// R1.1: multiplication
		{Name: "mul", Args: []string{"3", "*", "4"}},
		// R1.1: integer division
		{Name: "div", Args: []string{"10", "/", "3"}},
		// R1.1: modulo
		{Name: "mod", Args: []string{"10", "%", "3"}},
		// R1.1: negative result
		{Name: "negative_result", Args: []string{"3", "-", "10"}},
		// R1.1: zero result from subtraction (exit 1)
		{Name: "zero_sub", Args: []string{"5", "-", "5"}},

		// R1.2: less than (true)
		{Name: "lt_true", Args: []string{"5", "<", "10"}},
		// R1.2: less than (false)
		{Name: "lt_false", Args: []string{"10", "<", "5"}},
		// R1.2: equal (true)
		{Name: "eq_true", Args: []string{"5", "=", "5"}},
		// R1.2: equal (false)
		{Name: "eq_false", Args: []string{"5", "=", "10"}},
		// R1.2: not equal (true)
		{Name: "ne_true", Args: []string{"5", "!=", "10"}},
		// R1.2: greater than (true)
		{Name: "gt_true", Args: []string{"10", ">", "5"}},
		// R1.2: less or equal
		{Name: "le_true", Args: []string{"5", "<=", "5"}},
		// R1.2: greater or equal
		{Name: "ge_true", Args: []string{"10", ">=", "5"}},
		// R1.2: string comparison (lexicographic)
		{Name: "str_lt", Args: []string{"abc", "<", "def"}},
		// R1.2: string equality
		{Name: "str_eq", Args: []string{"abc", "=", "abc"}},

		// R1.3: or with non-zero first
		{Name: "or_nonzero", Args: []string{"5", "|", "3"}},
		// R1.3: or with zero first
		{Name: "or_zero_first", Args: []string{"0", "|", "3"}},
		// R1.3: or both zero
		{Name: "or_both_zero", Args: []string{"0", "|", "0"}},
		// R1.3: and with both non-zero
		{Name: "and_both", Args: []string{"5", "&", "3"}},
		// R1.3: and with zero
		{Name: "and_zero", Args: []string{"0", "&", "3"}},
		// R1.3: and with both zero
		{Name: "and_both_zero", Args: []string{"0", "&", "0"}},

		// R1.4: parentheses for grouping
		{Name: "parens", Args: []string{"(", "1", "+", "2", ")", "*", "3"}},
		// R1.4: nested parentheses
		{Name: "nested_parens", Args: []string{"(", "(", "2", "+", "3", ")", "*", "4", ")"}},

		// R3.2: precedence — * before +
		{Name: "precedence_mul_add", Args: []string{"2", "+", "3", "*", "4"}},
		// R3.2: precedence — | before &... actually | is lower than &
		{Name: "precedence_or_and", Args: []string{"0", "|", "1", "&", "0"}},

		// R3.3: left associativity
		{Name: "left_assoc_sub", Args: []string{"10", "-", "3", "-", "2"}},
		// R3.3: left associativity for division
		{Name: "left_assoc_div", Args: []string{"100", "/", "10", "/", "2"}},

		// R3.4: division by zero
		{
			Name:      "div_by_zero",
			Args:      []string{"1", "/", "0"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.4: modulo by zero
		{
			Name:      "mod_by_zero",
			Args:      []string{"1", "%", "0"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R4.1: non-zero result exits 0
		{Name: "exit0_nonzero", Args: []string{"42"}},
		// R4.1: string atom exits 0
		{Name: "exit0_string", Args: []string{"hello"}},
		// R4.2: zero result exits 1
		{Name: "exit1_zero", Args: []string{"0"}},

		// Error: no arguments
		{
			Name:      "error_no_args",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeTryHelp},
		},
		// Error: non-integer in arithmetic
		{
			Name:      "error_non_integer",
			Args:      []string{"foo", "+", "1"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// Complex expression: 1 + 2 * 3 - 1 = 6
		{Name: "complex_expr", Args: []string{"1", "+", "2", "*", "3", "-", "1"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
