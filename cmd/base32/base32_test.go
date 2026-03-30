// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base32 against gbase32 (Homebrew coreutils).
//
// Covers prd079-base32: R1.1 (encode stdin/file), R1.2 (default wrap),
// R1.3 (-w COLS wrap control), R1.4 (exit 1 on missing file),
// R2.1 (-d decode), R2.2 (whitespace ignored), R2.3 (--ignore-garbage),
// R2.4 (exit 1 on invalid decode input).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramName normalizes the program name prefix in error messages
// so "gbase32: ..." and "base32: ..." compare as equal.
var stderrProgramName = regexp.MustCompile(`^[a-z0-9]+:`)

// normalizeProgramName strips the leading program name from stderr lines.
func normalizeProgramName(data []byte) []byte {
	return stderrProgramName.ReplaceAll(data, []byte("base32:"))
}

// decodeErrDetail trims trailing detail after "invalid input" so
// "invalid input: illegal base32 data at input byte 0" becomes "invalid input".
var decodeErrDetail = regexp.MustCompile(`invalid input[^\n]*`)

// normalizeDecodeErrDetail strips Go-specific error detail from decode errors.
func normalizeDecodeErrDetail(data []byte) []byte {
	return decodeErrDetail.ReplaceAll(data, []byte("invalid input"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("file content\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:  "encode_stdin",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "encode_empty",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "decode_basic",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "wrap_zero",
			Args:  []string{"-w", "0"},
			Stdin: []byte("hello world this is a longer string\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "wrap_custom",
			Args:  []string{"-w", "20"},
			Stdin: []byte("hello world this is a longer string for wrapping\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "decode_long_flag",
			Args:  []string{"--decode"},
			Stdin: []byte("NBSWY3DP\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "encode_binary_data",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe},
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:      "missing_file",
			Args:      []string{"/nonexistent/file/path"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name: "encode_from_file",
			Args: []string{inputFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: decode with -d flag
		{
			Name:  "decode_short_flag",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: whitespace in encoded input silently ignored during decode
		{
			Name:  "decode_multiline_whitespace",
			Args:  []string{"--decode"},
			Stdin: []byte("NBSWY3DP\nEB3W64TM\nMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2 + R2.3: spaces/tabs within a line are non-alphabet; use -i to skip them
		{
			Name:  "decode_spaces_tabs_with_ignore",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("NBSWY3DP \t EB3W64TMMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: --ignore-garbage skips non-alphabet characters
		{
			Name:  "decode_ignore_garbage_short",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("NBSWY3DP!!@@##\n"),
			Env:   []string{"LC_ALL=C"},
		},
		{
			Name:  "decode_ignore_garbage_long",
			Args:  []string{"--decode", "--ignore-garbage"},
			Stdin: []byte("NBSWY3DP$$%%^^\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: invalid Base32 input without --ignore-garbage exits 1
		{
			Name:      "decode_invalid_input",
			Args:      []string{"-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeDecodeErrDetail},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
