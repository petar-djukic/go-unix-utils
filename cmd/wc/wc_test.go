// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc against gwc (Homebrew GNU coreutils).
// Implements prd005-wc R4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "gwc"

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
		// R1.1: Default (lines, words, bytes).
		{
			Name:  "default three lines",
			Args:  []string{},
			Stdin: []byte("foo\nbar baz\nqux\n"),
		},
		// R2.1: -l counts newlines.
		{
			Name:  "lines only",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo\nthree\n"),
		},
		// R2.2: -w counts words.
		{
			Name:  "words only",
			Args:  []string{"-w"},
			Stdin: []byte("hello world\ngoodbye  cruel   world\n"),
		},
		// R2.3: -c counts bytes.
		{
			Name:  "bytes only",
			Args:  []string{"-c"},
			Stdin: []byte("abc\n"),
		},
		// R2.4, R5.1, R5.2: -m with LC_ALL=C equals -c.
		{
			Name:  "chars lc c",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
		},
		// R2.5: -L max line length.
		{
			Name:  "max line length",
			Args:  []string{"-L"},
			Stdin: []byte("short\na much longer line here\nmed\n"),
		},
		// R2.6: Combined flags produce counts in fixed order.
		{
			Name:  "combined flags order",
			Args:  []string{"-w", "-l", "-c"},
			Stdin: []byte("one two\nthree\n"),
		},
		// R4.1: "-" as explicit filename reads stdin.
		{
			Name:  "stdin dash",
			Args:  []string{"-"},
			Stdin: []byte("stdin content\n"),
		},
		// R4.3: Empty stdin.
		{
			Name:  "empty stdin",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// R2.3: -c/-m last wins (c wins).
		{
			Name:  "c m last wins c",
			Args:  []string{"-mc"},
			Stdin: []byte("abc\n"),
		},
		// R2.3: -c/-m last wins (m wins).
		{
			Name:  "c m last wins m",
			Args:  []string{"-cm"},
			Stdin: []byte("abc\n"),
		},
		// -L combined with other flags.
		{
			Name:  "L combined with lwc",
			Args:  []string{"-lwcL"},
			Stdin: []byte("short\na longer line\n"),
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

// TestDiffMultiFile tests multi-file output, totals, and error handling.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R1.4, R3.1, R3.2: Multi-file with total.
	t.Run("multi file total", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file1.txt"), "hello\nworld\n")
		writeFile(t, filepath.Join(dir, "file2.txt"), "foo bar baz\n")

		tests := []testutils.DiffTest{
			{
				Name:    "two files",
				Args:    []string{"file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.1: Single file, no total line.
	t.Run("single file no total", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file.txt"), "one two three\nfour\n")

		tests := []testutils.DiffTest{
			{
				Name:    "single file",
				Args:    []string{"file.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R3.1: Multi-file with -l only.
	t.Run("multi file lines only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "a.txt"), "one\ntwo\n")
		writeFile(t, filepath.Join(dir, "b.txt"), "three\n")

		tests := []testutils.DiffTest{
			{
				Name:    "two files lines",
				Args:    []string{"-l", "a.txt", "b.txt"},
				WorkDir: dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R6.2: Missing file error, continue processing.
	t.Run("missing file error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "real.txt"), "data\n")

		tests := []testutils.DiffTest{
			{
				Name:      "missing then real",
				Args:      []string{"nonexistent.txt", "real.txt"},
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
