// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tac against the GNU reference binary (gtac).
//
// Implements prd021-tac acceptance criteria AC1-AC5 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtac")
	if err != nil {
		t.Skipf("reference binary gtac not in PATH: %v", err)
	}

	// Create test fixture files in a shared temp directory.
	tmpDir := t.TempDir()
	writeFixture(t, tmpDir, "file1.txt", "alpha\nbeta\ngamma\n")
	writeFixture(t, tmpDir, "file2.txt", "one\ntwo\nthree\n")
	writeFixture(t, tmpDir, "notail.txt", "a\nb\nc")
	writeFixture(t, tmpDir, "single.txt", "only\n")
	writeFixture(t, tmpDir, "empty.txt", "")
	writeFixture(t, tmpDir, "colons.txt", "a:b:c:")
	writeFixture(t, tmpDir, "bcolons.txt", ":a:b:c")

	tests := []testutils.DiffTest{
		// R1.1, R1.2: Default reversal with trailing newline.
		{
			Name:  "tac_default_reversal",
			Stdin: []byte("alpha\nbeta\ngamma\n"),
		},
		// R1.2: No trailing newline.
		{
			Name:  "tac_no_trailing_newline",
			Stdin: []byte("a\nb\nc"),
		},
		// R1.1: Single line.
		{
			Name:  "tac_single_line",
			Stdin: []byte("only\n"),
		},
		// R1.3: Stdin with no file arguments.
		{
			Name:  "tac_stdin_default",
			Stdin: []byte("first\nsecond\nthird\n"),
		},
		// R1.3: "-" reads from stdin.
		{
			Name:  "tac_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("x\ny\nz\n"),
		},
		// R1.4: Single file argument.
		{
			Name: "tac_single_file",
			Args: []string{filepath.Join(tmpDir, "file1.txt")},
		},
		// R1.4: Multiple files processed independently.
		{
			Name: "tac_multi_file",
			Args: []string{filepath.Join(tmpDir, "file1.txt"), filepath.Join(tmpDir, "file2.txt")},
		},
		// R2.1: Custom separator -s ':'.
		{
			Name:  "tac_custom_separator",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c:"),
		},
		// R2.2: -b places separator before record.
		{
			Name:  "tac_before_flag",
			Args:  []string{"-b", "-s", ":"},
			Stdin: []byte(":a:b:c"),
		},
		// R2.1: Custom multi-char separator.
		{
			Name:  "tac_multi_char_separator",
			Args:  []string{"-s", "::"},
			Stdin: []byte("a::b::c::"),
		},
		// R2.3: -r regex separator.
		{
			Name:  "tac_regex_separator",
			Args:  []string{"-r", "-s", "[:|]"},
			Stdin: []byte("a:b|c:"),
		},
		// Empty input.
		{
			Name:  "tac_empty_input",
			Stdin: []byte{},
		},
		// File with no trailing newline.
		{
			Name: "tac_file_no_trailing_newline",
			Args: []string{filepath.Join(tmpDir, "notail.txt")},
		},
		// Empty file.
		{
			Name: "tac_empty_file",
			Args: []string{filepath.Join(tmpDir, "empty.txt")},
		},
		// R3.2: Missing file prints error, continues with remaining files.
		{
			Name:      "tac_missing_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "file1.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// Two lines only.
		{
			Name:  "tac_two_lines",
			Stdin: []byte("hello\nworld\n"),
		},
		// Blank lines in input.
		{
			Name:  "tac_blank_lines",
			Stdin: []byte("a\n\nb\n\nc\n"),
		},
		// -s with newline separator (explicit default).
		{
			Name:  "tac_explicit_newline_sep",
			Args:  []string{"-s", "\n"},
			Stdin: []byte("x\ny\nz\n"),
		},
		// Custom separator without trailing separator.
		{
			Name:  "tac_custom_sep_no_trailing",
			Args:  []string{"-s", ":"},
			Stdin: []byte("a:b:c"),
		},
		// -b with newline separator.
		{
			Name:  "tac_before_newline",
			Args:  []string{"-b"},
			Stdin: []byte("\na\nb\nc"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrBinaryNameNormalizer replaces the binary name prefix in stderr so
// messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gtac:"), []byte("tac:"))
	return b
}

// writeFixture creates a test file with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("failed to write fixture %s: %v", name, err)
	}
}
