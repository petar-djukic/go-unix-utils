// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against the GNU reference binary gcat.
//
// Implements prd006-cat R1, R2, R3, R4, R5 via differential testing
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

// goBinaryPath is the path to the Go cat binary built in TestMain.
// refBinaryPath is the path to the GNU reference binary (gcat).
var (
	goBinaryPath  string
	refBinaryPath string
)

func TestMain(m *testing.M) {
	// Locate GNU reference binary gcat (Homebrew coreutils).
	refPath, err := exec.LookPath("gcat")
	if err != nil {
		fmt.Println("gcat not found on PATH; skipping cat differential tests")
		os.Exit(0)
	}
	refBinaryPath = refPath

	// Build the Go cat binary from the current package.
	tmpDir, err := os.MkdirTemp("", "cat-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinaryPath = filepath.Join(tmpDir, "cat")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building cat: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// R1: Default behavior and stdin processing (prd006-cat R1)
// ---------------------------------------------------------------------------

func TestCat_DefaultAndStdin(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	os.WriteFile(file1, []byte("hello\nworld\n"), 0o644)
	os.WriteFile(file2, []byte("foo bar\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name:  "default_stdin_no_args",
			Args:  nil,
			Stdin: "hello\nworld\n",
		},
		{
			Name:  "empty_stdin",
			Args:  nil,
			Stdin: "",
		},
		{
			Name:  "stdin_dash_explicit",
			Args:  []string{"-"},
			Stdin: "from stdin\n",
		},
		{
			Name: "single_named_file",
			Args: []string{file1},
		},
		{
			Name: "multiple_files_concatenated",
			Args: []string{file1, file2},
		},
		{
			Name:  "stdin_no_trailing_newline",
			Args:  nil,
			Stdin: "no newline at end",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R1: Binary passthrough (prd006-cat R1.4)
// ---------------------------------------------------------------------------

func TestCat_BinaryPassthrough(t *testing.T) {
	// Build a 256-byte input with all byte values 0x00-0xFF.
	var allBytes [256]byte
	for i := range allBytes {
		allBytes[i] = byte(i)
	}

	tests := []testutils.DiffTest{
		{
			Name:  "binary_all_bytes",
			Args:  nil,
			Stdin: string(allBytes[:]),
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R2: Line numbering flags (prd006-cat R2)
// ---------------------------------------------------------------------------

func TestCat_LineNumbering(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "flag_n_number_all",
			Args:  []string{"-n"},
			Stdin: "alpha\n\nbeta\n",
		},
		{
			Name:  "flag_b_number_nonblank",
			Args:  []string{"-b"},
			Stdin: "first\n\n\nsecond\n",
		},
		{
			Name:  "b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: "line1\n\nline2\n",
		},
		{
			Name:  "b_overrides_n_reversed_order",
			Args:  []string{"-b", "-n"},
			Stdin: "line1\n\nline2\n",
		},
		{
			Name:  "long_flag_number",
			Args:  []string{"--number"},
			Stdin: "a\nb\nc\n",
		},
		{
			Name:  "long_flag_number_nonblank",
			Args:  []string{"--number-nonblank"},
			Stdin: "x\n\ny\n",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R3: Blank-line squeezing (prd006-cat R3)
// ---------------------------------------------------------------------------

func TestCat_SqueezeBlank(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "squeeze_consecutive_blanks",
			Args:  []string{"-s"},
			Stdin: "a\n\n\n\nb\n",
		},
		{
			Name:  "squeeze_no_consecutive",
			Args:  []string{"-s"},
			Stdin: "a\n\nb\n\nc\n",
		},
		{
			Name:  "squeeze_with_n",
			Args:  []string{"-n", "-s"},
			Stdin: "a\n\n\n\nb\n",
		},
		{
			Name:  "squeeze_with_b",
			Args:  []string{"-b", "-s"},
			Stdin: "a\n\n\n\nb\n",
		},
		{
			Name:  "long_flag_squeeze_blank",
			Args:  []string{"--squeeze-blank"},
			Stdin: "x\n\n\ny\n",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R3: Cross-file state continuity (prd006-cat R3.2, R2.1 cross-file)
// ---------------------------------------------------------------------------

func TestCat_CrossFileState(t *testing.T) {
	tmpDir := t.TempDir()
	fileA := filepath.Join(tmpDir, "a.txt")
	fileB := filepath.Join(tmpDir, "b.txt")
	fileC := filepath.Join(tmpDir, "c.txt")

	// Line numbering continues across files.
	os.WriteFile(fileA, []byte("line1\nline2\n"), 0o644)
	os.WriteFile(fileB, []byte("line3\nline4\n"), 0o644)

	// Blank squeezing across file boundaries:
	// fileA ends with blank lines, fileC starts with blank lines.
	fileBlankEnd := filepath.Join(tmpDir, "blank_end.txt")
	fileBlankStart := filepath.Join(tmpDir, "blank_start.txt")
	os.WriteFile(fileBlankEnd, []byte("content\n\n"), 0o644)
	os.WriteFile(fileBlankStart, []byte("\n\nmore\n"), 0o644)
	os.WriteFile(fileC, []byte("data\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "cross_file_line_numbering",
			Args: []string{"-n", fileA, fileB},
		},
		{
			Name: "cross_file_squeeze_blank",
			Args: []string{"-s", fileBlankEnd, fileBlankStart},
		},
		{
			Name: "cross_file_number_nonblank",
			Args: []string{"-b", fileA, fileB},
		},
		{
			Name: "cross_file_squeeze_and_number",
			Args: []string{"-n", "-s", fileBlankEnd, fileBlankStart, fileC},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R4: Non-printing display flags (prd006-cat R4)
// ---------------------------------------------------------------------------

func TestCat_NonPrintingDisplay(t *testing.T) {
	// Build inputs with specific non-printing bytes.
	nonPrintInput := "\x01\x09\x1b\x7f\x80\xff"
	tabInput := "col1\tcol2\tcol3\n"
	mixedInput := "\x01\thello\n"

	tests := []testutils.DiffTest{
		{
			Name:  "flag_v_nonprinting",
			Args:  []string{"-v"},
			Stdin: nonPrintInput,
		},
		{
			Name:  "flag_E_show_ends",
			Args:  []string{"-E"},
			Stdin: "line one\nline two\n",
		},
		{
			Name:  "flag_T_show_tabs",
			Args:  []string{"-T"},
			Stdin: tabInput,
		},
		{
			Name:  "flag_A_show_all",
			Args:  []string{"-A"},
			Stdin: mixedInput,
		},
		{
			Name:  "flag_e_vE",
			Args:  []string{"-e"},
			Stdin: "\x01hello\n",
		},
		{
			Name:  "flag_t_vT",
			Args:  []string{"-t"},
			Stdin: mixedInput,
		},
		{
			Name:  "flag_u_accepted",
			Args:  []string{"-u"},
			Stdin: "test\n",
		},
		{
			Name:  "long_flag_show_ends",
			Args:  []string{"--show-ends"},
			Stdin: "hello\nworld\n",
		},
		{
			Name:  "long_flag_show_tabs",
			Args:  []string{"--show-tabs"},
			Stdin: tabInput,
		},
		{
			Name:  "long_flag_show_nonprinting",
			Args:  []string{"--show-nonprinting"},
			Stdin: "\x01\x7f\n",
		},
		{
			Name:  "long_flag_show_all",
			Args:  []string{"--show-all"},
			Stdin: mixedInput,
		},
		{
			Name:  "v_high_bytes_range",
			Args:  []string{"-v"},
			Stdin: "\x80\x9f\xa0\xfe\xff",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R4: Combined and interacting flags (prd006-cat R4.9)
// ---------------------------------------------------------------------------

func TestCat_CombinedFlags(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name:  "combined_short_vET",
			Args:  []string{"-vET"},
			Stdin: "\x01\thello\n",
		},
		{
			Name:  "combined_short_nb",
			Args:  []string{"-nb"},
			Stdin: "line1\n\nline2\n",
		},
		{
			Name:  "combined_n_v_E",
			Args:  []string{"-n", "-v", "-E"},
			Stdin: "\x01hello\n\nworld\n",
		},
		{
			Name:  "combined_b_s_v",
			Args:  []string{"-b", "-s", "-v"},
			Stdin: "\x01line\n\n\n\x02line\n",
		},
		{
			Name:  "all_flags_combined",
			Args:  []string{"-n", "-s", "-A"},
			Stdin: "\x01\thello\n\n\n\nworld\n",
		},
		{
			Name:  "squeeze_nonprint_ends_number",
			Args:  []string{"-s", "-v", "-E", "-T", "-n"},
			Stdin: "\x01\ttab\n\n\n\nnext\n",
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R5: Error handling and exit codes (prd006-cat R5)
// ---------------------------------------------------------------------------

func TestCat_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.txt")
	nonexistent := filepath.Join(tmpDir, "nonexistent.txt")

	os.WriteFile(realFile, []byte("data\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "nonexistent_file_with_real",
			Args: []string{nonexistent, realFile},
		},
		{
			Name: "nonexistent_file_only",
			Args: []string{nonexistent},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R5: Flag terminator '--' (prd006-cat R5, flag parsing)
// ---------------------------------------------------------------------------

func TestCat_FlagTerminator(t *testing.T) {
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "real.txt")
	dashFile := filepath.Join(tmpDir, "-n")

	os.WriteFile(realFile, []byte("content\n"), 0o644)
	os.WriteFile(dashFile, []byte("dashfile\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "double_dash_stops_flag_parsing",
			Args: []string{"--", dashFile},
		},
		{
			Name: "double_dash_with_real_file",
			Args: []string{"--", realFile},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}
