// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// wc_test.go implements differential tests for cmd/wc against the GNU
// reference binary (gwc). Covers prd005-wc R1.1–R1.4, R2.1–R2.4,
// R3.1, R3.2, R4.4.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// programNameRe matches the program name prefix in error messages
// (e.g., "gwc:" or "wc:") so both binaries produce identical stderr.
var programNameRe = regexp.MustCompile(`^(?:g?wc):`)

// tryHelpRe matches the full Try '...' for more information line, which
// may contain the full binary path (e.g., '/opt/homebrew/bin/gwc --help').
var tryHelpRe = regexp.MustCompile(`Try '.*' for more information\.`)

// noSuchFileRe normalizes case differences in "no such file or directory".
var noSuchFileRe = regexp.MustCompile(`(?i)no such file or directory`)

// normalizeStderr replaces the program name prefix and normalizes error
// message casing so Go and GNU error messages match.
func normalizeStderr(data []byte) []byte {
	data = programNameRe.ReplaceAll(data, []byte("wc:"))
	data = tryHelpRe.ReplaceAll(data,
		[]byte("Try 'wc --help' for more information."))
	return noSuchFileRe.ReplaceAll(data, []byte("no such file or directory"))
}

// TestDiff runs differential tests comparing cmd/wc against gwc.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	stdinTests := buildStdinTests()
	testutils.RunDiffTests(t, goBin, refBin, stdinTests)
}

// TestDiffMultiFile runs differential tests that require fixture files.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	t.Run("wc_multi_file_total", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "file1.txt", "hello\nworld\n")
		writeFixture(t, dir, "file2.txt", "foo bar baz\n")

		tests := []testutils.DiffTest{{
			Name:    "two_files_with_total",
			Args:    []string{"file1.txt", "file2.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("wc_missing_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "real.txt", "data\n")

		tests := []testutils.DiffTest{{
			Name:      "nonexistent_and_real",
			Args:      []string{"nonexistent.txt", "real.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("wc_single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "single.txt", "one two three\nfour\n")

		tests := []testutils.DiffTest{{
			Name:    "single_file_no_total",
			Args:    []string{"single.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("wc_three_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "a.txt", "a\n")
		writeFixture(t, dir, "b.txt", "bb\ncc\n")
		writeFixture(t, dir, "c.txt", "ddd eee fff\n")

		tests := []testutils.DiffTest{{
			Name:    "three_files_with_total",
			Args:    []string{"a.txt", "b.txt", "c.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffFiles0From runs differential tests for --files0-from.
func TestDiffFiles0From(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	t.Run("files0_from_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "a.txt", "hello\nworld\n")
		writeFixture(t, dir, "b.txt", "foo bar\n")
		writeFixture(t, dir, "manifest",
			"a.txt\x00b.txt\x00")

		tests := []testutils.DiffTest{{
			Name:    "files0_from_file",
			Args:    []string{"--files0-from=manifest"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("files0_from_stdin", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "x.txt", "one two three\n")

		tests := []testutils.DiffTest{{
			Name:    "files0_from_stdin",
			Args:    []string{"--files0-from=-"},
			Stdin:   []byte("x.txt\x00"),
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("files0_from_extra_operand", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "a.txt", "test\n")
		writeFixture(t, dir, "manifest", "a.txt\x00")

		tests := []testutils.DiffTest{{
			Name:      "files0_from_extra_operand",
			Args:      []string{"--files0-from=manifest", "a.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("files0_from_multiple_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "p.txt", "alpha\nbeta\n")
		writeFixture(t, dir, "q.txt", "gamma\n")
		writeFixture(t, dir, "r.txt", "delta epsilon\n")
		writeFixture(t, dir, "list",
			"p.txt\x00q.txt\x00r.txt\x00")

		tests := []testutils.DiffTest{{
			Name:    "files0_from_three_files",
			Args:    []string{"--files0-from=list"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// buildStdinTests returns DiffTest cases that use stdin input.
func buildStdinTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			// R1.1, R1.2, R1.3: default counts from stdin.
			Name:  "wc_default_three_lines",
			Args:  []string{},
			Stdin: []byte("foo\nbar baz\nqux\n"),
		},
		{
			// R2.4/R3.2: no file args reads stdin, no filename in output.
			Name:  "wc_empty_stdin",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			// R4.1: "-" as file operand means stdin.
			Name:  "wc_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("stdin content\n"),
		},
		{
			// R4.1: "-" with other content.
			Name:  "wc_stdin_dash_multiword",
			Args:  []string{"-"},
			Stdin: []byte("hello world\ngoodbye\n"),
		},
		{
			// R4.2: binary input must not corrupt output.
			Name:  "wc_binary_input",
			Args:  []string{},
			Stdin: []byte{0x00, 0xFF, 0x0A, 0x41, 0x20, 0x42, 0x0A},
		},
	}
}

// writeFixture creates a file with the given content in dir.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writeFixture %s: %v", name, err)
	}
}
