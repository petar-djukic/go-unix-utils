// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Timeout is the maximum duration for each binary invocation.
// R2.3: defaults to 10 seconds; tests may override this value.
var Timeout = 10 * time.Second

// binResult holds the captured output of a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests executes both binaries with identical inputs for each DiffTest
// and compares stdout, stderr, and exit code.
// R2.1: each test case runs as a named subtest via t.Run.
// R2.6: LC_ALL=C is set by default unless overridden by DiffTest.Env.
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

			refResult := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			goResult := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

			// R3.1: apply normalizers before comparison.
			refStdout := applyNormalizers(refResult.stdout, tc.Normalize)
			goStdout := applyNormalizers(goResult.stdout, tc.Normalize)
			refStderr := applyNormalizers(refResult.stderr, tc.Normalize)
			goStderr := applyNormalizers(goResult.stderr, tc.Normalize)

			failed := false

			// R3.4: compare exit codes.
			if refResult.exitCode != goResult.exitCode {
				failed = true
			}

			// R3.2: compare stdout byte-for-byte.
			if !bytes.Equal(refStdout, goStdout) {
				failed = true
			}

			// R3.3: compare stderr byte-for-byte.
			if !bytes.Equal(refStderr, goStderr) {
				failed = true
			}

			if failed {
				// R3.5: report divergence with full context.
				stdinDisplay := truncateBytes(tc.Stdin, 256)
				t.Fatalf("differential test divergence\n"+
					"args:        %v\n"+
					"stdin:       %s\n"+
					"ref stdout:  %s\n"+
					"go  stdout:  %s\n"+
					"ref stderr:  %s\n"+
					"go  stderr:  %s\n"+
					"ref exit:    %d\n"+
					"go  exit:    %d",
					tc.Args, stdinDisplay,
					refStdout, goStdout,
					refStderr, goStderr,
					refResult.exitCode, goResult.exitCode)
			}

			// R5.1: check expected files if specified.
			if tc.ExpectedFiles != nil {
				for path, expected := range tc.ExpectedFiles {
					fullPath := path
					if !filepath.IsAbs(path) {
						fullPath = filepath.Join(workDir, path)
					}
					actual, err := os.ReadFile(fullPath)
					if err != nil {
						t.Fatalf("expected file %s: %v", path, err)
					}
					// R5.2: compare byte-for-byte.
					if !bytes.Equal(expected, actual) {
						t.Fatalf("file content divergence\n"+
							"file:     %s\n"+
							"expected: %s\n"+
							"actual:   %s",
							path, expected, actual)
					}
				}
			}
		})
	}
}

// runBinary executes a binary and captures its output.
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) binResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = workDir

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("binary %s timed out after %v", binary, Timeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("binary %s failed to execute: %v", binary, err)
		}
	}

	return binResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// buildEnv constructs the environment for binary execution.
// R2.6: sets LC_ALL=C by default, then merges DiffTest.Env overrides.
func buildEnv(testEnv []string) []string {
	env := os.Environ()

	// Set LC_ALL=C as default.
	env = setEnvVar(env, "LC_ALL", "C")

	// Merge test-specific environment variables.
	for _, entry := range testEnv {
		if k, v, ok := strings.Cut(entry, "="); ok {
			env = setEnvVar(env, k, v)
		}
	}

	return env
}

// setEnvVar sets or overrides an environment variable in the slice.
func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// applyNormalizers applies a slice of NormalizeFunc to data in order.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// truncateBytes returns data truncated to maxLen bytes for display.
func truncateBytes(data []byte, maxLen int) []byte {
	if len(data) <= maxLen {
		return data
	}
	truncated := make([]byte, maxLen+len("...(truncated)"))
	copy(truncated, data[:maxLen])
	copy(truncated[maxLen:], "...(truncated)")
	return truncated
}

