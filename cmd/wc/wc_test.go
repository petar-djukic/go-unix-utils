// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc.
// Covers prd005-wc R3.3, R4.1-R4.4, R5.1.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not in PATH")
	}

	// Create test fixture files in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "hello.txt", "hello world\n")
	writeTestFile(t, tmpDir, "multi.txt", "one\ntwo\nthree\n")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "binary.bin", "\x00\x01\x02\xff\xfe\n")
	writeTestFile(t, tmpDir, "filelist.txt", "hello.txt\x00multi.txt\x00")

	helloPath := filepath.Join(tmpDir, "hello.txt")
	multiPath := filepath.Join(tmpDir, "multi.txt")
	emptyPath := filepath.Join(tmpDir, "empty.txt")
	binaryPath := filepath.Join(tmpDir, "binary.bin")
	filelistPath := filepath.Join(tmpDir, "filelist.txt")

	// R5.1: all tests run with LC_ALL=C (set by default in RunDiffTests).
	tests := []testutils.DiffTest{
		// R3.3: --total=always prints total even for one file
		{
			Name: "total_always_single_file",
			Args: []string{"--total=always", helloPath},
		},
		// R3.3: --total=never suppresses total for multiple files
		{
			Name: "total_never_multiple_files",
			Args: []string{"--total=never", helloPath, multiPath},
		},
		// R3.3: --total=only prints only the total line
		{
			Name: "total_only_multiple_files",
			Args: []string{"--total=only", helloPath, multiPath},
		},
		// R3.3: --total=auto (default) prints total for multiple files
		{
			Name: "total_auto_multiple_files",
			Args: []string{"--total=auto", helloPath, multiPath},
		},
		// R3.3: --total=auto single file has no total
		{
			Name: "total_auto_single_file",
			Args: []string{"--total=auto", helloPath},
		},
		// R4.1: "-" means stdin
		{
			Name:  "stdin_dash_argument",
			Args:  []string{"-"},
			Stdin: []byte("hello from stdin\n"),
		},
		// R4.2: binary input does not corrupt output
		{
			Name: "binary_input_file",
			Args: []string{binaryPath},
		},
		// R4.2: binary input via stdin
		{
			Name:  "binary_input_stdin",
			Stdin: []byte{0x00, 0x01, 0xFF, 0xFE, '\n'},
		},
		// R4.3: empty input produces zero counts
		{
			Name: "empty_file",
			Args: []string{emptyPath},
		},
		// R4.3: empty stdin
		{
			Name:  "empty_stdin",
			Stdin: []byte{},
		},
		// R4.4: --files0-from reads NUL-delimited filenames from a file
		{
			Name:    "files0_from_file",
			Args:    []string{"--files0-from=" + filelistPath},
			WorkDir: tmpDir,
		},
		// R4.4: --files0-from=- reads filenames from stdin
		{
			Name:    "files0_from_stdin",
			Args:    []string{"--files0-from=-"},
			Stdin:   []byte("hello.txt\x00multi.txt\x00"),
			WorkDir: tmpDir,
		},
		// R5.1: -m with LC_ALL=C matches -c (each byte = one char)
		{
			Name: "chars_lc_all_c",
			Args: []string{"-m", helloPath},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
}
