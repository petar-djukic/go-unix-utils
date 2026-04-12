// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base64: srd080 R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests comparing our base64 against gbase64.
// Traces: srd080 R1.1 (encoding), R1.2 (default wrap), R1.3 (wrap flag),
// R1.4 (file open error, exit codes).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: encode stdin input using RFC 4648 Base64.
		{
			Name:  "encode short string from stdin",
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "encode empty input",
			Stdin: []byte(""),
		},
		{
			Name:  "encode single byte",
			Stdin: []byte("A"),
		},
		{
			Name:  "encode binary data",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		{
			Name:  "encode all zero bytes",
			Stdin: []byte{0x00, 0x00, 0x00, 0x00},
		},
		// R1.1: explicit stdin with "-" argument.
		{
			Name:  "encode stdin explicit dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
		},
		// R1.2: default wrap at 76 columns.
		{
			Name:  "default wrap at 76 columns",
			Stdin: bytes.Repeat([]byte("A"), 200),
		},
		{
			Name:  "encode exactly 57 bytes wraps to one line",
			Stdin: bytes.Repeat([]byte("X"), 57),
		},
		{
			Name:  "encode 58 bytes wraps to two lines",
			Stdin: bytes.Repeat([]byte("X"), 58),
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
		{
			Name:  "wrap with long flag equals syntax",
			Args:  []string{"--wrap=20"},
			Stdin: bytes.Repeat([]byte("D"), 100),
		},
		{
			Name:  "wrap at 4 columns",
			Args:  []string{"-w", "4"},
			Stdin: []byte("hello world"),
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

// TestDiffDecode runs differential tests for decode mode.
// Traces: srd080 R2.1 (decode flag), R2.2 (whitespace handling),
// R2.3 (ignore-garbage), R2.4 (invalid input error).
func TestDiffDecode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R2.1: -d decodes Base64 input.
		{
			Name:  "decode simple base64",
			Args:  []string{"-d"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "decode with --decode long flag",
			Args:  []string{"--decode"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "decode empty input",
			Args:  []string{"-d"},
			Stdin: []byte(""),
		},
		{
			Name:  "decode padded input",
			Args:  []string{"-d"},
			Stdin: []byte("YQ==\n"),
		},
		{
			Name:  "decode double padded",
			Args:  []string{"-d"},
			Stdin: []byte("YWI=\n"),
		},
		{
			Name:  "decode no padding needed",
			Args:  []string{"-d"},
			Stdin: []byte("YWJj\n"),
		},
		// R2.2: whitespace in encoded input is handled transparently.
		{
			Name:  "decode multiline base64",
			Args:  []string{"-d"},
			Stdin: []byte("aGVs\nbG8K\n"),
		},
		// R2.3: -i ignores non-alphabet characters.
		{
			Name:  "decode ignore-garbage short flag",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("aGVs!!!bG8K\n"),
		},
		{
			Name:  "decode ignore-garbage long flag",
			Args:  []string{"-d", "--ignore-garbage"},
			Stdin: []byte("aGVs@#$bG8K\n"),
		},
		{
			Name:  "decode ignore-garbage with various special chars",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("a~G`V{s}b[G]8(K)\n"),
		},
		{
			Name:  "decode with spaces and ignore-garbage",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("aGVs bG8K\n"),
		},
		// R2.4: invalid Base64 input produces error and exit 1.
		{
			Name:      "decode invalid input exits 1",
			Args:      []string{"-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "decode garbage only exits 1",
			Args:      []string{"-d"},
			Stdin:     []byte("!@#$%\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecodeFile tests decoding from a file argument.
// Traces: srd080 R2.1 (decode from FILE).
func TestDiffDecodeFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "encoded.txt")
	if err := os.WriteFile(inputFile, []byte("aGVsbG8K\n"), 0o644); err != nil {
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

// TestDiffFileInput tests encoding from a file argument.
// Traces: srd080 R1.1 (read from FILE).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
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

// TestDiffExtraOperand tests that extra file arguments are rejected.
// Traces: srd080 R3.2 (exit 1 on error).
func TestDiffExtraOperand(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.txt")
	file2 := filepath.Join(dir, "b.txt")
	writeTestFile(t, file1, "part one\n")
	writeTestFile(t, file2, "part two\n")

	tests := []testutils.DiffTest{
		{
			Name:      "extra operand rejected",
			Args:      []string{file1, file2},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorPaths runs differential tests for error reporting.
// Traces: srd080 R3.1 (exit 0 on success), R3.2 (exit 1 on error).
func TestDiffErrorPaths(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tests := []testutils.DiffTest{
		// R3.1: --version exits 0.
		{
			Name:      "version flag exits 0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R3.1: --help exits 0.
		{
			Name:      "help flag exits 0",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R3.2: invalid option exits 1.
		{
			Name:      "invalid long flag exits 1",
			Args:      []string{"--nonexistent"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffPermissionError tests file permission error reporting.
// Traces: srd080 R3.2 (exit 1 on error).
func TestDiffPermissionError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	dir := t.TempDir()
	noReadFile := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(noReadFile, []byte("data"), 0o000); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "permission denied on encode",
			Args:      []string{noReadFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "permission denied on decode",
			Args:      []string{"-d", noReadFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFlagCombinations tests various flag combinations.
// Traces: srd080 R1.3 (wrap), R2.1 (decode), R2.3 (ignore-garbage),
// R3.1 (exit 0 on success).
func TestDiffFlagCombinations(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tests := []testutils.DiffTest{
		// Decode with wrap flag (wrap is ignored in decode mode).
		{
			Name:  "decode with wrap flag",
			Args:  []string{"-d", "-w", "40"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		// Decode with ignore-garbage and valid input.
		{
			Name:  "decode ignore-garbage valid input",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		// Decode with all flags combined.
		{
			Name:  "decode all flags combined",
			Args:  []string{"-d", "-i", "-w", "0"},
			Stdin: []byte("aGVs!!!bG8K\n"),
		},
		// Encode with ignore-garbage flag (no-op in encode mode).
		{
			Name:  "encode with ignore-garbage flag",
			Args:  []string{"-i"},
			Stdin: []byte("hello\n"),
		},
		// Encode large input with default wrap.
		{
			Name:  "encode large input default wrap",
			Stdin: bytes.Repeat([]byte("ABCDEFGHIJ"), 100),
		},
		// Wrap at 1 column.
		{
			Name:  "wrap at 1 column",
			Args:  []string{"-w", "1"},
			Stdin: []byte("AB"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// normStderr normalizes error output so that gbase64 and our binary
// produce comparable stderr. Handles program name, OS error case,
// and drops "Try" lines (binary paths differ).
func normStderr(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out []byte
	for _, line := range lines {
		line = bytes.ReplaceAll(line, []byte("gbase64: "), []byte("base64: "))
		line = bytes.ReplaceAll(line, []byte("No such file"), []byte("no such file"))
		line = bytes.ReplaceAll(line, []byte("Permission denied"), []byte("permission denied"))
		// Drop "Try '...' for more information." lines — binary paths differ.
		if bytes.HasPrefix(line, []byte("Try '")) {
			continue
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

// clearOutput clears all output for tests where only exit code matters.
func clearOutput(data []byte) []byte {
	return nil
}
