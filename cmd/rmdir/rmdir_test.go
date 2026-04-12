// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against grmdir (GNU coreutils).
// Implements srd035 R4.1 (compare stdout/stderr/exit codes),
// R4.2 (test coverage), R4.3 (-p stops ascending when parent not empty).
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

const refBinName = "grmdir"
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
		{"Directory not empty", "directory not empty"},
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/rmdir against grmdir.
// R4.1: compare stdout, stderr, exit codes.
// R4.2: covers empty removal, non-empty error, non-existent error,
// -p, --ignore-fail-on-non-empty, -v.
// R4.3: verifies -p stops ascending when parent is not empty.
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

	// Create a non-empty directory for error tests.
	nonempty := filepath.Join(workDir, "nonempty")
	if err := os.MkdirAll(nonempty, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonempty, "file"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "missing_operand", Args: []string{},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "nonexistent", Args: []string{"doesnotexist"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "nonempty_dir", Args: []string{"nonempty"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "ignore_fail_nonempty", Args: []string{"--ignore-fail-on-non-empty", "nonempty"},
			WorkDir: workDir, ExitCode: 0, Normalize: norms,
		},
		{
			Name: "ignore_fail_nonexistent", Args: []string{"--ignore-fail-on-non-empty", "doesnotexist"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a removal test that runs each binary in its own dir.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string) // creates fixture dirs
}

// runRemovalTests runs tests where each binary operates in an isolated
// temp directory since rmdir mutates the filesystem.
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

// setupEmptyDir creates a single empty directory.
func setupEmptyDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(dir, "emptydir"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// setupMultipleDirs creates multiple empty directories.
func setupMultipleDirs(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"d1", "d2", "d3"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
}

// setupNestedEmpty creates a nested chain of empty directories.
func setupNestedEmpty(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// setupParentNotEmpty creates a/b/c where a contains a file,
// so -p should remove c and b but fail on a.
func setupParentNotEmpty(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	path := filepath.Join(dir, "a", "keep.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// removalCases returns the table of isolated removal test cases.
// R4.2: single empty, multiple, -p nested, non-empty error,
// --ignore-fail-on-non-empty, -v verbose.
// R4.3: -p stops ascending when parent is not empty.
func removalCases() []isolatedCase {
	return []isolatedCase{
		{
			name:  "single_empty",
			args:  []string{"emptydir"},
			setup: setupEmptyDir,
		},
		{
			name:  "multiple_empty",
			args:  []string{"d1", "d2", "d3"},
			setup: setupMultipleDirs,
		},
		{
			name:  "parents_nested",
			args:  []string{"-p", "a/b/c"},
			setup: setupNestedEmpty,
		},
		{
			name:  "verbose_single",
			args:  []string{"-v", "emptydir"},
			setup: setupEmptyDir,
		},
		{
			name:  "verbose_parents",
			args:  []string{"-v", "-p", "a/b/c"},
			setup: setupNestedEmpty,
		},
		{
			name:  "parents_stops_at_nonempty",
			args:  []string{"-p", "a/b/c"},
			setup: setupParentNotEmpty,
		},
		{
			name: "parents_ignore_fail_stops_at_nonempty",
			args: []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"},
			setup: setupParentNotEmpty,
		},
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, and exit code.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
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
