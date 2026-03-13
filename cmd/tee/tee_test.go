// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee.
//
// Implements: prd017-tee R4.1, R4.3
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGtee = "gtee"

// teeErrRe matches a tee or gtee error line and normalizes the program name
// and error message format differences between the Go and GNU implementations.
var teeErrRe = regexp.MustCompile(`(?m)^g?tee: (?:open )?(.+?): .+$`)

// normalizeTeeErrors replaces tee/gtee error lines with a canonical form.
func normalizeTeeErrors(b []byte) []byte {
	return teeErrRe.ReplaceAll(b, []byte("PROG: $1: ERROR"))
}

// TestDiff runs differential tests comparing stdout, stderr, and exit codes
// between the Go tee binary and the GNU gtee reference binary.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGtee)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGtee, err)
	}

	dir := t.TempDir()
	readOnlyPath := filepath.Join(dir, "no-such-dir", "file.txt")

	tests := []testutils.DiffTest{
		// R1.2: passthrough mode — no file arguments, stdin copied to stdout only.
		{
			Name:     "r1.2_passthrough_stdin_to_stdout",
			Stdin:    []byte("hello\nworld\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: empty stdin — exits 0, no output.
		{
			Name:     "r1.1_empty_stdin",
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.1: binary data passes through unchanged.
		{
			Name:     "r1.1_binary_data",
			Stdin:    []byte("\x00\x01\x02\xff\xfe\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R3.2: write to a nonexistent directory fails with exit 1.
		{
			Name:      "r3.2_write_error_exit1",
			Args:      []string{readOnlyPath},
			Stdin:     []byte("data\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeTeeErrors},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestFileOutput verifies that tee writes correct file content. These tests
// run only the Go binary since both binaries in differential testing would
// write to the same file path, making file content checks unreliable.
func TestFileOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// R1.1, R1.3: single file — stdin written to stdout and file.
	t.Run("single_file_create", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		outFile := filepath.Join(outDir, "out.txt")
		input := []byte("hello\nworld\n")

		stdout, exitCode := runGoBin(t, goBin, []string{outFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R4.3: file content matches stdout byte-for-byte.
		assertFileContent(t, outFile, input)
	})

	// R1.1: multiple files — stdin written to stdout and all files.
	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		fileA := filepath.Join(outDir, "a.txt")
		fileB := filepath.Join(outDir, "b.txt")
		input := []byte("line1\nline2\n")

		stdout, exitCode := runGoBin(t, goBin, []string{fileA, fileB}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		if !bytes.Equal(stdout, input) {
			t.Errorf("stdout mismatch: got %q, want %q", stdout, input)
		}
		// R4.3: both files match stdout.
		assertFileContent(t, fileA, input)
		assertFileContent(t, fileB, input)
	})

	// R1.3: overwrite existing file — file is truncated before writing.
	t.Run("overwrite_existing", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		existingFile := filepath.Join(outDir, "existing.txt")
		if err := os.WriteFile(existingFile, []byte("old content that is longer\n"), 0o644); err != nil {
			t.Fatalf("writing existing file: %v", err)
		}
		input := []byte("new\n")

		_, exitCode := runGoBin(t, goBin, []string{existingFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		assertFileContent(t, existingFile, input)
	})

	// R1.3: file created when it does not exist.
	t.Run("creates_nonexistent_file", func(t *testing.T) {
		t.Parallel()
		outDir := t.TempDir()
		newFile := filepath.Join(outDir, "brand-new.txt")
		input := []byte("created\n")

		_, exitCode := runGoBin(t, goBin, []string{newFile}, input)
		if exitCode != 0 {
			t.Errorf("expected exit 0, got %d", exitCode)
		}
		assertFileContent(t, newFile, input)
	})
}

// runGoBin executes the Go tee binary with the given args and stdin,
// returning captured stdout and exit code.
func runGoBin(t *testing.T, binary string, args []string, stdin []byte) (stdout []byte, exitCode int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary %q: %v", binary, err)
		}
	}
	return outBuf.Bytes(), exitCode
}

// assertFileContent reads the file at path and compares its content to expected.
func assertFileContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !bytes.Equal(content, expected) {
		t.Errorf("file %s content mismatch: got %q, want %q", path, content, expected)
	}
}
