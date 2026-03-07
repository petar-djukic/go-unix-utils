// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/paste against the GNU reference binary (gpaste).
// Implements prd027-paste R1-R4 verification.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrEraser clears stderr so error message wording and program name
// differences between our binary and gpaste do not cause false failures.
var stderrEraser testutils.NormalizeFunc = func(b []byte) []byte {
	return nil
}

// setupFiles creates temporary files in dir and returns the directory.
func setupFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpaste")
	if err != nil {
		t.Skipf("reference binary gpaste not in PATH: %v", err)
	}

	// R1.1: Two files merged with default tab delimiter.
	t.Run("two_files_default", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\n",
			"file2.txt": "1\n2\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "two_files_default",
				Args:    []string{"file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		})
	})

	// R2.1: Custom delimiter with -d.
	t.Run("custom_delimiter", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\n",
			"file2.txt": "1\n2\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "custom_delimiter",
				Args:    []string{"-d:", "file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		})
	})

	// R3.1: Serial mode with -s.
	t.Run("serial_mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "serial_mode",
				Args:    []string{"-s", "file1.txt"},
				WorkDir: dir,
			},
		})
	})

	// R3.1: Serial mode with multiple files.
	t.Run("serial_multiple_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\nc\n",
			"file2.txt": "1\n2\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "serial_multiple_files",
				Args:    []string{"-s", "file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		})
	})

	// R1.2: Unequal file lengths — shorter file contributes empty fields.
	t.Run("unequal_length", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\nc\n",
			"file2.txt": "1\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "unequal_length",
				Args:    []string{"file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		})
	})

	// R1.3: stdin via "-" operand.
	t.Run("stdin_dash", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file2.txt": "1\n2\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "stdin_dash",
				Args:    []string{"-d:", "-", "file2.txt"},
				Stdin:   []byte("x\ny\n"),
				WorkDir: dir,
			},
		})
	})

	// R2.1/R2.3: Delimiter cycling across fields.
	t.Run("delimiter_cycling", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\n",
			"file2.txt": "1\n2\n",
			"file3.txt": "x\ny\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "delimiter_cycling",
				Args:    []string{"-d,:", "file1.txt", "file2.txt", "file3.txt"},
				WorkDir: dir,
			},
		})
	})

	// R2.2: Backslash escape \t in delimiter list.
	t.Run("escape_tab", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\n",
			"file2.txt": "1\n2\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "escape_tab",
				Args:    []string{"-d\t", "file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		})
	})

	// R1.4: No files — passthrough stdin.
	t.Run("stdin_passthrough", func(t *testing.T) {
		t.Parallel()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:  "stdin_passthrough",
				Args:  []string{},
				Stdin: []byte("hello\nworld\n"),
			},
		})
	})

	// R3.2: Serial mode with custom delimiter.
	t.Run("serial_custom_delim", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\nc\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "serial_custom_delim",
				Args:    []string{"-s", "-d:", "file1.txt"},
				WorkDir: dir,
			},
		})
	})

	// R4.2: Error on nonexistent file.
	t.Run("error_nonexistent", func(t *testing.T) {
		t.Parallel()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "error_nonexistent",
				Args:      []string{"nonexistent.txt"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{stderrEraser},
			},
		})
	})

	// Three files with default delimiter.
	t.Run("three_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"a.txt": "a\nb\n",
			"b.txt": "1\n2\n",
			"c.txt": "x\ny\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "three_files",
				Args:    []string{"a.txt", "b.txt", "c.txt"},
				WorkDir: dir,
			},
		})
	})

	// Single file in parallel mode.
	t.Run("single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\nc\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "single_file",
				Args:    []string{"file1.txt"},
				WorkDir: dir,
			},
		})
	})

	// R2.2: Backslash escape \0 (empty delimiter).
	t.Run("escape_zero", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"file1.txt": "a\nb\n",
			"file2.txt": "1\n2\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "escape_zero",
				Args:    []string{`-d\0`, "file1.txt", "file2.txt"},
				WorkDir: dir,
			},
		})
	})

	// Empty files.
	t.Run("empty_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"empty1.txt": "",
			"empty2.txt": "",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "empty_files",
				Args:    []string{"empty1.txt", "empty2.txt"},
				WorkDir: dir,
			},
		})
	})

	// One empty, one non-empty file.
	t.Run("one_empty_one_nonempty", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFiles(t, dir, map[string]string{
			"empty.txt":    "",
			"nonempty.txt": "a\nb\n",
		})
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "one_empty_one_nonempty",
				Args:    []string{"empty.txt", "nonempty.txt"},
				WorkDir: dir,
			},
		})
	})
}
