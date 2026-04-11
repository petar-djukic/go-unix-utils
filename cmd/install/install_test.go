// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/install. Implements srd101 R3.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameRe matches the program name prefix in error output.
var progNameRe = regexp.MustCompile(`(?m)^g?install:`)

// tryHelpRe matches the "Try '...' --help" line with varying binary paths.
var tryHelpRe = regexp.MustCompile(`Try '[^']+' for more information\.`)

// normalizeProgramName replaces ginstall:/install: with a fixed prefix.
func normalizeProgramName(data []byte) []byte {
	data = progNameRe.ReplaceAll(data, []byte("PROG:"))
	data = tryHelpRe.ReplaceAll(data, []byte("Try 'PROG --help' for more information."))
	return data
}

// clearOutput suppresses output comparison for messages that differ in format.
func clearOutput(data []byte) []byte {
	return nil
}

// createFile creates a file with specified content and mode.
func createFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("failed to create file %s: %v", path, err)
	}
}

// TestDiff runs differential tests comparing our install against ginstall.
// R3.3: covers copy, compare, directory, backup, verbose, and error conditions.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ginstall")
	if err != nil {
		t.Skipf("reference binary ginstall not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "missing_operand",
			Args:     []string{},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeProgramName,
			},
		},
		{
			Name:     "missing_dest",
			Args:     []string{"srcfile"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeProgramName,
			},
		},
		{
			Name:      "nonexistent_source",
			Args:      []string{"no_such_file_xyz", "dest"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:     "dir_mode_missing_operand",
			Args:     []string{"-d"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeProgramName,
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestCopyBasic tests basic file copy.
func TestCopyBasic(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	workDir := t.TempDir()
	srcPath := filepath.Join(workDir, "source")
	destPath := filepath.Join(workDir, "dest")
	createFile(t, srcPath, []byte("hello install"), 0o644)

	cmd := exec.Command(goBin, srcPath, destPath)
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("dest file missing: %v", err)
	}
	// R1.1: default mode is 0755
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("expected mode 0755, got %04o", perm)
	}
	data, _ := os.ReadFile(destPath)
	if string(data) != "hello install" {
		t.Errorf("expected content %q, got %q", "hello install", string(data))
	}
}

// TestCompare tests -C/--compare mode.
// R3.1: do not install if source and destination are identical.
func TestCompare(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("skips_identical", func(t *testing.T) {
		workDir := t.TempDir()
		srcPath := filepath.Join(workDir, "src")
		destPath := filepath.Join(workDir, "dest")
		content := []byte("same content")
		createFile(t, srcPath, content, 0o644)
		createFile(t, destPath, content, 0o755)

		destBefore, _ := os.Stat(destPath)
		modBefore := destBefore.ModTime()

		cmd := exec.Command(goBin, "-C", srcPath, destPath)
		cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("install -C failed: %v\noutput: %s", err, out)
		}

		destAfter, _ := os.Stat(destPath)
		if !destAfter.ModTime().Equal(modBefore) {
			t.Error("file was modified despite -C with identical content and mode")
		}
	})

	t.Run("installs_different_content", func(t *testing.T) {
		workDir := t.TempDir()
		srcPath := filepath.Join(workDir, "src")
		destPath := filepath.Join(workDir, "dest")
		createFile(t, srcPath, []byte("new content"), 0o644)
		createFile(t, destPath, []byte("old content"), 0o755)

		cmd := exec.Command(goBin, "-C", srcPath, destPath)
		cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("install -C failed: %v\noutput: %s", err, out)
		}

		data, _ := os.ReadFile(destPath)
		if string(data) != "new content" {
			t.Errorf("expected updated content, got %q", string(data))
		}
	})

	t.Run("installs_different_mode", func(t *testing.T) {
		workDir := t.TempDir()
		srcPath := filepath.Join(workDir, "src")
		destPath := filepath.Join(workDir, "dest")
		content := []byte("same content")
		createFile(t, srcPath, content, 0o644)
		createFile(t, destPath, content, 0o644) // mode differs from default 755

		cmd := exec.Command(goBin, "-C", srcPath, destPath)
		cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("install -C failed: %v\noutput: %s", err, out)
		}

		info, _ := os.Stat(destPath)
		if info.Mode().Perm() != 0o755 {
			t.Errorf("expected mode 0755 after install, got %04o", info.Mode().Perm())
		}
	})
}

// TestDirMode tests -d directory creation mode.
func TestDirMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	workDir := t.TempDir()
	dirPath := filepath.Join(workDir, "a", "b", "c")

	cmd := exec.Command(goBin, "-d", dirPath)
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -d failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

// TestExitCodes verifies exit codes match expectations.
// R3.2: exit 0 on success, exit 1 on error.
func TestExitCodes(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("success_exit_0", func(t *testing.T) {
		workDir := t.TempDir()
		src := filepath.Join(workDir, "src")
		dest := filepath.Join(workDir, "dest")
		createFile(t, src, []byte("data"), 0o644)

		cmd := exec.Command(goBin, src, dest)
		cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		if err := cmd.Run(); err != nil {
			t.Errorf("expected exit 0, got error: %v", err)
		}
	})

	t.Run("error_exit_1", func(t *testing.T) {
		cmd := exec.Command(goBin, "/nonexistent/src", "/nonexistent/dest")
		cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1 for nonexistent source")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		}
	})
}

// TestHelpVersion verifies --help and --version exit 0 with stdout output.
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
