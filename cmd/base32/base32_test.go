// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base32 against gbase32 (Homebrew coreutils).
// Covers prd079-base32 R1.1–R1.4, R2.1–R2.4, R3.1–R3.3.
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramName normalizes the program name prefix in stderr messages
// so that "gbase32:" and "base32:" compare equal.
var stderrProgramName = regexp.MustCompile(`^(gbase32|base32): `)

// normalizeStderr replaces the program name prefix in error messages so
// that the Go and reference binaries' stderr output can be compared.
func normalizeStderr(b []byte) []byte {
	return stderrProgramName.ReplaceAll(b, []byte("BASE32: "))
}


func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	tests := []testutils.DiffTest{
		// Encoding tests (R1.x, R2.1-R2.2)
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
		{
			Name:      "missing_file",
			Args:      []string{"/nonexistent/path/to/file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
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
		// Decode tests (R2.1-R2.4)
		{
			Name:  "decode_hello",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
		},
		{
			Name:  "decode_simple",
			Args:  []string{"--decode"},
			Stdin: []byte("JBSWY3DP\n"),
		},
		{
			Name:  "decode_multiline_input",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DP\nEB3W64TM\nMQ======\n"),
		},
		{
			Name:  "decode_empty_stdin",
			Args:  []string{"-d"},
			Stdin: []byte(""),
		},
		{
			Name:      "decode_invalid_input",
			Args:      []string{"-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		{
			Name:  "decode_ignore_garbage",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("NBSWY3DP\n!!!garbage!!!\nEB3W64TMMQ======\n"),
		},
		{
			Name:  "decode_ignore_garbage_long",
			Args:  []string{"--decode", "--ignore-garbage"},
			Stdin: []byte("JBSWY3DP\n"),
		},
		{
			Name:  "decode_combined_short_flags",
			Args:  []string{"-di"},
			Stdin: []byte("JBSWY3DP\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
