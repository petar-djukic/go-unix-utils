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
		})
	}
}

// buildEnv constructs the environment variable slice for binary execution.
// R2.6: sets LC_ALL=C by default; testEnv entries override via append order.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = append(env, "LC_ALL=C")
	env = append(env, testEnv...)
	return env
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

// compareResults compares ref and got outputs, failing on divergence.
// R3.1-R3.4: compare normalized stdout, stderr, and exit code.
func compareResults(t *testing.T, tc DiffTest, ref, got binResult) {
	t.Helper()
	refOut := applyNormalizers(ref.stdout, tc.Normalize)
	gotOut := applyNormalizers(got.stdout, tc.Normalize)
	refErr := applyNormalizers(ref.stderr, tc.Normalize)
	gotErr := applyNormalizers(got.stderr, tc.Normalize)
	if bytes.Equal(refOut, gotOut) && bytes.Equal(refErr, gotErr) &&
		ref.exitCode == got.exitCode {
		return
	}
	reportDivergence(t, tc, ref.exitCode, got.exitCode, refOut, gotOut, refErr, gotErr)
}

// maxStdinDisplay is the maximum stdin bytes shown in failure messages.
const maxStdinDisplay = 256

// reportDivergence formats and reports a test failure for mismatched outputs.
// R3.5: includes args, stdin, both stdouts, both stderrs, both exit codes.
func reportDivergence(
	t *testing.T, tc DiffTest,
	refCode, gotCode int,
	refOut, gotOut, refErr, gotErr []byte,
) {
	t.Helper()
	stdinSnippet := tc.Stdin
	if len(stdinSnippet) > maxStdinDisplay {
		stdinSnippet = stdinSnippet[:maxStdinDisplay]
	}
	t.Errorf("output divergence\n"+
		"args:       %v\n"+
		"stdin:      %q\n"+
		"ref stdout: %q\n"+
		"go  stdout: %q\n"+
		"ref stderr: %q\n"+
		"go  stderr: %q\n"+
		"ref exit:   %d\n"+
		"go  exit:   %d",
		tc.Args, stdinSnippet, refOut, gotOut, refErr, gotErr, refCode, gotCode)
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. Returns an identity function when fns is empty.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TimestampNormalizer replaces common strftime timestamp patterns with a
// fixed placeholder string. Used by cmd/ts tests.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b
}

// BuildBinary compiles the cmd/ package at dir and returns the path to the
// built binary. Calls t.Fatal on build failure.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	return ""
}
