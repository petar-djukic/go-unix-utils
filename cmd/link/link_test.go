// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/link against glink (GNU coreutils).
// Implements srd084 R1.1-R1.4, R2.1-R2.3.
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

const refBinName = "glink"
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
		{"File exists", "file exists"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
		{"Cross-device link", "cross-device link"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/link against glink.
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
	t.Run("creation", func(t *testing.T) {
		t.Parallel()
		runCreationTests(t, goBin, refBin, norm)
	})
}

// runErrorTests uses RunDiffTests for error cases where neither binary
// mutates the filesystem state.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	workDir := t.TempDir()

	// Create a file for "already exists" test.
	for _, name := range []string{"existing1", "existing2"} {
		path := filepath.Join(workDir, name)
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "zero_args", Args: []string{},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "one_arg", Args: []string{"existing1"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "three_args", Args: []string{"a", "b", "c"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "nonexistent_source", Args: []string{"doesnotexist", "newlink"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "target_exists", Args: []string{"existing1", "existing2"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a creation test that runs each binary in its own dir.
type isolatedCase struct {
	name  string
	args  []string
	setup func(t *testing.T, dir string)
}

// runCreationTests runs tests where each binary operates in an isolated
// temp directory since link creates files.
func runCreationTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := creationCases()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, norm, tc)
		})
	}
}

// setupSourceFile creates a regular file named "source" in dir.
func setupSourceFile(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "source")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// creationCases returns the table of isolated link creation test cases.
func creationCases() []isolatedCase {
	return []isolatedCase{
		{
			name:  "regular_file",
			args:  []string{"source", "dest"},
			setup: setupSourceFile,
		},
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and verifies hard link creation.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)

	// Verify hard link was created and shares inode with source.
	verifyHardLink(t, goDir, tc.args[0], tc.args[1])
}

// verifyHardLink checks that dest is a hard link to source (same inode).
func verifyHardLink(t *testing.T, dir, source, dest string) {
	t.Helper()
	srcInfo, err := os.Stat(filepath.Join(dir, source))
	if err != nil {
		t.Fatalf("source stat failed: %v", err)
	}
	dstInfo, err := os.Stat(filepath.Join(dir, dest))
	if err != nil {
		t.Fatalf("dest stat failed: %v", err)
	}
	if !os.SameFile(srcInfo, dstInfo) {
		t.Errorf("dest is not a hard link to source")
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
