// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gjoin:" with "join:" so stderr messages
// from the reference binary match our binary's program name.
func normalizeProgramName(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gjoin:"), []byte("join:"))
}

// normalizeFileError normalizes file-open error messages across platforms.
func normalizeFileError(b []byte) []byte {
	return bytes.ToLower(b)
}

// writeTestFiles creates file1.txt and file2.txt in a temp directory.
func writeTestFiles(t *testing.T, content1, content2 string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(content1), 0o644); err != nil {
		t.Fatalf("writing file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(content2), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}
	return dir
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gjoin")
	if err != nil {
		t.Skipf("reference binary gjoin not in PATH: %v", err)
	}

	// Setup test file pairs for different scenarios.
	dirAllMatch := writeTestFiles(t, "a 1\nb 2\nc 3\n", "a X\nb Y\nc Z\n")
	dirPartial := writeTestFiles(t, "a 1\nb 2\nc 3\n", "b Y\nc Z\nd W\n")
	dirNoMatch := writeTestFiles(t, "a 1\nc 3\n", "b 2\nd 4\n")
	dirEmpty1 := writeTestFiles(t, "", "a X\nb Y\n")
	dirEmpty2 := writeTestFiles(t, "a 1\nb 2\n", "")
	dirBothEmpty := writeTestFiles(t, "", "")
	dirMultiField := writeTestFiles(t, "a 1 2\nb 3 4\n", "a X Y\nb Z W\n")
	dirDupKeys := writeTestFiles(t, "a 1\na 2\n", "a X\na Y\n")
	dirSingleField := writeTestFiles(t, "a\nb\nc\n", "a\nc\n")

	errNorm := []testutils.NormalizeFunc{normalizeProgramName, normalizeFileError}

	tests := []testutils.DiffTest{
		// R1.1: Join lines where first field matches, one output line per pair.
		{
			Name:    "R1.1_default_join_all_match",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},
		{
			Name:    "R1.1_partial_match",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirPartial,
		},
		{
			Name:    "R1.1_no_matching_keys",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirNoMatch,
		},
		{
			Name:    "R1.1_duplicate_keys_cartesian",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirDupKeys,
		},
		{
			Name:    "R1.1_single_field_lines",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirSingleField,
		},

		// R1.2: Whitespace field separator, space output separator.
		{
			Name:    "R1.2_multiple_fields_space_output",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirMultiField,
		},

		// R1.3: Unpairable lines suppressed by default.
		{
			Name:    "R1.3_file1_empty_all_suppressed",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirEmpty1,
		},
		{
			Name:    "R1.3_file2_empty_all_suppressed",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirEmpty2,
		},
		{
			Name:    "R1.3_both_empty",
			Args:    []string{"file1.txt", "file2.txt"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirBothEmpty,
		},

		// R1.4: stdin as '-' for one of the file arguments.
		{
			Name:    "R1.4_stdin_as_file1",
			Args:    []string{"-", "file2.txt"},
			Stdin:   []byte("a 1\nb 2\nc 3\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},
		{
			Name:    "R1.4_stdin_as_file2",
			Args:    []string{"file1.txt", "-"},
			Stdin:   []byte("a X\nb Y\nc Z\n"),
			Env:     []string{"LC_ALL=C"},
			WorkDir: dirAllMatch,
		},

		// Error: missing file exits 1.
		{
			Name:      "error_missing_file",
			Args:      []string{"nonexistent.txt", "file2.txt"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   dirAllMatch,
			ExitCode:  1,
			Normalize: errNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
