// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying Go
// implementations of Unix utilities against GNU reference binaries.
//
// Implements prd001-testutils (R1-R5).
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

// DefaultTimeout is the maximum duration for a single binary invocation.
// R2.3: configurable timeout with 10-second default.
var DefaultTimeout = 10 * time.Second

// timestampPlaceholder is the fixed string that replaces timestamps during normalization.
const timestampPlaceholder = "<TIMESTAMP>"

// maxStdinReport is the maximum number of stdin bytes shown in failure reports.
// R3.5: stdin content truncated to 256 bytes if longer.
const maxStdinReport = 256

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: named type alias for normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case with fields for input
// arguments, stdin, environment, expected exit code, and expected output.
// R1.1: all fields per prd001-testutils specification.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin provides input data. nil means both binaries receive EOF immediately.
	// An empty non-nil slice ([]byte{}) produces no bytes but represents an open
	// stdin that is immediately closed (R1.2).
	Stdin []byte
	// Env specifies environment variable overrides as KEY=VALUE pairs. nil means
	// use defaults only (LC_ALL=C). Non-nil entries are merged into the inherited
	// environment: matching keys are overridden, new keys are added (R1.3).
	Env []string
	// WorkDir sets the working directory for both binaries. Empty means a per-test
	// temporary directory created via t.TempDir() (R2.5).
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize lists functions applied in order to stdout and stderr of both
	// binaries before comparison. nil or empty means no normalization (R4.1, R4.3).
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative file paths to expected byte content after
	// execution, for file-output utilities such as sponge and cp (R5.1).
	ExpectedFiles map[string][]byte
}

// binResult holds the captured output from a single binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary and
// refBinary with identical inputs and comparing their outputs.
// R2.1: accepts Go binary and GNU reference binary paths.
// R3.6: each test runs as a named subtest via t.Run.
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

			// Run reference binary first so file-output utilities leave the Go
			// binary's output on disk for ExpectedFiles comparison.
			refResult, err := runBinary(refBinary, tc.Args, tc.Stdin, env, workDir)
			if err != nil {
				t.Fatalf("reference binary execution failed: %v", err)
			}

			goResult, err := runBinary(goBinary, tc.Args, tc.Stdin, env, workDir)
			if err != nil {
				t.Fatalf("Go binary execution failed: %v", err)
			}

			// R3.1: apply normalization to both outputs before comparison.
			refStdout := applyNormalizers(refResult.stdout, tc.Normalize)
			goStdout := applyNormalizers(goResult.stdout, tc.Normalize)
			refStderr := applyNormalizers(refResult.stderr, tc.Normalize)
			goStderr := applyNormalizers(goResult.stderr, tc.Normalize)

			failed := false
			var reasons []string

			// R3.4: compare exit codes.
			if refResult.exitCode != goResult.exitCode {
				failed = true
				reasons = append(reasons, fmt.Sprintf("exit code: ref=%d go=%d", refResult.exitCode, goResult.exitCode))
			}

			// R3.2: compare stdout byte-for-byte.
			if !bytes.Equal(refStdout, goStdout) {
				failed = true
				reasons = append(reasons, "stdout differs")
			}

			// R3.3: compare stderr byte-for-byte.
			if !bytes.Equal(refStderr, goStderr) {
				failed = true
				reasons = append(reasons, "stderr differs")
			}

			if failed {
				// R3.5: report divergence with full context.
				t.Fatalf("differential test divergence: %s\n"+
					"args: %v\n"+
					"stdin: %s\n"+
					"ref stdout: %q\n"+
					"go  stdout: %q\n"+
					"ref stderr: %q\n"+
					"go  stderr: %q\n"+
					"ref exit code: %d\n"+
					"go  exit code: %d",
					strings.Join(reasons, "; "),
					tc.Args,
					truncateStdin(tc.Stdin),
					refStdout,
					goStdout,
					refStderr,
					goStderr,
					refResult.exitCode,
					goResult.exitCode,
				)
			}

			// R5.1, R5.2: file-state comparison.
			if tc.ExpectedFiles != nil {
				for relPath, expected := range tc.ExpectedFiles {
					absPath := filepath.Join(workDir, relPath)
					actual, readErr := os.ReadFile(absPath)
					if readErr != nil {
						t.Fatalf("expected file %s: %v", relPath, readErr)
					}
					if !bytes.Equal(expected, actual) {
						t.Fatalf("file content divergence: %s\n"+
							"expected: %q\n"+
							"actual:   %q",
							relPath,
							expected,
							actual,
						)
					}
				}
			}
		})
	}
}

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory, returning the captured stdout, stderr, and exit code.
// R2.2: captures stdout, stderr, and exit code independently.
// R2.3: imposes DefaultTimeout on each invocation.
func runBinary(binary string, args []string, stdin []byte, env []string, workDir string) (*binResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// R1.2: nil stdin means EOF immediately; non-nil provides the bytes.
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	runErr := cmd.Run()

	// R2.3: check for timeout before inspecting exit code.
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("binary %s exceeded timeout of %v", binary, DefaultTimeout)
	}

	code := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("executing %s: %w", binary, runErr)
		}
	}

	return &binResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: code,
	}, nil
}

// buildEnv constructs the environment for binary execution.
// R2.6: sets LC_ALL=C by default, then merges DiffTest.Env overrides.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")

	for _, e := range testEnv {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			env = setEnvVar(env, k, v)
		}
	}

	return env
}

// setEnvVar sets or replaces a KEY=VALUE pair in the environment slice.
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

// applyNormalizers applies each NormalizeFunc in order to the data.
// R4.1, R4.3: functions are applied in sequence.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order.
// R4.4: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPatterns matches common strftime timestamp formats used by ts and
// other utilities.
var timestampPatterns = []*regexp.Regexp{
	// "Feb 19 12:34:56" — default ts format (%b %d %H:%M:%S)
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}`),
	// "2024-02-19 12:34:56" — ISO-like (%Y-%m-%d %H:%M:%S)
	regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`),
	// "12:34:56" — time only (%H:%M:%S)
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. It handles syslog-style ("Feb 19 12:34:56"), ISO-style
// ("2024-02-19 12:34:56"), and time-only ("12:34:56") formats.
// R4.2: built-in normalizer for cmd/ts differential tests.
func TimestampNormalizer(data []byte) []byte {
	result := data
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}

// truncateStdin returns a displayable representation of stdin, truncated to
// maxStdinReport bytes per R3.5.
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) <= maxStdinReport {
		return fmt.Sprintf("%q", stdin)
	}
	return fmt.Sprintf("%q... (%d bytes total)", stdin[:maxStdinReport], len(stdin))
}
