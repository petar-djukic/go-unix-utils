// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// defaultTimeout is the maximum duration for a single binary invocation.
// Per prd001-testutils R2.3.
const defaultTimeout = 10 * time.Second

// stdinTruncateLen is the maximum number of stdin bytes shown in failure messages.
// Per prd001-testutils R3.5.
const stdinTruncateLen = 256

// FileExpectation describes an expected file state after binary execution. This
// supports utilities whose primary output is a file rather than stdout (sponge,
// cp, mv).
//
// Per prd001-testutils R5.1.
type FileExpectation struct {
	// Path is the file path to check after execution. When relative, it is
	// resolved against the test working directory.
	Path string

	// Content is the expected byte-for-byte content of the file.
	Content []byte
}

// DiffTest defines a single differential test case. A cmd/ package defines a
// []DiffTest slice and calls RunDiffTests from a standard TestXxx function.
//
// Per prd001-testutils R1.1-R1.4.
type DiffTest struct {
	// Name identifies this test case in go test output via t.Run.
	// Per prd001-testutils R3.6.
	Name string

	// Args is the command-line arguments passed to both binaries.
	Args []string

	// Stdin is the bytes fed to both binaries on stdin. When nil, the binary
	// receives no stdin input (EOF immediately). Per prd001-testutils R1.2.
	Stdin []byte

	// Env is the environment variable overrides for both binaries. When nil,
	// both binaries inherit the test process environment. Per prd001-testutils R1.3.
	Env []string

	// Normalize is a slice of NormalizeFunc applied in order to both stdout and
	// stderr before comparison. When nil or empty, no normalization is applied.
	// Per prd001-testutils R1.4 and R4.3.
	Normalize []NormalizeFunc

	// Timeout overrides the default 10-second timeout for this test case.
	// When zero, the default timeout is used. Per prd001-testutils R2.3.
	Timeout time.Duration

	// WorkDir sets the working directory for both binary invocations. When
	// empty, a per-test temporary directory is used. Per prd001-testutils R2.5.
	WorkDir string

	// FileExpectations lists files whose content should be compared after
	// execution. Per prd001-testutils R5.1.
	FileExpectations []FileExpectation
}

// binResult holds the captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests executes each DiffTest by running both the Go binary and the GNU
// reference binary with identical inputs, then comparing their outputs. Each test
// case runs as a named subtest.
//
// Per prd001-testutils AC1: a cmd/ package calls RunDiffTests(t, goBinary,
// refBinary, tests) with no other setup.
func RunDiffTests(t *testing.T, goBinary string, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleDiffTest runs one DiffTest case, executing both binaries and comparing
// their outputs.
func runSingleDiffTest(t *testing.T, goBinary string, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	timeout := tc.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	// Execute both binaries. Per prd001-testutils R2.1.
	goResult, err := executeBinary(goBinary, tc.Args, tc.Stdin, tc.Env, workDir, timeout)
	if err != nil {
		t.Fatalf("executing Go binary %s: %v", goBinary, err)
	}

	refResult, err := executeBinary(refBinary, tc.Args, tc.Stdin, tc.Env, workDir, timeout)
	if err != nil {
		t.Fatalf("executing reference binary %s: %v", refBinary, err)
	}

	// Apply normalization. Per prd001-testutils R3.1.
	normalize := ComposeNormalizers(tc.Normalize)

	goStdout := normalize(goResult.stdout)
	goStderr := normalize(goResult.stderr)
	refStdout := normalize(refResult.stdout)
	refStderr := normalize(refResult.stderr)

	// Compare and report. Per prd001-testutils R3.2-R3.5.
	failed := false

	if !bytes.Equal(refStdout, goStdout) {
		failed = true
		t.Errorf("stdout divergence")
	}

	if !bytes.Equal(refStderr, goStderr) {
		failed = true
		t.Errorf("stderr divergence")
	}

	if refResult.exitCode != goResult.exitCode {
		failed = true
		t.Errorf("exit code divergence: reference=%d, go=%d", refResult.exitCode, goResult.exitCode)
	}

	if failed {
		t.Errorf("%s", formatDivergenceReport(tc, refResult, goResult, refStdout, goStdout, refStderr, goStderr))
	}

	// File-state comparison. Per prd001-testutils R5.1-R5.2.
	for _, fe := range tc.FileExpectations {
		filePath := fe.Path
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(workDir, filePath)
		}

		actual, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("reading expected file %s: %v", fe.Path, err)
			continue
		}

		if !bytes.Equal(fe.Content, actual) {
			t.Errorf("file content divergence for %s:\n  expected (%d bytes): %s\n  actual   (%d bytes): %s",
				fe.Path,
				len(fe.Content), truncateBytes(fe.Content, stdinTruncateLen),
				len(actual), truncateBytes(actual, stdinTruncateLen),
			)
		}
	}
}

// executeBinary runs a single binary with the given args, stdin, environment, and
// working directory under a timeout. It captures stdout, stderr, and exit code.
//
// Per prd001-testutils R2.1-R2.5.
func executeBinary(binary string, args []string, stdin []byte, env []string, workDir string, timeout time.Duration) (*binResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir

	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	// Check for timeout. Per prd001-testutils R2.3 and AC5.
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("binary %s exceeded %v timeout", binary, timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("running binary %s: %w", binary, err)
		}
	}

	return &binResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}, nil
}

// formatDivergenceReport builds the failure message with all context needed to
// diagnose the divergence.
//
// Per prd001-testutils R3.5.
func formatDivergenceReport(tc DiffTest, ref *binResult, goRes *binResult, refStdoutNorm []byte, goStdoutNorm []byte, refStderrNorm []byte, goStderrNorm []byte) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\n--- Divergence Report ---\n")
	fmt.Fprintf(&buf, "Args: %v\n", tc.Args)
	fmt.Fprintf(&buf, "Stdin (%d bytes): %s\n", len(tc.Stdin), truncateBytes(tc.Stdin, stdinTruncateLen))
	fmt.Fprintf(&buf, "Reference exit code: %d\n", ref.exitCode)
	fmt.Fprintf(&buf, "Go        exit code: %d\n", goRes.exitCode)
	fmt.Fprintf(&buf, "Reference stdout (normalized):\n%s\n", refStdoutNorm)
	fmt.Fprintf(&buf, "Go        stdout (normalized):\n%s\n", goStdoutNorm)
	fmt.Fprintf(&buf, "Reference stderr (normalized):\n%s\n", refStderrNorm)
	fmt.Fprintf(&buf, "Go        stderr (normalized):\n%s\n", goStderrNorm)
	return buf.String()
}

// truncateBytes returns a string representation of b, truncated to maxLen bytes
// with an ellipsis suffix if truncated.
func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}
