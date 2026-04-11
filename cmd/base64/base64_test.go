// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base64: srd080 R1.1-R1.4.
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
// Traces: srd080 R1.4 (exit 1 on error).
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
// Traces: srd080 R1.4 (exit codes).
func TestDiffErrorPaths(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase64")
	if err != nil {
		t.Skip("reference binary gbase64 not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "version flag exits 0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "help flag exits 0",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffPermissionError tests file permission error reporting.
// Traces: srd080 R1.4 (exit 1 on error).
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
// produce comparable stderr.
func normStderr(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out []byte
	for _, line := range lines {
		line = bytes.ReplaceAll(line, []byte("gbase64: "), []byte("base64: "))
		line = bytes.ReplaceAll(line, []byte("No such file"), []byte("no such file"))
		line = bytes.ReplaceAll(line, []byte("Permission denied"), []byte("permission denied"))
		// Drop "Try '...' for more information." lines — paths differ.
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
