// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat (prd006-cat R1.5, R2.1-R2.3).

package main_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
	"github.com/stretchr/testify/require"
)

// TestDiff runs differential tests against the gcat reference binary.
// Tests cover R1.5 (newline preservation), R2.1 (-n line numbering),
// R2.2 (-b non-blank numbering), and R2.3 (-b overrides -n).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// --- R1.5: newline preservation ---
		{
			// R1.5: file with trailing newline preserves it exactly.
			Name:     "r1.5_trailing_newline",
			Stdin:    []byte("hello\nworld\n"),
			ExitCode: 0,
		},
		{
			// R1.5: file without trailing newline preserves absence.
			Name:     "r1.5_no_trailing_newline",
			Stdin:    []byte("hello\nworld"),
			ExitCode: 0,
		},
		{
			// R1.5: consecutive newlines preserved verbatim.
			Name:     "r1.5_consecutive_newlines",
			Stdin:    []byte("a\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R1.5: empty input produces empty output.
			Name:     "r1.5_empty_input",
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// --- R1.5: non-existent file error ---
		{
			// R1.5/R5.2: non-existent file produces error on stderr, exit 1.
			Name:      "r1.5_nonexistent_file",
			Args:      []string{"__nonexistent_test_file__"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errorNormalizer},
		},
		{
			// R5.2: non-existent file with valid stdin, continues processing.
			Name:      "r1.5_nonexistent_with_stdin",
			Args:      []string{"-", "__nonexistent_test_file__"},
			Stdin:     []byte("valid\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errorNormalizer},
		},
		// --- R2.1: -n numbers all lines ---
		{
			// R2.1: basic line numbering with -n.
			Name:     "r2.1_number_all_basic",
			Args:     []string{"-n"},
			Stdin:    []byte("alpha\nbeta\ngamma\n"),
			ExitCode: 0,
		},
		{
			// R2.1: -n numbers blank lines too.
			Name:     "r2.1_number_all_with_blanks",
			Args:     []string{"-n"},
			Stdin:    []byte("a\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R2.1: -n with no trailing newline.
			Name:     "r2.1_number_no_trailing_newline",
			Args:     []string{"-n"},
			Stdin:    []byte("line1\nline2"),
			ExitCode: 0,
		},
		{
			// R2.1: -n with single line.
			Name:     "r2.1_number_single_line",
			Args:     []string{"-n"},
			Stdin:    []byte("only\n"),
			ExitCode: 0,
		},
		// --- R2.2: -b numbers non-blank lines only ---
		{
			// R2.2: -b skips blank lines.
			Name:     "r2.2_number_nonblank",
			Args:     []string{"-b"},
			Stdin:    []byte("a\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R2.2: -b with all blank lines.
			Name:     "r2.2_all_blank",
			Args:     []string{"-b"},
			Stdin:    []byte("\n\n\n"),
			ExitCode: 0,
		},
		{
			// R2.2: -b with no blank lines.
			Name:     "r2.2_no_blank",
			Args:     []string{"-b"},
			Stdin:    []byte("x\ny\nz\n"),
			ExitCode: 0,
		},
		{
			// R2.4: lines with spaces/tabs are not blank for -b.
			Name:     "r2.2_spaces_not_blank",
			Args:     []string{"-b"},
			Stdin:    []byte("a\n \n\t\nb\n"),
			ExitCode: 0,
		},
		// --- R2.3: -b takes precedence over -n ---
		{
			// R2.3: -b -n together, -b wins (blank lines not numbered).
			Name:     "r2.3_b_overrides_n",
			Args:     []string{"-b", "-n"},
			Stdin:    []byte("a\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R2.3: -n -b order, -b still wins.
			Name:     "r2.3_n_then_b",
			Args:     []string{"-n", "-b"},
			Stdin:    []byte("a\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R2.3: combined -nb flag, -b wins.
			Name:     "r2.3_combined_nb",
			Args:     []string{"-nb"},
			Stdin:    []byte("a\n\nb\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies --help prints usage to stdout and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()

	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--help")
	stdout, err := cmd.Output()
	require.NoError(t, err, "expected exit 0 for --help")

	out := string(stdout)
	require.True(t, strings.Contains(out, "Usage:"), "help output should contain Usage:")
	require.True(t, strings.Contains(out, "--help"), "help output should mention --help")
	require.True(t, strings.Contains(out, "--version"), "help output should mention --version")
}

// TestVersion verifies --version prints version info to stdout and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()

	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--version")
	stdout, err := cmd.Output()
	require.NoError(t, err, "expected exit 0 for --version")

	out := string(stdout)
	require.True(t, strings.Contains(out, "cat"), "version output should contain 'cat'")
}

// errorNormalizer normalizes error messages to handle two platform differences:
// 1. Binary name: gcat vs cat in the error prefix.
// 2. Error string casing: Go syscall (lowercase) vs C strerror (capitalized).
func errorNormalizer(b []byte) []byte {
	// Normalize binary name prefix: "gcat:" → "cat:"
	b = bytes.ReplaceAll(b, []byte("gcat:"), []byte("cat:"))
	return bytes.ToLower(b)
}
