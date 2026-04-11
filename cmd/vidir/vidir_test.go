// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/vidir against the Homebrew moreutils reference binary.
// Implements srd114-vidir acceptance criteria.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// withEditor returns a full environment with EDITOR overridden.
func withEditor(t *testing.T, editor string) []string {
	t.Helper()
	return append(os.Environ(), "EDITOR="+editor)
}

// createFile is a test helper that writes content to a file.
func createFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("creating test file %s: %v", path, err)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("vidir")
	if err != nil {
		t.Skipf("reference binary vidir not in PATH: %v", err)
	}

	// Create a shared directory with test files for non-destructive tests.
	// EDITOR=true means the editor does nothing, so no files are modified.
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "alpha.txt"), "a")
	createFile(t, filepath.Join(dir, "beta.txt"), "b")
	createFile(t, filepath.Join(dir, "gamma.txt"), "c")

	tests := []testutils.DiffTest{
		{
			// R1.1: list specific files with no-op editor
			Name: "noop_editor_file_args",
			Args: []string{
				filepath.Join(dir, "alpha.txt"),
				filepath.Join(dir, "beta.txt"),
			},
			Env: withEditor(t, "true"),
		},
		{
			// R1.1: list directory contents with no-op editor
			Name: "noop_editor_dir_arg",
			Args: []string{dir},
			Env:  withEditor(t, "true"),
		},
		{
			// R1.1: stdin mode reads filenames from piped input
			Name:  "stdin_noop",
			Stdin: []byte("foo.txt\nbar.txt\n"),
			Env:   withEditor(t, "true"),
		},
		{
			// R1.1: empty stdin produces no listing
			Name:  "empty_stdin",
			Stdin: []byte(""),
			Env:   withEditor(t, "true"),
		},
		{
			// R1.1: single file argument
			Name: "single_file",
			Args: []string{filepath.Join(dir, "alpha.txt")},
			Env:  withEditor(t, "true"),
		},
		{
			// R1.1: all three files
			Name: "three_files",
			Args: []string{
				filepath.Join(dir, "alpha.txt"),
				filepath.Join(dir, "beta.txt"),
				filepath.Join(dir, "gamma.txt"),
			},
			Env: withEditor(t, "true"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
