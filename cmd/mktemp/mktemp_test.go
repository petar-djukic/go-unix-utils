// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential and structural tests for cmd/mktemp.
// Implements srd036 R4.1, R4.2, R4.3, R4.4.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizePath replaces absolute paths with their directory component.
// Non-absolute and empty data is returned cleared (nil) to suppress comparison.
// R4.4: mktemp random names differ; directory prefix is deterministic.
func normalizePath(data []byte) []byte {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return data
	}
	if filepath.IsAbs(s) {
		return []byte(filepath.Dir(s) + "\n")
	}
	return nil
}

// TestDiff runs differential tests comparing cmd/mktemp against gmktemp.
// R4.1: uses testutils.BuildBinary and testutils.RunDiffTests.
// R4.4: skips when gmktemp or main.go is not found.
func TestDiff(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat("main.go"); os.IsNotExist(err) {
		t.Skip("cmd/mktemp not yet generated")
	}
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}
	tmpDir := t.TempDir()
	tests := buildDiffTests(tmpDir)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildDiffTests constructs the DiffTest slice for mktemp.
// R4.3: default, -d, custom template, --suffix, -p, -t, -u, and errors.
func buildDiffTests(tmpDir string) []testutils.DiffTest {
	env := []string{"TMPDIR=" + tmpDir}
	norm := []testutils.NormalizeFunc{normalizePath}

	return []testutils.DiffTest{
		{Name: "default", Env: env, Normalize: norm},
		{Name: "directory_mode", Args: []string{"-d"}, Env: env, Normalize: norm},
		{Name: "custom_template", Args: []string{"myapp.XXXXXX"}, Env: env, Normalize: norm},
		{Name: "suffix", Args: []string{"--suffix=.txt"}, Env: env, Normalize: norm},
		{Name: "p_explicit_dir", Args: []string{"-p", tmpDir, "test.XXXXXX"}, Env: env, Normalize: norm},
		{Name: "t_legacy_mode", Args: []string{"-t", "legacy.XXXXXX"}, Env: env, Normalize: norm},
		{Name: "dry_run", Args: []string{"-u"}, Env: env, Normalize: norm},
		{Name: "error_few_xs", Args: []string{"badXX"}, Env: env, ExitCode: 1, Normalize: norm},
		{Name: "error_suffix_slash", Args: []string{"--suffix=/bad"}, Env: env, ExitCode: 1, Normalize: norm},
	}
}

// TestStructural verifies structural properties of mktemp output.
// R4.2: path validity, existence, permissions, template pattern matching.
func TestStructural(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat("main.go"); os.IsNotExist(err) {
		t.Skip("cmd/mktemp not yet generated")
	}
	goBin := testutils.BuildBinary(t, ".")

	t.Run("default_file", func(t *testing.T) {
		t.Parallel()
		verifyDefaultFile(t, goBin)
	})
	t.Run("directory_mode", func(t *testing.T) {
		t.Parallel()
		verifyDirMode(t, goBin)
	})
	t.Run("custom_template", func(t *testing.T) {
		t.Parallel()
		verifyCustomTemplate(t, goBin)
	})
	t.Run("suffix", func(t *testing.T) {
		t.Parallel()
		verifySuffix(t, goBin)
	})
	t.Run("p_dir", func(t *testing.T) {
		t.Parallel()
		verifyPDir(t, goBin)
	})
	t.Run("dry_run", func(t *testing.T) {
		t.Parallel()
		verifyDryRun(t, goBin)
	})
}

// verifyDefaultFile checks default mktemp creates a 0600 file matching tmp.XXXXXXXXXX.
func verifyDefaultFile(t *testing.T, goBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := runMktemp(t, goBin, nil, tmpDir)
	assertFileAt(t, path, 0o600)
	assertInDir(t, path, tmpDir)
	assertNameMatches(t, path, `^tmp\..{10}$`)
}

// verifyDirMode checks -d creates a 0700 directory in TMPDIR.
func verifyDirMode(t *testing.T, goBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := runMktemp(t, goBin, []string{"-d"}, tmpDir)
	assertDirAt(t, path, 0o700)
	assertInDir(t, path, tmpDir)
}

// verifyCustomTemplate checks a user-supplied template produces the correct name pattern.
func verifyCustomTemplate(t *testing.T, goBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := runMktemp(t, goBin, []string{"myapp.XXXXXX"}, tmpDir)
	assertFileAt(t, path, 0o600)
	assertNameMatches(t, path, `^myapp\..{6}$`)
}

// verifySuffix checks --suffix appends the extension to the generated name.
func verifySuffix(t *testing.T, goBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := runMktemp(t, goBin, []string{"--suffix=.txt"}, tmpDir)
	assertFileAt(t, path, 0o600)
	if !strings.HasSuffix(filepath.Base(path), ".txt") {
		t.Errorf("expected .txt suffix in %q", filepath.Base(path))
	}
}

// verifyPDir checks -p DIR places the file in the specified directory.
func verifyPDir(t *testing.T, goBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	targetDir := t.TempDir()
	path := runMktemp(t, goBin, []string{"-p", targetDir, "test.XXXXXX"}, tmpDir)
	assertFileAt(t, path, 0o600)
	assertInDir(t, path, targetDir)
	assertNameMatches(t, path, `^test\..{6}$`)
}

// verifyDryRun checks -u prints a path without creating anything.
func verifyDryRun(t *testing.T, goBin string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := runMktemp(t, goBin, []string{"-u"}, tmpDir)
	if path == "" {
		t.Fatal("expected non-empty path from dry-run")
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("dry-run should not create entry at %q", path)
	}
}

// runMktemp executes the Go mktemp binary and returns the output path.
// Relative paths are resolved against the binary's working directory (tmpDir).
func runMktemp(t *testing.T, goBin string, args []string, tmpDir string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Env = []string{"LC_ALL=C", "TMPDIR=" + tmpDir}
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("mktemp %v failed: %v\nstderr: %s", args, err, errBuf.String())
	}
	path := strings.TrimSpace(outBuf.String())
	if !filepath.IsAbs(path) {
		path = filepath.Join(tmpDir, path)
	}
	return path
}

// assertFileAt verifies a regular file exists at path with the given permissions.
func assertFileAt(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file does not exist: %s", path)
	}
	if info.IsDir() {
		t.Errorf("expected file, got directory: %s", path)
	}
	if info.Mode().Perm() != perm {
		t.Errorf("perm: want %04o, got %04o for %s", perm, info.Mode().Perm(), path)
	}
}

// assertDirAt verifies a directory exists at path with the given permissions.
func assertDirAt(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory does not exist: %s", path)
	}
	if !info.IsDir() {
		t.Errorf("expected directory, got file: %s", path)
	}
	if info.Mode().Perm() != perm {
		t.Errorf("perm: want %04o, got %04o for %s", perm, info.Mode().Perm(), path)
	}
}

// assertInDir verifies the file's parent directory matches the expected dir.
func assertInDir(t *testing.T, path, dir string) {
	t.Helper()
	if filepath.Dir(path) != dir {
		t.Errorf("parent dir: want %s, got %s", dir, filepath.Dir(path))
	}
}

// assertNameMatches verifies the base name matches the given regex pattern.
func assertNameMatches(t *testing.T, path, pattern string) {
	t.Helper()
	base := filepath.Base(path)
	matched, err := regexp.MatchString(pattern, base)
	if err != nil {
		t.Fatalf("bad regex %q: %v", pattern, err)
	}
	if !matched {
		t.Errorf("name %q does not match pattern %q", base, pattern)
	}
}
