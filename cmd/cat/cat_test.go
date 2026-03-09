// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd006-cat R1.1–R1.5, R2.1–R2.4, R3.1–R3.3 via differential testing
// against gcat (Homebrew GNU cat).
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
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Setup: create test fixture files in a temp directory.
	dir := t.TempDir()
	writeFixture(t, dir, "file1.txt", "hello\nworld\n")
	writeFixture(t, dir, "file2.txt", "aaa\n")
	writeFixture(t, dir, "file3.txt", "bbb\n")
	writeFixture(t, dir, "real.txt", "data\n")
	// R3.2: files for cross-boundary squeeze test.
	writeFixture(t, dir, "endsblank.txt", "x\n\n")
	writeFixture(t, dir, "startsblank.txt", "\ny\n")
	// All 256 byte values for binary passthrough test.
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	writeFixture(t, dir, "binary.dat", string(allBytes))

	tests := []testutils.DiffTest{
		// R1.1, R1.5: text passthrough.
		{
			Name:    "cat_default_passthrough",
			Args:    []string{filepath.Join(dir, "file1.txt")},
			WorkDir: dir,
		},
		// R1.4: binary passthrough.
		{
			Name:    "cat_binary_passthrough",
			Stdin:   allBytes,
			WorkDir: dir,
		},
		// R1.1, R1.3: multiple files concatenated.
		{
			Name:    "cat_multiple_files",
			Args:    []string{filepath.Join(dir, "file2.txt"), filepath.Join(dir, "file3.txt")},
			WorkDir: dir,
		},
		// R1.2: stdin via "-".
		{
			Name:    "cat_stdin_dash",
			Args:    []string{"-"},
			Stdin:   []byte("from stdin\n"),
			WorkDir: dir,
		},
		// R2.1: -n numbers all lines including blanks.
		{
			Name:    "cat_line_numbering_n",
			Args:    []string{"-n"},
			Stdin:   []byte("alpha\n\nbeta\n"),
			WorkDir: dir,
		},
		// R2.2, R2.4: -b numbers non-blank lines only.
		{
			Name:    "cat_line_numbering_b",
			Args:    []string{"-b"},
			Stdin:   []byte("first\n\n\nsecond\n"),
			WorkDir: dir,
		},
		// R2.3: -b takes precedence over -n.
		{
			Name:    "cat_b_overrides_n",
			Args:    []string{"-n", "-b"},
			Stdin:   []byte("a\n\nb\n"),
			WorkDir: dir,
		},
		// R2.4: lines with spaces/tabs are not blank.
		{
			Name:    "cat_spaces_not_blank",
			Args:    []string{"-b"},
			Stdin:   []byte("a\n \n\t\nb\n"),
			WorkDir: dir,
		},
		// R3.1: -s squeezes consecutive blank lines.
		{
			Name:    "cat_squeeze_blanks",
			Args:    []string{"-s"},
			Stdin:   []byte("a\n\n\n\nb\n"),
			WorkDir: dir,
		},
		// R3.1: -s with single blank (no squeeze needed).
		{
			Name:    "cat_squeeze_single_blank",
			Args:    []string{"-s"},
			Stdin:   []byte("a\n\nb\n"),
			WorkDir: dir,
		},
		// R3.1: -s with many consecutive blanks.
		{
			Name:    "cat_squeeze_many_blanks",
			Args:    []string{"-s"},
			Stdin:   []byte("a\n\n\n\n\n\n\nb\n"),
			WorkDir: dir,
		},
		// R3.2: -s across file boundaries.
		{
			Name:    "cat_squeeze_across_files",
			Args:    []string{filepath.Join(dir, "endsblank.txt"), filepath.Join(dir, "startsblank.txt")},
			WorkDir: dir,
		},
		// R3.2: -s across file boundaries with squeeze active.
		{
			Name:    "cat_squeeze_across_files_s",
			Args:    []string{"-s", filepath.Join(dir, "endsblank.txt"), filepath.Join(dir, "startsblank.txt")},
			WorkDir: dir,
		},
		// R3.3: -n -s combined; squeeze before numbering.
		{
			Name:    "cat_combined_ns",
			Args:    []string{"-n", "-s"},
			Stdin:   []byte("a\n\n\n\nb\n"),
			WorkDir: dir,
		},
		// R3.3: -b -s combined; squeeze before numbering, blanks unnumbered.
		{
			Name:    "cat_combined_bs",
			Args:    []string{"-b", "-s"},
			Stdin:   []byte("a\n\n\n\nb\n"),
			WorkDir: dir,
		},
		// R3.1: -s at start of input (leading blanks).
		{
			Name:    "cat_squeeze_leading_blanks",
			Args:    []string{"-s"},
			Stdin:   []byte("\n\n\na\n"),
			WorkDir: dir,
		},
		// R3.1: -s at end of input (trailing blanks).
		{
			Name:    "cat_squeeze_trailing_blanks",
			Args:    []string{"-s"},
			Stdin:   []byte("a\n\n\n\n"),
			WorkDir: dir,
		},
		// R5.2: missing file.
		{
			Name:      "cat_missing_file",
			Args:      []string{filepath.Join(dir, "nonexistent.txt"), filepath.Join(dir, "real.txt")},
			ExitCode:  1,
			WorkDir:   dir,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		// -u accepted without error. R4.8.
		{
			Name:    "cat_flag_u_accepted",
			Args:    []string{"-u"},
			Stdin:   []byte("test\n"),
			WorkDir: dir,
		},
		// Combined -nbs flags.
		{
			Name:    "cat_combined_nbs",
			Args:    []string{"-nbs"},
			Stdin:   []byte("a\n\n\n\nb\n"),
			WorkDir: dir,
		},
		// -s with no blanks (no change).
		{
			Name:    "cat_squeeze_no_blanks",
			Args:    []string{"-s"},
			Stdin:   []byte("a\nb\nc\n"),
			WorkDir: dir,
		},
		// -s with empty input.
		{
			Name:    "cat_squeeze_empty",
			Args:    []string{"-s"},
			Stdin:   []byte{},
			WorkDir: dir,
		},
		// Long flags: --squeeze-blank.
		{
			Name:    "cat_long_squeeze_blank",
			Args:    []string{"--squeeze-blank"},
			Stdin:   []byte("a\n\n\n\nb\n"),
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeBinaryName replaces "gcat:" with "cat:" in output so stderr from
// the reference binary matches our binary's error prefix.
func normalizeBinaryName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gcat:"), []byte("cat:"))
}

// writeFixture creates a file in dir with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFixture %s: %v", name, err)
	}
}
