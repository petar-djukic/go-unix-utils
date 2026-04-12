// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkfifo against gmkfifo (GNU coreutils).
// Implements srd092 R2.1 (TestDiff with RunDiffTests), R2.2 (test cases),
// R2.3 (graceful skip and cleanup).
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

const refBinName = "gmkfifo"
const execTimeout = 30 * time.Second

// makeNormalizer creates a NormalizeFunc that replaces binary names and
// normalizes syscall error message capitalization between GNU and Go.
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
		{"File exists", "file exists"},
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/mkfifo against gmkfifo.
// R2.1: uses testutils.BuildBinary and testutils.RunDiffTests.
// R2.2: covers basic creation, -m mode, multiple args, error cases.
// R2.3: skips when gmkfifo is not in PATH.
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

// runErrorTests uses RunDiffTests for error cases where both binaries
// see the same filesystem state and neither creates a FIFO.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	workDir := t.TempDir()

	// Pre-create a regular file for the "existing path" error case.
	existingPath := filepath.Join(workDir, "existing")
	if err := os.WriteFile(existingPath, []byte{}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "missing_operand", Args: []string{},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "existing_path", Args: []string{"existing"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
		{
			Name: "missing_parent_dir", Args: []string{"no/such/dir/pipe"},
			WorkDir: workDir, ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a creation test that runs each binary in its own dir.
type isolatedCase struct {
	name       string
	args       []string
	checkFIFOs []string // relative paths whose mode to compare
}

// runCreationTests runs tests where each binary operates in an isolated
// temp directory so filesystem mutations do not interfere.
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

// creationCases returns the table of isolated creation test cases.
// R2.2: single FIFO, multiple FIFOs, -m mode flag.
func creationCases() []isolatedCase {
	return []isolatedCase{
		{
			name: "single_fifo", args: []string{"pipe1"},
			checkFIFOs: []string{"pipe1"},
		},
		{
			name: "multiple_fifos", args: []string{"p1", "p2", "p3"},
			checkFIFOs: []string{"p1", "p2", "p3"},
		},
		{
			name: "mode_0600", args: []string{"-m", "0600", "secure"},
			checkFIFOs: []string{"secure"},
		},
		{
			name: "mode_0644", args: []string{"-m", "0644", "readable"},
			checkFIFOs: []string{"readable"},
		},
		{
			name: "mode_equals", args: []string{"--mode=0700", "priv"},
			checkFIFOs: []string{"priv"},
		},
	}
}

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and FIFO permissions.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	refRes := runBin(t, refBin, tc.args, refDir)
	goRes := runBin(t, goBin, tc.args, goDir)

	compareOutputs(t, norm, refRes, goRes)
	compareFIFOPerms(t, refDir, goDir, tc.checkFIFOs)
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
			stdout: outBuf.Bytes(), stderr: errBuf.Bytes(),
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

// compareFIFOPerms verifies FIFO permissions match between ref and go dirs.
func compareFIFOPerms(t *testing.T, refDir, goDir string, checkFIFOs []string) {
	t.Helper()
	for _, rel := range checkFIFOs {
		comparePerm(t, rel, filepath.Join(refDir, rel), filepath.Join(goDir, rel))
	}
}

// comparePerm checks that a single FIFO exists in both trees with matching
// permission bits and that it is indeed a named pipe.
func comparePerm(t *testing.T, name, refPath, goPath string) {
	t.Helper()
	refInfo, err := os.Stat(refPath)
	if err != nil {
		t.Errorf("ref FIFO %s missing: %v", name, err)
		return
	}
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Errorf("go FIFO %s missing: %v", name, err)
		return
	}
	verifyIsFIFO(t, name, refInfo, goInfo)
	if refInfo.Mode().Perm() != goInfo.Mode().Perm() {
		t.Errorf("perm mismatch %s: ref=%o got=%o",
			name, refInfo.Mode().Perm(), goInfo.Mode().Perm())
	}
}

// verifyIsFIFO checks that both ref and go entries are named pipes.
func verifyIsFIFO(t *testing.T, name string, refInfo, goInfo os.FileInfo) {
	t.Helper()
	if refInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("ref %s is not a FIFO: mode=%v", name, refInfo.Mode())
	}
	if goInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("go %s is not a FIFO: mode=%v", name, goInfo.Mode())
	}
}
