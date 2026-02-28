// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for go-unix-utils.
//
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code. Divergences are reported as test
// failures via the standard testing package.
//
// Implements: prd001-testutils R1, R2, R3, R4, R5.
package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the maximum time allowed for each binary invocation
// (prd001-testutils R2.3).
const defaultTimeout = 10 * time.Second

// maxStdinReport is the maximum number of stdin bytes shown in failure messages
// (prd001-testutils R3.5).
const maxStdinReport = 256

// timestampPlaceholder is the fixed string that replaces timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// NormalizeFunc transforms raw output bytes before comparison.
// Applied to both reference and Go binary output (prd001-testutils R1.4).
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case run against two binaries.
// See prd001-testutils R1 for field semantics.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is piped to both binaries. nil means both receive EOF immediately.
	// An empty non-nil slice ([]byte{}) also produces no bytes but is
	// semantically distinct (open stdin that is immediately closed).
	Stdin []byte
	// Env holds KEY=VALUE pairs merged into the inherited environment.
	// nil means use defaults only (LC_ALL=C); non-nil pairs override or
	// extend the inherited environment.
	Env []string
	// WorkDir is the working directory for both binaries. Empty means a
	// per-test t.TempDir() is used.
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize functions are applied in order to stdout and stderr of both
	// binaries before comparison. nil or empty means no normalization.
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative paths to expected byte content after
	// execution. Used for file-output utilities (sponge, cp).
	ExpectedFiles map[string][]byte
}

// RunDiffTests executes each DiffTest as a named subtest, invoking both
// goBinary and refBinary with identical inputs and comparing their outputs.
// See prd001-testutils R2, R3.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order (prd001-testutils R4.4).
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPattern matches common strftime timestamp formats:
//   - "Mon DD HH:MM:SS" (e.g., "Feb 19 12:34:56")
//   - "YYYY-MM-DD HH:MM:SS" (e.g., "2026-02-19 12:34:56")
//   - "HH:MM:SS" (e.g., "12:34:56")
var timestampPattern = regexp.MustCompile(
	`(?:` +
		`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		`\d{2}:\d{2}:\d{2}` +
		`)`,
)

// TimestampNormalizer replaces common strftime-formatted timestamps with a
// fixed placeholder string (prd001-testutils R4.2).
func TimestampNormalizer(data []byte) []byte {
	return timestampPattern.ReplaceAll(data, []byte(timestampPlaceholder))
}

// runSingleDiffTest runs one DiffTest case against both binaries and reports
// divergence.
func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(tc.Env)

	refStdout, refStderr, refExit, err := runBinary(refBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("reference binary failed to execute: %v", err)
	}

	goStdout, goStderr, goExit, err := runBinary(goBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("Go binary failed to execute: %v", err)
	}

	// Apply normalization to both outputs (prd001-testutils R3.1, R4).
	normRefStdout := applyNormalizers(refStdout, tc.Normalize)
	normRefStderr := applyNormalizers(refStderr, tc.Normalize)
	normGoStdout := applyNormalizers(goStdout, tc.Normalize)
	normGoStderr := applyNormalizers(goStderr, tc.Normalize)

	failed := !bytes.Equal(normRefStdout, normGoStdout) ||
		!bytes.Equal(normRefStderr, normGoStderr) ||
		refExit != goExit

	if failed {
		t.Fatalf("differential test divergence\n"+
			"args:       %v\n"+
			"stdin:      %s\n"+
			"ref stdout: %s\n"+
			"go  stdout: %s\n"+
			"ref stderr: %s\n"+
			"go  stderr: %s\n"+
			"ref exit:   %d\n"+
			"go  exit:   %d",
			tc.Args,
			truncateStdin(tc.Stdin),
			normRefStdout,
			normGoStdout,
			normRefStderr,
			normGoStderr,
			refExit,
			goExit,
		)
	}

	// File-state comparison (prd001-testutils R5).
	checkExpectedFiles(t, tc, workDir)
}

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory. Returns stdout, stderr, exit code, and any execution error.
func runBinary(binary string, args []string, stdin []byte, env []string, workDir string) (stdout, stderr []byte, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	runErr := cmd.Run()
	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout, stderr, -1, fmt.Errorf("binary %s timed out after %v", binary, defaultTimeout)
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		return stdout, stderr, -1, fmt.Errorf("executing %s: %w", binary, runErr)
	}

	return stdout, stderr, 0, nil
}

// buildEnv constructs the environment for binary invocations. It inherits the
// test process environment, sets LC_ALL=C as default, then merges any
// DiffTest.Env overrides (prd001-testutils R2.6).
func buildEnv(testEnv []string) []string {
	env := os.Environ()

	// Set LC_ALL=C as default (prd001-testutils R2.6).
	env = setEnvVar(env, "LC_ALL", "C")

	// Merge test-specific overrides.
	for _, kv := range testEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			env = setEnvVar(env, parts[0], parts[1])
		}
	}

	return env
}

// setEnvVar sets or replaces an environment variable in a slice.
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

// applyNormalizers applies each NormalizeFunc in order to the input bytes.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// checkExpectedFiles verifies file-state after binary execution
// (prd001-testutils R5).
func checkExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()

	for relPath, expected := range tc.ExpectedFiles {
		absPath := filepath.Join(workDir, relPath)
		actual, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("expected file %s: %v", relPath, err)
		}
		if !bytes.Equal(expected, actual) {
			t.Fatalf("file content divergence for %s\n"+
				"expected: %s\n"+
				"actual:   %s",
				relPath, expected, actual)
		}
	}
}

// truncateStdin returns a display-safe representation of stdin, truncated to
// maxStdinReport bytes if longer (prd001-testutils R3.5).
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) > maxStdinReport {
		return fmt.Sprintf("%s... (%d bytes total)", stdin[:maxStdinReport], len(stdin))
	}
	return string(stdin)
}
