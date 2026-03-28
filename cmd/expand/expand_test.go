// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd024-expand R4.1-R4.3: differential tests comparing Go expand
// against gexpand for all flag combinations, edge cases, and error conditions.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrClearer returns a normalizer that empties stderr output.
// Used for error-condition tests where exit code is the primary check.
func stderrClearer() testutils.NormalizeFunc {
	return func(b []byte) []byte { return nil }
}

// TestDiff runs differential tests comparing Go expand against gexpand.
// R4.1: byte-for-byte comparison via pkg/testutils.RunDiffTests.
// R4.3: LC_ALL=C set by RunDiffTests default.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}
	tests := buildDiffTests(t)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildDiffTests returns all differential test cases.
func buildDiffTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	var tests []testutils.DiffTest
	tests = append(tests, defaultTabTests()...)
	tests = append(tests, customTabTests()...)
	tests = append(tests, edgeCaseTests()...)
	tests = append(tests, fileInputTests(t)...)
	tests = append(tests, errorTests(t)...)
	return tests
}

// defaultTabTests covers R1.1-R1.4: default tab expansion at every 8 columns.
func defaultTabTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "default_single_tab",
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "default_multiple_tabs",
			Stdin: []byte("\t\t\n"),
		},
		{
			Name:  "default_tab_at_column_boundary",
			Stdin: []byte("12345678\tx\n"),
		},
		{
			Name:  "default_consecutive_tabs_mid_line",
			Stdin: []byte("a\t\t\tb\n"),
		},
		{
			Name:  "default_multiline",
			Stdin: []byte("a\tb\ncc\tdd\n"),
		},
		{
			Name:  "no_tabs_passthrough",
			Stdin: []byte("hello world\n"),
		},
	}
}

// customTabTests covers R2.1-R2.4: -t with single interval and tab stop list.
func customTabTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "tab_interval_4",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "tab_interval_1",
			Args:  []string{"-t", "1"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "tab_interval_short_form",
			Args:  []string{"-t4"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "tabs_long_form_equals",
			Args:  []string{"--tabs=4"},
			Stdin: []byte("x\ty\n"),
		},
		{
			Name:  "tabs_long_form_separate",
			Args:  []string{"--tabs", "4"},
			Stdin: []byte("x\ty\n"),
		},
		{
			Name:  "tab_list_comma",
			Args:  []string{"-t", "4,8,12"},
			Stdin: []byte("\t\t\t\tx\n"),
		},
		{
			Name:  "tab_list_past_last_stop",
			Args:  []string{"-t", "1,5,9"},
			Stdin: []byte("a\tb\tc\n"),
		},
		{
			Name:  "tab_list_single_value",
			Args:  []string{"-t", "3"},
			Stdin: []byte("a\tb\tc\n"),
		},
	}
}

// edgeCaseTests covers R4.2: edge cases including empty input and long lines.
func edgeCaseTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "empty_input",
			Stdin: []byte{},
		},
		{
			Name:  "only_newline",
			Stdin: []byte("\n"),
		},
		{
			Name:  "only_tabs",
			Stdin: []byte("\t\t\t\n"),
		},
		{
			Name:  "binary_data_with_nulls",
			Stdin: []byte("a\x00b\tc\n"),
		},
		{
			Name:  "long_line_with_tabs",
			Stdin: []byte(strings.Repeat("x", 200) + "\t" + "y\n"),
		},
		{
			Name:  "tab_at_end_of_line",
			Stdin: []byte("hello\t\n"),
		},
		{
			Name:  "no_trailing_newline",
			Stdin: []byte("a\tb"),
		},
		{
			Name:  "stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("a\tb\n"),
		},
	}
}

// fileInputTests covers R4.2: multiple file input.
func fileInputTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	f1 := filepath.Join(dir, "f1.txt")
	f2 := filepath.Join(dir, "f2.txt")
	writeTestFile(t, f1, "a\tb\n")
	writeTestFile(t, f2, "c\td\n")
	return []testutils.DiffTest{
		{
			Name: "single_file",
			Args: []string{f1},
		},
		{
			Name: "multiple_files",
			Args: []string{f1, f2},
		},
		{
			Name:  "file_and_stdin",
			Args:  []string{f1, "-"},
			Stdin: []byte("e\tf\n"),
		},
		{
			Name: "file_with_custom_tabs",
			Args: []string{"-t", "4", f1},
		},
	}
}

// errorTests covers R4.3: error conditions with exit code verification.
// Stderr is cleared via normalizer because error message format differs
// between Go expand and gexpand; exit code is the primary check.
func errorTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	missing := filepath.Join(dir, "nonexistent.txt")
	errNorm := []testutils.NormalizeFunc{stderrClearer()}
	return []testutils.DiffTest{
		{
			Name:      "missing_file",
			Args:      []string{missing},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "invalid_tab_value_zero",
			Args:      []string{"-t", "0"},
			Stdin:     []byte("a\n"),
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "invalid_tab_value_negative",
			Args:      []string{"-t", "-1"},
			Stdin:     []byte("a\n"),
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "invalid_tab_value_alpha",
			Args:      []string{"-t", "abc"},
			Stdin:     []byte("a\n"),
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
}

// writeTestFile creates a test file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}
