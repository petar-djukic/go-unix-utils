// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpr")
	if err != nil {
		t.Skip("reference binary not found")
	}

	normBinName := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("gexpr:"), []byte("expr:"))
	})

	tests := []testutils.DiffTest{
		// R1.1: integer arithmetic operators
		{Name: "add", Args: []string{"2", "+", "3"}},
		{Name: "subtract", Args: []string{"10", "-", "3"}},
		{Name: "multiply", Args: []string{"4", "*", "5"}},
		{Name: "divide", Args: []string{"20", "/", "4"}},
		{Name: "modulo", Args: []string{"17", "%", "5"}},
		{Name: "add_negative", Args: []string{"-3", "+", "7"}},
		{Name: "subtract_negative_result", Args: []string{"3", "-", "10"}},
		{Name: "multiply_by_zero", Args: []string{"5", "*", "0"}},
		{Name: "divide_exact", Args: []string{"15", "/", "3"}},
		{Name: "divide_truncate", Args: []string{"7", "/", "2"}},
		{Name: "modulo_no_remainder", Args: []string{"10", "%", "5"}},
		{Name: "add_large", Args: []string{"999999", "+", "1"}},
		{Name: "chained_add", Args: []string{"1", "+", "2", "+", "3"}},
		{Name: "chained_mul", Args: []string{"2", "*", "3", "*", "4"}},
		{Name: "mixed_add_mul", Args: []string{"2", "+", "3", "*", "4"}},
		{Name: "mixed_mul_add", Args: []string{"2", "*", "3", "+", "4"}},
		{Name: "sub_then_add", Args: []string{"10", "-", "3", "+", "2"}},

		// R1.2: comparison operators — numeric
		{Name: "lt_true", Args: []string{"5", "<", "10"}},
		{Name: "lt_false", Args: []string{"10", "<", "5"}},
		{Name: "lt_equal", Args: []string{"5", "<", "5"}},
		{Name: "le_true", Args: []string{"5", "<=", "10"}},
		{Name: "le_equal", Args: []string{"5", "<=", "5"}},
		{Name: "le_false", Args: []string{"10", "<=", "5"}},
		{Name: "eq_true", Args: []string{"5", "=", "5"}},
		{Name: "eq_false", Args: []string{"5", "=", "6"}},
		{Name: "ne_true", Args: []string{"5", "!=", "6"}},
		{Name: "ne_false", Args: []string{"5", "!=", "5"}},
		{Name: "ge_true", Args: []string{"10", ">=", "5"}},
		{Name: "ge_equal", Args: []string{"5", ">=", "5"}},
		{Name: "ge_false", Args: []string{"5", ">=", "10"}},
		{Name: "gt_true", Args: []string{"10", ">", "5"}},
		{Name: "gt_false", Args: []string{"5", ">", "10"}},
		{Name: "gt_equal", Args: []string{"5", ">", "5"}},
		{Name: "cmp_negative", Args: []string{"-1", "<", "0"}},
		{Name: "cmp_negative_gt", Args: []string{"0", ">", "-1"}},

		// R1.2: comparison operators — lexicographic (string)
		{Name: "str_lt", Args: []string{"abc", "<", "def"}},
		{Name: "str_gt", Args: []string{"def", ">", "abc"}},
		{Name: "str_eq", Args: []string{"abc", "=", "abc"}},
		{Name: "str_ne", Args: []string{"abc", "!=", "def"}},
		{Name: "str_le", Args: []string{"abc", "<=", "abc"}},
		{Name: "str_ge", Args: []string{"abc", ">=", "abc"}},

		// R1.3: logical operators
		{Name: "or_first_nonzero", Args: []string{"5", "|", "3"}},
		{Name: "or_first_zero", Args: []string{"0", "|", "3"}},
		{Name: "or_both_zero", Args: []string{"0", "|", "0"}},
		{Name: "or_string", Args: []string{"hello", "|", "world"}},
		{Name: "or_empty_first", Args: []string{"", "|", "world"}},
		{Name: "and_both_nonzero", Args: []string{"5", "&", "3"}},
		{Name: "and_first_zero", Args: []string{"0", "&", "3"}},
		{Name: "and_second_zero", Args: []string{"5", "&", "0"}},
		{Name: "and_both_zero", Args: []string{"0", "&", "0"}},
		{Name: "and_strings", Args: []string{"hello", "&", "world"}},

		// R1.4: parentheses grouping
		{Name: "paren_simple", Args: []string{"(", "2", "+", "3", ")"}},
		{Name: "paren_override_precedence", Args: []string{"(", "2", "+", "3", ")", "*", "4"}},
		{Name: "paren_nested", Args: []string{"(", "(", "1", "+", "2", ")", "*", "3", ")"}},
		{Name: "paren_left", Args: []string{"(", "10", "-", "3", ")", "+", "2"}},
		{Name: "paren_right", Args: []string{"10", "-", "(", "3", "+", "2", ")"}},
		{Name: "paren_comparison", Args: []string{"(", "5", "+", "3", ")", ">", "7"}},
		{Name: "paren_or_grouping", Args: []string{"(", "0", "|", "5", ")", "+", "1"}},

		// R3.4: division by zero
		{Name: "div_by_zero", Args: []string{"1", "/", "0"}, Normalize: []testutils.NormalizeFunc{normBinName}},
		{Name: "mod_by_zero", Args: []string{"1", "%", "0"}, Normalize: []testutils.NormalizeFunc{normBinName}},

		// Exit code tests — result is zero or null
		{Name: "exit1_zero_result", Args: []string{"0", "+", "0"}},
		{Name: "exit1_comparison_false", Args: []string{"5", "<", "3"}},

		// Precedence: verify mul binds tighter than add
		{Name: "precedence_mul_over_add", Args: []string{"2", "+", "3", "*", "4"}},
		// Precedence: verify comparison binds looser than add
		{Name: "precedence_add_over_cmp", Args: []string{"2", "+", "3", "=", "5"}},
		// Precedence: verify and binds looser than comparison
		{Name: "precedence_cmp_over_and", Args: []string{"1", "=", "1", "&", "2", "=", "2"}},
		// Precedence: verify or binds looser than and
		{Name: "precedence_and_over_or", Args: []string{"0", "&", "1", "|", "3"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
