// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unlink against GNU gunlink.
// Covers prd038-unlink R2.4 (directory rejection),
// R3.1-R3.3 (differential testing, file removal verification).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gunlink and Go unlink.
// Handles binary name differences and error message capitalization (GNU uses
// capitalized strerror messages, Go uses lowercase).
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?unlink|gunlink`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("unlink"))
		b = tryHelp.ReplaceAll(b, nil)
		b = bytes.ToLower(b)
		return b
	}
}

// TestDiffErrors runs differential tests for error cases where both binaries
// produce identical errors without mutating filesystem state.
func TestDiffErrors(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// R2.4: create a directory for the directory-rejection test.
	dirPath := filepath.Join(t.TempDir(), "testdir")
	if mkErr := os.Mkdir(dirPath, 0o755); mkErr != nil {
		t.Fatalf("create test directory: %v", mkErr)
	}

	tests := []testutils.DiffTest{
		// R2.1/R3.2: zero arguments.
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.2/R3.2: extra arguments.
		{
			Name:      "extra_args",
			Args:      []string{"a", "b"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.3/R3.2: nonexistent file.
		{
			Name:      "nonexistent_file",
			Args:      []string{"no_such_file_xyz"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R2.4/R3.2: directory argument.
		{
			Name:      "directory_arg",
			Args:      []string{dirPath},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runUnlink runs a binary with the given args and returns captured output.
func runUnlink(t *testing.T, binary string, args []string) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, runErr)
		}
	}
	return cmdResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

// assertRemoved verifies that the path no longer exists.
func assertRemoved(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s: file %s still exists after unlink", label, path)
	}
}

// compareResults asserts that ref and go binary outputs match.
func compareResults(t *testing.T, ref, got cmdResult) {
	t.Helper()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("stdout mismatch\n  ref: %q\n  go:  %q", ref.stdout, got.stdout)
	}
	if !bytes.Equal(ref.stderr, got.stderr) {
		t.Errorf("stderr mismatch\n  ref: %q\n  go:  %q", ref.stderr, got.stderr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch: ref=%d go=%d", ref.exitCode, got.exitCode)
	}
}

// createTempFile creates a regular file in dir and returns its path.
func createTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("test content\n"), 0o644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	return p
}

// createTempSymlink creates a symbolic link in dir pointing to target.
func createTempSymlink(t *testing.T, dir, target, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.Symlink(target, p); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	return p
}

// TestUnlinkRegularFile verifies successful removal of a regular file.
// R3.1: compares stdout, stderr, exit code between Go and ref binaries.
// R3.3: verifies file no longer exists after invocation.
func TestUnlinkRegularFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	// Run reference binary on a fresh file.
	refFile := createTempFile(t, t.TempDir(), "target")
	refRes := runUnlink(t, refBin, []string{refFile})
	assertRemoved(t, refFile, "ref")

	// Run Go binary on a fresh file.
	goFile := createTempFile(t, t.TempDir(), "target")
	goRes := runUnlink(t, goBin, []string{goFile})
	assertRemoved(t, goFile, "go")

	// R3.1/R3.3: compare outputs.
	compareResults(t, refRes, goRes)
}

// TestUnlinkSymlink verifies successful removal of a symbolic link.
// R3.2: covers symbolic link removal case.
// R3.3: verifies link no longer exists after invocation.
func TestUnlinkSymlink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	// Run reference binary on a symlink.
	refDir := t.TempDir()
	refTarget := createTempFile(t, refDir, "real")
	refLink := createTempSymlink(t, refDir, refTarget, "link")
	refRes := runUnlink(t, refBin, []string{refLink})
	assertRemoved(t, refLink, "ref-link")

	// Run Go binary on a symlink.
	goDir := t.TempDir()
	goTarget := createTempFile(t, goDir, "real")
	goLink := createTempSymlink(t, goDir, goTarget, "link")
	goRes := runUnlink(t, goBin, []string{goLink})
	assertRemoved(t, goLink, "go-link")

	// R3.1: compare outputs.
	compareResults(t, refRes, goRes)
}
