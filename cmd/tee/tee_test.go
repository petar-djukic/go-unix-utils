// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against gtee (Homebrew GNU coreutils).
// Implements prd017-tee R4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "gtee"

// blankOutput blanks output so only exit codes are compared.
// Used for --help, --version, and error messages where text differs.
var blankOutput testutils.NormalizeFunc = func(b []byte) []byte { return nil }

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.2: Passthrough mode (no files).
		{
			Name:  "passthrough",
			Args:  []string{},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.1: Single file output.
		{
			Name:  "single file",
			Args:  []string{filepath.Join(t.TempDir(), "out.txt")},
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.1: Multiple file output.
		{
			Name:  "multiple files",
			Args:  []string{filepath.Join(t.TempDir(), "a.txt"), filepath.Join(t.TempDir(), "b.txt")},
			Stdin: []byte("line1\nline2\n"),
		},
		// R1.2: Empty stdin.
		{
			Name:  "empty stdin",
			Args:  []string{filepath.Join(t.TempDir(), "empty.txt")},
			Stdin: []byte{},
		},
		// R2.1: Append mode.
		// Note: append mode tested separately below with pre-existing file content.

		// R2.2: Ignore interrupts flag (compilation check only).
		{
			Name:  "ignore interrupts flag",
			Args:  []string{"-i"},
			Stdin: []byte("data\n"),
		},
		// R2.3: Combined flags.
		{
			Name:  "combined ai flags",
			Args:  []string{"-ai", filepath.Join(t.TempDir(), "combined.txt")},
			Stdin: []byte("combined\n"),
		},
		// R1.4: Dash as file argument (additional stdout reference).
		{
			Name:  "dash file argument",
			Args:  []string{"-"},
			Stdin: []byte("dash\n"),
		},
		// --help and --version: blank output, only exit code matters.
		{
			Name:      "help flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		{
			Name:      "version flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		// R3.2, R3.3: Write error on read-only path.
		{
			Name:      "write error readonly",
			Args:      []string{"/dev/null/impossible"},
			Stdin:     []byte("data\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestAppendMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Test append mode by pre-creating a file with content, then running tee -a.
	for _, bin := range []struct {
		name string
		path string
	}{
		{"go", goBin},
		{"ref", refBin},
	} {
		t.Run(bin.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			outFile := filepath.Join(dir, "append.txt")

			// Write initial content.
			if err := os.WriteFile(outFile, []byte("existing\n"), 0o644); err != nil {
				t.Fatalf("writing initial file: %v", err)
			}

			// Run tee -a.
			cmd := exec.Command(bin.path, "-a", outFile)
			cmd.Stdin = strings.NewReader("new\n")
			cmd.Env = append(os.Environ(), "LC_ALL=C")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("running %s: %v", bin.name, err)
			}

			// Verify stdout.
			if string(out) != "new\n" {
				t.Errorf("%s stdout = %q, want %q", bin.name, out, "new\n")
			}

			// Verify file content: existing + new.
			got, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatalf("reading output file: %v", err)
			}
			want := "existing\nnew\n"
			if string(got) != want {
				t.Errorf("%s file content = %q, want %q", bin.name, got, want)
			}
		})
	}
}
