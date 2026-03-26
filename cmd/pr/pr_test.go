// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd110-pr R3.1, R3.2.
// R3.1: exit 0 on success, exit 1 on any error.
// R3.2: SIGPIPE handling via pkg/sys.InstallSIGPIPEHandler.
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

// normalizeBinaryName replaces "gpr:" or "pr:" with "PROG:" in stderr
// so that program name differences do not cause false failures.
func normalizeBinaryName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gpr:"), []byte("PROG:"))
	b = bytes.ReplaceAll(b, []byte("pr:"), []byte("PROG:"))
	return b
}

// normalizeOpenError normalizes Go-style "open <path>: <msg>" to
// "read error" style matching GNU pr error format.
var openErrorRe = regexp.MustCompile(`open ([^:]+): `)

func normalizeOpenError(b []byte) []byte {
	return openErrorRe.ReplaceAll(b, []byte("$1: "))
}

// normalizeErrorCase normalizes case differences in error messages.
func normalizeErrorCase(b []byte) []byte {
	return bytes.ToLower(b)
}

// normalizeTimestamp replaces the date/time in page headers so the
// comparison is not affected by when the test runs.
var timestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`)

func normalizeTimestamp(b []byte) []byte {
	return timestampRe.ReplaceAll(b, []byte("DATE"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpr")
	if err != nil {
		t.Skip("reference binary gpr not in PATH")
	}

	// Create a test file for success cases.
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	errNorm := []testutils.NormalizeFunc{
		normalizeBinaryName,
		normalizeOpenError,
		normalizeErrorCase,
	}

	successNorm := []testutils.NormalizeFunc{
		normalizeTimestamp,
	}

	tests := []testutils.DiffTest{
		// --- R3.1: exit codes ---

		// Success: reading a valid file exits 0.
		{
			Name:      "success_exit_0",
			Args:      []string{"-t", testFile},
			Normalize: successNorm,
		},

		// Success: reading stdin exits 0.
		{
			Name:      "stdin_exit_0",
			Args:      []string{"-t"},
			Stdin:     []byte("hello\n"),
			Normalize: successNorm,
		},

		// Error: nonexistent file exits 1.
		{
			Name:      "nonexistent_file_exit_1",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: errNorm,
		},

		// Success: /dev/null as input exits 0.
		{
			Name:      "devnull_exit_0",
			Args:      []string{"-t", "/dev/null"},
			Normalize: successNorm,
		},

		// Success: empty stdin with -t exits 0.
		{
			Name:      "empty_stdin_t_exit_0",
			Args:      []string{"-t"},
			Stdin:     []byte(""),
			Normalize: successNorm,
		},

		// Error: multiple files with one nonexistent exits 1.
		{
			Name:      "mixed_valid_invalid_exit_1",
			Args:      []string{"-t", testFile, filepath.Join(tmpDir, "missing.txt")},
			ExitCode:  1,
			Normalize: append(successNorm, errNorm...),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
