// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stat against gstat (GNU coreutils).
// Implements srd082 R1.1, R2.1-R2.3, R3.1, R4.2, R5.1, R6.1, R7.1-R7.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gstat"

// makeNormalizer creates a NormalizeFunc that replaces binary-specific names
// and normalizes syscall error message capitalization.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(programName))
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(programName))
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeFsLiveValues replaces large decimal numbers (7+ digits) with
// a placeholder. Live filesystem stats (free blocks, inodes) change
// between the reference and Go binary invocations.
func normalizeFsLiveValues(b []byte) []byte {
	re := regexp.MustCompile(`\d{7,}`)
	return re.ReplaceAll(b, []byte("NNNNNNN"))
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// setupFixtures creates test files in dir for differential testing.
func setupFixtures(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(
		filepath.Join(dir, "hello.txt"),
		[]byte("hello\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink("hello.txt",
		filepath.Join(dir, "link")); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// TestDiff runs differential tests comparing cmd/stat against gstat.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	workDir := t.TempDir()
	setupFixtures(t, workDir)

	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		runDefaultTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("format", func(t *testing.T) {
		t.Parallel()
		runFormatTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("modes", func(t *testing.T) {
		t.Parallel()
		runModeTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("filesystem", func(t *testing.T) {
		t.Parallel()
		runFilesystemTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("dereference", func(t *testing.T) {
		t.Parallel()
		runDereferenceTests(t, goBin, refBin, workDir, norms)
	})
	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		runErrorTests(t, goBin, refBin, workDir, norms)
	})
}

// runDefaultTests tests default multi-line stat output.
func runDefaultTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name: "regular_file", Args: []string{"hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "directory", Args: []string{"subdir"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "symlink", Args: []string{"link"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "multiple_files",
			Args:    []string{"hello.txt", "subdir"},
			WorkDir: workDir, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runFormatTests tests -c format string expansion.
func runFormatTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name: "size_name",
			Args:    []string{"-c", "%s %n", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "perms",
			Args:    []string{"-c", "%a %A", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "ids",
			Args:    []string{"-c", "%u %U %g %G", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "device_inode",
			Args:    []string{"-c", "%d %D %i %h", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "file_type",
			Args:    []string{"-c", "%F", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "blocks_size",
			Args:    []string{"-c", "%b %B %o %f", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "epoch_times",
			Args:    []string{"-c", "%X %Y %Z %W", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "human_times",
			Args:    []string{"-c", "%x", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "quoted_name",
			Args:    []string{"-c", "%N", "link"},
			WorkDir: workDir, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runModeTests tests terse mode, dereference, and error cases.
func runModeTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name: "terse", Args: []string{"-t", "hello.txt"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name: "dereference", Args: []string{"-L", "link"},
			WorkDir: workDir, Normalize: norms,
		},
		{
			Name:      "missing_file",
			Args:      []string{"nonexistent"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{"hello.txt", "nonexistent", "subdir"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runFilesystemTests tests -f filesystem stat mode.
// R5.1: filesystem status via -f flag.
// R6.1: filesystem format directives.
func runFilesystemTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	// Filesystem stats (free blocks, inodes) change between runs.
	// Add a normalizer to replace large numbers with a placeholder.
	fsNorms := append(norms, normalizeFsLiveValues)
	tests := []testutils.DiffTest{
		{
			Name:      "fs_default",
			Args:      []string{"-f", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_format_blocks",
			Args:      []string{"-f", "-c", "%a %b %f", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_format_inodes",
			Args:      []string{"-f", "-c", "%c %d", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_format_sizes",
			Args:      []string{"-f", "-c", "%s %S", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_format_type",
			Args:      []string{"-f", "-c", "%t %T", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_format_id_namelen",
			Args:      []string{"-f", "-c", "%i %l", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_format_name",
			Args:      []string{"-f", "-c", "%n", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_terse",
			Args:      []string{"-f", "-t", "."},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
		{
			Name:      "fs_on_file",
			Args:      []string{"-f", "hello.txt"},
			WorkDir:   workDir,
			Normalize: fsNorms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runDereferenceTests tests -L dereference mode.
// R2.2: -L follows symlinks and reports target file status.
func runDereferenceTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name:      "deref_default",
			Args:      []string{"-L", "link"},
			WorkDir:   workDir,
			Normalize: norms,
		},
		{
			Name:      "deref_format_type",
			Args:      []string{"-L", "-c", "%F", "link"},
			WorkDir:   workDir,
			Normalize: norms,
		},
		{
			Name:      "deref_format_size",
			Args:      []string{"-L", "-c", "%s %n", "link"},
			WorkDir:   workDir,
			Normalize: norms,
		},
		{
			Name:      "deref_terse",
			Args:      []string{"-L", "-t", "link"},
			WorkDir:   workDir,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runErrorTests tests error handling and edge cases.
// R7.1: exit 0 when all files processed successfully.
// R7.2: exit 1 when any file cannot be accessed.
func runErrorTests(
	t *testing.T, goBin, refBin, workDir string,
	norms []testutils.NormalizeFunc,
) {
	t.Helper()
	tests := []testutils.DiffTest{
		{
			Name:      "no_operand",
			Args:      []string{},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "multiple_missing",
			Args:      []string{"nofile1", "nofile2"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "valid_then_missing",
			Args:      []string{"hello.txt", "no_such_file"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: norms,
		},
		{
			Name:      "missing_then_valid",
			Args:      []string{"no_such_file", "hello.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersionFlag verifies that --version prints version info and exits 0.
// R7.3: --version flag support.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version exited with error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--version produced no output")
	}
	if !bytes.Contains(out, []byte("stat")) {
		t.Errorf("--version output does not contain 'stat': %q", out)
	}
}

// TestHelpFlag verifies that --help prints usage info and exits 0.
// R7.3: --help flag support.
func TestHelpFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help exited with error: %v", err)
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Errorf("--help output does not contain 'Usage:': %q", out)
	}
}
