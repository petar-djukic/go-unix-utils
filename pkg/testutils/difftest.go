// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness that executes a Go binary
// and a GNU reference binary with identical inputs and compares their outputs.
//
// Implements: prd001-testutils (R1-R5)
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const defaultTimeout = 10 * time.Second

// NormalizeFunc transforms output bytes before comparison.
// Applied to both binary outputs to handle known acceptable differences.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case for comparing a Go binary
// against a GNU reference binary.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is the standard input sent to both binaries.
	// nil means both binaries receive EOF immediately.
	// An empty non-nil slice ([]byte{}) sends no bytes but opens stdin before closing.
	Stdin []byte
	// Env contains KEY=VALUE pairs merged into the inherited environment.
	// nil means use defaults only (LC_ALL=C). Non-nil entries override or extend
	// the inherited environment.
	Env []string
	// WorkDir is the working directory for both binaries.
	// Empty means use a per-test t.TempDir().
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize contains functions applied in order to stdout and stderr of both
	// binaries before comparison. nil or empty means no normalization.
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative paths to expected byte content after execution.
	// Used for file-output utilities (sponge, cp). nil means no file checks.
	ExpectedFiles map[string][]byte
	// Timeout overrides the default 10-second timeout per binary invocation.
	// Zero means use the default.
	Timeout time.Duration
}

// runResult holds captured output from a single binary execution.
type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests executes each DiffTest as a named subtest via t.Run. For each test
// case, it runs both goBinary and refBinary with identical Args, Stdin, and Env,
// then compares stdout, stderr, and exit code byte-for-byte after normalization.
//
// Implements: prd001-testutils R2, R3
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tt)
		})
	}
}

func runSingleDiffTest(t *testing.T, goBinary, refBinary string, dt DiffTest) {
	t.Helper()

	timeout := dt.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	workDir := dt.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(dt.Env)

	refResult, err := execBinary(refBinary, dt.Args, dt.Stdin, env, workDir, timeout)
	if err != nil {
		t.Fatalf("failed to execute reference binary %s: %v", refBinary, err)
	}

	goResult, err := execBinary(goBinary, dt.Args, dt.Stdin, env, workDir, timeout)
	if err != nil {
		t.Fatalf("failed to execute Go binary %s: %v", goBinary, err)
	}

	// Apply normalization to both outputs before comparison (R3.1)
	goStdout := applyNormalizers(goResult.stdout, dt.Normalize)
	goStderr := applyNormalizers(goResult.stderr, dt.Normalize)
	refStdout := applyNormalizers(refResult.stdout, dt.Normalize)
	refStderr := applyNormalizers(refResult.stderr, dt.Normalize)

	var failures []string

	if !bytes.Equal(goStdout, refStdout) {
		failures = append(failures, fmt.Sprintf("stdout differs:\n    reference: %q\n    go binary: %q", refStdout, goStdout))
	}

	if !bytes.Equal(goStderr, refStderr) {
		failures = append(failures, fmt.Sprintf("stderr differs:\n    reference: %q\n    go binary: %q", refStderr, goStderr))
	}

	if goResult.exitCode != refResult.exitCode {
		failures = append(failures, fmt.Sprintf("exit code differs: reference=%d go=%d", refResult.exitCode, goResult.exitCode))
	}

	// Check expected files after both binaries complete (R5.1, R5.2)
	if dt.ExpectedFiles != nil {
		for path, expected := range dt.ExpectedFiles {
			fullPath := filepath.Join(workDir, path)
			actual, err := os.ReadFile(fullPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("expected file %s: %v", path, err))
				continue
			}
			if !bytes.Equal(actual, expected) {
				failures = append(failures, fmt.Sprintf("file %s differs:\n    expected: %q\n    actual:   %q", path, expected, actual))
			}
		}
	}

	if len(failures) > 0 {
		stdinDisplay := dt.Stdin
		if len(stdinDisplay) > 256 {
			stdinDisplay = stdinDisplay[:256]
		}
		t.Fatalf("differential test failed\n"+
			"  args:       %v\n"+
			"  stdin:      %q\n"+
			"  ref stdout: %q\n"+
			"  go stdout:  %q\n"+
			"  ref stderr: %q\n"+
			"  go stderr:  %q\n"+
			"  ref exit:   %d\n"+
			"  go exit:    %d\n"+
			"  failures:\n    %s",
			dt.Args, stdinDisplay,
			refStdout, goStdout,
			refStderr, goStderr,
			refResult.exitCode, goResult.exitCode,
			strings.Join(failures, "\n    "))
	}
}

// buildEnv constructs the environment for binary execution. It starts with the
// current process environment, applies LC_ALL=C as default, then merges any
// user-specified KEY=VALUE pairs (R2.6).
func buildEnv(userEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")

	for _, entry := range userEnv {
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			env = setEnvVar(env, entry[:idx], entry[idx+1:])
		}
	}

	return env
}

// setEnvVar sets or overrides a KEY=VALUE pair in the environment slice.
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

// execBinary runs a binary with the given arguments, stdin, environment, and working
// directory, returning the captured stdout, stderr, and exit code.
func execBinary(binary string, args []string, stdin []byte, env []string, workDir string, timeout time.Duration) (*runResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("binary %s exceeded timeout of %v", binary, timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("executing %s: %w", binary, err)
		}
	}

	return &runResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}, nil
}

// applyNormalizers applies each normalizer function in order to the data.
func applyNormalizers(data []byte, normalizers []NormalizeFunc) []byte {
	for _, fn := range normalizers {
		data = fn(data)
	}
	return data
}

// Precompiled patterns for TimestampNormalizer.
var (
	// Mon DD HH:MM:SS (e.g., "Feb 19 12:34:56")
	tsPatternCtime = regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`)
	// YYYY-MM-DD HH:MM:SS (e.g., "2024-02-19 12:34:56")
	tsPatternISO = regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`)
	// HH:MM:SS (e.g., "12:34:56")
	tsPatternTime = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
)

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. Used by cmd/ts tests to handle wall-clock differences.
//
// Implements: prd001-testutils R4.2
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	placeholder := []byte(timestampPlaceholder)
	data = tsPatternCtime.ReplaceAll(data, placeholder)
	data = tsPatternISO.ReplaceAll(data, placeholder)
	data = tsPatternTime.ReplaceAll(data, placeholder)
	return data
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order.
//
// Implements: prd001-testutils R4.4
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}
