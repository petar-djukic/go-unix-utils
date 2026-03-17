// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unlink against gunlink (GNU coreutils).
// Implements prd038-unlink R3.1-R3.3 test coverage for R1.1-R1.3, R2.1-R2.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameRe matches a full binary path or bare program name for unlink/gunlink
// in error output (e.g., "/opt/homebrew/bin/gunlink" or "/tmp/.../unlink").
var progNameRe = regexp.MustCompile(`(?:\S*/)?g?unlink`)

// normalizeProgramName replaces program name paths in output so that
// differences between binary paths are ignored.
func normalizeProgramName(b []byte) []byte {
	return progNameRe.ReplaceAll(b, []byte("unlink"))
}

// clearOutput replaces all output with empty bytes so version/help text
// differences are ignored; only exit code is compared.
func clearOutput(b []byte) []byte {
	return nil
}

// TestDiff runs differential tests for error cases that do not require
// filesystem mutation via RunDiffTests directly.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.1: no arguments prints error to stderr and exits non-zero.
		{
			Name:      "no_args",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.2: extra operand prints error to stderr and exits non-zero.
		{
			Name:      "extra_operand",
			Args:      []string{"a", "b"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.3: non-existent file prints error to stderr and exits non-zero.
		{
			Name:      "nonexistent_file",
			Args:      []string{"no_such_file_xyzzy"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// --version exits 0.
		{
			Name:      "version_exit_0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// --help exits 0.
		{
			Name:      "help_exit_0",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestUnlinkRegularFile verifies that unlinking a regular file removes it.
// R1.1-R1.3, R3.3: file is removed and exit code is 0.
func TestUnlinkRegularFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd := exec.Command(goBin, target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unlink exited non-zero: %v\noutput: %s", err, out)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file still exists after unlink")
	}
}

// TestUnlinkSymlink verifies that unlinking a symbolic link removes the link
// itself, not the target. R3.2: symbolic link removal case.
func TestUnlinkSymlink(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "realfile")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
	link := filepath.Join(tmpDir, "symlink")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cmd := exec.Command(goBin, link)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unlink exited non-zero: %v\noutput: %s", err, out)
	}

	// Symlink should be gone.
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("symlink still exists after unlink")
	}
	// Target should still exist.
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target file was removed, expected it to remain: %v", err)
	}
}

// TestUnlinkDirectory verifies that unlinking a directory fails with an error.
// R2.4: directory argument exits non-zero.
func TestUnlinkDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	cmd := exec.Command(goBin, dir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected unlink of directory to fail, but it succeeded\noutput: %s", out)
	}

	// Directory should still exist.
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory was removed, expected it to remain: %v", err)
	}
}

// TestDiffDirectory runs a differential test for unlinking a directory,
// comparing our output against gunlink.
func TestDiffDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "directory_argument",
			Args:      []string{dir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
