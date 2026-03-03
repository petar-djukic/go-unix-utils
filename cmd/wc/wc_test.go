// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc against the GNU reference binary gwc.
//
// Implements prd005-wc R1, R2, R3, R4, R5, R6 via differential testing
// using pkg/testutils.RunDiffTests.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the Go wc binary built in TestMain.
// refBinaryPath is the path to the GNU reference binary (gwc).
var (
	goBinaryPath  string
	refBinaryPath string
)

func TestMain(m *testing.M) {
	// Locate GNU reference binary gwc (Homebrew coreutils).
	refPath, err := exec.LookPath("gwc")
	if err != nil {
		fmt.Println("gwc not found on PATH; skipping wc differential tests")
		os.Exit(0)
	}
	refBinaryPath = refPath

	// Build the Go wc binary from the current package.
	tmpDir, err := os.MkdirTemp("", "wc-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinaryPath = filepath.Join(tmpDir, "wc")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building wc: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// R1: Default behavior and stdin processing (prd005-wc R1)
// ---------------------------------------------------------------------------

func TestWC_DefaultAndStdin(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "default_three_lines",
			Args:  nil,
			Stdin: "foo\nbar baz\nqux\n",
		},
		{
			Name:  "default_stdin_no_trailing_newline",
			Args:  nil,
			Stdin: "hello world",
		},
		{
			Name:  "empty_stdin",
			Args:  nil,
			Stdin: "",
		},
		{
			Name:  "stdin_dash_explicit",
			Args:  []string{"-"},
			Stdin: "stdin content\n",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R1: Named file input (prd005-wc R1.2, R1.3, R1.4)
// ---------------------------------------------------------------------------

func TestWC_NamedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	os.WriteFile(file1, []byte("hello\nworld\n"), 0o644)
	os.WriteFile(file2, []byte("foo bar baz\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "single_named_file",
			Args: []string{file1},
		},
		{
			Name: "multiple_files_with_total",
			Args: []string{file1, file2},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R2, R5: Flag behavior (prd005-wc R2, R5)
// ---------------------------------------------------------------------------

func TestWC_Flags(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "flag_lines_only",
			Args:  []string{"-l"},
			Stdin: "one\ntwo\nthree\n",
		},
		{
			Name:  "flag_words_only",
			Args:  []string{"-w"},
			Stdin: "hello world\ngoodbye  cruel   world\n",
		},
		{
			Name:  "flag_bytes_only",
			Args:  []string{"-c"},
			Stdin: "abc\n",
		},
		{
			Name:  "flag_chars_lc_c",
			Args:  []string{"-m"},
			Stdin: "hello\n",
		},
		{
			Name:  "flag_max_line_length",
			Args:  []string{"-L"},
			Stdin: "short\na much longer line here\nmed\n",
		},
		{
			Name:  "combined_short_flags_lwc",
			Args:  []string{"-lwc"},
			Stdin: "one two\nthree\n",
		},
		{
			Name:  "combined_flags_reordered",
			Args:  []string{"-w", "-l", "-c"},
			Stdin: "one two\nthree\n",
		},
		{
			Name:  "m_precedence_over_c",
			Args:  []string{"-c", "-m"},
			Stdin: "test\n",
		},
		{
			Name:  "long_flag_lines",
			Args:  []string{"--lines"},
			Stdin: "a\nb\nc\n",
		},
		{
			Name:  "long_flag_words",
			Args:  []string{"--words"},
			Stdin: "one two three\n",
		},
		{
			Name:  "long_flag_bytes",
			Args:  []string{"--bytes"},
			Stdin: "abcdef\n",
		},
		{
			Name:  "long_flag_chars",
			Args:  []string{"--chars"},
			Stdin: "abcdef\n",
		},
		{
			Name:  "long_flag_max_line_length",
			Args:  []string{"--max-line-length"},
			Stdin: "hello\nworld!\n",
		},
		{
			Name:  "tab_expansion_for_L",
			Args:  []string{"-L"},
			Stdin: "\txy\n",
		},
		{
			Name:  "flag_L_with_l",
			Args:  []string{"-l", "-L"},
			Stdin: "first line\nsecond\n",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R3, R4, R6: Output formatting, total modes, and edge cases
//             (prd005-wc R3, R4, R6)
// ---------------------------------------------------------------------------

func TestWC_TotalModes(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "a.txt")
	file2 := filepath.Join(tmpDir, "b.txt")

	os.WriteFile(file1, []byte("a\n"), 0o644)
	os.WriteFile(file2, []byte("b\nc\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "total_always_single_file",
			Args: []string{"--total=always", file1},
		},
		{
			Name: "total_always_two_files",
			Args: []string{"--total=always", file1, file2},
		},
		{
			Name: "total_only",
			Args: []string{"--total=only", file1, file2},
		},
		{
			Name: "total_never",
			Args: []string{"--total=never", file1, file2},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestWC_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	noNewline := filepath.Join(tmpDir, "nonewline.txt")
	realFile := filepath.Join(tmpDir, "real.txt")
	nonexistent := filepath.Join(tmpDir, "nonexistent.txt")

	os.WriteFile(emptyFile, []byte{}, 0o644)
	os.WriteFile(noNewline, []byte("no trailing newline"), 0o644)
	os.WriteFile(realFile, []byte("data\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "empty_file",
			Args: []string{emptyFile},
		},
		{
			Name: "file_no_trailing_newline",
			Args: []string{noNewline},
		},
		{
			Name: "nonexistent_file_with_real",
			Args: []string{nonexistent, realFile},
		},
		{
			Name: "column_width_alignment",
			Args: []string{emptyFile, realFile},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}
