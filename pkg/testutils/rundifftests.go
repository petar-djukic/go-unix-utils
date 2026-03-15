// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R2.1-R2.4, R3.1-R3.6, R5.1-R5.2.

package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// defaultTimeout is the per-binary invocation timeout.
// R2.3: default timeout is 10 seconds.
const defaultTimeout = 10 * time.Second

// maxStdinDisplay is the maximum number of stdin bytes shown in failure messages.
// R3.5: truncate stdin to 256 bytes.
const maxStdinDisplay = 256

// RunDiffTests runs each DiffTest as a named subtest, executing both goBinary
// and refBinary with identical inputs and comparing their outputs.
//
// R2.1: accepts *testing.T, goBinary path, refBinary path, and []DiffTest.
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

			// R2.2: execute both binaries with identical Args, Stdin, Env, WorkDir.
			refStdout, refStderr, refExit := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			goStdout, goStderr, goExit := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

			// R3.1: apply Normalize functions to stdout and stderr before comparison.
			refStdout = applyNormalizers(refStdout, tc.Normalize)
			refStderr = applyNormalizers(refStderr, tc.Normalize)
			goStdout = applyNormalizers(goStdout, tc.Normalize)
			goStderr = applyNormalizers(goStderr, tc.Normalize)

			// R3.4: compare exit codes.
			if goExit != refExit {
				t.Errorf("exit code mismatch\n%s\nreference exit code: %d\ngo binary exit code: %d",
					formatContext(tc.Args, tc.Stdin), refExit, goExit)
			}

			// R3.2: compare stdout byte-for-byte.
			if !bytes.Equal(goStdout, refStdout) {
				t.Errorf("stdout mismatch\n%s\nreference stdout:\n%s\ngo binary stdout:\n%s",
					formatContext(tc.Args, tc.Stdin), refStdout, goStdout)
			}

			// R3.3: compare stderr byte-for-byte.
			if !bytes.Equal(goStderr, refStderr) {
				t.Errorf("stderr mismatch\n%s\nreference stderr:\n%s\ngo binary stderr:\n%s",
					formatContext(tc.Args, tc.Stdin), refStderr, goStderr)
			}

			// R5.1-R5.2: check ExpectedFiles if present.
			if tc.ExpectedFiles != nil {
				for path, expected := range tc.ExpectedFiles {
					fullPath := path
					if !strings.HasPrefix(path, "/") {
						fullPath = workDir + "/" + path
					}
					actual, err := os.ReadFile(fullPath)
					if err != nil {
						t.Errorf("expected file %s: %v\n%s",
							path, err, formatContext(tc.Args, tc.Stdin))
						continue
					}
					if !bytes.Equal(actual, expected) {
						t.Errorf("file content mismatch for %s\n%s\nexpected:\n%s\nactual:\n%s",
							path, formatContext(tc.Args, tc.Stdin), expected, actual)
					}
				}
			}
		})
	}
}

// runBinary executes a binary and returns its stdout, stderr, and exit code.
// Calls t.Fatal if the binary cannot be started.
//
// R2.2: capture stdout, stderr, and exit code independently.
// R2.3: 10-second default timeout.
// R2.4: extract exit codes from exec.ExitError.
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	err := cmd.Run()
	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()

	if err == nil {
		return stdout, stderr, 0
	}

	// R2.4: extract exit code from exec.ExitError.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		// Handle signal-killed processes: ExitCode() returns -1 for signal kills.
		if code == -1 {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					code = 128 + int(status.Signal())
				}
			}
		}
		return stdout, stderr, code
	}

	// Binary failed to start (not found, permission denied, etc.).
	t.Fatalf("failed to execute %s: %v", binary, err)
	return nil, nil, 0
}

// buildEnv constructs the environment for binary execution.
// R2.6: sets LC_ALL=C by default unless overridden by testEnv.
func buildEnv(testEnv []string) []string {
	env := os.Environ()

	// R2.6: apply LC_ALL=C default.
	hasLCALL := false
	for _, e := range testEnv {
		if strings.HasPrefix(e, "LC_ALL=") {
			hasLCALL = true
			break
		}
	}

	result := make([]string, 0, len(env)+len(testEnv)+1)
	for _, e := range env {
		// Remove existing LC_ALL so we can set the default.
		if strings.HasPrefix(e, "LC_ALL=") {
			continue
		}
		result = append(result, e)
	}

	if hasLCALL {
		// DiffTest.Env overrides LC_ALL.
		result = append(result, testEnv...)
	} else {
		result = append(result, "LC_ALL=C")
		result = append(result, testEnv...)
	}

	return result
}

// applyNormalizers applies normalize functions in order to output bytes.
// Returns the original bytes if fns is nil or empty.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// formatContext builds a context string for failure messages showing args and stdin.
// R3.5: truncate stdin to 256 bytes.
func formatContext(args []string, stdin []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "args: %v", args)
	if len(stdin) > 0 {
		display := stdin
		if len(display) > maxStdinDisplay {
			display = display[:maxStdinDisplay]
			fmt.Fprintf(&b, "\nstdin (%d bytes, truncated): %q", len(stdin), display)
		} else {
			fmt.Fprintf(&b, "\nstdin: %q", display)
		}
	}
	return b.String()
}
