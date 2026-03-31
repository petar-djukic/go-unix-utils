// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// vidir_test.go implements differential tests for prd114-vidir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
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
	refBin, err := exec.LookPath("vidir")
	if err != nil {
		t.Skipf("reference binary vidir not in PATH: %v", err)
	}

	// Create a directory with known files for listing tests.
	fixDir := t.TempDir()
	createFixtureFiles(t, fixDir, []string{"aaa", "bbb", "ccc"})

	tests := []testutils.DiffTest{
		{
			// R1.1: EDITOR=cat outputs the listing unchanged to stdout.
			Name: "list_format_cat_editor",
			Args: []string{fixDir},
			Env:  []string{"LC_ALL=C", "EDITOR=cat"},
		},
		{
			// R1.1: empty directory produces no output.
			Name: "empty_directory",
			Args: []string{t.TempDir()},
			Env:  []string{"LC_ALL=C", "EDITOR=cat"},
		},
		{
			// R1.1: single file in directory.
			Name: "single_file",
			Args: []string{createDirWithFiles(t, []string{"only"})},
			Env:  []string{"LC_ALL=C", "EDITOR=cat"},
		},
		{
			// R2.1: success exits 0.
			Name:     "success_exit_zero",
			Args:     []string{createDirWithFiles(t, []string{"x", "y"})},
			Env:      []string{"LC_ALL=C", "EDITOR=cat"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestEditorFailure verifies R2.1: non-zero editor exit causes vidir to
// exit non-zero. Tested as a unit test because the Go and Perl implementations
// produce different stderr messages and exit codes (Go: 1, Perl: 2).
func TestEditorFailure(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := createDirWithFiles(t, []string{"file1"})

	cmd := exec.Command(goBin, dir)
	cmd.Env = append(os.Environ(), "EDITOR=false", "LC_ALL=C")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit when EDITOR=false")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatal("expected non-zero exit code when editor fails")
	}
}

// createFixtureFiles creates files in dir for testing.
func createFixtureFiles(t *testing.T, dir string, names []string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name+"\n"), 0o644); err != nil {
			t.Fatalf("creating fixture file %s: %v", path, err)
		}
	}
}

// createDirWithFiles creates a temp dir with the named files and returns
// the directory path.
func createDirWithFiles(t *testing.T, names []string) string {
	t.Helper()
	dir := t.TempDir()
	createFixtureFiles(t, dir, names)
	return dir
}
