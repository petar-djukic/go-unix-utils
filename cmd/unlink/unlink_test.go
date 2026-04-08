// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unlink against gunlink (GNU coreutils).
// Implements srd038 R3.1 (compare stdout/stderr/exit codes),
// R3.2 (test coverage for all cases), R3.3 (verify file removal).
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gunlink"
const execTimeout = 30 * time.Second

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

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Is a directory", "is a directory"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/unlink against gunlink.
// R3.1: compare stdout, stderr, exit codes via pkg/testutils.
// R3.2: covers regular file, symlink, zero args, multi args, nonexistent, directory.
// R3.3: verify file no longer exists after successful invocation.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		runErrorTests(t, goBin, refBin, norm)
	})
	t.Run("removal", func(t *testing.T) {
		t.Parallel()
		runRemovalTests(t, goBin, refBin, norm)
	})
}

// runErrorTests uses RunDiffTests for error cases where neither binary
// mutates the filesystem state.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	workDir := t.TempDir()

	// Create a directory for the directory-argument error test.
	dirPath := filepath.Join(workDir, "adir")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "zero_args", Args: []string{},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "multi_args", Args: []string{"a", "b"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "nonexistent", Args: []string{"doesnotexist"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "directory", Args: []string{"adir"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a removal test that runs each binary in its own dir.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string)
}

// runRemovalTests runs tests where each binary operates in an isolated
// temp directory since unlink mutates the filesystem.
// R3.3: verifies the target file no longer exists after invocation.
func runRemovalTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := removalCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, norm, tc)
		})
	}
}

// setupRegularFile creates a regular file named "target" in dir.
func setupRegularFile(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "target")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// setupSymlink creates a symlink named "link" pointing to a regular file.
func setupSymlink(t *testing.T, dir string) {
	t.Helper()
	realFile := filepath.Join(dir, "real")
	if err := os.WriteFile(realFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "link")); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// removalCases returns the table of isolated removal test cases.
// R3.2: regular file removal and symbolic link removal.
func removalCases() []isolatedCase {
	return []isolatedCase{
		{
			name:  "regular_file",
			args:  []string{"target"},
			setup: setupRegularFile,
		},
		{
			name:  "symlink",
			args:  []string{"link"},
			setup: setupSymlink,
		},
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and verifies file removal.
// R3.3: checks that the target file no longer exists after invocation.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)

	// R3.3: verify target file no longer exists.
	target := tc.args[0]
	verifyRemoved(t, filepath.Join(goDir, target))
}

// verifyRemoved checks that the given path no longer exists.
// R3.3: confirm the target file was removed after successful invocation.
func verifyRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after unlink: %s", path)
	}
}

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary in workDir and captures stdout, stderr, exit code.
func runBin(t *testing.T, bin string, args []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Dir = workDir
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)

	return extractResult(t, cmd, ctx, &outBuf, &errBuf)
}

// extractResult runs the command and returns the captured result.
func extractResult(t *testing.T, cmd *exec.Cmd, ctx context.Context, outBuf, errBuf *bytes.Buffer) binResult {
	t.Helper()
	err := cmd.Run()
	if err == nil {
		return binResult{stdout: outBuf.Bytes(), stderr: errBuf.Bytes()}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return binResult{
			stdout:   outBuf.Bytes(),
			stderr:   errBuf.Bytes(),
			exitCode: exitErr.ExitCode(),
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s timed out after %v", cmd.Path, execTimeout)
	}
	t.Fatalf("%s failed: %v", cmd.Path, err)
	return binResult{} // unreachable
}

// compareOutputs compares stdout, stderr, and exit code between ref and go.
func compareOutputs(t *testing.T, norm testutils.NormalizeFunc, ref, got binResult) {
	t.Helper()
	refOut := norm(ref.stdout)
	gotOut := norm(got.stdout)
	refErr := norm(ref.stderr)
	gotErr := norm(got.stderr)

	if !bytes.Equal(refOut, gotOut) {
		t.Errorf("stdout mismatch\nref: %q\ngot: %q", refOut, gotOut)
	}
	if !bytes.Equal(refErr, gotErr) {
		t.Errorf("stderr mismatch\nref: %q\ngot: %q", refErr, gotErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch: ref=%d got=%d", ref.exitCode, got.exitCode)
	}
}
