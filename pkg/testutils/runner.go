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
)

// RunDiffTests executes each DiffTest as a named subtest, running both binaries
// with identical inputs and comparing their outputs. R2.1–R2.6, R3.1–R3.6.
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

			refStdout, refStderr, refExit := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			goStdout, goStderr, goExit := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

			// R3.1: apply normalizers before comparison
			norm := composeSlice(tc.Normalize)
			if norm != nil {
				refStdout = norm(refStdout)
				goStdout = norm(goStdout)
				refStderr = norm(refStderr)
				goStderr = norm(goStderr)
			}

			failed := false

			// R3.4: exit code comparison
			if refExit != goExit {
				t.Errorf("exit code mismatch: reference=%d go=%d", refExit, goExit)
				failed = true
			}

			// R3.2: stdout comparison
			if !bytes.Equal(refStdout, goStdout) {
				t.Errorf("stdout mismatch:\n  reference: %q\n  go:        %q", refStdout, goStdout)
				failed = true
			}

			// R3.3: stderr comparison
			if !bytes.Equal(refStderr, goStderr) {
				t.Errorf("stderr mismatch:\n  reference: %q\n  go:        %q", refStderr, goStderr)
				failed = true
			}

			// R3.5: report full context on failure
			if failed {
				stdinReport := tc.Stdin
				if len(stdinReport) > maxStdinReport {
					stdinReport = stdinReport[:maxStdinReport]
				}
				t.Errorf("divergence detail:\n  args:       %v\n  stdin:      %q\n  ref stdout: %q\n  go  stdout: %q\n  ref stderr: %q\n  go  stderr: %q\n  ref exit:   %d\n  go  exit:   %d",
					tc.Args, stdinReport, refStdout, goStdout, refStderr, goStderr, refExit, goExit)
			}

			// R5.1–R5.2: file-state comparison
			if tc.ExpectedFiles != nil {
				for relPath, expectedContent := range tc.ExpectedFiles {
					fullPath := filepath.Join(workDir, relPath)
					actual, err := os.ReadFile(fullPath)
					if err != nil {
						t.Errorf("expected file %s: %v", relPath, err)
						continue
					}
					if !bytes.Equal(expectedContent, actual) {
						t.Errorf("file content mismatch for %s:\n  expected: %q\n  actual:   %q", relPath, expectedContent, actual)
					}
				}
			}
		})
	}
}

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory. Returns stdout, stderr, and exit code.
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) ([]byte, []byte, int) {
	t.Helper()

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

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v with args %v", binary, defaultTimeout, args)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary %s: %v", binary, err)
		}
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode
}

// buildEnv constructs the environment for binary invocation. R1.3, R2.6.
// When testEnv is nil, inherits the current environment with LC_ALL=C.
// When non-nil, merges the KEY=VALUE pairs into the inherited environment.
func buildEnv(testEnv []string) []string {
	base := os.Environ()

	// R2.6: set LC_ALL=C as default
	env := setEnvVar(base, lcAllKey, "C")

	for _, kv := range testEnv {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			env = setEnvVar(env, parts[0], parts[1])
		}
	}

	return env
}

// setEnvVar sets or overrides an environment variable in the slice.
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
