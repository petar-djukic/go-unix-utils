// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basenc against gbasenc (Homebrew coreutils).
// Covers prd081-basenc R1.1–R1.4, R2.1–R2.2.
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramName normalizes the program name in stderr messages
// so that "gbasenc" and "basenc" compare equal.
var stderrProgramName = regexp.MustCompile(`gbasenc|basenc`)

// normalizeStderr replaces program names in error messages so that the
// Go and reference binaries' stderr output can be compared.
func normalizeStderr(b []byte) []byte {
	return stderrProgramName.ReplaceAll(b, []byte("BASENC"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasenc")
	if err != nil {
		t.Skip("reference binary gbasenc not in PATH")
	}

	tests := []testutils.DiffTest{
		// Base64 encoding (R1.1)
		{
			Name:  "base64_encode_hello",
			Args:  []string{"--base64"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base64_encode_empty",
			Args:  []string{"--base64"},
			Stdin: []byte(""),
		},
		{
			Name:  "base64_encode_binary",
			Args:  []string{"--base64"},
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		// Base64URL encoding (R1.2)
		{
			Name:  "base64url_encode",
			Args:  []string{"--base64url"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base64url_encode_special",
			Args:  []string{"--base64url"},
			Stdin: []byte{0xfb, 0xff, 0xfe},
		},
		// Base32 encoding (R1.3)
		{
			Name:  "base32_encode",
			Args:  []string{"--base32"},
			Stdin: []byte("hello\n"),
		},
		// Base32hex encoding (R1.4)
		{
			Name:  "base32hex_encode",
			Args:  []string{"--base32hex"},
			Stdin: []byte("hello\n"),
		},
		// Base16 encoding (R2.1)
		{
			Name:  "base16_encode",
			Args:  []string{"--base16"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base16_encode_empty",
			Args:  []string{"--base16"},
			Stdin: []byte(""),
		},
		// Z85 encoding (R2.2)
		{
			Name:  "z85_encode_4bytes",
			Args:  []string{"--z85"},
			Stdin: []byte("test"),
		},
		{
			Name:  "z85_encode_8bytes",
			Args:  []string{"--z85"},
			Stdin: []byte("testtest"),
		},
		{
			Name:  "z85_encode_empty",
			Args:  []string{"--z85"},
			Stdin: []byte(""),
		},
		// Wrap control (R2.4)
		{
			Name:  "base64_wrap_0",
			Args:  []string{"--base64", "-w", "0"},
			Stdin: []byte("The quick brown fox jumps over the lazy dog."),
		},
		{
			Name:  "base64_wrap_20",
			Args:  []string{"--base64", "-w", "20"},
			Stdin: []byte("hello world"),
		},
		{
			Name:  "base64_wrap_long_flag",
			Args:  []string{"--base64", "--wrap=10"},
			Stdin: []byte("test"),
		},
		{
			Name:  "base16_wrap_0",
			Args:  []string{"--base16", "--wrap=0"},
			Stdin: []byte("hello"),
		},
		// Decode tests (R2.3)
		{
			Name:  "base64_decode",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "base64_decode_long",
			Args:  []string{"--base64", "--decode"},
			Stdin: []byte("SGVsbG8=\n"),
		},
		{
			Name:  "base64_decode_empty",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte(""),
		},
		{
			Name:  "base64_decode_multiline",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte("aGVsbG8g\nd29ybGQ=\n"),
		},
		{
			Name:  "base16_decode",
			Args:  []string{"--base16", "-d"},
			Stdin: []byte("68656C6C6F\n"),
		},
		{
			Name:  "base32_decode",
			Args:  []string{"--base32", "-d"},
			Stdin: []byte("NBSWY3DP\n"),
		},
		// Ignore garbage (R3.1)
		{
			Name:  "base64_ignore_garbage",
			Args:  []string{"--base64", "-d", "-i"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "base64_ignore_garbage_mixed",
			Args:  []string{"--base64", "-d", "-i"},
			Stdin: []byte("aGVs\n###\nbG8K\n"),
		},
		{
			Name:  "base64_combined_flags_di",
			Args:  []string{"--base64", "-di"},
			Stdin: []byte("SGVsbG8=\n"),
		},
		// Decode error (R3.3)
		{
			Name:      "base64_decode_invalid",
			Args:      []string{"--base64", "-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// Stdin as "-" (R3.2)
		{
			Name:  "stdin_dash",
			Args:  []string{"--base64", "-"},
			Stdin: []byte("dash input\n"),
		},
		// File error (R3.3)
		{
			Name:      "missing_file",
			Args:      []string{"--base64", "/nonexistent/path/to/file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestNoEncoding verifies that running basenc without an encoding flag
// exits 1 with a diagnostic on stderr (R1.3).
func TestNoEncoding(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin)
	cmd.Stdin = strings.NewReader("hello")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for no encoding flag")
	}
	if !strings.Contains(stderr.String(), "missing encoding type") {
		t.Errorf("stderr should mention missing encoding type, got: %q", stderr.String())
	}
}

// TestVersion verifies --version prints version info and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version exited with error: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, "basenc") {
		t.Errorf("--version output missing 'basenc': %q", output)
	}
	if !strings.Contains(output, "go-unix-utils") {
		t.Errorf("--version output missing 'go-unix-utils': %q", output)
	}
}

// TestHelp verifies --help prints usage info and exits 0.
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
	if !strings.Contains(output, "--base64") {
		t.Errorf("--help output missing '--base64': %q", output)
	}
	if !strings.Contains(output, "--decode") {
		t.Errorf("--help output missing '--decode': %q", output)
	}
}
