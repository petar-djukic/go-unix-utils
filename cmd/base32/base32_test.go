// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base32 against gbase32 (Homebrew coreutils).
//
// Covers prd079-base32: R1.1 (encode stdin/file), R1.2 (default wrap),
// R1.3 (-w COLS wrap control), R1.4 (exit 1 on missing file).
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
