// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd104-test R3.1, R3.2, R4.1, R4.2.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtest")
	if err != nil {
		t.Skip("reference binary gtest not in PATH")
	}

	tests := []testutils.DiffTest{
		// R3.1: logical NOT operator
		{
			Name: "not_false_dir",
			Args: []string{"!", "-d", "/nonexistent_path_xyz"},
		},
		{
			Name: "not_true_file",
			Args: []string{"!", "-f", "/etc/passwd"},
		},
		{
			Name: "not_empty_string",
			Args: []string{"!", ""},
		},
		{
			Name: "not_nonempty_string",
			Args: []string{"!", "hello"},
		},
		// R3.1: logical AND operator
		{
			Name: "and_true_true",
			Args: []string{"-f", "/etc/passwd", "-a", "-d", "/tmp"},
		},
		{
			Name: "and_false_true",
			Args: []string{"-f", "/nonexistent_path_xyz", "-a", "-d", "/tmp"},
		},
		{
			Name: "and_true_false",
			Args: []string{"-d", "/tmp", "-a", "-f", "/nonexistent_path_xyz"},
		},
		// R3.1: logical OR operator
		{
			Name: "or_true_false",
			Args: []string{"-f", "/etc/passwd", "-o", "-d", "/nonexistent_path_xyz"},
		},
		{
			Name: "or_false_false",
			Args: []string{"-d", "/nonexistent_path_xyz", "-o", "-f", "/nonexistent_path_xyz"},
		},
		{
			Name: "or_false_true",
			Args: []string{"-f", "/nonexistent_path_xyz", "-o", "-d", "/tmp"},
		},
		// R3.1: parenthesized grouping
		{
			Name: "parens_simple",
			Args: []string{"(", "-f", "/etc/passwd", ")"},
		},
		{
			Name: "not_parens",
			Args: []string{"!", "(", "-d", "/nonexistent_path_xyz", ")"},
		},
		// R3.1: combined logical operators with precedence
		{
			Name: "not_and_combination",
			Args: []string{"!", "-d", "/nonexistent_path_xyz", "-a", "-f", "/etc/passwd"},
		},
		{
			Name: "or_with_and_precedence",
			Args: []string{"-f", "/nonexistent_path_xyz", "-o", "-f", "/etc/passwd", "-a", "-d", "/tmp"},
		},
		// R3.2 / R4.1: exit code 0 (true)
		{
			Name: "exit_true_nonempty_string",
			Args: []string{"hello"},
		},
		{
			Name: "exit_true_eq",
			Args: []string{"1", "-eq", "1"},
		},
		// R3.2 / R4.1: exit code 1 (false)
		{
			Name: "exit_false_no_args",
			Args: []string{},
		},
		{
			Name: "exit_false_empty_string",
			Args: []string{""},
		},
		{
			Name: "exit_false_ne",
			Args: []string{"1", "-eq", "2"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
