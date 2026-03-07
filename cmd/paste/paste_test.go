// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against the GNU reference binary (gpaste).
//
// Implements prd027-paste acceptance criteria AC1-AC5 via testutils.RunDiffTests.
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
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	// Create test fixture files in a shared temp directory.
	tmpDir := t.TempDir()
	writeFixture(t, tmpDir, "file1.txt", "a\nb\n")
	writeFixture(t, tmpDir, "file2.txt", "1\n2\n")
	writeFixture(t, tmpDir, "file3.txt", "a\nb\nc\n")
	writeFixture(t, tmpDir, "file4.txt", "1\n")

	f1 := filepath.Join(tmpDir, "file1.txt")
	f2 := filepath.Join(tmpDir, "file2.txt")
	f3 := filepath.Join(tmpDir, "file3.txt")
	f4 := filepath.Join(tmpDir, "file4.txt")

	tests := []testutils.DiffTest{
		// R1.1: Two files merged with default tab delimiter.
		{
			Name:    "paste_two_files_default",
			Args:    []string{f1, f2},
			WorkDir: tmpDir,
		},
		// R2.1: Custom delimiter.
		{
			Name:    "paste_custom_delimiter",
			Args:    []string{"-d:", f1, f2},
			WorkDir: tmpDir,
		},
		// R3.1: Serial mode.
		{
			Name:    "paste_serial_mode",
			Args:    []string{"-s", f1},
			WorkDir: tmpDir,
		},
		// R1.2: Unequal file lengths.
		{
			Name:    "paste_unequal_length",
			Args:    []string{f3, f4},
			WorkDir: tmpDir,
		},
		// R1.3: stdin via -.
		{
			Name:    "paste_stdin_dash",
			Args:    []string{"-d:", "-", f2},
			Stdin:   []byte("x\ny\n"),
			WorkDir: tmpDir,
		},
		// R2.1, R2.3: Delimiter cycling with three files.
		{
			Name:    "paste_delimiter_cycling",
			Args:    []string{"-d,:", f1, f2, f1},
			WorkDir: tmpDir,
		},
		// R3.1: Serial mode with multiple files.
		{
			Name:    "paste_serial_multiple",
			Args:    []string{"-s", f1, f2},
			WorkDir: tmpDir,
		},
		// R3.2: Serial mode with custom delimiter.
		{
			Name:    "paste_serial_custom_delim",
			Args:    []string{"-s", "-d:", f3},
			WorkDir: tmpDir,
		},
		// R1.4: No files, passthrough from stdin.
		{
			Name:  "paste_stdin_passthrough",
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.1: Single file.
		{
			Name:    "paste_single_file",
			Args:    []string{f1},
			WorkDir: tmpDir,
		},
		// R4.2: Missing file should exit 1.
		{
			Name:      "paste_missing_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			WorkDir:   tmpDir,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// R1.2: Both files exhausted at different times.
		{
			Name:    "paste_unequal_three_vs_two",
			Args:    []string{f3, f2},
			WorkDir: tmpDir,
		},
		// Empty input.
		{
			Name:  "paste_empty_stdin",
			Stdin: []byte{},
		},
		// R2.2: Backslash-n escape in delimiter.
		{
			Name:    "paste_delim_escape_n",
			Args:    []string{`-d\n`, f1, f2},
			WorkDir: tmpDir,
		},
		// R3.1: Serial with single line file.
		{
			Name:    "paste_serial_single_line",
			Args:    []string{"-s", f4},
			WorkDir: tmpDir,
		},
		// Combined -s and -d.
		{
			Name:    "paste_serial_with_delim",
			Args:    []string{"-s", "-d,", f1},
			WorkDir: tmpDir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrBinaryNameNormalizer replaces the binary name prefix in stderr so
// messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gpaste:"), []byte("paste:"))
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
