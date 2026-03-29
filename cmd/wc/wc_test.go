// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// wc_test.go implements differential tests for cmd/wc against the GNU
// reference binary (gwc). Covers prd005-wc R1.1–R1.4, R2.1–R2.6,
// R3.1–R3.2, R4.1–R4.4, R5.1–R5.2, R6.1–R6.3.

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
var programNameRe = regexp.MustCompile(`(?m)^(?:g?wc):`)

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

// TestDiffFlagSelection runs differential tests for selective column display.
// R3.3: print only the columns for the flags that were explicitly requested.
func TestDiffFlagSelection(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	tests := buildFlagSelectionTests()
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFlagSelectionFiles runs flag selection tests with file arguments.
func TestDiffFlagSelectionFiles(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	t.Run("lines_only_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "data.txt", "one\ntwo\nthree\n")

		tests := []testutils.DiffTest{{
			Name:    "lines_only_single_file",
			Args:    []string{"-l", "data.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("words_only_multi_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "a.txt", "hello world\n")
		writeFixture(t, dir, "b.txt", "foo bar baz\n")

		tests := []testutils.DiffTest{{
			Name:    "words_only_two_files",
			Args:    []string{"-w", "a.txt", "b.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("bytes_only_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "data.txt", "abcdef\n")

		tests := []testutils.DiffTest{{
			Name:    "bytes_only_single_file",
			Args:    []string{"-c", "data.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("lines_words_combined_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "data.txt", "one two\nthree\n")

		tests := []testutils.DiffTest{{
			Name:    "lines_words_combined_file",
			Args:    []string{"-lw", "data.txt"},
			WorkDir: dir,
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
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

// TestDiffErrorHandling runs differential tests for error paths.
// R4.1: print error to stderr and continue.
// R4.2/R4.3: exit 1 on error, include filename in error message.
// R4.4/D1: total line still appears when some files fail.
func TestDiffErrorHandling(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	t.Run("only_nonexistent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		tests := []testutils.DiffTest{{
			Name:      "single_nonexistent_file",
			Args:      []string{"nosuchfile.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("nonexistent_between_valid", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "first.txt", "aaa\n")
		writeFixture(t, dir, "last.txt", "bbb ccc\n")

		tests := []testutils.DiffTest{{
			Name:      "error_between_valid_files",
			Args:      []string{"first.txt", "missing.txt", "last.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("nonexistent_with_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeFixture(t, dir, "good.txt", "hello\n")

		tests := []testutils.DiffTest{{
			Name:      "error_with_lines_flag",
			Args:      []string{"-l", "bad.txt", "good.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		}}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R4.4/D1: totals line appears even when all files fail.
	t.Run("all_files_nonexistent_total", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		tests := []testutils.DiffTest{{
			Name:      "all_nonexistent_with_total",
			Args:      []string{"no1.txt", "no2.txt"},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
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

// TestDiffCharsFlag runs differential tests for -m (character count).
// R5.1: LC_ALL=C eliminates locale divergence.
// R5.2: under LC_ALL=C, -m and -c produce identical counts.
func TestDiffCharsFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	input := []byte("hello world\ngoodbye\n")

	tests := []testutils.DiffTest{
		{
			Name:  "chars_only_stdin",
			Args:  []string{"-m"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			// R2.3: -m takes precedence over -c when both given.
			Name:  "chars_and_bytes_together",
			Args:  []string{"-m", "-c"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "chars_combined_mc",
			Args:  []string{"-mc"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "chars_with_lines",
			Args:  []string{"-ml"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "chars_with_words",
			Args:  []string{"-mw"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCharsFile runs -m differential tests with file arguments.
func TestDiffCharsFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	dir := t.TempDir()
	writeFixture(t, dir, "data.txt", "hello\nworld\n")

	tests := []testutils.DiffTest{{
		Name:    "chars_single_file",
		Args:    []string{"-m", "data.txt"},
		WorkDir: dir,
		Env:     []string{"LC_ALL=C"},
	}}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMaxLineLength runs differential tests for -L (max line length).
// R2.5: max line length with tab expansion.
func TestDiffMaxLineLength(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:  "max_line_length_simple",
			Args:  []string{"-L"},
			Stdin: []byte("short\na much longer line here\nmed\n"),
		},
		{
			Name:  "max_line_length_with_tab",
			Args:  []string{"-L"},
			Stdin: []byte("a\tb\n"),
		},
		{
			Name:  "max_line_length_combined_lL",
			Args:  []string{"-lL"},
			Stdin: []byte("one\ntwo three four\n"),
		},
		{
			Name:  "max_line_length_empty",
			Args:  []string{"-L"},
			Stdin: []byte(""),
		},
		{
			Name:  "max_line_length_no_trailing_newline",
			Args:  []string{"-L"},
			Stdin: []byte("no newline at end"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMaxLineLengthFile runs -L differential tests with files.
func TestDiffMaxLineLengthFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	dir := t.TempDir()
	writeFixture(t, dir, "a.txt", "short\n")
	writeFixture(t, dir, "b.txt", "a longer line here\n")

	tests := []testutils.DiffTest{{
		Name:    "max_line_length_multi_file",
		Args:    []string{"-L", "a.txt", "b.txt"},
		WorkDir: dir,
	}}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColumnOrdering verifies that flag order does not affect output
// column ordering. R2.6/R5.2: columns always appear in the fixed order
// lines, words, chars, bytes, max-line-length.
func TestDiffColumnOrdering(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	dir := t.TempDir()
	writeFixture(t, dir, "data.txt", "one two\nthree four five\n")

	// AC2: -wl and -lw produce identical output.
	tests := []testutils.DiffTest{
		{
			Name:    "order_wl",
			Args:    []string{"-wl", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_lw",
			Args:    []string{"-lw", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_cw",
			Args:    []string{"-cw", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_wc",
			Args:    []string{"-wc", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_Ll",
			Args:    []string{"-Ll", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_lL",
			Args:    []string{"-lL", "data.txt"},
			WorkDir: dir,
		},
		// AC3: combined short flags like -lwc match gwc byte-for-byte.
		{
			Name:    "order_lwc",
			Args:    []string{"-lwc", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_cwl",
			Args:    []string{"-cwl", "data.txt"},
			WorkDir: dir,
		},
		{
			Name:    "order_wlc",
			Args:    []string{"-wlc", "data.txt"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
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

// buildFlagSelectionTests returns DiffTest cases for selective column display.
// R3.3: print only columns for explicitly requested flags.
// R5.1/R5.2: -m included, tests run under LC_ALL=C by default.
func buildFlagSelectionTests() []testutils.DiffTest {
	input := []byte("hello world\ngoodbye\nfoo bar baz\n")
	return []testutils.DiffTest{
		{
			Name:  "lines_only",
			Args:  []string{"-l"},
			Stdin: input,
		},
		{
			Name:  "words_only",
			Args:  []string{"-w"},
			Stdin: input,
		},
		{
			Name:  "bytes_only",
			Args:  []string{"-c"},
			Stdin: input,
		},
		{
			Name:  "chars_only",
			Args:  []string{"-m"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "max_line_length_only",
			Args:  []string{"-L"},
			Stdin: input,
		},
		{
			Name:  "lines_and_words",
			Args:  []string{"-l", "-w"},
			Stdin: input,
		},
		{
			Name:  "lines_and_bytes",
			Args:  []string{"-l", "-c"},
			Stdin: input,
		},
		{
			Name:  "words_and_bytes",
			Args:  []string{"-w", "-c"},
			Stdin: input,
		},
		{
			Name:  "combined_lw",
			Args:  []string{"-lw"},
			Stdin: input,
		},
		{
			Name:  "combined_lwc",
			Args:  []string{"-lwc"},
			Stdin: input,
		},
		{
			// R5.1/R5.2: -lwm under LC_ALL=C matches gwc.
			Name:  "combined_lwm",
			Args:  []string{"-lwm"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "combined_lL",
			Args:  []string{"-lL"},
			Stdin: input,
		},
		{
			Name:  "long_lines",
			Args:  []string{"--lines"},
			Stdin: input,
		},
		{
			Name:  "long_words",
			Args:  []string{"--words"},
			Stdin: input,
		},
		{
			Name:  "long_bytes",
			Args:  []string{"--bytes"},
			Stdin: input,
		},
		{
			Name:  "long_chars",
			Args:  []string{"--chars"},
			Stdin: input,
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "long_max_line_length",
			Args:  []string{"--max-line-length"},
			Stdin: input,
		},
		{
			Name:  "empty_input_lines_only",
			Args:  []string{"-l"},
			Stdin: []byte(""),
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
