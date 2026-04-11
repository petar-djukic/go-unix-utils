// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/base32: srd079 R1.1-R1.4.
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

// normStderr normalizes error output so that gbase32 and our binary
// produce comparable stderr. Handles program name and OS error case.
func normStderr(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gbase32: "), []byte("base32: "))
	data = bytes.ReplaceAll(data, []byte("No such file"), []byte("no such file"))
	return data
}
