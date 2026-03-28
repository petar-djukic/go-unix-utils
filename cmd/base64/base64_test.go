// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base64 against gbase64 (Homebrew coreutils).
// Covers prd080-base64 R1.1–R1.4, R2.1–R2.4, R3.1–R3.3.
package main

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramName normalizes the program name prefix in stderr messages
// so that "gbase64:" and "base64:" compare equal.
var stderrProgramName = regexp.MustCompile(`^(gbase64|base64): `)

// normalizeStderr replaces the program name prefix in error messages so
// that the Go and reference binaries' stderr output can be compared.
func normalizeStderr(b []byte) []byte {
	return stderrProgramName.ReplaceAll(b, []byte("BASE64: "))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tests := []testutils.DiffTest{
		// Encoding tests (R1.1, R1.2, R1.4)
		{
			Name:  "encode_hello_stdin",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "encode_empty_stdin",
			Stdin: []byte(""),
		},
		{
			Name:  "encode_binary_data",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		{
			Name:  "encode_long_input_wraps_at_76",
			Stdin: []byte("The quick brown fox jumps over the lazy dog. This is a longer string for testing wrap behavior at the default 76 column width."),
		},
		// Wrap control (R1.3)
		{
			Name:  "wrap_0_disables_wrapping",
			Args:  []string{"-w", "0"},
			Stdin: []byte("The quick brown fox jumps over the lazy dog. This is a longer string for testing wrap behavior."),
		},
		{
			Name:  "wrap_20",
			Args:  []string{"-w", "20"},
			Stdin: []byte("hello world"),
		},
		{
			Name:  "wrap_long_flag",
			Args:  []string{"--wrap=10"},
			Stdin: []byte("test"),
		},
		{
			Name:  "wrap_short_combined",
			Args:  []string{"-w0"},
			Stdin: []byte("abc"),
		},
		// File error (R1.4, R3.2)
		{
			Name:      "missing_file",
			Args:      []string{"/nonexistent/path/to/file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// Stdin as "-" (R1.1)
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("dash input\n"),
		},
		{
			Name:  "single_byte",
			Stdin: []byte("A"),
		},
		{
			Name:  "newline_only",
			Stdin: []byte("\n"),
		},
		// Decode tests (R2.1, R2.2)
		{
			Name:  "decode_hello",
			Args:  []string{"-d"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "decode_simple",
			Args:  []string{"--decode"},
			Stdin: []byte("SGVsbG8=\n"),
		},
		{
			Name:  "decode_multiline_input",
			Args:  []string{"-d"},
			Stdin: []byte("aGVsbG8g\nd29ybGQ=\n"),
		},
		{
			Name:  "decode_empty_stdin",
			Args:  []string{"-d"},
			Stdin: []byte(""),
		},
		// Decode error handling (R2.4, R3.2)
		{
			Name:      "decode_invalid_input",
			Args:      []string{"-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// Ignore garbage (R2.3)
		{
			Name:  "decode_ignore_garbage",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "decode_ignore_garbage_long",
			Args:  []string{"--decode", "--ignore-garbage"},
			Stdin: []byte("SGVsbG8=\n"),
		},
		{
			Name:  "decode_combined_short_flags",
			Args:  []string{"-di"},
			Stdin: []byte("SGVsbG8=\n"),
		},
		{
			Name:  "decode_ignore_garbage_mixed",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("aGVs\n###\nbG8g\nd29ybGQ=\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies --version prints version info and exits 0 (R3.1).
func TestVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version exited with error: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, "base64") {
		t.Errorf("--version output missing 'base64': %q", output)
	}
	if !strings.Contains(output, "go-unix-utils") {
		t.Errorf("--version output missing 'go-unix-utils': %q", output)
	}
}

// TestHelp verifies --help prints usage info and exits 0 (R3.2).
func TestHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help exited with error: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, "Usage:") {
		t.Errorf("--help output missing 'Usage:': %q", output)
	}
	if !strings.Contains(output, "--decode") {
		t.Errorf("--help output missing '--decode': %q", output)
	}
	if !strings.Contains(output, "--ignore-garbage") {
		t.Errorf("--help output missing '--ignore-garbage': %q", output)
	}
	if !strings.Contains(output, "--wrap") {
		t.Errorf("--help output missing '--wrap': %q", output)
	}
}
