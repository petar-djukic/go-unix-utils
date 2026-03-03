// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/wc differential tests verify output parity between the Go wc binary and
// the GNU reference binary gwc (Homebrew coreutils). All tests run with
// LC_ALL=C to eliminate locale-dependent divergence.
//
// Implements: prd005-wc R1-R6
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component, DD2, DD4, DD6)
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var (
	goBin  string
	refBin string
)

func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gwc")
	if err == nil {
		refBin = ref
	}

	tmpDir, err := os.MkdirTemp("", "wc-test-*")
	if err != nil {
		os.Stderr.WriteString("failed to create temp dir: " + err.Error() + "\n")
		os.Exit(1)
	}

	goBin = filepath.Join(tmpDir, "wc")
	build := exec.Command("go", "build", "-o", goBin, ".")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		os.RemoveAll(tmpDir) // best-effort cleanup
		os.Stderr.WriteString("go build failed: " + string(out) + "\n")
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// skipIfMissing skips the current test when gwc is not available on PATH.
// (AC4: tests skip gracefully)
func skipIfMissing(t *testing.T) {
	t.Helper()
	if refBin == "" {
		t.Skip("gwc not found in PATH")
	}
}

// writeFile creates a file in dir with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// progNameNormalizer replaces "gwc:" with "wc:" in output so error messages
// from the GNU reference binary (installed as gwc) match the Go binary's
// "wc:" prefix.
func progNameNormalizer(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gwc:"), []byte("wc:"))
}

// errPresenceNormalizer replaces any non-empty output with a fixed marker.
// Used for test cases where stderr format differs between implementations but
// both must produce non-empty error output. Safe when stdout is empty for both
// binaries.
func errPresenceNormalizer(b []byte) []byte {
	if len(b) > 0 {
		return []byte("OUTPUT\n")
	}
	return b
}

// TestWcStdin tests stdin-based invocations with various flag combinations.
// (prd005-wc R1.1, R1.2, R1.3, R2.1-R2.6, R4.1, R4.3, R5.1, R5.2, R6.1)
func TestWcStdin(t *testing.T) {
	skipIfMissing(t)

	tests := []testutils.DiffTest{
		{
			Name:     "default_three_lines",
			Stdin:    []byte("foo\nbar baz\nqux\n"),
			ExitCode: 0,
		},
		{
			Name:     "lines_only",
			Args:     []string{"-l"},
			Stdin:    []byte("one\ntwo\nthree\n"),
			ExitCode: 0,
		},
		{
			Name:     "words_only",
			Args:     []string{"-w"},
			Stdin:    []byte("hello world\ngoodbye  cruel   world\n"),
			ExitCode: 0,
		},
		{
			Name:     "bytes_only",
			Args:     []string{"-c"},
			Stdin:    []byte("abc\n"),
			ExitCode: 0,
		},
		{
			Name:     "chars_lc_c",
			Args:     []string{"-m"},
			Stdin:    []byte("hello\n"),
			ExitCode: 0,
		},
		{
			Name:     "max_line_length",
			Args:     []string{"-L"},
			Stdin:    []byte("short\na much longer line here\nmed\n"),
			ExitCode: 0,
		},
		{
			Name:     "combined_lwc_flag_order",
			Args:     []string{"-w", "-l", "-c"},
			Stdin:    []byte("one two\nthree\n"),
			ExitCode: 0,
		},
		{
			Name:     "combined_lw",
			Args:     []string{"-lw"},
			Stdin:    []byte("hello world\n"),
			ExitCode: 0,
		},
		{
			Name:     "m_c_precedence",
			Args:     []string{"-m", "-c"},
			Stdin:    []byte("test\n"),
			ExitCode: 0,
		},
		{
			Name:     "empty_stdin",
			Stdin:    []byte(""),
			ExitCode: 0,
		},
		{
			Name:     "stdin_dash",
			Args:     []string{"-"},
			Stdin:    []byte("stdin content\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWcFiles tests file argument handling including multi-file output and
// --total flag modes. (prd005-wc R1.4, R3.1, R3.2, R3.3)
func TestWcFiles(t *testing.T) {
	skipIfMissing(t)

	dir := t.TempDir()
	writeFile(t, dir, "file1.txt", "hello\nworld\n")
	writeFile(t, dir, "file2.txt", "foo bar baz\n")

	tests := []testutils.DiffTest{
		{
			Name:     "single_file",
			Args:     []string{"file1.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "multi_file_total",
			Args:     []string{"file1.txt", "file2.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "total_always_single",
			Args:     []string{"--total=always", "file1.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "total_only",
			Args:     []string{"--total=only", "file1.txt", "file2.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:     "total_never",
			Args:     []string{"--total=never", "file1.txt", "file2.txt"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWcErrors tests error handling for nonexistent files and invalid flags.
// (prd005-wc R6.1, R6.2)
func TestWcErrors(t *testing.T) {
	skipIfMissing(t)

	dir := t.TempDir()
	writeFile(t, dir, "real.txt", "data\n")

	tests := []testutils.DiffTest{
		{
			Name:      "missing_file",
			Args:      []string{"nonexistent.txt", "real.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer},
		},
		{
			Name:      "invalid_flag",
			Args:      []string{"-z"},
			Stdin:     []byte(""),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errPresenceNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWcFiles0From tests --files0-from with a NUL-delimited file list.
// (prd005-wc R4.4)
func TestWcFiles0From(t *testing.T) {
	skipIfMissing(t)

	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "alpha\nbeta\n")
	writeFile(t, dir, "b.txt", "gamma\n")
	writeFile(t, dir, "filelist", "a.txt\x00b.txt\x00")

	tests := []testutils.DiffTest{
		{
			Name:     "files0_from",
			Args:     []string{"--files0-from=filelist"},
			WorkDir:  dir,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
