// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against the GNU reference binary (gcat).
//
// Implements prd006-cat acceptance criteria AC1-AC6 via testutils.RunDiffTests.
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
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Create test fixture files in a shared temp directory.
	tmpDir := t.TempDir()
	writeFixture(t, tmpDir, "file1.txt", "hello\nworld\n")
	writeFixture(t, tmpDir, "file2.txt", "aaa\n")
	writeFixture(t, tmpDir, "file3.txt", "bbb\n")
	writeFixture(t, tmpDir, "real.txt", "data\n")

	// Build all 256 byte values for binary passthrough test.
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}

	tests := []testutils.DiffTest{
		// R1.1, R1.5: Default text passthrough.
		{
			Name:    "cat_default_passthrough",
			Args:    []string{filepath.Join(tmpDir, "file1.txt")},
			WorkDir: tmpDir,
		},
		// R1.4: Binary passthrough.
		{
			Name:  "cat_binary_passthrough",
			Stdin: allBytes,
		},
		// R2.1: -n numbers all lines.
		{
			Name:  "cat_line_numbering_n",
			Args:  []string{"-n"},
			Stdin: []byte("alpha\n\nbeta\n"),
		},
		// R2.2, R2.4: -b numbers non-blank lines only.
		{
			Name:  "cat_line_numbering_b",
			Args:  []string{"-b"},
			Stdin: []byte("first\n\n\nsecond\n"),
		},
		// R2.3: -b overrides -n.
		{
			Name:  "cat_b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("one\n\ntwo\n"),
		},
		// R3.1: -s squeezes consecutive blank lines.
		{
			Name:  "cat_squeeze_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R4.1, R4.2: -v non-printing display.
		{
			Name:  "cat_show_nonprinting",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
		},
		// R4.3: -E shows $ at end of lines.
		{
			Name:  "cat_show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("line one\nline two\n"),
		},
		// R4.4: -T shows tabs as ^I.
		{
			Name:  "cat_show_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("col1\tcol2\tcol3\n"),
		},
		// R4.5: -A = -vET.
		{
			Name:  "cat_show_all",
			Args:  []string{"-A"},
			Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		// R4.6: -e = -vE.
		{
			Name:  "cat_flag_e",
			Args:  []string{"-e"},
			Stdin: []byte{0x01, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		// R4.7: -t = -vT.
		{
			Name:  "cat_flag_t",
			Args:  []string{"-t"},
			Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		// R4.8: -u accepted without error.
		{
			Name:  "cat_flag_u_accepted",
			Args:  []string{"-u"},
			Stdin: []byte("test\n"),
		},
		// R3.3, R4.9: -n -s combined.
		{
			Name:  "cat_combined_ns",
			Args:  []string{"-n", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R1.1, R1.3: Multiple files concatenated.
		{
			Name:    "cat_multiple_files",
			Args:    []string{filepath.Join(tmpDir, "file2.txt"), filepath.Join(tmpDir, "file3.txt")},
			WorkDir: tmpDir,
		},
		// R1.2: "-" reads stdin.
		{
			Name:  "cat_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
		},
		// R5.2: Missing file prints error, exits 1, continues.
		{
			Name:      "cat_missing_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "real.txt")},
			ExitCode:  1,
			WorkDir:   tmpDir,
			Normalize: []testutils.NormalizeFunc{stderrBinaryNameNormalizer},
		},
		// Combined flags: -b -s.
		{
			Name:  "cat_combined_bs",
			Args:  []string{"-b", "-s"},
			Stdin: []byte("x\n\n\n\ny\n"),
		},
		// -v on high bytes range.
		{
			Name:  "cat_nonprinting_high_bytes",
			Args:  []string{"-v"},
			Stdin: []byte{0x80, 0x9f, 0xa0, 0xfe, 0xff},
		},
		// Empty input.
		{
			Name:  "cat_empty_input",
			Stdin: []byte{},
		},
		// Line not ending in newline.
		{
			Name:  "cat_no_trailing_newline",
			Args:  []string{"-n"},
			Stdin: []byte("no newline"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrBinaryNameNormalizer replaces the binary name prefix ("cat:" or "gcat:")
// with a generic prefix so stderr messages from both binaries compare equal.
var stderrBinaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gcat:"), []byte("cat:"))
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
