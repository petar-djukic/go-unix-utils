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
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the maximum duration for a single binary invocation.
// (prd001-testutils R2.3)
const defaultTimeout = 10 * time.Second

// maxStdinDisplay is the maximum number of stdin bytes shown in failure messages.
// (prd001-testutils R3.5)
const maxStdinDisplay = 256

// runResult holds the captured output from a single binary execution.
type runResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing both goBinary
// and refBinary with identical inputs and comparing their outputs.
// (prd001-testutils R2.1, R3.6)
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			t.Parallel()

			workDir := tc.WorkDir
			if workDir == "" {
				workDir = t.TempDir()
			}

			env := buildEnv(tc.Env)

			refOut, err := runBinary(refBinary, tc.Args, tc.Stdin, env, workDir)
			if err != nil {
				t.Fatalf("running reference binary: %v", err)
			}

			goOut, err := runBinary(goBinary, tc.Args, tc.Stdin, env, workDir)
			if err != nil {
				t.Fatalf("running Go binary: %v", err)
			}

			// Apply normalizers to stdout and stderr. (R3.1)
			refStdout := applyNormalizers(refOut.Stdout, tc.Normalize)
			goStdout := applyNormalizers(goOut.Stdout, tc.Normalize)
			refStderr := applyNormalizers(refOut.Stderr, tc.Normalize)
			goStderr := applyNormalizers(goOut.Stderr, tc.Normalize)

			failed := false

			// Compare stdout. (R3.2)
			if !bytes.Equal(refStdout, goStdout) {
				t.Errorf("stdout mismatch")
				failed = true
			}

			// Compare stderr. (R3.3)
			if !bytes.Equal(refStderr, goStderr) {
				t.Errorf("stderr mismatch")
				failed = true
			}

			// Compare exit codes. (R3.4)
			if refOut.ExitCode != goOut.ExitCode {
				t.Errorf("exit code mismatch: reference=%d, go=%d", refOut.ExitCode, goOut.ExitCode)
				failed = true
			}

			if failed {
				t.Errorf("%s", formatDivergence(tc, refStdout, goStdout, refStderr, goStderr, refOut.ExitCode, goOut.ExitCode))
			}

			// Check expected files. (R5.1, R5.2)
			if tc.ExpectedFiles != nil {
				checkExpectedFiles(t, workDir, tc.ExpectedFiles)
			}
		})
	}
}

// buildEnv constructs the environment for binary execution. It starts with the
// current process environment, sets LC_ALL=C as default, then merges any
// DiffTest.Env overrides. (prd001-testutils R2.6, R1.3)
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")
	for _, kv := range testEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			env = setEnvVar(env, parts[0], parts[1])
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

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory, returning the captured stdout, stderr, and exit code.
// (prd001-testutils R2.2, R2.3, R2.5)
func runBinary(binary string, args []string, stdin []byte, env []string, workDir string) (*runResult, error) {
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

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("binary %s timed out after %v", binary, defaultTimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("executing %s: %w", binary, err)
		}
	}

	return &runResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
	}, nil
}

// applyNormalizers applies each NormalizeFunc in order to the given bytes.
// (prd001-testutils R3.1, R4.1, R4.3)
func applyNormalizers(b []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		b = fn(b)
	}
	return b
}

// formatDivergence builds a failure message with full context for debugging.
// (prd001-testutils R3.5)
func formatDivergence(tc DiffTest, refStdout, goStdout, refStderr, goStderr []byte, refExit, goExit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n--- Divergence ---\n")
	fmt.Fprintf(&b, "Args: %v\n", tc.Args)

	if tc.Stdin != nil {
		stdinDisplay := tc.Stdin
		if len(stdinDisplay) > maxStdinDisplay {
			stdinDisplay = stdinDisplay[:maxStdinDisplay]
			fmt.Fprintf(&b, "Stdin (truncated to %d bytes): %q\n", maxStdinDisplay, stdinDisplay)
		} else {
			fmt.Fprintf(&b, "Stdin: %q\n", stdinDisplay)
		}
	}

	fmt.Fprintf(&b, "Reference stdout: %q\n", refStdout)
	fmt.Fprintf(&b, "Go stdout:        %q\n", goStdout)
	fmt.Fprintf(&b, "Reference stderr: %q\n", refStderr)
	fmt.Fprintf(&b, "Go stderr:        %q\n", goStderr)
	fmt.Fprintf(&b, "Reference exit:   %d\n", refExit)
	fmt.Fprintf(&b, "Go exit:          %d\n", goExit)

	return b.String()
}

// checkExpectedFiles verifies that files in the working directory match the
// expected content after binary execution. (prd001-testutils R5.1, R5.2)
func checkExpectedFiles(t *testing.T, workDir string, expected map[string][]byte) {
	t.Helper()
	for relPath, wantContent := range expected {
		fullPath := filepath.Join(workDir, relPath)
		gotContent, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("expected file %s: %v", relPath, err)
			continue
		}
		if !bytes.Equal(wantContent, gotContent) {
			t.Errorf("file %s content mismatch:\n  expected: %q\n  actual:   %q", relPath, wantContent, gotContent)
		}
	}
}
