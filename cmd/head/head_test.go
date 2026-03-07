// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head against ghead (Homebrew GNU coreutils).
// Implements prd018-head R4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "ghead"

// blankOutput blanks output so only exit codes are compared.
var blankOutput testutils.NormalizeFunc = func(b []byte) []byte { return nil }

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: Default 10 lines.
		{
			Name:  "default 10 lines",
			Args:  []string{},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"),
		},
		// R1.1: Empty stdin.
		{
			Name:  "empty stdin",
			Args:  []string{},
			Stdin: []byte{},
		},
		// R1.2: Explicit -n 5.
		{
			Name:  "n 5",
			Args:  []string{"-n", "5"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		// R1.2: Fewer lines than requested.
		{
			Name:  "fewer lines than n",
			Args:  []string{"-n", "100"},
			Stdin: []byte("a\nb\n"),
		},
		// R1.3: Negative line count.
		{
			Name:  "n negative 5",
			Args:  []string{"-n", "-5"},
			Stdin: []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
		},
		// R1.4: Stdin via dash.
		{
			Name:  "stdin dash",
			Args:  []string{"-n", "2", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R1.5: No trailing newline.
		{
			Name:  "no trailing newline",
			Args:  []string{"-n", "2"},
			Stdin: []byte("a\nb"),
		},
		// R2.1: Byte count.
		{
			Name:  "c 5 bytes",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.2: Negative byte count (short input).
		{
			Name:  "c negative 100 short",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short\n"),
		},
		// R2.2: Negative byte count (sufficient input).
		{
			Name:  "c negative 3",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefgh"),
		},
		// R2.3: Byte count with K suffix.
		{
			Name:  "c 1K suffix",
			Args:  []string{"-c", "1K"},
			Stdin: []byte(strings.Repeat("a", 2048)),
		},
		// R3.3: Quiet mode suppresses headers (uses stdin for simplicity).
		{
			Name:  "quiet flag",
			Args:  []string{"-q"},
			Stdin: []byte("data\n"),
		},
		// --help and --version: blank output, only exit code matters.
		{
			Name:      "help flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		{
			Name:      "version flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMultiFile tests multi-file headers and error handling with real files.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R3.1: Multi-file headers.
	t.Run("multi file headers", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file1.txt"), "1\n2\n")
		writeFile(t, filepath.Join(dir, "file2.txt"), "3\n4\n")

		tests := []testutils.DiffTest{
			{
				Name:    "two files",
				Args:    []string{"file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.2: Single file no header.
	t.Run("single file no header", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file.txt"), "1\n2\n3\n")

		tests := []testutils.DiffTest{
			{
				Name:    "single file",
				Args:    []string{"file.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.3: Quiet mode with multiple files.
	t.Run("quiet multi file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file1.txt"), "1\n")
		writeFile(t, filepath.Join(dir, "file2.txt"), "2\n")

		tests := []testutils.DiffTest{
			{
				Name:    "quiet two files",
				Args:    []string{"-q", "file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.4: Verbose mode with single file.
	t.Run("verbose single file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file.txt"), "data\n")

		tests := []testutils.DiffTest{
			{
				Name:    "verbose single",
				Args:    []string{"-v", "file.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.5, R4.2: Error on missing file, continue processing.
	t.Run("missing file error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "real.txt"), "data\n")

		tests := []testutils.DiffTest{
			{
				Name:      "missing then real",
				Args:      []string{"missing.txt", "real.txt"},
				WorkDir:   dir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{blankOutput},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
