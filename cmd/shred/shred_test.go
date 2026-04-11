// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/shred. Implements srd099-shred R3.3.
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

// createTestFile creates a file filled with repeating data.
func createTestFile(t *testing.T, path string, size int) {
	t.Helper()
	data := bytes.Repeat([]byte("A"), size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to create test file %s: %v", path, err)
	}
}

// patternRe matches overwrite pattern names in verbose output.
var patternRe = regexp.MustCompile(`\([^)]+\)`)

// progNameRe matches the program name prefix (shred: or gshred:) in output.
var progNameRe = regexp.MustCompile(`(?m)^g?shred:`)

// normalizeVerbose replaces pattern names and program name in verbose output
// so differences between implementations do not cause failures.
func normalizeVerbose(data []byte) []byte {
	data = patternRe.ReplaceAll(data, []byte("(PATTERN)"))
	data = progNameRe.ReplaceAll(data, []byte("PROG:"))
	return data
}

// clearOutput returns nil to suppress output comparison.
// Used for error cases where message format differs between implementations.
func clearOutput(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests comparing our shred against gshred.
// R3.3: covers basic overwrite, -n, -s, -z, -v, and error conditions.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshred")
	if err != nil {
		t.Skipf("reference binary gshred not in PATH: %v", err)
	}

	workDir := t.TempDir()
	createTestFile(t, filepath.Join(workDir, "basic"), 1024)
	createTestFile(t, filepath.Join(workDir, "iter"), 1024)
	createTestFile(t, filepath.Join(workDir, "sized"), 4096)
	createTestFile(t, filepath.Join(workDir, "zero"), 1024)
	createTestFile(t, filepath.Join(workDir, "verbose_n1"), 1024)
	createTestFile(t, filepath.Join(workDir, "verbose_z"), 1024)

	tests := []testutils.DiffTest{
		{
			Name:    "basic_overwrite",
			Args:    []string{"basic"},
			WorkDir: workDir,
		},
		{
			Name:    "iterations_1",
			Args:    []string{"-n", "1", "iter"},
			WorkDir: workDir,
		},
		{
			Name:    "size_512",
			Args:    []string{"-s", "512", "sized"},
			WorkDir: workDir,
		},
		{
			Name:    "zero_pass",
			Args:    []string{"-z", "zero"},
			WorkDir: workDir,
		},
		{
			Name:      "verbose_n1",
			Args:      []string{"-v", "-n", "1", "verbose_n1"},
			WorkDir:   workDir,
			Normalize: []testutils.NormalizeFunc{normalizeVerbose},
		},
		{
			Name:      "verbose_n1_zero",
			Args:      []string{"-v", "-n", "1", "-z", "verbose_z"},
			WorkDir:   workDir,
			Normalize: []testutils.NormalizeFunc{normalizeVerbose},
		},
		{
			Name:      "nonexistent_error",
			Args:      []string{"no_such_file_xyz"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestRemove verifies that -u removes the file after shredding.
// Tested separately because RunDiffTests shares a WorkDir and the
// ref binary would remove the file before the Go binary runs.
func TestRemove(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("file_removed", func(t *testing.T) {
		workDir := t.TempDir()
		createTestFile(t, filepath.Join(workDir, "rmfile"), 512)

		cmd := exec.Command(goBin, "-u", "-n", "1", "rmfile")
		cmd.Dir = workDir
		cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)

		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("shred -u failed: %v\noutput: %s", err, out)
		}

		path := filepath.Join(workDir, "rmfile")
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Errorf("file should have been removed by shred -u")
		}
	})

	t.Run("diff_exit_code", func(t *testing.T) {
		refBin, err := exec.LookPath("gshred")
		if err != nil {
			t.Skipf("reference binary gshred not in PATH: %v", err)
		}

		runBin := func(bin string) int {
			dir := t.TempDir()
			createTestFile(t, filepath.Join(dir, "rmfile"), 512)
			cmd := exec.Command(bin, "-u", "-n", "1", "rmfile")
			cmd.Dir = dir
			cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
			_ = cmd.Run() // exit code checked via ProcessState
			return cmd.ProcessState.ExitCode()
		}

		refCode := runBin(refBin)
		goCode := runBin(goBin)
		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}
	})
}

// TestHelpVersion verifies --help and --version exit 0 with stdout output.
// R3.2: --help and --version produce output on stdout and exit 0.
func TestHelpVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("help", func(t *testing.T) {
		cmd := exec.Command(goBin, "--help")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if len(out) == 0 {
			t.Error("--help produced no output")
		}
	})

	t.Run("version", func(t *testing.T) {
		cmd := exec.Command(goBin, "--version")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if len(out) == 0 {
			t.Error("--version produced no output")
		}
	})
}
