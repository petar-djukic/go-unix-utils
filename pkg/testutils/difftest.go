// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for comparing
// Go binary output against GNU reference binaries. Implements srd001-testutils.
package testutils

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// NormalizeFunc transforms raw output bytes before comparison.
// Applied to stdout and stderr of both binaries to strip non-deterministic
// content (timestamps, PIDs, etc.) before byte-for-byte comparison.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case comparing a Go binary
// against a GNU reference binary with identical inputs.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is piped to both binaries. nil = EOF immediately.
	Stdin []byte
	// Env is KEY=VALUE pairs merged into the inherited environment.
	// nil = use defaults only (LC_ALL=C).
	Env []string
	// WorkDir sets the working directory for both binaries.
	// Empty = per-test t.TempDir().
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize is applied in order to stdout and stderr of both
	// binaries before comparison. nil or empty = no normalization.
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative paths to expected byte content
	// after execution, for file-output utilities (sponge, cp).
	ExpectedFiles map[string][]byte
}

// defaultTimeout is the maximum duration each binary invocation is allowed.
const defaultTimeout = 10 * time.Second

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary
// and refBinary with identical inputs and comparing their outputs.
// R1.3: iterates tests, runs t.Run for each, compares stdout/stderr/exit code.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			workDir := tc.WorkDir
			if workDir == "" {
				workDir = t.TempDir()
			}
			env := buildEnv(tc.Env)
			ref := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			got := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)
			compareResults(t, tc, ref, got)
			verifyExpectedFiles(t, tc, workDir)
		})
	}
}

// buildEnv constructs the environment variable slice for binary execution.
// R2.5/R2.6: when testEnv is nil, inherit os.Environ() with LC_ALL=C prepended.
// When testEnv is non-nil, prepend LC_ALL=C to the provided slice only.
func buildEnv(testEnv []string) []string {
	if testEnv == nil {
		env := append([]string{"LC_ALL=C"}, os.Environ()...)
		return env
	}
	return append([]string{"LC_ALL=C"}, testEnv...)
}

// runBinary executes a binary and captures its stdout, stderr, and exit code.
// R1.4: uses os/exec.Command with Stdin, Env, Dir, and bytes.Buffer capture.
func runBinary(t *testing.T, bin string, args []string, stdin []byte, env []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Env = env
	cmd.Dir = workDir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	exitCode := executeAndExtractCode(t, cmd, ctx)
	return binResult{stdout: outBuf.Bytes(), stderr: errBuf.Bytes(), exitCode: exitCode}
}

// executeAndExtractCode runs the command and extracts the exit code.
// R1.4: nil error = code 0; *exec.ExitError = ExitCode(); other = fatal.
func executeAndExtractCode(t *testing.T, cmd *exec.Cmd, ctx context.Context) int {
	t.Helper()
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v", cmd.Path, defaultTimeout)
	}
	t.Fatalf("binary %s failed to execute: %v", cmd.Path, err)
	return -1 // unreachable
}

// applyNormalizers runs each NormalizeFunc on the data in order.
// D4: pass-through when slice is nil or empty.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// compareResults compares ref and got outputs across all dimensions.
// R2.1: stdout comparison after normalizers.
// R2.2: stderr comparison after normalizers (independent of stdout).
// R2.3: exit code comparison with sanity check.
// R2.4: uses t.Errorf so all mismatches are reported, not just the first.
func compareResults(t *testing.T, tc DiffTest, ref, got binResult) {
	t.Helper()
	refOut := applyNormalizers(ref.stdout, tc.Normalize)
	gotOut := applyNormalizers(got.stdout, tc.Normalize)
	refErr := applyNormalizers(ref.stderr, tc.Normalize)
	gotErr := applyNormalizers(got.stderr, tc.Normalize)
	compareStdout(t, tc, refOut, gotOut)
	compareStderr(t, tc, refErr, gotErr)
	compareExitCode(t, tc, ref.exitCode, got.exitCode)
}

// compareStdout reports stdout divergence between reference and Go binary.
// R2.1: includes test name, args, and both expected/actual with lengths.
func compareStdout(t *testing.T, tc DiffTest, refOut, gotOut []byte) {
	t.Helper()
	if bytes.Equal(refOut, gotOut) {
		return
	}
	t.Errorf("stdout mismatch\n"+
		"args:            %v\n"+
		"expected (ref):  %q (len=%d)\n"+
		"actual   (go):   %q (len=%d)",
		tc.Args, refOut, len(refOut), gotOut, len(gotOut))
}

// compareStderr reports stderr divergence between reference and Go binary.
// R2.2: independent of stdout comparison; both are always checked.
func compareStderr(t *testing.T, tc DiffTest, refErr, gotErr []byte) {
	t.Helper()
	if bytes.Equal(refErr, gotErr) {
		return
	}
	t.Errorf("stderr mismatch\n"+
		"args:            %v\n"+
		"expected (ref):  %q (len=%d)\n"+
		"actual   (go):   %q (len=%d)",
		tc.Args, refErr, len(refErr), gotErr, len(gotErr))
}

// compareExitCode reports exit code divergence and sanity-checks the reference.
// R2.3: when DiffTest.ExitCode is non-zero, verifies the reference binary
// actually returned that code.
func compareExitCode(t *testing.T, tc DiffTest, refCode, gotCode int) {
	t.Helper()
	if tc.ExitCode != 0 && refCode != tc.ExitCode {
		t.Errorf("exit code sanity check failed\n"+
			"args:              %v\n"+
			"expected (DiffTest): %d\n"+
			"actual   (ref):      %d",
			tc.Args, tc.ExitCode, refCode)
	}
	if refCode == gotCode {
		return
	}
	t.Errorf("exit code mismatch\n"+
		"args:            %v\n"+
		"expected (ref):  %d\n"+
		"actual   (go):   %d",
		tc.Args, refCode, gotCode)
}

// verifyExpectedFiles checks that files written by the Go binary match the
// expected content from DiffTest.ExpectedFiles.
// R3.1/R3.2: paths are relative to workDir; reports missing files and content mismatches.
func verifyExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()
	if tc.ExpectedFiles == nil {
		return
	}
	for relPath, expected := range tc.ExpectedFiles {
		fullPath := filepath.Join(workDir, relPath)
		verifyOneFile(t, tc, relPath, fullPath, expected)
	}
}

// verifyOneFile checks a single expected file for existence and content match.
// R3.2: reports missing file path or first differing byte position.
func verifyOneFile(t *testing.T, tc DiffTest, relPath, fullPath string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(fullPath)
	if err != nil {
		t.Errorf("expected file missing\n"+
			"args: %v\n"+
			"path: %s",
			tc.Args, relPath)
		return
	}
	if bytes.Equal(expected, actual) {
		return
	}
	pos := firstDiffPos(expected, actual)
	t.Errorf("expected file content mismatch\n"+
		"args:              %v\n"+
		"path:              %s\n"+
		"expected (len):    %d\n"+
		"actual   (len):    %d\n"+
		"first diff at byte: %d",
		tc.Args, relPath, len(expected), len(actual), pos)
}

// firstDiffPos returns the index of the first byte that differs between a and b.
func firstDiffPos(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}