// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat (prd006-cat R1.5, R2.1-R2.4, R3.1-R3.3).

package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
	"github.com/stretchr/testify/require"
)

// TestDiff runs differential tests against the gcat reference binary.
// Tests cover R1.5 (newline preservation), R2.1 (-n line numbering),
// R2.2 (-b non-blank numbering), R2.3 (-b overrides -n), R2.4 (blank
// line definition), R3.1 (-s squeeze), R3.2 (squeeze across files),
// and R3.3 (-s combined with -n/-b).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Create temp files for R3.2 cross-file boundary tests.
	dir := t.TempDir()
	endsBlank := filepath.Join(dir, "ends_blank.txt")
	startsBlank := filepath.Join(dir, "starts_blank.txt")
	writeTestFile(t, endsBlank, "hello\n\n\n")
	writeTestFile(t, startsBlank, "\n\nworld\n")

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
		// --- R3.1: -s suppresses consecutive blank lines ---
		{
			// R3.1: basic squeeze with multiple consecutive blank lines.
			Name:     "r3.1_squeeze_basic",
			Args:     []string{"-s"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R3.1: single blank line is not suppressed.
			Name:     "r3.1_squeeze_single_blank",
			Args:     []string{"-s"},
			Stdin:    []byte("a\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R3.1: many consecutive blank lines squeezed to one.
			Name:     "r3.1_squeeze_many_blanks",
			Args:     []string{"-s"},
			Stdin:    []byte("a\n\n\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R3.1: multiple groups of blank lines each squeezed independently.
			Name:     "r3.1_squeeze_multiple_groups",
			Args:     []string{"-s"},
			Stdin:    []byte("a\n\n\nb\n\n\nc\n"),
			ExitCode: 0,
		},
		{
			// R3.1: all blank lines squeezed to one.
			Name:     "r3.1_squeeze_all_blank",
			Args:     []string{"-s"},
			Stdin:    []byte("\n\n\n\n"),
			ExitCode: 0,
		},
		{
			// R3.1: no blank lines, -s has no effect.
			Name:     "r3.1_squeeze_no_blanks",
			Args:     []string{"-s"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		{
			// R2.4/R3.1: lines with spaces/tabs are not blank for -s.
			Name:     "r3.1_squeeze_spaces_not_blank",
			Args:     []string{"-s"},
			Stdin:    []byte("a\n \n\t\n \nb\n"),
			ExitCode: 0,
		},
		{
			// R3.1: empty input with -s.
			Name:     "r3.1_squeeze_empty",
			Args:     []string{"-s"},
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// --- R3.2: -s applies across file boundaries ---
		{
			// R3.2: squeeze across two files where blanks span the boundary.
			Name:     "r3.2_squeeze_across_files",
			Args:     []string{"-s", endsBlank, startsBlank},
			ExitCode: 0,
		},
		// --- R3.3: -s combined with -n and -b ---
		{
			// R3.3: -s -n squeezes before numbering; suppressed lines don't
			// consume line numbers.
			Name:     "r3.3_squeeze_with_n",
			Args:     []string{"-sn"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R3.3: -s -b squeezes before numbering non-blank lines.
			Name:     "r3.3_squeeze_with_b",
			Args:     []string{"-sb"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R3.3: -s -n -b together; -b overrides -n, -s squeezes.
			Name:     "r3.3_squeeze_with_nb",
			Args:     []string{"-snb"},
			Stdin:    []byte("a\n\n\n\nb\n"),
			ExitCode: 0,
		},
		{
			// R3.3: -s -n with spaces-only lines (not blank, not squeezed).
			Name:     "r3.3_squeeze_n_spaces_not_blank",
			Args:     []string{"-sn"},
			Stdin:    []byte("a\n \n\n\nb\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile is a test helper that writes content to a file.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
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
