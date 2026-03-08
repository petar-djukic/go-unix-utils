// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/head against the GNU reference binary (ghead).
//
// Implements prd018-head acceptance criteria AC1-AC6 via testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skipf("reference binary ghead not in PATH: %v", err)
	}

	// Create test fixture files in a shared temp directory.
	tmpDir := t.TempDir()
	writeFixture(t, tmpDir, "file1.txt", "1\n2\n")
	writeFixture(t, tmpDir, "file2.txt", "3\n4\n")
	writeFixture(t, tmpDir, "file.txt", "1\n2\n3\n")
	writeFixture(t, tmpDir, "real.txt", "data\n")
	writeFixture(t, tmpDir, "data.txt", "data\n")

	// Build input for default 10-line test.
	lines12 := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"
	lines10 := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"

	// Build 2048 bytes of 'a' for suffix test.
	bigInput := strings.Repeat("a", 2048)

	tests := []testutils.DiffTest{
		// R1.1: Default 10 lines.
		{
			Name:  "head_default_10_lines",
			Stdin: []byte(lines12),
		},
		// R1.2: Explicit -n 5.
		{
			Name:  "head_n_5",
			Args:  []string{"-n", "5"},
			Stdin: []byte(lines10),
		},
		// R1.3: Negative line count -n -5.
		{
			Name:  "head_n_negative_5",
			Args:  []string{"-n", "-5"},
			Stdin: []byte(lines10),
		},
		// R2.1: Byte count -c 5.
		{
			Name:  "head_c_5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		// R2.2: Negative byte count -c -100 on short input.
		{
			Name:  "head_c_negative_100",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short\n"),
		},
		// R2.2: Negative byte count -c -3.
		{
			Name:  "head_c_negative_3",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefgh"),
		},
		// R1.4: Stdin with dash.
		{
			Name:  "head_stdin_dash",
			Args:  []string{"-n", "2", "-"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R3.1: Multi-file headers.
		{
			Name:    "head_multi_file_headers",
			Args:    []string{filepath.Join(tmpDir, "file1.txt"), filepath.Join(tmpDir, "file2.txt")},
			WorkDir: tmpDir,
		},
		// R3.2: Single file no header.
		{
			Name:    "head_single_file_no_header",
			Args:    []string{filepath.Join(tmpDir, "file.txt")},
			WorkDir: tmpDir,
		},
		// R3.3: Quiet mode suppresses headers.
		{
			Name:    "head_quiet_multi_file",
			Args:    []string{"-q", filepath.Join(tmpDir, "file1.txt"), filepath.Join(tmpDir, "file2.txt")},
			WorkDir: tmpDir,
		},
		// R3.4: Verbose mode forces header on single file.
		{
			Name:    "head_verbose_single_file",
			Args:    []string{"-v", filepath.Join(tmpDir, "data.txt")},
			WorkDir: tmpDir,
		},
		// R3.5, R4.2: Non-existent file error.
		{
			Name:      "head_missing_file",
			Args:      []string{filepath.Join(tmpDir, "missing.txt"), filepath.Join(tmpDir, "real.txt")},
			ExitCode:  1,
			WorkDir:   tmpDir,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// R1.2: Fewer lines than requested.
		{
			Name:  "head_fewer_lines_than_n",
			Args:  []string{"-n", "100"},
			Stdin: []byte("a\nb\n"),
		},
		// R1.1: Empty stdin.
		{
			Name:  "head_empty_stdin",
			Stdin: []byte{},
		},
		// R1.5: No trailing newline.
		{
			Name:  "head_no_trailing_newline",
			Args:  []string{"-n", "2"},
			Stdin: []byte("a\nb"),
		},
		// R2.3: Byte count with K suffix.
		{
			Name:  "head_c_1K",
			Args:  []string{"-c", "1K"},
			Stdin: []byte(bigInput),
		},
		// Combined: -n with --lines= last wins.
		{
			Name:  "head_n_3",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc\nd\ne\n"),
		},
		// -c 0 outputs nothing.
		{
			Name:  "head_c_0",
			Args:  []string{"-c", "0"},
			Stdin: []byte("hello\n"),
		},
		// -n 0 outputs nothing.
		{
			Name:  "head_n_0",
			Args:  []string{"-n", "0"},
			Stdin: []byte("hello\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrBinaryNameNormalizer replaces the binary name prefix in stderr so
// messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("ghead:"), []byte("head:"))
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
