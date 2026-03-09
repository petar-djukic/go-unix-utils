// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code byte-for-byte.
//
// Implements: prd001-testutils (R1–R4).
package testutils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the maximum duration each binary invocation is allowed
// before the test fails. R2.3: default 10 seconds.
const defaultTimeout = 10 * time.Second

// maxStdinReport is the maximum number of stdin bytes shown in failure messages.
// R3.5: truncated to 256 bytes if longer.
const maxStdinReport = 256

// lcAllKey is the environment variable key for locale override.
const lcAllKey = "LC_ALL"

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: type alias so callers can use plain func([]byte) []byte values.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. R1.1–R1.4.
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (both binaries receive EOF immediately)
	Env           []string          // nil = use defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged into inherited environment
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout and stderr of both binaries before comparison; nil or empty = no normalization
	ExpectedFiles map[string][]byte // optional: path -> expected byte content after execution, for file-output utilities (sponge, cp)
}

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

// composeSlice applies a slice of NormalizeFunc in order. Returns nil if the
// slice is empty.
func composeSlice(fns []NormalizeFunc) NormalizeFunc {
	if len(fns) == 0 {
		return nil
	}
	return ComposeNormalizers(fns...)
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPatterns matches common strftime-formatted timestamp patterns. R4.2.
var timestampPatterns = []*regexp.Regexp{
	// ISO 8601: 2026-02-19 12:34:56 or 2026-02-19T12:34:56
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
	// ctime-style: Feb 19 12:34:56
	regexp.MustCompile(`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// Time only: 12:34:56
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// timestampPlaceholder is the fixed string that replaces timestamp patterns.
const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. R4.2.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	result := data
	for _, re := range timestampPatterns {
		result = re.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
