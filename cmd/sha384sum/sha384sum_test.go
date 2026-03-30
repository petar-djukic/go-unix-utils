// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/sha384sum implementing prd075-sha384sum R1.1-R1.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests against the gsha384sum reference binary.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha384sum")
	if err != nil {
		t.Skip("reference binary gsha384sum not in PATH")
	}

	dir := t.TempDir()
	singleFile := filepath.Join(dir, "hello.txt")
	writeTestFile(t, singleFile, "hello world\n")

	multiFile1 := filepath.Join(dir, "a.txt")
	writeTestFile(t, multiFile1, "aaa\n")
	multiFile2 := filepath.Join(dir, "b.txt")
	writeTestFile(t, multiFile2, "bbb\n")

	tests := []testutils.DiffTest{
		// R1.1: single file digest in GNU text mode.
		{
			Name:     "single_file",
			Args:     []string{singleFile},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: stdin digest when no files given.
		{
			Name:     "stdin_digest",
			Args:     []string{},
			Stdin:    []byte("abc"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: explicit "-" reads stdin.
		{
			Name:     "dash_reads_stdin",
			Args:     []string{"-"},
			Stdin:    []byte("test input\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.3: multiple files produce one line each.
		{
			Name:     "multiple_files",
			Args:     []string{multiFile1, multiFile2},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R1.2: empty stdin produces the SHA-384 of empty input.
		{
			Name:     "empty_stdin",
			Args:     []string{},
			Stdin:    []byte{},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
