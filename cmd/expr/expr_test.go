// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/expr: differential testing against gexpr.
// Covers srd066-expr R4.1, R4.2, R4.3, R4.4.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeStderr strips the program name prefix from error messages so
// "gexpr: ..." and "expr: ..." compare as equal. Also normalizes the
// --help path hint since reference binary has full path.
func normalizeStderr(data []byte) []byte {
	// Replace program name prefix: "gexpr: " or "expr: " -> "PROG: "
	re := regexp.MustCompile(`(?m)^(?:g?expr|[^\s:]+/gexpr): `)
	data = re.ReplaceAll(data, []byte("PROG: "))
	// Normalize --help suggestion line with full binary path
	helpRe := regexp.MustCompile(`Try '[^']*' for more information\.`)
	data = helpRe.ReplaceAll(data, []byte("Try 'PROG --help' for more information."))
	return data
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpr")
	if err != nil {
		t.Skipf("reference binary gexpr not in PATH: %v", err)
	}
	norm := []testutils.NormalizeFunc{normalizeStderr}
	tests := []testutils.DiffTest{
		// R1.1: integer arithmetic — all operators
		{Name: "addition", Args: []string{"2", "+", "3"}},
		{Name: "subtraction", Args: []string{"10", "-", "3"}},
		{Name: "multiplication", Args: []string{"4", "*", "5"}},
		{Name: "division", Args: []string{"20", "/", "4"}},
		{Name: "modulo", Args: []string{"17", "%", "5"}},
		// R1.2: comparisons — numeric
		{Name: "less_true", Args: []string{"5", "<", "10"}},
		{Name: "less_false", Args: []string{"10", "<", "5"}, ExitCode: 1},
		{Name: "less_equal_true", Args: []string{"5", "<=", "5"}},
		{Name: "equal_true", Args: []string{"42", "=", "42"}},
		{Name: "equal_false", Args: []string{"42", "=", "43"}, ExitCode: 1},
		{Name: "not_equal_true", Args: []string{"1", "!=", "2"}},
		{Name: "greater_equal_true", Args: []string{"10", ">=", "10"}},
		{Name: "greater_true", Args: []string{"10", ">", "5"}},
		// R1.2: comparisons — string (lexicographic)
		{Name: "string_less", Args: []string{"abc", "<", "abd"}},
		{Name: "string_equal", Args: []string{"abc", "=", "abc"}},
		{Name: "string_not_equal", Args: []string{"abc", "!=", "xyz"}},
		// R1.3: logical operators
		{Name: "or_first_nonzero", Args: []string{"5", "|", "3"}},
		{Name: "or_first_zero", Args: []string{"0", "|", "3"}},
		{Name: "or_both_zero", Args: []string{"0", "|", "0"}, ExitCode: 1},
		{Name: "and_both_nonzero", Args: []string{"5", "&", "3"}},
		{Name: "and_first_zero", Args: []string{"0", "&", "3"}, ExitCode: 1},
		// R1.4: parentheses
		{Name: "parentheses", Args: []string{"(", "2", "+", "3", ")", "*", "4"}},
		{Name: "nested_parens", Args: []string{"(", "(", "1", "+", "2", ")", "*", "3", ")"}},
		// R2.1: match — character count (no group)
		{Name: "match_count", Args: []string{"match", "abcdef", "abc"}},
		// R2.1: match — group extraction
		{Name: "match_group", Args: []string{"match", "abcdef", `abc\(.*\)`}},
		// R2.1: colon operator
		{Name: "colon_match", Args: []string{"abcdef", ":", "abc"}},
		// R2.2: substr
		{Name: "substr", Args: []string{"substr", "hello", "2", "3"}},
		{Name: "substr_out_of_range", Args: []string{"substr", "hi", "5", "3"}, ExitCode: 1},
		// R2.3: index
		{Name: "index_found", Args: []string{"index", "hello", "lo"}},
		{Name: "index_not_found", Args: []string{"index", "hello", "xyz"}, ExitCode: 1},
		// R2.4: length
		{Name: "length", Args: []string{"length", "hello"}},
		{Name: "length_empty", Args: []string{"length", ""}, ExitCode: 1},
		// R3.1: + escaping
		{Name: "plus_escape_keyword", Args: []string{"+", "length"}},
		{Name: "plus_escape_operator", Args: []string{"+", "+"}},
		// R3.2, R3.3: precedence and associativity
		{Name: "precedence_mul_add", Args: []string{"2", "+", "3", "*", "4"}},
		{Name: "left_assoc_sub", Args: []string{"10", "-", "3", "-", "2"}},
		// R3.4, R4.3: division by zero — exit 2
		{
			Name: "division_by_zero", Args: []string{"1", "/", "0"},
			ExitCode: 2, Normalize: norm,
		},
		{
			Name: "modulo_by_zero", Args: []string{"1", "%", "0"},
			ExitCode: 2, Normalize: norm,
		},
		// R4.1: exit 0 for non-null, non-zero result
		{Name: "exit_0_nonzero", Args: []string{"42"}},
		{Name: "exit_0_string", Args: []string{"+", "hello"}},
		// R4.2: exit 1 for null or zero result
		{Name: "exit_1_zero", Args: []string{"0", "+", "0"}, ExitCode: 1},
		{Name: "exit_1_empty_match", Args: []string{"match", "abc", `xyz\(.*\)`}, ExitCode: 1},
		// R4.3: exit 2 on syntax error / missing operand
		{
			Name: "missing_operand", Args: []string{},
			ExitCode: 2, Normalize: norm,
		},
		{
			Name: "non_integer_arith", Args: []string{"abc", "+", "1"},
			ExitCode: 2, Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
