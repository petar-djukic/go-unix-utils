// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/test against gtest reference binary.
// Implements: srd104-test R1.1, R1.2, R2.1, R2.2, R3.1, R3.2.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer strips the program name prefix from error messages so
// "gtest: ..." and "test: ..." compare equal.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?m)^(?:gtest|test): `)
	return re.ReplaceAll(b, []byte("test: "))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtest")
	if err != nil {
		t.Skipf("reference binary gtest not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R2.1: zero-argument form (false)
		{
			Name:     "zero_args_false",
			Args:     []string{},
			ExitCode: 1,
		},
		// R2.1: single string argument (non-empty = true)
		{
			Name:     "single_nonempty_string",
			Args:     []string{"hello"},
			ExitCode: 0,
		},
		// R2.1: single empty string argument (false)
		{
			Name:     "single_empty_string",
			Args:     []string{""},
			ExitCode: 1,
		},
		// R2.1: -z zero-length string
		{
			Name:     "z_empty",
			Args:     []string{"-z", ""},
			ExitCode: 0,
		},
		{
			Name:     "z_nonempty",
			Args:     []string{"-z", "abc"},
			ExitCode: 1,
		},
		// R2.1: -n non-zero-length string
		{
			Name:     "n_nonempty",
			Args:     []string{"-n", "abc"},
			ExitCode: 0,
		},
		{
			Name:     "n_empty",
			Args:     []string{"-n", ""},
			ExitCode: 1,
		},
		// R2.1: string equality
		{
			Name:     "string_equal",
			Args:     []string{"abc", "=", "abc"},
			ExitCode: 0,
		},
		{
			Name:     "string_not_equal",
			Args:     []string{"abc", "=", "def"},
			ExitCode: 1,
		},
		// R2.1: string inequality
		{
			Name:     "string_neq",
			Args:     []string{"abc", "!=", "def"},
			ExitCode: 0,
		},
		{
			Name:     "string_neq_same",
			Args:     []string{"abc", "!=", "abc"},
			ExitCode: 1,
		},
		// R2.2: integer comparisons
		{
			Name:     "int_eq_true",
			Args:     []string{"1", "-eq", "1"},
			ExitCode: 0,
		},
		{
			Name:     "int_eq_false",
			Args:     []string{"1", "-eq", "2"},
			ExitCode: 1,
		},
		{
			Name:     "int_ne_true",
			Args:     []string{"1", "-ne", "2"},
			ExitCode: 0,
		},
		{
			Name:     "int_ne_false",
			Args:     []string{"1", "-ne", "1"},
			ExitCode: 1,
		},
		{
			Name:     "int_lt_true",
			Args:     []string{"1", "-lt", "2"},
			ExitCode: 0,
		},
		{
			Name:     "int_lt_false",
			Args:     []string{"2", "-lt", "1"},
			ExitCode: 1,
		},
		{
			Name:     "int_le_true",
			Args:     []string{"1", "-le", "1"},
			ExitCode: 0,
		},
		{
			Name:     "int_le_false",
			Args:     []string{"2", "-le", "1"},
			ExitCode: 1,
		},
		{
			Name:     "int_gt_true",
			Args:     []string{"2", "-gt", "1"},
			ExitCode: 0,
		},
		{
			Name:     "int_gt_false",
			Args:     []string{"1", "-gt", "2"},
			ExitCode: 1,
		},
		{
			Name:     "int_ge_true",
			Args:     []string{"2", "-ge", "2"},
			ExitCode: 0,
		},
		{
			Name:     "int_ge_false",
			Args:     []string{"1", "-ge", "2"},
			ExitCode: 1,
		},
		// R2.2: invalid integer operands => exit 2
		{
			Name:      "int_invalid_operand",
			Args:      []string{"abc", "-eq", "1"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.1: negation
		{
			Name:     "negation_true",
			Args:     []string{"!", ""},
			ExitCode: 0,
		},
		{
			Name:     "negation_false",
			Args:     []string{"!", "hello"},
			ExitCode: 1,
		},
		// R3.1: -a (and)
		{
			Name:     "and_both_true",
			Args:     []string{"abc", "-a", "def"},
			ExitCode: 0,
		},
		{
			Name:     "and_one_false",
			Args:     []string{"abc", "-a", ""},
			ExitCode: 1,
		},
		// R3.1: -o (or)
		{
			Name:     "or_one_true",
			Args:     []string{"", "-o", "abc"},
			ExitCode: 0,
		},
		{
			Name:     "or_both_false",
			Args:     []string{"", "-o", ""},
			ExitCode: 1,
		},
		// R3.1: parenthesized group
		{
			Name:     "parens_group",
			Args:     []string{"(", "abc", ")"},
			ExitCode: 0,
		},
		// R1.1: -d directory test
		{
			Name:     "d_dir_exists",
			Args:     []string{"-d", "/tmp"},
			ExitCode: 0,
		},
		{
			Name:     "d_nonexistent",
			Args:     []string{"-d", "/nonexistent_path_xyz"},
			ExitCode: 1,
		},
		// R1.1: -e exists test
		{
			Name:     "e_exists",
			Args:     []string{"-e", "/tmp"},
			ExitCode: 0,
		},
		{
			Name:     "e_nonexistent",
			Args:     []string{"-e", "/nonexistent_path_xyz"},
			ExitCode: 1,
		},
		// Negation with file test
		{
			Name:     "not_d_nonexistent",
			Args:     []string{"!", "-d", "/nonexistent_path_xyz"},
			ExitCode: 0,
		},
		// R2.2: negative integers
		{
			Name:     "int_negative_lt",
			Args:     []string{"-1", "-lt", "0"},
			ExitCode: 0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
