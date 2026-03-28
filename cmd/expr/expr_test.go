// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/expr against GNU gexpr.
// Covers prd066-expr R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNorm normalizes error messages between GNU gexpr and Go expr.
func stderrNorm() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?expr|gexpr`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("expr"))
		b = tryHelp.ReplaceAll(b, nil)
		return b
	}
}

// stderrPresenceNorm replaces non-empty output with a fixed marker.
// Used when both binaries produce stderr but messages differ in detail.
func stderrPresenceNorm() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		if len(b) > 0 {
			return []byte("<error>\n")
		}
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpr")
	if err != nil {
		t.Skipf("reference binary gexpr not in PATH: %v", err)
	}

	tests := buildExitZeroTests()
	tests = append(tests, buildExitOneTests()...)
	tests = append(tests, buildErrorTests()...)
	tests = append(tests, buildComprehensiveTests()...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildExitZeroTests returns tests for R4.1: exit 0 on non-null, non-zero.
func buildExitZeroTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.1: simple addition yields non-zero.
		{Name: "add_nonzero", Args: []string{"2", "+", "3"}},
		// R4.1: multiplication yields non-zero.
		{Name: "mul_nonzero", Args: []string{"4", "*", "5"}},
		// R4.1: comparison true yields "1".
		{Name: "cmp_true", Args: []string{"5", "<", "10"}},
		// R4.1: string length non-zero.
		{Name: "length_nonzero", Args: []string{"length", "hello"}},
		// R4.1: non-empty string literal.
		{Name: "string_literal", Args: []string{"abc"}},
		// R4.1: negative result is non-zero.
		{Name: "negative_result", Args: []string{"3", "-", "10"}},
	}
}

// buildExitOneTests returns tests for R4.2: exit 1 on null or "0".
func buildExitOneTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.2: zero literal.
		{Name: "zero_literal", Args: []string{"0"}},
		// R4.2: false comparison yields "0".
		{Name: "cmp_false", Args: []string{"10", "<", "5"}},
		// R4.2: subtraction yielding zero.
		{Name: "sub_zero", Args: []string{"5", "-", "5"}},
		// R4.2: empty string from failed grouped match.
		{Name: "match_fail_grouped", Args: []string{"match", "abc", `\(xyz\)`}},
		// R4.2: or with both null/zero.
		{Name: "or_both_zero", Args: []string{"0", "|", "0"}},
		// R4.2: and with zero first arg.
		{Name: "and_zero", Args: []string{"0", "&", "5"}},
	}
}

// buildErrorTests returns tests for R4.3: exit 2 on syntax/error.
func buildErrorTests() []testutils.DiffTest {
	errNorm := stderrNorm()
	return []testutils.DiffTest{
		// R4.3: missing operand.
		{
			Name:      "error_missing_operand",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: division by zero.
		{
			Name:      "error_div_zero",
			Args:      []string{"1", "/", "0"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: modulo by zero.
		{
			Name:      "error_mod_zero",
			Args:      []string{"1", "%", "0"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: non-integer in arithmetic.
		{
			Name:      "error_non_integer",
			Args:      []string{"abc", "+", "1"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3: extra unexpected argument.
		{
			Name:      "error_extra_arg",
			Args:      []string{"1", "+", "2", "3"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
}

// buildComprehensiveTests returns tests for R4.4: comprehensive coverage.
func buildComprehensiveTests() []testutils.DiffTest {
	errNorm := stderrNorm()
	return []testutils.DiffTest{
		// R4.4: integer arithmetic — all operators.
		{Name: "arith_add", Args: []string{"10", "+", "20"}},
		{Name: "arith_sub", Args: []string{"20", "-", "7"}},
		{Name: "arith_mul", Args: []string{"6", "*", "7"}},
		{Name: "arith_div", Args: []string{"20", "/", "3"}},
		{Name: "arith_mod", Args: []string{"20", "%", "3"}},
		// R4.4: negative operands.
		{Name: "arith_neg_add", Args: []string{"-5", "+", "3"}},
		{Name: "arith_neg_mul", Args: []string{"-3", "*", "-4"}},

		// R4.4: comparisons — numeric.
		{Name: "cmp_lt_true", Args: []string{"3", "<", "10"}},
		{Name: "cmp_lt_false", Args: []string{"10", "<", "3"}},
		{Name: "cmp_le_eq", Args: []string{"5", "<=", "5"}},
		{Name: "cmp_eq_true", Args: []string{"42", "=", "42"}},
		{Name: "cmp_eq_false", Args: []string{"42", "=", "43"}},
		{Name: "cmp_ne_true", Args: []string{"1", "!=", "2"}},
		{Name: "cmp_ne_false", Args: []string{"1", "!=", "1"}},
		{Name: "cmp_ge_true", Args: []string{"10", ">=", "5"}},
		{Name: "cmp_gt_true", Args: []string{"10", ">", "5"}},
		{Name: "cmp_gt_false", Args: []string{"5", ">", "10"}},

		// R4.4: comparisons — string (lexicographic).
		{Name: "cmp_str_lt", Args: []string{"abc", "<", "def"}},
		{Name: "cmp_str_gt", Args: []string{"def", ">", "abc"}},
		{Name: "cmp_str_eq", Args: []string{"abc", "=", "abc"}},
		{Name: "cmp_str_ne", Args: []string{"abc", "!=", "def"}},

		// R4.4: logical operators.
		{Name: "or_first_nonzero", Args: []string{"5", "|", "0"}},
		{Name: "or_first_zero", Args: []string{"0", "|", "3"}},
		{Name: "or_string", Args: []string{"hello", "|", "world"}},
		{Name: "and_both_nonzero", Args: []string{"5", "&", "3"}},
		{Name: "and_second_zero", Args: []string{"5", "&", "0"}},

		// R4.4: string operations — match.
		{Name: "match_count", Args: []string{"match", "abcdef", "abc"}},
		{Name: "match_group", Args: []string{"match", "abcdef", `abc\(.*\)`}},
		{Name: "match_no_match", Args: []string{"match", "abcdef", "xyz"}},
		// R4.4: colon operator form of match.
		{Name: "colon_match", Args: []string{"abcdef", ":", `abc\(.*\)`}},

		// R4.4: string operations — substr.
		{Name: "substr_basic", Args: []string{"substr", "abcdef", "2", "3"}},
		{Name: "substr_end", Args: []string{"substr", "hello", "3", "10"}},
		{Name: "substr_out_of_range", Args: []string{"substr", "ab", "5", "1"}},

		// R4.4: string operations — index.
		{Name: "index_found", Args: []string{"index", "abcdef", "dc"}},
		{Name: "index_not_found", Args: []string{"index", "abcdef", "xyz"}},

		// R4.4: string operations — length.
		{Name: "length_basic", Args: []string{"length", "abcdef"}},
		{Name: "length_empty", Args: []string{"length", ""}},

		// R4.4: parentheses.
		{Name: "parens_override", Args: []string{
			"(", "2", "+", "3", ")", "*", "4",
		}},
		{Name: "parens_nested", Args: []string{
			"(", "(", "1", "+", "2", ")", "*", "3", ")",
		}},

		// R4.4: + escaping.
		{Name: "plus_escape_match", Args: []string{"+", "match"}},
		{Name: "plus_escape_length", Args: []string{"length", "+", "match"}},
		{Name: "plus_escape_parens", Args: []string{"+", "("}},

		// R4.4: division by zero (also R4.3).
		{
			Name:      "divzero_comprehensive",
			Args:      []string{"10", "/", "0"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// R4.4: missing operand errors — stderr messages differ in detail,
		// so normalize to verify non-empty stderr and matching exit code.
		{
			Name:      "missing_operand_arith",
			Args:      []string{"1", "+"},
			Normalize: []testutils.NormalizeFunc{errNorm, stderrPresenceNorm()},
		},
	}
}
