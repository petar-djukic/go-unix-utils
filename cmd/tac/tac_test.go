// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd021-tac R4.1–R4.3
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for tac.
const refBinaryName = "gtac"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create temp files for file-based tests.
	tmpDir := t.TempDir()

	// R1.1: single file with trailing newline.
	fileABC := filepath.Join(tmpDir, "abc.txt")
	if err := os.WriteFile(fileABC, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R1.1: single file without trailing newline.
	fileNoTrail := filepath.Join(tmpDir, "notrail.txt")
	if err := os.WriteFile(fileNoTrail, []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R1.4: second file for multi-file test.
	fileXY := filepath.Join(tmpDir, "xy.txt")
	if err := os.WriteFile(fileXY, []byte("x\ny\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R1.1: empty file.
	fileEmpty := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(fileEmpty, []byte(""), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R1.1: single line with newline.
	fileSingle := filepath.Join(tmpDir, "single.txt")
	if err := os.WriteFile(fileSingle, []byte("only\n"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// R4.3: LC_ALL=C for all tests.
	env := []string{"LC_ALL=C"}

	// Normalize stderr since error messages differ in formatting between
	// Go os.Open errors and gtac's C-style error messages.
	clearStderr := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		// R1.1: reverse lines of a single file with trailing newline.
		{
			Name: "single file with trailing newline",
			Args: []string{fileABC},
			Env:  env,
		},
		// R1.1, R1.2: file without trailing newline.
		{
			Name: "single file without trailing newline",
			Args: []string{fileNoTrail},
			Env:  env,
		},
		// R1.3: read from stdin when no arguments given.
		{
			Name:  "stdin no args",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		// R1.3: stdin via "-" argument.
		{
			Name:  "stdin via dash",
			Args:  []string{"-"},
			Stdin: []byte("x\ny\nz\n"),
			Env:   env,
		},
		// R1.4: multiple files reversed independently.
		{
			Name: "multiple files",
			Args: []string{fileABC, fileXY},
			Env:  env,
		},
		// R1.1: empty file produces no output.
		{
			Name: "empty file",
			Args: []string{fileEmpty},
			Env:  env,
		},
		// R1.1: single line file.
		{
			Name: "single line file",
			Args: []string{fileSingle},
			Env:  env,
		},
		// R1.3: stdin without trailing newline.
		{
			Name:  "stdin no trailing newline",
			Args:  []string{},
			Stdin: []byte("a\nb\nc"),
			Env:   env,
		},
		// R1.4: nonexistent file causes exit 1 but remaining files still process.
		{
			Name:      "nonexistent file with valid file",
			Args:      []string{filepath.Join(tmpDir, "nosuchfile.txt"), fileABC},
			Env:       env,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
