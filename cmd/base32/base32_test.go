// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base32: srd079 R1.1-R1.4, R2.1-R2.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing our base32 against gbase32.
// Traces: srd079 R1.1 (encoding), R1.2 (default wrap), R1.3 (wrap flag),
// R1.4 (file open error).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: encode stdin input using RFC 4648 Base32.
		{
			Name:  "encode short string from stdin",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "encode empty input",
			Stdin: []byte(""),
		},
		{
			Name:  "encode binary data",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		// R1.2: default wrap at 76 columns.
		{
			Name:  "default wrap at 76 columns",
			Stdin: bytes.Repeat([]byte("A"), 200),
		},
		// R1.3: -w flag controls wrap column.
		{
			Name:  "wrap 0 disables wrapping",
			Args:  []string{"-w", "0"},
			Stdin: bytes.Repeat([]byte("B"), 200),
		},
		{
			Name:  "custom wrap at 40",
			Args:  []string{"-w", "40"},
			Stdin: bytes.Repeat([]byte("C"), 200),
		},
		// R1.4: missing file exits 1.
		{
			Name:      "missing file exits 1",
			Args:      []string{"nonexistent_file_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileInput tests encoding from a file argument.
// Traces: srd079 R1.1 (read from FILE).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputFile, []byte("file content\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name: "encode from file argument",
			Args: []string{inputFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecode runs differential tests for decode mode.
// Traces: srd079 R2.1 (decode mode), R2.2 (whitespace tolerance).
func TestDiffDecode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.1: decode Base32 input back to binary.
		{
			Name:  "decode simple string",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
		},
		{
			Name:  "decode hello",
			Args:  []string{"--decode"},
			Stdin: []byte("NBSWY3DP\n"),
		},
		{
			Name:  "decode empty input",
			Args:  []string{"-d"},
			Stdin: []byte(""),
		},
		// R2.2: ignore whitespace (newlines, spaces) during decoding.
		{
			Name:  "decode with embedded newlines",
			Args:  []string{"-d"},
			Stdin: []byte("NBSW\nY3DP\n"),
		},
		{
			Name:  "decode multiline wrapped input",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecodeIgnoreGarbage runs differential tests for --ignore-garbage.
// Traces: srd079 R2.3 (ignore non-alphabet chars).
func TestDiffDecodeIgnoreGarbage(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.3: -i ignores non-alphabet characters during decode.
		{
			Name:  "ignore garbage short flag",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("NBS!WY@3D#P\n"),
		},
		{
			Name:  "ignore garbage long flag",
			Args:  []string{"--decode", "--ignore-garbage"},
			Stdin: []byte("NBS***WY3DP\n"),
		},
		{
			Name:  "ignore garbage with various special chars",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("N~B`S{W}Y[3]D(P)\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecodeInvalidInput runs differential tests for invalid input errors.
// Traces: srd079 R2.4 (exit 1 on invalid Base32 characters without --ignore-garbage).
func TestDiffDecodeInvalidInput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.4: invalid chars without --ignore-garbage cause exit 1.
		{
			Name:      "invalid chars without ignore-garbage",
			Args:      []string{"-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "garbage at start without ignore-garbage",
			Args:      []string{"-d"},
			Stdin:     []byte("!@#$%\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecodeFromFile tests decoding from a file argument.
// Traces: srd079 R2.1 (decode from FILE).
func TestDiffDecodeFromFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary gbase32 not in PATH")
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "encoded.txt")
	if err := os.WriteFile(inputFile, []byte("NBSWY3DP\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name: "decode from file argument",
			Args: []string{"-d", inputFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normStderr normalizes error output so that gbase32 and our binary
// produce comparable stderr. Handles program name and OS error case.
func normStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gbase32: "), []byte("base32: "))
	data = bytes.ReplaceAll(data, []byte("No such file"), []byte("no such file"))
	return data
}
