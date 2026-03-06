// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tac (prd021-tac R4).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing Go tac against gtac.
// R4.1: byte-for-byte comparison via RunDiffTests.
// R4.3: LC_ALL=C set via DiffTest.Env.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// D2: Graceful skip if gtac is not in PATH.
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	// Create temp files for file-based tests.
	dir := t.TempDir()
	file1 := writeTestFile(t, dir, "f1.txt", "alpha\nbeta\ngamma\n")
	file2 := writeTestFile(t, dir, "f2.txt", "x\ny\n")
	fileNoNL := writeTestFile(t, dir, "nonl.txt", "a\nb\nc")

	tests := []testutils.DiffTest{
		// R1.1: Default reversal with trailing newline.
		{
			Name:  "tac_default_reversal",
			Stdin: []byte("alpha\nbeta\ngamma\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: No trailing newline preserved.
		{
			Name:  "tac_no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: Custom separator.
		{
			Name:  "tac_custom_separator",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: Before flag with custom separator.
		{
			Name:  "tac_before_flag",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: Single line reverses to itself.
		{
			Name:  "tac_single_line",
			Stdin: []byte("only\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: Read from file argument.
		{
			Name: "tac_file_arg",
			Args: []string{file1},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: Multiple files reversed independently.
		{
			Name: "tac_multifile",
			Args: []string{file1, file2},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: Stdin via "-" argument.
		{
			Name:  "tac_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("one\ntwo\nthree\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// Empty input produces no output.
		{
			Name:  "tac_empty_input",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: File with no trailing newline.
		{
			Name: "tac_file_no_trailing_newline",
			Args: []string{fileNoNL},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: Regex separator.
		{
			Name:  "tac_regex_separator",
			Args:  []string{"-r", "-s", "[0-9]+"},
			Stdin: []byte("a1b22c"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: Regex with -b (before mode).
		{
			Name:  "tac_regex_before",
			Args:  []string{"-r", "-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file in dir with the given content and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", name, err)
	}
	return path
}
