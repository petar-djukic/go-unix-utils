// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against the sponge reference binary.
// Implements prd007-sponge AC1-AC5 via pkg/testutils.RunDiffTests and file comparison.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff tests sponge passthrough mode (no output file) via RunDiffTests.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1: No filename -> write to stdout.
		{
			Name:  "sponge_passthrough_stdout",
			Stdin: []byte("hello world\n"),
		},
		// R4.3: Passthrough with empty stdin.
		{
			Name:  "sponge_passthrough_empty",
			Stdin: []byte{},
		},
		// R4.1: Passthrough with multi-line stdin.
		{
			Name:  "sponge_passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
		// R4.1: Passthrough binary data.
		{
			Name:  "sponge_passthrough_binary",
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiff_FileOutput tests sponge file output by running both binaries
// in isolated directories and comparing the resulting files.
func TestDiff_FileOutput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	type fileTest struct {
		name    string
		args    []string
		stdin   []byte
		setup   func(t *testing.T, dir string)
		outFile string
	}

	tests := []fileTest{
		// R1.1, R2.1: Small stdin to file.
		{
			name:    "sponge_small_stdin_to_file",
			args:    []string{"outfile.txt"},
			stdin:   []byte("hello\nworld\n"),
			outFile: "outfile.txt",
		},
		// R1.1: Empty stdin to file.
		{
			name:    "sponge_empty_stdin_to_file",
			args:    []string{"empty_out.txt"},
			stdin:   []byte{},
			outFile: "empty_out.txt",
		},
		// R3.1: Append mode with existing file.
		{
			name:  "sponge_append_mode",
			args:  []string{"-a", "existing.txt"},
			stdin: []byte("appended line\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, "existing.txt"),
					[]byte("original line\n"), 0o644); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			outFile: "existing.txt",
		},
		// R3.2: Append mode with non-existent file.
		{
			name:    "sponge_append_no_existing",
			args:    []string{"-a", "newfile.txt"},
			stdin:   []byte("new content\n"),
			outFile: "newfile.txt",
		},
		// R1.1: Soak-before-write contract.
		{
			name:  "sponge_soak_before_write",
			args:  []string{"data.txt"},
			stdin: []byte("line1\nline2\nline3\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, "data.txt"),
					[]byte("line1\nline2\nline3\n"), 0o644); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			outFile: "data.txt",
		},
		// R1.3: Large stdin (>1 MB).
		{
			name:    "sponge_large_stdin",
			args:    []string{"large_out.txt"},
			stdin:   generateLargeInput(),
			outFile: "large_out.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			goDir := t.TempDir()
			refDir := t.TempDir()

			if tc.setup != nil {
				tc.setup(t, goDir)
				tc.setup(t, refDir)
			}

			goExit := runBinaryInDir(t, goBin, goDir, tc.args, tc.stdin)
			refExit := runBinaryInDir(t, refBin, refDir, tc.args, tc.stdin)

			if goExit != refExit {
				t.Errorf("exit code: go=%d ref=%d", goExit, refExit)
			}

			goContent, goErr := os.ReadFile(filepath.Join(goDir, tc.outFile))
			refContent, refErr := os.ReadFile(filepath.Join(refDir, tc.outFile))

			if (goErr == nil) != (refErr == nil) {
				t.Fatalf("file existence differs: go err=%v ref err=%v", goErr, refErr)
			}

			if !bytes.Equal(goContent, refContent) {
				t.Errorf("file content differs:\n  go:  %q\n  ref: %q",
					truncate(goContent, 200), truncate(refContent, 200))
			}
		})
	}
}

// TestBuild verifies that the binary compiles without errors (AC1).
func TestBuild(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build failed: %v", err)
	}
}

// runBinaryInDir executes a binary in the given directory and returns the exit code.
func runBinaryInDir(t *testing.T, bin, dir string, args []string, stdin []byte) int {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		t.Fatalf("running %s: %v", filepath.Base(bin), err)
	}

	return 0
}

// generateLargeInput produces a payload larger than 1 MB for spill testing.
func generateLargeInput() []byte {
	var b strings.Builder
	line := "abcdefghijklmnopqrstuvwxyz0123456789\n"
	for b.Len() < 1_100_000 {
		b.WriteString(line)
	}
	return []byte(b.String())
}

// truncate returns at most n bytes for display in error messages.
func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
