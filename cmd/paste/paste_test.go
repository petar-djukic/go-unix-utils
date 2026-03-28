// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against GNU gpaste.
// Covers prd027-paste R4.1-R4.4 (differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// writeTestFile creates a file with the given content in dir and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", p, err)
	}
	return p
}

// stderrNormalizer normalizes error messages between GNU gpaste and Go paste.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?paste|gpaste`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	permDenied := regexp.MustCompile(`(?i)permission denied`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("paste"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		b = permDenied.ReplaceAll(b, []byte("Permission denied"))
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	dir := t.TempDir()

	// Test input files.
	fileA := writeTestFile(t, dir, "a.txt", "a\nb\nc\n")
	fileB := writeTestFile(t, dir, "b.txt", "1\n2\n3\n")
	fileShort := writeTestFile(t, dir, "short.txt", "x\n")
	fileLong := writeTestFile(t, dir, "long.txt", "p\nq\nr\ns\n")
	fileEmpty := writeTestFile(t, dir, "empty.txt", "")
	fileSingle := writeTestFile(t, dir, "single.txt", "only\n")
	fileNoNewline := writeTestFile(t, dir, "nonl.txt", "noterminal")

	tests := []testutils.DiffTest{
		// --- R4.1: Serial mode (-s) differential tests ---

		// R3.1: serial mode with a single file.
		{
			Name: "serial_single_file",
			Args: []string{"-s", fileA},
		},
		// R3.1: serial mode with multiple files.
		{
			Name: "serial_multiple_files",
			Args: []string{"-s", fileA, fileB},
		},
		// R3.2: serial mode with custom delimiter.
		{
			Name: "serial_custom_delimiter",
			Args: []string{"-s", "-d:", fileA},
		},
		// R3.2: serial mode with delimiter list cycling.
		{
			Name: "serial_delimiter_list",
			Args: []string{"-s", "-d:,", fileA},
		},
		// R3.1/R3.2: serial mode with multiple files and delimiter list.
		{
			Name: "serial_multi_file_delim_list",
			Args: []string{"-s", "-d:,", fileA, fileB},
		},
		// Serial mode with a single-line file.
		{
			Name: "serial_single_line_file",
			Args: []string{"-s", fileSingle},
		},
		// Serial mode with empty file.
		{
			Name: "serial_empty_file",
			Args: []string{"-s", fileEmpty},
		},
		// Serial mode with empty and non-empty files mixed.
		{
			Name: "serial_mixed_empty",
			Args: []string{"-s", fileEmpty, fileA, fileEmpty},
		},

		// --- R4.2: Error handling differential tests ---

		// Nonexistent file — exit 1.
		{
			Name:      "error_nonexistent_file",
			Args:      []string{"/nonexistent-path/no-such-file.txt"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Nonexistent file in serial mode — exit 1.
		{
			Name:      "error_nonexistent_serial",
			Args:      []string{"-s", "/nonexistent-path/no-such-file.txt"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Nonexistent file mixed with valid file.
		{
			Name:      "error_nonexistent_mixed",
			Args:      []string{fileA, "/nonexistent-path/no-such-file.txt"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// --- R4.3: Stdin as '-' differential tests ---

		// Single stdin via '-'.
		{
			Name:  "stdin_single_dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
		},
		// Stdin mixed with a file.
		{
			Name:  "stdin_mixed_with_file",
			Args:  []string{"-", fileA},
			Stdin: []byte("x\ny\nz\n"),
		},
		// File then stdin.
		{
			Name:  "file_then_stdin",
			Args:  []string{fileA, "-"},
			Stdin: []byte("1\n2\n3\n"),
		},
		// Multiple '-' arguments (sequential reads from stdin).
		{
			Name:  "stdin_multiple_dashes",
			Args:  []string{"-", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// Stdin in serial mode.
		{
			Name:  "stdin_serial",
			Args:  []string{"-s", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// No files given (implicit stdin).
		{
			Name:  "implicit_stdin",
			Args:  []string{},
			Stdin: []byte("line1\nline2\n"),
		},

		// --- R4.4: Edge cases differential tests ---

		// Empty file in parallel mode.
		{
			Name: "edge_empty_file_parallel",
			Args: []string{fileEmpty, fileA},
		},
		// Both files empty.
		{
			Name: "edge_both_empty",
			Args: []string{fileEmpty, fileEmpty},
		},
		// Single-line file paired with multi-line file (unequal lengths).
		{
			Name: "edge_unequal_lengths",
			Args: []string{fileShort, fileLong},
		},
		// Three files with different lengths.
		{
			Name: "edge_three_unequal",
			Args: []string{fileShort, fileA, fileLong},
		},
		// File without trailing newline.
		{
			Name: "edge_no_trailing_newline",
			Args: []string{fileNoNewline},
		},
		// File without trailing newline paired with normal file.
		{
			Name: "edge_no_newline_parallel",
			Args: []string{fileNoNewline, fileA},
		},
		// Serial mode with file without trailing newline.
		{
			Name: "edge_no_newline_serial",
			Args: []string{"-s", fileNoNewline},
		},
		// Empty stdin.
		{
			Name:  "edge_empty_stdin",
			Args:  []string{"-"},
			Stdin: []byte{},
		},
		// Single-line file in serial mode.
		{
			Name: "edge_single_line_serial",
			Args: []string{"-s", fileSingle},
		},
		// Mixed stdin and file with empty stdin.
		{
			Name:  "edge_empty_stdin_with_file",
			Args:  []string{"-", fileA},
			Stdin: []byte{},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
