// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expr.
// Covers prd066-expr R1.1 (arithmetic), R1.2 (comparisons),
// R1.3 (logical operators), R1.4 (parentheses), R2.1 (match/:),
// R2.2 (substr), R2.3 (index), R2.4 (length),
// R3.1 (+ token escaping), R3.2 (precedence), R3.3 (left-associativity),
// R3.4 (division by zero), R4.1 (exit 0), R4.2 (exit 1),
// R4.3 (exit 2/3), R4.4 (differential tests).
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

		// R2.1: match keyword and : operator
		{Name: "match_no_group", Args: []string{"match", "abcdef", "abc"}, ExitCode: 0},
		{Name: "match_with_group", Args: []string{"match", "abcdef", `abc\(.*\)`}, ExitCode: 0},
		{Name: "match_failure", Args: []string{"match", "abcdef", "xyz"}, ExitCode: 1},
		{Name: "match_group_failure", Args: []string{"match", "abcdef", `xyz\(.*\)`}, ExitCode: 1},
		{Name: "colon_no_group", Args: []string{"abcdef", ":", "abc"}, ExitCode: 0},
		{Name: "colon_with_group", Args: []string{"abcdef", ":", `abc\(.*\)`}, ExitCode: 0},
		{Name: "colon_failure", Args: []string{"abcdef", ":", "xyz"}, ExitCode: 1},
		{Name: "colon_dot_star", Args: []string{"hello", ":", ".*"}, ExitCode: 0},
		{Name: "match_empty_group", Args: []string{"match", "abc", `abc\(\)`}, ExitCode: 1},

		// R2.2: substr
		{Name: "substr_basic", Args: []string{"substr", "hello", "2", "3"}, ExitCode: 0},
		{Name: "substr_full", Args: []string{"substr", "hello", "1", "5"}, ExitCode: 0},
		{Name: "substr_single", Args: []string{"substr", "hello", "3", "1"}, ExitCode: 0},
		{Name: "substr_past_end", Args: []string{"substr", "hello", "3", "10"}, ExitCode: 0},
		{Name: "substr_pos_zero", Args: []string{"substr", "hello", "0", "3"}, ExitCode: 1},
		{Name: "substr_len_zero", Args: []string{"substr", "hello", "1", "0"}, ExitCode: 1},
		{Name: "substr_pos_beyond", Args: []string{"substr", "hello", "10", "3"}, ExitCode: 1},

		// R2.3: index
		{Name: "index_found", Args: []string{"index", "hello", "el"}, ExitCode: 0},
		{Name: "index_first_char", Args: []string{"index", "abcdef", "a"}, ExitCode: 0},
		{Name: "index_last_char", Args: []string{"index", "abcdef", "f"}, ExitCode: 0},
		{Name: "index_not_found", Args: []string{"index", "hello", "xyz"}, ExitCode: 1},
		{Name: "index_multi_chars", Args: []string{"index", "hello", "lo"}, ExitCode: 0},

		// R2.4: length
		{Name: "length_basic", Args: []string{"length", "hello"}, ExitCode: 0},
		{Name: "length_empty", Args: []string{"length", ""}, ExitCode: 1},
		{Name: "length_one", Args: []string{"length", "x"}, ExitCode: 0},
		{Name: "length_numeric_string", Args: []string{"length", "12345"}, ExitCode: 0},

		// R3.1: + TOKEN escapes keywords and operators as string literals
		{Name: "plus_escape_match", Args: []string{"+", "match", "=", "+", "match"}, ExitCode: 0},
		{Name: "plus_escape_length", Args: []string{"length", "+", "length"}, ExitCode: 0},
		{Name: "plus_escape_index", Args: []string{"+", "index", "=", "+", "index"}, ExitCode: 0},
		{Name: "plus_escape_substr", Args: []string{"+", "substr", "=", "+", "substr"}, ExitCode: 0},
		{Name: "plus_escape_paren", Args: []string{"+", "(", "=", "+", "("}, ExitCode: 0},

		// R3.2: Operator precedence (lowest to highest: | & comp +- */%)
		{Name: "precedence_or_and", Args: []string{"0", "|", "5", "&", "3"}, ExitCode: 0},
		{Name: "precedence_comp_add", Args: []string{"1", "+", "2", "=", "3"}, ExitCode: 0},
		{Name: "precedence_add_mul", Args: []string{"2", "+", "3", "*", "4"}, ExitCode: 0},

		// R3.3: Left-associativity
		{Name: "left_assoc_sub", Args: []string{"10", "-", "3", "-", "2"}, ExitCode: 0},
		{Name: "left_assoc_div", Args: []string{"100", "/", "10", "/", "2"}, ExitCode: 0},
		{Name: "left_assoc_comp", Args: []string{"1", "<", "2", "<", "1"}, ExitCode: 1},

		// R3.4: Division by zero exits 2 with error on stderr
		{
			Name:      "div_by_zero_r3",
			Args:      []string{"1", "/", "0"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "mod_by_zero_r3",
			Args:      []string{"10", "%", "0"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},

		// R4.1: Exit 0 for non-null, non-zero results
		{Name: "exit0_nonzero_int", Args: []string{"1"}, ExitCode: 0},
		{Name: "exit0_string_value", Args: []string{"match", "abc", `a\(.*\)`}, ExitCode: 0},
		{Name: "exit0_negative_int", Args: []string{"0", "-", "1"}, ExitCode: 0},

		// R4.2: Exit 1 for null or zero results
		{Name: "exit1_zero_subtraction", Args: []string{"5", "-", "5"}, ExitCode: 1},
		{Name: "exit1_literal_zero", Args: []string{"0"}, ExitCode: 1},
		{Name: "exit1_empty_match", Args: []string{"abc", ":", `xyz\(.*\)`}, ExitCode: 1},

		// R4.3: Exit 2 on syntax errors
		{
			Name:      "exit2_non_integer_add",
			Args:      []string{"abc", "+", "1"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "exit2_unmatched_paren",
			Args:      []string{"(", "1", "+", "2"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "exit2_extra_token",
			Args:      []string{"1", "2"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "exit2_non_integer_mul",
			Args:      []string{"abc", "*", "2"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
