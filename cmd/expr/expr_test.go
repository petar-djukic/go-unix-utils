// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd066-expr R1.1–R1.4, R2.1–R2.4, R3.1–R3.4,
// R4.1–R4.4.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// syntaxErrDetailRe matches "syntax error: <detail>" and strips the detail.
var syntaxErrDetailRe = regexp.MustCompile(`syntax error[^\n]*`)

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

// normalizeSyntaxError strips the detailed suffix after "syntax error"
// since GNU expr provides specific context that our implementation omits.
func normalizeSyntaxError(data []byte) []byte {
	return syntaxErrDetailRe.ReplaceAll(data, []byte("syntax error"))
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

		// R2.1: colon operator — match with group
		{Name: "colon_match_group", Args: []string{"abcdef", ":", `abc\(.*\)`}},
		// R2.1: colon operator — match count (no group)
		{Name: "colon_match_count", Args: []string{"abcdef", ":", "abc"}},
		// R2.1: colon operator — no match returns 0
		{Name: "colon_no_match", Args: []string{"abcdef", ":", "xyz"}},
		// R2.1: colon operator — no match with group returns empty
		{Name: "colon_no_match_group", Args: []string{"abcdef", ":", `xyz\(.*\)`}},
		// R2.1: colon operator — dot matches any char
		{Name: "colon_dot_match", Args: []string{"abc", ":", "a.c"}},
		// R2.1: colon operator — anchored at start
		{Name: "colon_anchored", Args: []string{"xabc", ":", "abc"}},
		// R2.1: colon operator — capture single char group
		{Name: "colon_single_group", Args: []string{"abc", ":", `a\(.\)`}},

		// R2.1: match keyword — with group
		{Name: "match_group", Args: []string{"match", "abcdef", `abc\(.*\)`}},
		// R2.1: match keyword — count (no group)
		{Name: "match_count", Args: []string{"match", "abcdef", "abc"}},
		// R2.1: match keyword — no match
		{Name: "match_no_match", Args: []string{"match", "abcdef", "xyz"}},

		// R2.2: substr — basic extraction
		{Name: "substr_basic", Args: []string{"substr", "hello", "2", "3"}},
		// R2.2: substr — full string
		{Name: "substr_full", Args: []string{"substr", "hello", "1", "5"}},
		// R2.2: substr — length exceeds string
		{Name: "substr_overflow", Args: []string{"substr", "hello", "3", "10"}},
		// R2.2: substr — pos zero (out of range)
		{Name: "substr_zero_pos", Args: []string{"substr", "hello", "0", "3"}},
		// R2.2: substr — negative length (out of range)
		{Name: "substr_neg_len", Args: []string{"substr", "hello", "1", "-1"}},

		// R2.3: index — found
		{Name: "index_found", Args: []string{"index", "hello", "lo"}},
		// R2.3: index — not found
		{Name: "index_not_found", Args: []string{"index", "hello", "xyz"}},
		// R2.3: index — first char
		{Name: "index_first_char", Args: []string{"index", "hello", "h"}},

		// R2.4: length — basic
		{Name: "length_basic", Args: []string{"length", "hello"}},
		// R2.4: length — empty string
		{Name: "length_empty", Args: []string{"length", ""}},
		// R2.4: length — single char
		{Name: "length_single", Args: []string{"length", "x"}},

		// R3.1: + escapes keyword match
		{Name: "plus_escape_match", Args: []string{"+", "match"}},
		// R3.1: + escapes keyword length
		{Name: "plus_escape_length", Args: []string{"+", "length"}},
		// R3.1: + escapes operator
		{Name: "plus_escape_operator", Args: []string{"+", "+"}},

		// R4.1: non-null non-zero expression exits 0
		{Name: "exit0_add_result", Args: []string{"2", "+", "3"}},
		// R4.1: string comparison true exits 0
		{Name: "exit0_cmp_true", Args: []string{"10", ">", "5"}},
		// R4.1: non-empty string result exits 0
		{Name: "exit0_nonempty_string", Args: []string{"abc"}},
		// R4.1: match with group returns non-empty exits 0
		{Name: "exit0_match_group", Args: []string{"abc", ":", `\(abc\)`}},

		// R4.2: zero arithmetic result exits 1
		{Name: "exit1_zero_arith", Args: []string{"3", "-", "3"}},
		// R4.2: false comparison exits 1
		{Name: "exit1_cmp_false", Args: []string{"5", ">", "10"}},
		// R4.2: match no-group no-match returns "0" exits 1
		{Name: "exit1_no_match", Args: []string{"abc", ":", "xyz"}},
		// R4.2: empty string result exits 1
		{Name: "exit1_empty_string", Args: []string{""}},
		// R4.2: or both zero exits 1
		{Name: "exit1_or_zeros", Args: []string{"0", "|", "0"}},
		// R4.2: match with group no-match returns empty exits 1
		{Name: "exit1_match_group_nomatch", Args: []string{"abc", ":", `xyz\(.*\)`}},

		// R4.3: syntax error — unmatched left paren exits 2
		{
			Name:      "exit2_unmatched_paren",
			Args:      []string{"(", "1", "+", "2"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeSyntaxError},
		},
		// R4.3: syntax error — missing right operand exits 2
		{
			Name:      "exit2_missing_operand",
			Args:      []string{"1", "+"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeSyntaxError},
		},
		// R4.3: syntax error — extra arguments after expression exits 2
		{
			Name:      "exit2_extra_args",
			Args:      []string{"1", "2"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeSyntaxError},
		},
		// R4.3: syntax error — non-integer in arithmetic exits 2
		{
			Name:      "exit2_non_integer_arith",
			Args:      []string{"abc", "*", "2"},
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R4.4: logical with strings
		{Name: "or_string_first", Args: []string{"hello", "|", "world"}},
		// R4.4: and with strings
		{Name: "and_strings", Args: []string{"hello", "&", "world"}},
		// R4.4: and with empty string
		{Name: "and_empty_string", Args: []string{"", "&", "world"}},
		// R4.4: complex mixed expression
		{Name: "complex_mixed", Args: []string{"(", "1", "+", "2", ")", "*", "(", "3", "-", "1", ")"}},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
