// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
//
// Implements prd001-testutils R1-R4: DiffTest struct, binary execution,
// comparison and reporting, and normalization hooks.
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the per-binary execution timeout. Per prd001-testutils R2.3,
// the default is 10 seconds.
const defaultTimeout = 10 * time.Second

// stdinTruncateLen is the maximum number of stdin bytes shown in failure messages.
// Per prd001-testutils R3.5, stdin is truncated to 256 bytes.
const stdinTruncateLen = 256

// Normalize is a function that transforms raw output bytes before comparison.
// Per prd001-testutils R1.4, when nil no normalization is applied.
type Normalize func([]byte) []byte

// DiffTest defines a single differential test case. Per prd001-testutils R1.1-R1.4.
type DiffTest struct {
	// Name is used as the subtest name in t.Run. Per prd001-testutils R3.6.
	Name string

	// Args are the command-line arguments passed to both binaries.
	Args []string

	// Stdin is the input fed to both binaries. Nil means no stdin (immediate EOF).
	// Per prd001-testutils R1.2.
	Stdin []byte

	// Env specifies environment variable overrides. Nil means inherit the test
	// process environment. Per prd001-testutils R1.3.
	Env []string

	// ExitCode is the expected exit code from both binaries.
	ExitCode int

	// WorkDir is the working directory for both binaries. Empty means use a
	// per-test temporary directory. Per prd001-testutils R2.5.
	WorkDir string

	// Normalize is a slice of normalization functions applied in order to both
	// binaries' stdout and stderr before comparison. Nil or empty means no
	// normalization. Per prd001-testutils R4.3.
	Normalize []Normalize
}

// RunDiffTests executes each DiffTest as a named subtest, running goBinary and
// refBinary with identical inputs and comparing their outputs.
// Per prd001-testutils R2.1, R3.6, and AC1.
func RunDiffTests(t *testing.T, goBinary string, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleDiffTest executes a single DiffTest case.
func runSingleDiffTest(t *testing.T, goBinary string, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	goStdout, goStderr, goExit, goErr := runBinary(goBinary, tc.Args, tc.Stdin, tc.Env, workDir)
	refStdout, refStderr, refExit, refErr := runBinary(refBinary, tc.Args, tc.Stdin, tc.Env, workDir)

	if goErr != nil {
		t.Fatalf("Go binary execution failed: %v", goErr)
	}
	if refErr != nil {
		t.Fatalf("reference binary execution failed: %v", refErr)
	}

	goStdoutNorm := applyNormalizers(goStdout, tc.Normalize)
	goStderrNorm := applyNormalizers(goStderr, tc.Normalize)
	refStdoutNorm := applyNormalizers(refStdout, tc.Normalize)
	refStderrNorm := applyNormalizers(refStderr, tc.Normalize)

	stdoutMatch := bytes.Equal(goStdoutNorm, refStdoutNorm)
	stderrMatch := bytes.Equal(goStderrNorm, refStderrNorm)
	exitMatch := goExit == refExit

	if stdoutMatch && stderrMatch && exitMatch {
		return
	}

	reportDivergence(t, tc, refStdout, goStdout, refStderr, goStderr, refExit, goExit,
		refStdoutNorm, goStdoutNorm, refStderrNorm, goStderrNorm)
}

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory. It returns stdout, stderr, exit code, and any execution error.
// A timeout of defaultTimeout (10 seconds) is imposed per prd001-testutils R2.3.
func runBinary(binary string, args []string, stdin []byte, env []string, workDir string) (stdout []byte, stderr []byte, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir

	if env != nil {
		cmd.Env = env
	}

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()

	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		return stdout, stderr, -1, fmt.Errorf("binary %s timed out after %s", binary, defaultTimeout)
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, exitErr.ExitCode(), nil
		}
		return stdout, stderr, -1, fmt.Errorf("executing %s: %w", binary, runErr)
	}

	return stdout, stderr, 0, nil
}

// applyNormalizers applies a slice of Normalize functions in order. When the
// slice is nil or empty, returns the input unchanged. Per prd001-testutils R3.1.
func applyNormalizers(data []byte, normalizers []Normalize) []byte {
	for _, fn := range normalizers {
		if fn != nil {
			data = fn(data)
		}
	}
	return data
}

// reportDivergence reports a test failure with full context per prd001-testutils R3.5.
func reportDivergence(t *testing.T, tc DiffTest,
	refStdout, goStdout, refStderr, goStderr []byte,
	refExit, goExit int,
	refStdoutNorm, goStdoutNorm, refStderrNorm, goStderrNorm []byte) {
	t.Helper()

	var b strings.Builder
	b.WriteString("differential test divergence\n")
	fmt.Fprintf(&b, "  args:            %v\n", tc.Args)
	fmt.Fprintf(&b, "  stdin:           %s\n", truncateForDisplay(tc.Stdin))
	fmt.Fprintf(&b, "  ref exit code:   %d\n", refExit)
	fmt.Fprintf(&b, "  go  exit code:   %d\n", goExit)
	fmt.Fprintf(&b, "  ref stdout:      %q\n", refStdout)
	fmt.Fprintf(&b, "  go  stdout:      %q\n", goStdout)
	fmt.Fprintf(&b, "  ref stderr:      %q\n", refStderr)
	fmt.Fprintf(&b, "  go  stderr:      %q\n", goStderr)

	if !bytes.Equal(refStdoutNorm, goStdoutNorm) {
		fmt.Fprintf(&b, "  ref stdout (normalized): %q\n", refStdoutNorm)
		fmt.Fprintf(&b, "  go  stdout (normalized): %q\n", goStdoutNorm)
	}
	if !bytes.Equal(refStderrNorm, goStderrNorm) {
		fmt.Fprintf(&b, "  ref stderr (normalized): %q\n", refStderrNorm)
		fmt.Fprintf(&b, "  go  stderr (normalized): %q\n", goStderrNorm)
	}

	t.Fatal(b.String())
}

// truncateForDisplay returns a display-friendly representation of stdin content,
// truncating to stdinTruncateLen bytes. Per prd001-testutils R3.5.
func truncateForDisplay(data []byte) string {
	if data == nil {
		return "<nil>"
	}
	if len(data) <= stdinTruncateLen {
		return fmt.Sprintf("%q", data)
	}
	return fmt.Sprintf("%q... (%d bytes total)", data[:stdinTruncateLen], len(data))
}

// timestampPatterns matches common strftime timestamp formats used by ts.
// Per prd001-testutils R4.2:
//   - Syslog/default ts format: "Feb 19 12:34:56" (%b %d %H:%M:%S)
//   - Elapsed time format: "00:00:05" (HH:MM:SS)
var timestampPatterns = []*regexp.Regexp{
	// Matches "%b %d %H:%M:%S" (e.g., "Feb 19 12:34:56" or "Feb  5 12:34:56")
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}`),
	// Matches HH:MM:SS elapsed time (e.g., "00:00:05")
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// placeholderTimestamp is the fixed replacement string for normalized timestamps.
const placeholderTimestamp = "TIMESTAMP"

// TimestampNormalizer replaces common strftime timestamp patterns with the fixed
// placeholder "TIMESTAMP". Per prd001-testutils R4.2 and AC3.
func TimestampNormalizer(data []byte) []byte {
	result := data
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(placeholderTimestamp))
	}
	return result
}

// ComposeNormalizers returns a single Normalize function that applies the given
// normalizers in order. When called with no arguments, returns nil.
// Per prd001-testutils R4.1 and R4.3.
func ComposeNormalizers(normalizers ...Normalize) Normalize {
	if len(normalizers) == 0 {
		return nil
	}
	return func(data []byte) []byte {
		for _, fn := range normalizers {
			if fn != nil {
				data = fn(data)
			}
		}
		return data
	}
}
