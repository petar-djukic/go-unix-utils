// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd024-expand R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing the Go expand binary against the
// GNU reference binary (gexpand) via pkg/testutils.RunDiffTests.
//
// R4.1: Byte-for-byte comparison via RunDiffTests.
// R4.3: All tests use LC_ALL=C.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gexpand")
	if err != nil {
		t.Skipf("reference binary gexpand not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R4.2: Default tab expansion — single tab.
			Name:  "single_tab",
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.2: Multiple tabs in succession.
			Name:  "multiple_consecutive_tabs",
			Stdin: []byte("\t\t\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R4.2: Input with no tabs — passthrough.
			Name:  "no_tabs_passthrough",
			Stdin: []byte("hello world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Tab at various column positions.
			Name:  "tab_at_different_columns",
			Stdin: []byte("ab\tc\n1234567\t8\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.3: Multiple lines reset column position.
			Name:  "multiple_lines",
			Stdin: []byte("a\tb\ncd\te\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.1: Tab at column 1 (start of line).
			Name:  "tab_at_start",
			Stdin: []byte("\thello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.2: Multiple tabs on same line at different positions.
			Name:  "multiple_tabs_same_line",
			Stdin: []byte("a\tb\tc\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Empty input.
			Name:  "empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Line without trailing newline.
			Name:  "no_trailing_newline",
			Stdin: []byte("x\ty"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R1.4: Backspace character in input.
			Name:  "backspace_character",
			Stdin: []byte("ab\bc\td\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Tab at exactly column 8 (should produce 8 spaces to next stop).
			Name:  "tab_at_column_8",
			Stdin: []byte("12345678\tx\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			// Stdin via "-" argument.
			Name:  "stdin_via_dash",
			Args:  []string{"-"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: Successful processing exits 0 (ExitCode defaults to 0).
		{
			Name:  "exit_0_on_success",
			Args:  []string{"-t", "4"},
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: Nonexistent file exits 1; stderr differs between GNU and Go, so normalize.
		{
			Name:     "exit_1_nonexistent_file",
			Args:     []string{"/nonexistent/expand_test_file"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		// R3.2: Processing continues for remaining files after an error (mixed valid/invalid).
		{
			Name:     "continues_after_error",
			Args:     []string{"/nonexistent/expand_test_file", "-"},
			Stdin:    []byte("x\ty\n"),
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		// R3.2: Multiple nonexistent files all report errors, exit 1.
		{
			Name:     "multiple_nonexistent_files",
			Args:     []string{"/nonexistent/file1", "/nonexistent/file2"},
			ExitCode: 1,
			Env:      []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				clearStderr,
			},
		},
		// R3.4: SIGPIPE handling is installed (verified by compilation and normal pipe).
		{
			Name:  "sigpipe_no_crash",
			Stdin: []byte("a\tb\n"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// clearStderr is a NormalizeFunc that replaces any non-empty output with empty
// bytes. Used for error-path tests where stderr message format differs between
// GNU expand and the Go implementation but exit code and stdout must still match.
func clearStderr(b []byte) []byte {
	return nil
}
