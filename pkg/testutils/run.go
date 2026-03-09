// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Binary execution and stdout/stderr/exit-code comparison logic used by
// RunDiffTests. Implements prd001-testutils R2.1, R2.2, R2.3, R2.4, R2.5,
// R2.6, R3.1, R3.2, R3.3, R3.4, R3.5, R3.6, R4.1, R4.3, R5.1, R5.2.
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

// defaultTimeout is the maximum duration for each binary invocation.
// (prd001-testutils R2.3)
const defaultTimeout = 10 * time.Second

// defaultLCALL is the locale set by the harness for deterministic output.
// (prd001-testutils R2.6)
const defaultLCALL = "LC_ALL=C"

// maxStdinReport is the maximum number of stdin bytes shown in failure messages.
// (prd001-testutils R3.5)
const maxStdinReport = 256

// RunDiffTests executes both goBinary and refBinary with identical inputs for
// each DiffTest, then compares stdout, stderr, and exit code. Each test case
// runs as a named subtest via t.Run. (prd001-testutils R2.1)
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

			refOut, refErr, refCode := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			goOut, goErr, goCode := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

			// Apply normalizers (prd001-testutils R3.1, R4.1, R4.3)
			for _, fn := range tc.Normalize {
				refOut = fn(refOut)
				goOut = fn(goOut)
				refErr = fn(refErr)
				goErr = fn(goErr)
			}

			failed := false

			// Compare stdout (prd001-testutils R3.2)
			if !bytes.Equal(refOut, goOut) {
				failed = true
			}

			// Compare stderr (prd001-testutils R3.3)
			if !bytes.Equal(refErr, goErr) {
				failed = true
			}

			// Compare exit codes (prd001-testutils R3.4)
			if refCode != goCode {
				failed = true
			}

			if failed {
				stdinReport := tc.Stdin
				if len(stdinReport) > maxStdinReport {
					stdinReport = stdinReport[:maxStdinReport]
				}
				// Divergence reporting (prd001-testutils R2.4, R3.5)
				t.Fatalf("divergence detected\n"+
					"  args:        %v\n"+
					"  stdin:       %q\n"+
					"  ref stdout:  %q\n"+
					"  go  stdout:  %q\n"+
					"  ref stderr:  %q\n"+
					"  go  stderr:  %q\n"+
					"  ref exit:    %d\n"+
					"  go  exit:    %d",
					tc.Args, stdinReport,
					refOut, goOut,
					refErr, goErr,
					refCode, goCode)
			}

			// File-state comparison (prd001-testutils R5.1, R5.2)
			for relPath, expected := range tc.ExpectedFiles {
				absPath := filepath.Join(workDir, relPath)
				actual, err := os.ReadFile(absPath)
				if err != nil {
					t.Fatalf("ExpectedFiles: cannot read %q: %v", relPath, err)
				}
				if !bytes.Equal(expected, actual) {
					t.Fatalf("ExpectedFiles divergence for %q\n"+
						"  expected: %q\n"+
						"  actual:   %q",
						relPath, expected, actual)
				}
			}
		})
	}
}

// buildEnv constructs the environment for binary execution. It starts with the
// current process environment, sets LC_ALL=C as a default, then merges any
// user-provided overrides. (prd001-testutils R2.6, R1.3)
func buildEnv(userEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, defaultLCALL)

	for _, kv := range userEnv {
		env = setEnvVar(env, kv)
	}
	return env
}

// setEnvVar sets or overrides a KEY=VALUE pair in the env slice.
func setEnvVar(env []string, kv string) []string {
	key, _, _ := strings.Cut(kv, "=")
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = kv
			return env
		}
	}
	return append(env, kv)
}

// runBinary executes a binary with the given args, stdin, env, and working
// directory. Returns stdout, stderr, and exit code. Calls t.Fatal on timeout.
// (prd001-testutils R2.2, R2.3, R2.5)
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("timeout: %s exceeded %v deadline with args %v", binary, defaultTimeout, args)
	}

	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("executing %s: %v", binary, err)
		}
	}

	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}
