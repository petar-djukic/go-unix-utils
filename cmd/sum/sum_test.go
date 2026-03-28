// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sum against gsum (GNU coreutils).
// Implements prd078-sum AC1-AC4 via pkg/testutils.RunDiffTests.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName replaces "gsum:" with "sum:" in stderr output
// so that error messages from the reference binary match our binary.
func normalizeProgramName(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gsum:"), []byte("sum:"))
}

// normalizeVersionString reduces version output to a generic marker.
// GNU outputs "sum (GNU coreutils) X.Y" with extra lines; ours outputs
// "sum (go-unix-utils) dev". Both produce non-empty stdout and exit 0.
func normalizeVersionString(b []byte) []byte {
	if len(b) > 0 {
		return []byte("version output present\n")
	}
	return b
}

// normalizeHelpOutput keeps only the first line of help output.
// GNU sum's --help has more detail than ours; we only compare
// that both produce non-empty stdout and exit 0.
func normalizeHelpOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("help output present\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsum")
	if err != nil {
		t.Skip("reference binary gsum not in PATH")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(helloFile, []byte("Hello, world!\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	binaryData := make([]byte, 256)
	for i := range binaryData {
		binaryData[i] = byte(i)
	}
	if err := os.WriteFile(binaryFile, binaryData, 0o644); err != nil {
		t.Fatal(err)
	}
	largeFile := filepath.Join(tmpDir, "large.txt")
	largeData := make([]byte, 2048)
	for i := range largeData {
		largeData[i] = byte('A' + (i % 26))
	}
	if err := os.WriteFile(largeFile, largeData, 0o644); err != nil {
		t.Fatal(err)
	}

	norm := []testutils.NormalizeFunc{normalizeProgramName}
	versionNorm := []testutils.NormalizeFunc{normalizeVersionString}
	helpNorm := []testutils.NormalizeFunc{normalizeHelpOutput}

	tests := []testutils.DiffTest{
		// R1.1: BSD checksum on a file
		{
			Name: "bsd_file",
			Args: []string{helloFile},
		},
		// R1.1: BSD checksum on empty file
		{
			Name: "bsd_empty_file",
			Args: []string{emptyFile},
		},
		// R1.2: BSD checksum from stdin (no args)
		{
			Name:  "bsd_stdin",
			Args:  []string{},
			Stdin: []byte("Hello, world!\n"),
		},
		// R1.4: stdin via explicit dash
		{
			Name:  "bsd_stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("test input\n"),
		},
		// R1.3: multiple files
		{
			Name: "bsd_multiple_files",
			Args: []string{helloFile, emptyFile, binaryFile},
		},
		// R1.4/R3.2: nonexistent file errors, exit 1
		{
			Name:      "bsd_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: norm,
		},
		// R1.4/R3.2: mix of valid and invalid files — continues processing
		{
			Name:      "bsd_mixed_valid_invalid",
			Args:      []string{helloFile, filepath.Join(tmpDir, "nope.txt"), emptyFile},
			ExitCode:  1,
			Normalize: norm,
		},
		// R2.1: explicit -r (BSD) flag
		{
			Name: "bsd_explicit_r",
			Args: []string{"-r", helloFile},
		},
		// R2.2: System V mode
		{
			Name: "sysv_file",
			Args: []string{"-s", helloFile},
		},
		// R2.2: System V on empty file
		{
			Name: "sysv_empty",
			Args: []string{"-s", emptyFile},
		},
		// R2.2: System V from stdin
		{
			Name:  "sysv_stdin",
			Args:  []string{"-s"},
			Stdin: []byte("Hello, world!\n"),
		},
		// R2.2: System V on binary data
		{
			Name: "sysv_binary",
			Args: []string{"-s", binaryFile},
		},
		// R2.2: System V multiple files
		{
			Name: "sysv_multiple",
			Args: []string{"-s", helloFile, binaryFile},
		},
		// BSD on large file spanning multiple blocks
		{
			Name: "bsd_large_file",
			Args: []string{largeFile},
		},
		// System V on large file
		{
			Name: "sysv_large_file",
			Args: []string{"-s", largeFile},
		},
		// BSD binary data
		{
			Name: "bsd_binary",
			Args: []string{binaryFile},
		},
		// R3.2 (task): --version prints version info and exits 0
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: versionNorm,
		},
		// R3.3 (task): --help prints usage info and exits 0
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		// R3.2: error on nonexistent with -s continues
		{
			Name:      "sysv_nonexistent",
			Args:      []string{"-s", filepath.Join(tmpDir, "missing.txt")},
			ExitCode:  1,
			Normalize: norm,
		},
		// R3.2: error on nonexistent with -s, valid file still processed
		{
			Name:      "sysv_mixed_valid_invalid",
			Args:      []string{"-s", filepath.Join(tmpDir, "gone.txt"), helloFile},
			ExitCode:  1,
			Normalize: norm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
