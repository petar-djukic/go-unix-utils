// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base64 against gbase64 (Homebrew coreutils).
//
// Covers prd080-base64: R1.1 (encode stdin/file), R1.2 (default wrap),
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
// so "gbase64: ..." and "base64: ..." compare as equal.
var stderrProgramName = regexp.MustCompile(`^[a-z0-9]+:`)

// normalizeAllStdout replaces all stdout with empty bytes so only exit code
// is compared. Used for --version and --help where output text differs.
var normalizeAllRe = regexp.MustCompile(`(?s).*`)

// normalizeProgramName strips the leading program name from stderr lines.
func normalizeProgramName(data []byte) []byte {
	return stderrProgramName.ReplaceAll(data, []byte("base64:"))
}

// normalizeAllStdout replaces all stdout with empty bytes.
func normalizeAllStdout(data []byte) []byte {
	return normalizeAllRe.ReplaceAll(data, []byte(""))
}

// normalizeAllStderr replaces all stderr with empty bytes.
func normalizeAllStderr(data []byte) []byte {
	return normalizeAllRe.ReplaceAll(data, []byte(""))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("file content\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R1.1: long input that produces multi-line output at default wrap
	longInput := []byte("The quick brown fox jumps over the lazy dog. " +
		"This string is long enough to produce multiple wrapped lines.\n")

	tests := []testutils.DiffTest{
		// R1.1: encode from stdin
		{
			Name:  "encode_stdin",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: encode empty input
		{
			Name:  "encode_empty",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: encode from file
		{
			Name: "encode_from_file",
			Args: []string{inputFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: encode binary data
		{
			Name:  "encode_binary_data",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe},
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: stdin via explicit "-"
		{
			Name:  "encode_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("dash input\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.2: default wrap at 76 columns
		{
			Name:  "encode_default_wrap",
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --wrap=0 disables wrapping
		{
			Name:  "wrap_zero",
			Args:  []string{"-w", "0"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: custom wrap column
		{
			Name:  "wrap_custom_20",
			Args:  []string{"-w", "20"},
			Stdin: []byte("hello world this is a longer string for wrapping\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --wrap=N long form
		{
			Name:  "wrap_long_form",
			Args:  []string{"--wrap=40"},
			Stdin: longInput,
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: missing file exits 1
		{
			Name:      "missing_file",
			Args:      []string{"/nonexistent/file/path"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// --version exits 0
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllStdout},
		},
		// --help exits 0
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllStdout, normalizeAllStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
