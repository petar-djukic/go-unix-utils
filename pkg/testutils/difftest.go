// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// difftest.go defines the DiffTest struct and RunDiffTests entry point for the
// differential testing harness.
// Implements prd001-testutils R1.1-R1.4, R2.1-R2.6, R3.6, R4.1-R4.2.

package testutils

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the maximum duration each binary invocation may run.
// R2.3: configurable default of 10 seconds.
const defaultTimeout = 10 * time.Second

// DiffTest defines a single differential test case.
// R1.1: all fields match the prd001-testutils contract.
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (EOF); empty = open stdin closed immediately
	Env           []string          // nil = defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout/stderr before comparison
	ExpectedFiles map[string][]byte // path -> expected content after execution
}

// runResult holds captured output from a single binary execution.
type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing both binaries
// and comparing their stdout, stderr, and exit code.
// R2.1: accepts Go binary and reference binary paths.
// R3.6: each test case runs as a named subtest via t.Run.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleTest executes both binaries and compares their outputs.
// R2.5: uses WorkDir when set, otherwise t.TempDir().
func runSingleTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()
	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}
	env := buildEnv(tc.Env)
	refResult := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
	goResult := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)
	refStdout, goStdout := applyNormalizers(tc.Normalize, refResult.stdout, goResult.stdout)
	refStderr, goStderr := applyNormalizers(tc.Normalize, refResult.stderr, goResult.stderr)
	compareResults(t, tc, refStdout, goStdout, refStderr, goStderr, refResult.exitCode, goResult.exitCode)
	checkExpectedFiles(t, tc, workDir)
}

// buildEnv constructs the environment for binary execution.
// R2.6: sets LC_ALL=C by default, then merges DiffTest.Env overrides.
// R4.1: propagates DiffTest.Env entries to both binaries.
// D3: starts with a minimal base (LC_ALL=C) without inheriting parent env.
func buildEnv(testEnv []string) []string {
	env := []string{"LC_ALL=C"}
	for _, kv := range testEnv {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env = setEnvVar(env, k, v)
		}
	}
	return env
}

// setEnvVar sets or overrides a key=value pair in an environment slice.
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

// runBinary executes a binary and captures its output.
// R2.2: captures stdout, stderr, and exit code independently.
// R2.3: enforces defaultTimeout.
// R4.2: always sets stdin to a reader; nil Stdin yields an empty reader.
func runBinary(
	t *testing.T, binary string, args []string,
	stdin []byte, env []string, workDir string,
) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = workDir
	// D4: each binary gets its own reader instance; nil stdin → empty reader.
	cmd.Stdin = bytes.NewReader(stdin)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return handleRunResult(t, binary, err, ctx, &stdoutBuf, &stderrBuf)
}

// handleRunResult extracts the exit code from a completed command.
func handleRunResult(
	t *testing.T, binary string, err error,
	ctx context.Context, stdoutBuf, stderrBuf *bytes.Buffer,
) runResult {
	t.Helper()
	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("binary %s timed out after %v", binary, defaultTimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}
	return runResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}
}

// applyNormalizers runs all normalizers on both reference and Go output.
func applyNormalizers(fns []NormalizeFunc, ref, got []byte) ([]byte, []byte) {
	for _, fn := range fns {
		ref = fn(ref)
		got = fn(got)
	}
	return ref, got
}
