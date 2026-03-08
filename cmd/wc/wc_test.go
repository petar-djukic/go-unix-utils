// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc against the GNU reference binary (gwc).
//
// Implements prd005-wc acceptance criteria AC1-AC5 via testutils.RunDiffTests.
// All tests run with LC_ALL=C per R5.1 to avoid locale-dependent divergence.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	// Create test fixture files in a shared temp directory.
	tmpDir := t.TempDir()
	writeFixture(t, tmpDir, "file1.txt", "hello\nworld\n")
	writeFixture(t, tmpDir, "file2.txt", "foo bar baz\n")
	writeFixture(t, tmpDir, "real.txt", "data\n")
	writeFixture(t, tmpDir, "empty.txt", "")
	writeFixture(t, tmpDir, "notail.txt", "no newline")
	writeFixture(t, tmpDir, "tabs.txt", "a\tb\n")
	writeFixture(t, tmpDir, "a.txt", "a\n")
	writeFixture(t, tmpDir, "bc.txt", "b\nc\n")
	writeFixture(t, tmpDir, "x.txt", "x\n")
	writeFixture(t, tmpDir, "y.txt", "y\n")

	tests := []testutils.DiffTest{
		// R1.1: Default flags — lines, words, bytes.
		{
			Name:  "wc_default_three_lines",
			Stdin: []byte("foo\nbar baz\nqux\n"),
		},
		// R2.1: -l counts newlines.
		{
			Name:  "wc_lines_only",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo\nthree\n"),
		},
		// R2.2: -w counts words.
		{
			Name:  "wc_words_only",
			Args:  []string{"-w"},
			Stdin: []byte("hello world\ngoodbye  cruel   world\n"),
		},
		// R2.3: -c counts bytes.
		{
			Name:  "wc_bytes_only",
			Args:  []string{"-c"},
			Stdin: []byte("abc\n"),
		},
		// R2.4, R5.1, R5.2: -m with LC_ALL=C; char count equals byte count.
		{
			Name:  "wc_chars_lc_c",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
		},
		// R2.5: -L reports longest line length.
		{
			Name:  "wc_max_line_length",
			Args:  []string{"-L"},
			Stdin: []byte("short\na much longer line here\nmed\n"),
		},
		// R2.6: Combined flags produce counts in fixed order.
		{
			Name:  "wc_combined_flags_order",
			Args:  []string{"-w", "-l", "-c"},
			Stdin: []byte("one two\nthree\n"),
		},
		// R1.4, R3.1, R3.2: Multi-file with total line.
		{
			Name: "wc_multi_file_total",
			Args: []string{filepath.Join(tmpDir, "file1.txt"), filepath.Join(tmpDir, "file2.txt")},
		},
		// R4.1: "-" reads from stdin.
		{
			Name:  "wc_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("stdin content\n"),
		},
		// R4.3: Empty stdin.
		{
			Name:  "wc_empty_stdin",
			Stdin: []byte{},
		},
		// R6.2: Missing file error, still processes other files.
		{
			Name:      "wc_missing_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "real.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// R4.2: Binary input.
		{
			Name:  "wc_binary_input",
			Stdin: []byte{0x00, 0x01, 0xff, 'a', '\n', 0x80},
		},
		// No trailing newline.
		{
			Name:  "wc_no_trailing_newline",
			Stdin: []byte("no newline"),
		},
		// Single file — no total line.
		{
			Name: "wc_single_file",
			Args: []string{filepath.Join(tmpDir, "file1.txt")},
		},
		// -l single file.
		{
			Name: "wc_lines_file",
			Args: []string{"-l", filepath.Join(tmpDir, "file1.txt")},
		},
		// -w single file.
		{
			Name: "wc_words_file",
			Args: []string{"-w", filepath.Join(tmpDir, "file2.txt")},
		},
		// -c single file.
		{
			Name: "wc_bytes_file",
			Args: []string{"-c", filepath.Join(tmpDir, "file1.txt")},
		},
		// -L with tabs — tab expansion.
		{
			Name: "wc_max_line_tabs",
			Args: []string{"-L", filepath.Join(tmpDir, "tabs.txt")},
		},
		// All flags combined.
		{
			Name:  "wc_all_flags",
			Args:  []string{"-l", "-w", "-c", "-L"},
			Stdin: []byte("hello world\nbye\n"),
		},
		// Empty file.
		{
			Name: "wc_empty_file",
			Args: []string{filepath.Join(tmpDir, "empty.txt")},
		},
		// File with no trailing newline.
		{
			Name: "wc_file_no_trailing_newline",
			Args: []string{filepath.Join(tmpDir, "notail.txt")},
		},
		// R3.3: --total=only.
		{
			Name: "wc_total_only",
			Args: []string{"--total=only", filepath.Join(tmpDir, "a.txt"), filepath.Join(tmpDir, "bc.txt")},
		},
		// R3.3: --total=never.
		{
			Name: "wc_total_never",
			Args: []string{"--total=never", filepath.Join(tmpDir, "x.txt"), filepath.Join(tmpDir, "y.txt")},
		},
		// R3.3: --total=always with single file.
		{
			Name: "wc_total_always",
			Args: []string{"--total=always", filepath.Join(tmpDir, "file1.txt")},
		},
		// -m and -c together: -m takes precedence.
		{
			Name:  "wc_m_overrides_c",
			Args:  []string{"-m", "-c"},
			Stdin: []byte("test\n"),
		},
		// -l -L combined.
		{
			Name:  "wc_lines_and_maxlen",
			Args:  []string{"-l", "-L"},
			Stdin: []byte("short\nlong line here\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrBinaryNameNormalizer replaces the binary name prefix in stderr so
// messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gwc:"), []byte("wc:"))
	return b
}

// writeFixture creates a test file with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("failed to write fixture %s: %v", name, err)
	}
}
