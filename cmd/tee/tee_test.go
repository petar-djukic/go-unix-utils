// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against gtee (GNU coreutils).
//
// Covers prd017-tee R1.1, R1.2, R1.3, R1.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for --help and --version where GNU output differs from ours.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skip("reference binary gtee not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.2: no file arguments — passthrough mode
		{
			Name:     "R1.2_passthrough",
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		// R1.1: single file output
		{
			Name:     "R1.1_single_file",
			Args:     []string{filepath.Join(t.TempDir(), "out.txt")},
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		// R1.1: multiple file output
		{
			Name:     "R1.1_multiple_files",
			Args:     []string{filepath.Join(t.TempDir(), "a.txt"), filepath.Join(t.TempDir(), "b.txt")},
			Stdin:    []byte("line1\nline2\n"),
			ExitCode: 0,
		},
		// R1.3: truncate existing file
		{
			Name:     "R1.3_truncate_existing",
			Args:     []string{setupExistingFile(t, "old content\n")},
			Stdin:    []byte("new\n"),
			ExitCode: 0,
		},
		// R1.4: append mode -a
		{
			Name:     "R1.4_append_short",
			Args:     appendArgs("-a", setupExistingFile(t, "old\n")),
			Stdin:    []byte("new\n"),
			ExitCode: 0,
		},
		// R1.4: append mode --append
		{
			Name:     "R1.4_append_long",
			Args:     appendArgs("--append", setupExistingFile(t, "old\n")),
			Stdin:    []byte("new\n"),
			ExitCode: 0,
		},
		// --help exits 0
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// --version exits 0
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// empty stdin — no output
		{
			Name:     "empty_stdin",
			Stdin:    []byte{},
			ExitCode: 0,
		},
		// multi-line input
		{
			Name:     "multi_line",
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupExistingFile creates a temp file with initial content and returns its path.
func setupExistingFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "existing.txt")
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("setupExistingFile: %v", err)
	}
	return path
}

// appendArgs builds an argument slice with the flag followed by file paths.
func appendArgs(flag string, files ...string) []string {
	args := []string{flag}
	args = append(args, files...)
	return args
}
