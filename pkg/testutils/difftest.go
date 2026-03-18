// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R1.1 (DiffTest), R2.1–R2.6 (RunDiffTests),
// R3.1–R3.6 (comparison and reporting), R5.1–R5.2 (file-state comparison).
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

// defaultTimeout is the per-binary execution timeout. Implements R2.3.
const defaultTimeout = 10 * time.Second

// maxStdinDisplay is the maximum bytes of stdin shown in failure messages.
// Implements R3.5 (truncated to 256 bytes).
const maxStdinDisplay = 256

// DiffTest represents a single differential test case comparing a Go binary
// against a GNU reference binary. Implements prd001-testutils R1.1.
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = EOF immediately; empty non-nil = open then close
	Env           []string          // nil = defaults only (LC_ALL=C); non-nil = KEY=VALUE merged
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout/stderr before comparison
	ExpectedFiles map[string][]byte // path -> expected content after execution
}

// binaryResult holds captured output from a single binary execution.
type binaryResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// RunDiffTests executes each DiffTest against both binaries and compares
// stdout, stderr, and exit code. Each test runs as a named subtest.
// Implements prd001-testutils R2.1, R3.6.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleTest executes one DiffTest case against both binaries and
// compares their outputs. Implements R2.5 (WorkDir).
func runSingleTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()
	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}
	env := buildEnv(tc.Env)
	refResult := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
	goResult := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)
	compareOutputs(t, tc, refResult, goResult)
	checkExpectedFiles(t, tc, workDir)
}

// runBinary executes a binary with the given args, stdin, env, and working
// directory, capturing stdout, stderr, and exit code. Fails the test on
// timeout (R2.3) or execution errors.
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) binaryResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return handleRunResult(t, binary, err, ctx, stdout.Bytes(), stderr.Bytes())
}

// handleRunResult extracts exit code from cmd.Run result and handles
// timeout and unexpected errors.
func handleRunResult(t *testing.T, binary string, err error, ctx context.Context, stdout, stderr []byte) binaryResult {
	t.Helper()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v", binary, defaultTimeout)
	}
	if err == nil {
		return binaryResult{Stdout: stdout, Stderr: stderr, ExitCode: 0}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return binaryResult{Stdout: stdout, Stderr: stderr, ExitCode: exitErr.ExitCode()}
	}
	t.Fatalf("binary %s failed to execute: %v", binary, err)
	return binaryResult{} // unreachable
}

// buildEnv constructs the environment for binary execution. Sets LC_ALL=C
// by default, then merges DiffTest.Env overrides. Implements R2.6.
func buildEnv(overrides []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")
	for _, override := range overrides {
		parts := strings.SplitN(override, "=", 2)
		if len(parts) == 2 {
			env = setEnvVar(env, parts[0], parts[1])
		}
	}
	return env
}

// setEnvVar sets or replaces an environment variable in the slice.
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

// compareOutputs compares stdout, stderr, and exit code between reference
// and Go binary results. Reports all divergence info on failure.
// Implements R3.1–R3.5.
func compareOutputs(t *testing.T, tc DiffTest, ref, got binaryResult) {
	t.Helper()
	refStdout := applyNormalizers(ref.Stdout, tc.Normalize)
	gotStdout := applyNormalizers(got.Stdout, tc.Normalize)
	refStderr := applyNormalizers(ref.Stderr, tc.Normalize)
	gotStderr := applyNormalizers(got.Stderr, tc.Normalize)
	stdoutMatch := bytes.Equal(refStdout, gotStdout)
	stderrMatch := bytes.Equal(refStderr, gotStderr)
	exitMatch := ref.ExitCode == got.ExitCode
	if stdoutMatch && stderrMatch && exitMatch {
		return
	}
	t.Fatalf("%s", formatDivergence(tc, refStdout, gotStdout, refStderr, gotStderr, ref.ExitCode, got.ExitCode))
}

// formatDivergence builds the failure message for a divergence between
// reference and Go binary outputs. Implements R3.5.
func formatDivergence(tc DiffTest, refOut, goOut, refErr, goErr []byte, refExit, goExit int) string {
	return fmt.Sprintf("divergence detected\n"+
		"args:       %s\n"+
		"stdin:      %s\n"+
		"ref stdout: %s\n"+
		"go  stdout: %s\n"+
		"ref stderr: %s\n"+
		"go  stderr: %s\n"+
		"ref exit:   %d\n"+
		"go  exit:   %d",
		strings.Join(tc.Args, " "), truncateStdin(tc.Stdin),
		refOut, goOut, refErr, goErr, refExit, goExit)
}

// truncateStdin returns stdin as a string, truncated to maxStdinDisplay bytes.
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) <= maxStdinDisplay {
		return string(stdin)
	}
	return string(stdin[:maxStdinDisplay]) + "...(truncated)"
}

// checkExpectedFiles verifies that files produced by the Go binary match
// the expected content. Implements R5.1–R5.2.
func checkExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()
	if tc.ExpectedFiles == nil {
		return
	}
	for path, expected := range tc.ExpectedFiles {
		fullPath := filepath.Join(workDir, path)
		actual, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
		if !bytes.Equal(expected, actual) {
			t.Fatalf("file %s divergence\nexpected: %q\nactual:   %q",
				path, expected, actual)
		}
	}
}
