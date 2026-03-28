// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd034-mkdir R4.1-R4.3: Differential testing for mkdir.
// R4.1: compare stdout, stderr, exit codes between Go binary and gmkdir.
// R4.2: cover single/multiple creation, -p, -m, -v, and error cases.
// R4.3: verify permission bits match for all -m and -p combinations.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binTimeout = 10 * time.Second

// makeBinNormalizer returns a normalizer that replaces binary paths and
// basenames with "mkdir" so output from ref and go binaries can be compared.
// GNU coreutils strips argv[0] to basename for error messages.
func makeBinNormalizer(refBin, goBin string) testutils.NormalizeFunc {
	refBase := filepath.Base(refBin)
	goBase := filepath.Base(goBin)
	return func(b []byte) []byte {
		// Replace full paths first, then basenames for remaining occurrences.
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("mkdir"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("mkdir"))
		if refBase != "mkdir" {
			b = bytes.ReplaceAll(b, []byte(refBase), []byte("mkdir"))
		}
		if goBase != "mkdir" {
			b = bytes.ReplaceAll(b, []byte(goBase), []byte("mkdir"))
		}
		return b
	}
}

// setupWithDir creates a temp dir containing a pre-created subdirectory.
func setupWithDir(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// setupWithNestedDir creates a temp dir with a nested directory tree.
func setupWithNestedDir(t *testing.T, path string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, path), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDiff runs differential tests that work in a shared workdir.
// R4.1: compares stdout, stderr, and exit codes via pkg/testutils.
// R4.2: covers error cases and -p on existing directories.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	norm := []testutils.NormalizeFunc{makeBinNormalizer(refBin, goBin)}

	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			Normalize: norm,
		},
		{
			Name:      "existing_target_error",
			Args:      []string{"testdir"},
			WorkDir:   setupWithDir(t, "testdir"),
			Normalize: norm,
		},
		{
			Name:      "missing_parent_error",
			Args:      []string{"a/b/c"},
			Normalize: norm,
		},
		{
			Name:    "parents_existing_dir",
			Args:    []string{"-p", "testdir"},
			WorkDir: setupWithDir(t, "testdir"),
		},
		{
			Name:    "parents_existing_nested",
			Args:    []string{"-p", "a/b/c"},
			WorkDir: setupWithNestedDir(t, "a/b/c"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMkdirCreation runs each binary in separate temp dirs to test
// directory creation. R4.2: covers single, multiple, -p, -v, -m cases.
func TestMkdirCreation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	normalize := makeBinNormalizer(refBin, goBin)

	tests := []struct {
		name string
		args []string
	}{
		{"single_dir", []string{"testdir"}},
		{"multiple_dirs", []string{"dir1", "dir2", "dir3"}},
		{"parents_nested", []string{"-p", "a/b/c"}},
		{"verbose_single", []string{"-v", "testdir"}},
		{"verbose_parents", []string{"-pv", "a/b/c"}},
		{"mode_octal", []string{"-m", "0755", "testdir"}},
		{"mode_with_parents", []string{"-m", "0700", "-p", "a/b/c"}},
		{"mode_parents_verbose", []string{"-m", "0755", "-pv", "a/b/c"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareCreation(t, goBin, refBin, tc.args, normalize)
		})
	}
}

// binResult holds captured output from a binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary with args in workDir and captures output.
func runBin(t *testing.T, bin string, args []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), binTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	return extractResult(t, bin, err, stdout.Bytes(), stderr.Bytes())
}

// extractResult converts exec results into a binResult.
func extractResult(
	t *testing.T, bin string, err error,
	stdout, stderr []byte,
) binResult {
	t.Helper()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}
	return binResult{stdout: stdout, stderr: stderr, exitCode: code}
}

// compareCreation runs both binaries in separate dirs and compares.
func compareCreation(
	t *testing.T, goBin, refBin string,
	args []string, normalize testutils.NormalizeFunc,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runBin(t, refBin, args, refDir)
	goRes := runBin(t, goBin, args, goDir)

	refStdout := normalize(refRes.stdout)
	goStdout := normalize(goRes.stdout)
	refStderr := normalize(refRes.stderr)
	goStderr := normalize(goRes.stderr)

	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout differs\n  ref: %q\n  go:  %q", refStdout, goStdout)
	}
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr differs\n  ref: %q\n  go:  %q", refStderr, goStderr)
	}
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("exit code differs: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}
}

// TestMkdirPermissions verifies permission bits match between Go and gmkdir.
// R4.3: covers -m with various modes and -p with -m combinations.
func TestMkdirPermissions(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	tests := []struct {
		name      string
		args      []string
		checkDirs []string
	}{
		{
			name:      "mode_0755",
			args:      []string{"-m", "0755", "testdir"},
			checkDirs: []string{"testdir"},
		},
		{
			name:      "mode_0700",
			args:      []string{"-m", "0700", "testdir"},
			checkDirs: []string{"testdir"},
		},
		{
			name:      "mode_0644",
			args:      []string{"-m", "0644", "testdir"},
			checkDirs: []string{"testdir"},
		},
		{
			name:      "default_mode",
			args:      []string{"testdir"},
			checkDirs: []string{"testdir"},
		},
		{
			name:      "parents_mode_final_only",
			args:      []string{"-m", "0700", "-p", "a/b/c"},
			checkDirs: []string{"a", "a/b", "a/b/c"},
		},
		{
			name:      "parents_mode_0755",
			args:      []string{"-m", "0755", "-p", "x/y"},
			checkDirs: []string{"x", "x/y"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			comparePermissions(t, goBin, refBin, tc.args, tc.checkDirs)
		})
	}
}

// comparePermissions runs both binaries and compares directory permissions.
func comparePermissions(
	t *testing.T, goBin, refBin string,
	args, checkDirs []string,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	runBin(t, refBin, args, refDir)
	runBin(t, goBin, args, goDir)

	for _, d := range checkDirs {
		refPerm := dirPerm(t, filepath.Join(refDir, d))
		goPerm := dirPerm(t, filepath.Join(goDir, d))
		if refPerm != goPerm {
			t.Errorf("permissions differ for %s: ref=%o go=%o",
				d, refPerm, goPerm)
		}
	}
}

// dirPerm returns the permission bits for a directory.
func dirPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
