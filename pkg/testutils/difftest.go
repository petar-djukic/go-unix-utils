// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It runs a Go binary and a GNU reference binary side by side with identical inputs
// and compares stdout, stderr, and exit code.
//
// Implements: prd001-testutils R1.1–R1.4, R2.1–R2.6, R3.1–R3.6, R4.1–R4.4, R5.1–R5.2.
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// defaultTimeout is the maximum duration each binary invocation is allowed
// before the test is failed. R2.3: default is 10 seconds.
const defaultTimeout = 10 * time.Second

// maxStdinDisplay is the maximum number of stdin bytes shown in failure messages.
// R3.5: truncate stdin to 256 bytes.
const maxStdinDisplay = 256

// NormalizeFunc transforms binary output before comparison to handle acceptable
// differences like timestamps. R1.4.
type NormalizeFunc = func([]byte) []byte

// DiffTest represents a single differential test case. R1.1.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is piped to both binaries. nil = EOF immediately; empty slice = open then close.
	Stdin []byte
	// Env is KEY=VALUE pairs merged into inherited environment. nil = defaults only (LC_ALL=C).
	Env []string
	// WorkDir is the working directory for both binaries. Empty = per-test t.TempDir().
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize functions applied in order to stdout and stderr before comparison.
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative paths to expected byte content after execution.
	ExpectedFiles map[string][]byte
}

// runResult holds the captured output of a single binary invocation.
type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing both goBinary and
// refBinary with identical inputs and comparing their outputs. R2.1, R3.6.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()

			workDir := tc.WorkDir
			if workDir == "" {
				workDir = t.TempDir() // R2.5
			}

			env := buildEnv(tc.Env) // R2.6

			refRes, err := runBinary(refBinary, tc.Args, tc.Stdin, env, workDir)
			if err != nil {
				t.Fatalf("reference binary failed to execute: %v", err)
			}

			goRes, err := runBinary(goBinary, tc.Args, tc.Stdin, env, workDir)
			if err != nil {
				t.Fatalf("Go binary failed to execute: %v", err)
			}

			// R3.1: apply normalizers to both outputs
			refStdout := applyNormalizers(refRes.stdout, tc.Normalize)
			goStdout := applyNormalizers(goRes.stdout, tc.Normalize)
			refStderr := applyNormalizers(refRes.stderr, tc.Normalize)
			goStderr := applyNormalizers(goRes.stderr, tc.Normalize)

			failed := false

			// R3.2: compare stdout byte-for-byte
			if !bytes.Equal(refStdout, goStdout) {
				failed = true
			}

			// R3.3: compare stderr byte-for-byte
			if !bytes.Equal(refStderr, goStderr) {
				failed = true
			}

			// R3.4: compare exit codes
			if refRes.exitCode != goRes.exitCode {
				failed = true
			}

			if failed {
				// R3.5: report divergence with full context
				t.Errorf("divergence detected\n"+
					"args:        %v\n"+
					"stdin:       %s\n"+
					"ref stdout:  %q\n"+
					"go  stdout:  %q\n"+
					"ref stderr:  %q\n"+
					"go  stderr:  %q\n"+
					"ref exit:    %d\n"+
					"go  exit:    %d",
					tc.Args,
					truncateStdin(tc.Stdin),
					refStdout, goStdout,
					refStderr, goStderr,
					refRes.exitCode, goRes.exitCode,
				)
			}

			// R5.1, R5.2: check expected files
			if tc.ExpectedFiles != nil {
				for path, expected := range tc.ExpectedFiles {
					fullPath := path
					if !strings.HasPrefix(path, "/") {
						fullPath = workDir + "/" + path
					}
					actual, err := os.ReadFile(fullPath)
					if err != nil {
						t.Errorf("expected file %s: %v", path, err)
						continue
					}
					if !bytes.Equal(expected, actual) {
						t.Errorf("file %s divergence\n"+
							"expected: %q\n"+
							"actual:   %q",
							path, expected, actual,
						)
					}
				}
			}
		})
	}
}

// runBinary executes a binary with the given arguments, stdin, environment, and
// working directory. It returns the captured stdout, stderr, and exit code. R2.2, R2.3.
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
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}, nil
}

// buildEnv constructs the environment for binary invocation. It starts with the
// current process environment, sets LC_ALL=C as default (R2.6), then merges any
// test-specific overrides (R1.3).
func buildEnv(testEnv []string) []string {
	base := os.Environ()

	// R2.6: set LC_ALL=C as default
	overrides := map[string]string{
		"LC_ALL": "C",
	}

	// Merge test-specific env vars
	for _, kv := range testEnv {
		if key, val, ok := strings.Cut(kv, "="); ok {
			overrides[key] = val
		}
	}

	// Apply overrides to base environment
	result := make([]string, 0, len(base)+len(overrides))
	seen := make(map[string]bool, len(overrides))
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if val, ok := overrides[key]; ok {
			result = append(result, key+"="+val)
			seen[key] = true
		} else {
			result = append(result, kv)
		}
	}
	// Add any overrides not already in base
	for key, val := range overrides {
		if !seen[key] {
			result = append(result, key+"="+val)
		}
	}

	return result
}

// applyNormalizers applies each NormalizeFunc in order to the data. R4.1, R4.3.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given functions
// in order. R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPattern matches common strftime-formatted timestamps. R4.2.
var timestampPattern = regexp.MustCompile(
	// Mon DD HH:MM:SS (e.g., "Feb 19 12:34:56")
	`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// YYYY-MM-DD HH:MM:SS
		`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// HH:MM:SS
		`\d{2}:\d{2}:\d{2}`,
)

// timestampPlaceholder is the fixed string that replaces timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. R4.2.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	return timestampPattern.ReplaceAll(data, []byte(timestampPlaceholder))
}

// truncateStdin returns a displayable representation of stdin, truncated to
// maxStdinDisplay bytes. R3.5.
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) <= maxStdinDisplay {
		return fmt.Sprintf("%q", stdin)
	}
	return fmt.Sprintf("%q... (%d bytes total)", stdin[:maxStdinDisplay], len(stdin))
}
