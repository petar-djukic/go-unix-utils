// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against gtee (GNU coreutils).
//
// Covers prd017-tee R4.1 (exit codes), R4.2 (test coverage), R4.3 (file-stdout match).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for --help and --version where GNU output text differs from ours.
func discardAll(data []byte) []byte {
	return nil
}

// normalizeDiag lowercases output and normalizes the program name prefix
// so "gtee:" and "tee:" compare identically.
func normalizeDiag(data []byte) []byte {
	data = bytes.ToLower(data)
	data = bytes.ReplaceAll(data, []byte("gtee:"), []byte("tee:"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not in PATH")
	}

	tests := buildDiffTests(t)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildDiffTests returns all differential test cases for tee.
func buildDiffTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	return []testutils.DiffTest{
		// R1.2: no file arguments — passthrough mode
		{
			Name:     "passthrough",
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		// R1.1: single file output
		{
			Name:     "single_file",
			Args:     []string{filepath.Join(t.TempDir(), "out.txt")},
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		// R1.1: multiple file output
		{
			Name:  "multiple_files",
			Args:  []string{filepath.Join(t.TempDir(), "a.txt"), filepath.Join(t.TempDir(), "b.txt")},
			Stdin: []byte("line1\nline2\n"),
		},
		// R1.3: truncate existing file
		{
			Name:  "truncate_existing",
			Args:  []string{setupExistingFile(t, "old content\n")},
			Stdin: []byte("new\n"),
		},
		// R2.1: append mode -a
		{
			Name:  "append_short",
			Args:  appendArgs("-a", setupExistingFile(t, "old\n")),
			Stdin: []byte("new\n"),
		},
		// R2.1: append mode --append
		{
			Name:  "append_long",
			Args:  appendArgs("--append", setupExistingFile(t, "old\n")),
			Stdin: []byte("new\n"),
		},
		// R2.2: -i flag (no signal, verifies normal operation is unaffected)
		{
			Name:  "ignore_interrupts",
			Args:  []string{"-i"},
			Stdin: []byte("data\n"),
		},
		// R2.3: combined -ai flags
		{
			Name:  "combined_ai",
			Args:  appendArgs("-ai", setupExistingFile(t, "old\n")),
			Stdin: []byte("new\n"),
		},
		// R3.1: exit 0 on success
		{
			Name:  "exit_0_success",
			Args:  []string{filepath.Join(t.TempDir(), "ok.txt")},
			Stdin: []byte("ok\n"),
		},
		// R3.2: exit 1 when output file cannot be opened
		{
			Name:      "open_error_exit_1",
			Args:      []string{"/nonexistent_tee_test_dir/file.txt"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDiag},
		},
		// R4.3: --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// empty stdin
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		// multi-line input
		{
			Name:  "multi_line",
			Stdin: []byte("a\nb\nc\n"),
		},
	}
}

// TestFileContentMatchesStdout verifies R4.3: data written to each
// output file matches stdout byte-for-byte.
func TestFileContentMatchesStdout(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	input := []byte("line1\nline2\nline3\n")
	outFile := filepath.Join(t.TempDir(), "verify.txt")

	stdout := runTee(t, goBin, []string{outFile}, input)
	fileContent := readFileOrFail(t, outFile)

	if !bytes.Equal(stdout, fileContent) {
		t.Errorf("file content does not match stdout\n"+
			"  stdout: %q\n  file:   %q", stdout, fileContent)
	}
}

// TestMultiFileContentMatch verifies R4.3 for multiple output files.
func TestMultiFileContentMatch(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	input := []byte("abc\ndef\n")
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")

	stdout := runTee(t, goBin, []string{fileA, fileB}, input)

	for _, path := range []string{fileA, fileB} {
		content := readFileOrFail(t, path)
		if !bytes.Equal(stdout, content) {
			t.Errorf("%s does not match stdout\n"+
				"  stdout: %q\n  file:   %q", path, stdout, content)
		}
	}
}

// runTee executes the tee binary and returns captured stdout.
func runTee(t *testing.T, bin string, args []string, stdin []byte) []byte {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("tee failed: %v", err)
	}
	return stdout.Bytes()
}

// readFileOrFail reads a file and fails the test on error.
func readFileOrFail(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// setupExistingFile creates a temp file with initial content and returns its path.
func setupExistingFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setupExistingFile: %v", err)
	}
	return path
}

// appendArgs builds an argument slice with the flag followed by file paths.
func appendArgs(flag string, files ...string) []string {
	return append([]string{flag}, files...)
}
