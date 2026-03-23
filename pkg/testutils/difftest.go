// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for comparing
// Go utility binaries against GNU reference binaries.
//
// Implements prd001-testutils R1.1–R1.4 (types), R2.1–R2.6 (execution),
// R3.1–R3.6 (comparison), R5.1–R5.2 (file state).
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the per-binary invocation timeout.
// R2.3: default timeout is 10 seconds.
const defaultTimeout = 10 * time.Second

// maxStdinReport is the maximum stdin bytes shown in failure messages.
// R3.5: truncate stdin to 256 bytes.
const maxStdinReport = 256

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: type alias for output normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case that runs a Go binary and
// a reference binary with identical inputs and compares their outputs.
//
// R1.1: all fields match the prd001-testutils contract.
type DiffTest struct {
	Name          string
	Args          []string
	Stdin         []byte
	Env           []string
	WorkDir       string
	ExitCode      int
	Normalize     []NormalizeFunc
	ExpectedFiles map[string][]byte
}

// binResult holds the captured output of a single binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests executes each DiffTest against both goBinary and refBinary,
// comparing stdout, stderr, and exit code.
//
// R2.1: runs each DiffTest as a named subtest via t.Run.
// R2.6: sets LC_ALL=C by default unless overridden by DiffTest.Env.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleDiffTest executes one DiffTest and compares results.
func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()
	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}
	env := buildEnv(tc.Env)

	refRes := runBinary(t, refBinary, tc, workDir, env)
	goRes := runBinary(t, goBinary, tc, workDir, env)

	goStdout, goStderr := applyNormalizers(goRes, tc.Normalize)
	refStdout, refStderr := applyNormalizers(refRes, tc.Normalize)

	compareOutputs(t, tc, refStdout, goStdout, refStderr, goStderr, refRes.exitCode, goRes.exitCode)
	checkExpectedFiles(t, tc, workDir)
}

// buildEnv constructs the environment for binary invocations.
// R2.6: LC_ALL=C is set by default unless DiffTest.Env overrides it.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	// R2.6: apply LC_ALL=C as default
	env = setEnvVar(env, "LC_ALL", "C")
	// Merge test-specific env vars (overrides defaults)
	for _, e := range testEnv {
		if idx := strings.IndexByte(e, '='); idx > 0 {
			env = setEnvVar(env, e[:idx], e[idx+1:])
		}
	}
	return env
}

// setEnvVar sets or replaces a variable in an env slice.
func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// runBinary executes a single binary and captures its output.
// R2.2: captures stdout, stderr, and exit code independently.
// R2.3: imposes a 10-second timeout.
func runBinary(t *testing.T, binary string, tc DiffTest, workDir string, env []string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, tc.Args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if tc.Stdin != nil {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s exceeded %v timeout", binary, defaultTimeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}

	return binResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

// applyNormalizers runs the normalize chain on stdout and stderr.
// R3.1: applies normalizers to both binaries' output before comparison.
func applyNormalizers(res binResult, norms []NormalizeFunc) ([]byte, []byte) {
	stdout := res.stdout
	stderr := res.stderr
	for _, fn := range norms {
		stdout = fn(stdout)
		stderr = fn(stderr)
	}
	return stdout, stderr
}

// compareOutputs checks stdout, stderr, and exit code and reports failures.
// R3.2–R3.5: byte-for-byte comparison with detailed failure reporting.
func compareOutputs(
	t *testing.T, tc DiffTest,
	refStdout, goStdout, refStderr, goStderr []byte,
	refExit, goExit int,
) {
	t.Helper()
	stdoutMatch := bytes.Equal(refStdout, goStdout)
	stderrMatch := bytes.Equal(refStderr, goStderr)
	exitMatch := refExit == goExit

	if stdoutMatch && stderrMatch && exitMatch {
		return
	}
	// R3.5: report with args, stdin, both outputs, both exit codes
	t.Errorf("divergence detected\n%s",
		formatDivergence(tc, refStdout, goStdout, refStderr, goStderr, refExit, goExit))
}

// formatDivergence builds the failure message for a divergent test.
// R3.5: includes args, stdin (truncated), both stdout/stderr, exit codes.
func formatDivergence(
	tc DiffTest,
	refStdout, goStdout, refStderr, goStderr []byte,
	refExit, goExit int,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  args: %v\n", tc.Args)
	fmt.Fprintf(&b, "  stdin: %s\n", truncateBytes(tc.Stdin, maxStdinReport))
	fmt.Fprintf(&b, "  ref stdout: %q\n", refStdout)
	fmt.Fprintf(&b, "  go  stdout: %q\n", goStdout)
	fmt.Fprintf(&b, "  ref stderr: %q\n", refStderr)
	fmt.Fprintf(&b, "  go  stderr: %q\n", goStderr)
	fmt.Fprintf(&b, "  ref exit: %d\n", refExit)
	fmt.Fprintf(&b, "  go  exit: %d\n", goExit)
	return b.String()
}

// truncateBytes returns a display string for stdin, truncated to maxLen.
func truncateBytes(b []byte, maxLen int) string {
	if b == nil {
		return "<nil>"
	}
	if len(b) <= maxLen {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q... (%d bytes total)", b[:maxLen], len(b))
}

// checkExpectedFiles verifies file-state expectations after execution.
// R5.1–R5.2: compare each ExpectedFiles entry byte-for-byte.
func checkExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()
	for relPath, expected := range tc.ExpectedFiles {
		fullPath := filepath.Join(workDir, relPath)
		actual, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("expected file %s: %v", relPath, err)
			continue
		}
		if !bytes.Equal(expected, actual) {
			t.Errorf("file %s divergence\n  expected: %q\n  actual:   %q",
				relPath, expected, actual)
		}
	}
}
