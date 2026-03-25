// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd101-install: Copy files and set attributes.
// R1.1 (basic file copy with 755 default), R1.2 (-m mode),
// R2.1 (-d directory creation), R3.2 (exit codes).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests against ginstall.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ginstall")
	if err != nil {
		t.Skipf("reference binary ginstall not in PATH: %v", err)
	}

	// Normalize stderr and stdout since error messages, help text,
	// and version strings differ between implementations.
	dropAll := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{dropAll},
		},
		{
			Name:      "missing_dest",
			Args:      []string{"nonexistent"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{dropAll},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInstallDefaultPermissions verifies AC3: default 0755 permissions.
func TestInstallDefaultPermissions(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	writeTestFile(t, srcFile, "hello\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")

	cmd := exec.Command(goBin, srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("cannot stat dest: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o755 {
		t.Errorf("permissions = %o, want 0755", got)
	}

	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("cannot read dest: %v", err)
	}
	if string(content) != "hello\n" {
		t.Errorf("content = %q, want %q", string(content), "hello\n")
	}
}

// TestInstallCustomMode verifies -m flag sets permissions.
func TestInstallCustomMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	writeTestFile(t, srcFile, "data\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")

	cmd := exec.Command(goBin, "-m", "0644", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -m failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("cannot stat dest: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o644 {
		t.Errorf("permissions = %o, want 0644", got)
	}
}

// TestInstallDirMode verifies AC4: -d creates directories.
func TestInstallDirMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "a", "b", "c")
	dir2 := filepath.Join(tmpDir, "x", "y")

	cmd := exec.Command(goBin, "-d", dir1, dir2)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -d failed: %v\noutput: %s", err, out)
	}

	assertDirExists(t, dir1)
	assertDirExists(t, dir2)
}

// TestInstallCopyToDirectory verifies AC5: copy to existing dir.
func TestInstallCopyToDirectory(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	writeTestFile(t, srcFile, "content\n", 0o644)
	destDir := filepath.Join(tmpDir, "destdir")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, srcFile, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install to dir failed: %v\noutput: %s", err, out)
	}

	expected := filepath.Join(destDir, "source.txt")
	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("expected file not found: %v", err)
	}
	if string(content) != "content\n" {
		t.Errorf("content = %q, want %q", string(content), "content\n")
	}

	info, err := os.Stat(expected)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
	}
}

// TestInstallMultipleSources verifies multiple source files to directory.
func TestInstallMultipleSources(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	src1 := filepath.Join(tmpDir, "a.txt")
	src2 := filepath.Join(tmpDir, "b.txt")
	writeTestFile(t, src1, "aaa\n", 0o644)
	writeTestFile(t, src2, "bbb\n", 0o644)
	destDir := filepath.Join(tmpDir, "out")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, src1, src2, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install multiple failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, filepath.Join(destDir, "a.txt"), "aaa\n")
	assertFileContent(t, filepath.Join(destDir, "b.txt"), "bbb\n")
}

// TestInstallExitCodeOnError verifies exit 1 on failure.
func TestInstallExitCodeOnError(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "/nonexistent/source", "/tmp/dest")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code for missing source")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}

// TestInstallCreateLeadingDirs verifies -D creates parent directories.
func TestInstallCreateLeadingDirs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	writeTestFile(t, srcFile, "data\n", 0o644)
	destFile := filepath.Join(tmpDir, "deep", "nested", "dest.txt")

	cmd := exec.Command(goBin, "-D", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -D failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, destFile, "data\n")
}

// writeTestFile creates a file with the given content and permissions.
func writeTestFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("cannot write test file %s: %v", path, err)
	}
}

// assertDirExists checks that a directory exists.
func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("directory %s does not exist: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", path)
	}
}

// assertFileContent checks that a file has the expected content.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("cannot read %s: %v", path, err)
		return
	}
	if string(got) != want {
		t.Errorf("content of %s = %q, want %q", path, string(got), want)
	}
}
