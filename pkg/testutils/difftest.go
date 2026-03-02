// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a reference GNU binary with identical inputs and
// compares stdout, stderr, and exit code byte-for-byte, with optional
// normalization hooks for non-deterministic output fields.
//
// Implements prd001-testutils (R1, R2, R3, R4, R5).
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// DefaultTimeout is the maximum duration each binary invocation is allowed
// before the test fails. Callers can override this per-harness via the
// Timeout field on RunConfig if added in the future; for now it applies
// globally.
//
// prd001-testutils R2.3.
const DefaultTimeout = 10 * time.Second

// NormalizeFunc transforms raw output bytes before comparison. Applied to
// both binaries' stdout and stderr so that non-deterministic fields (e.g.,
// timestamps) can be stripped or replaced with fixed placeholders.
//
// prd001-testutils R1.4.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. The harness executes both
// the Go binary and the GNU reference binary with identical Args, Stdin, and
// Env, then compares their outputs.
//
// prd001-testutils R1.1.
type DiffTest struct {
	// Name is the subtest name used with t.Run. Required.
	Name string

	// Args are command-line arguments passed to both binaries.
	Args []string

	// Stdin is piped to both binaries. nil means both binaries receive EOF
	// immediately. An empty non-nil slice ([]byte{}) also produces no bytes
	// but is semantically distinct (open stdin that is immediately closed).
	//
	// prd001-testutils R1.2.
	Stdin []byte

	// Env holds KEY=VALUE pairs merged into the inherited environment. nil
	// means use defaults only (LC_ALL=C). Non-nil entries override matching
	// keys and add new ones.
	//
	// prd001-testutils R1.3.
	Env []string

	// WorkDir is the working directory for both binaries. Empty means the
	// harness creates a per-test temporary directory via t.TempDir().
	//
	// prd001-testutils R2.5.
	WorkDir string

	// ExitCode is the expected exit code for both binaries.
	ExitCode int

	// Normalize holds functions applied in order to stdout and stderr of both
	// binaries before comparison. nil or empty means no normalization.
	//
	// prd001-testutils R4.1, R4.3.
	Normalize []NormalizeFunc

	// ExpectedFiles maps relative paths to expected byte content after
	// execution. For file-output utilities (sponge, cp). nil means no file
	// checks.
	//
	// prd001-testutils R5.1.
	ExpectedFiles map[string][]byte
}

// binResult holds the captured output of a single binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests executes each DiffTest as a named subtest via t.Run. For each
// test case, both goBinary and refBinary are invoked with identical Args,
// Stdin, and Env. Outputs are compared byte-for-byte after normalization.
// Divergence is reported via t.Fatalf with the triggering input, expected
// output (reference binary), and actual output (Go binary).
//
// prd001-testutils R2.1, R2.4, R3.6.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleDiffTest runs a single DiffTest case against both binaries and
// compares outputs.
func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(tc.Env)

	refResult := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
	goResult := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

	// Apply normalization hooks to both outputs.
	// prd001-testutils R3.1, R4.1, R4.3.
	refStdout := applyNormalizers(refResult.stdout, tc.Normalize)
	refStderr := applyNormalizers(refResult.stderr, tc.Normalize)
	goStdout := applyNormalizers(goResult.stdout, tc.Normalize)
	goStderr := applyNormalizers(goResult.stderr, tc.Normalize)

	// Compare stdout, stderr, and exit code.
	// prd001-testutils R3.2, R3.3, R3.4.
	diverged := false
	if !bytes.Equal(refStdout, goStdout) {
		diverged = true
	}
	if !bytes.Equal(refStderr, goStderr) {
		diverged = true
	}
	if refResult.exitCode != goResult.exitCode {
		diverged = true
	}

	if diverged {
		// prd001-testutils R3.5.
		t.Fatalf("output divergence\n"+
			"args:        %v\n"+
			"stdin:       %s\n"+
			"ref stdout:  %s\n"+
			"go  stdout:  %s\n"+
			"ref stderr:  %s\n"+
			"go  stderr:  %s\n"+
			"ref exit:    %d\n"+
			"go  exit:    %d",
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

	// File-state comparison.
	// prd001-testutils R5.1, R5.2.
	if tc.ExpectedFiles != nil {
		for relPath, expectedContent := range tc.ExpectedFiles {
			absPath := filepath.Join(workDir, relPath)
			actual, err := os.ReadFile(absPath)
			if err != nil {
				t.Fatalf("expected file %s: %v", relPath, err)
			}
			if !bytes.Equal(expectedContent, actual) {
				t.Fatalf("file divergence\n"+
					"file:     %s\n"+
					"expected: %s\n"+
					"actual:   %s",
					relPath,
					expectedContent,
					actual,
				)
			}
		}
	}
}

// buildEnv constructs the environment for binary invocation. It starts with
// the current process environment, applies LC_ALL=C as a default, then merges
// any user-provided overrides.
//
// prd001-testutils R2.6.
func buildEnv(userEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")
	for _, entry := range userEnv {
		if k, v, ok := parseEnvEntry(entry); ok {
			env = setEnvVar(env, k, v)
		}
	}
	return env
}

// setEnvVar sets or replaces a KEY=VALUE pair in the environment slice.
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

// parseEnvEntry splits a KEY=VALUE string. Returns false if the entry has no
// '=' separator.
func parseEnvEntry(entry string) (key, value string, ok bool) {
	idx := strings.IndexByte(entry, '=')
	if idx < 0 {
		return "", "", false
	}
	return entry[:idx], entry[idx+1:], true
}

// runBinary executes a binary with the given arguments, stdin, environment,
// and working directory. Returns captured stdout, stderr, and exit code.
//
// prd001-testutils R2.2, R2.3.
func runBinary(t *testing.T, binary string, args []string, stdin []byte, env []string, workDir string) binResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = workDir

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("timeout: %s exceeded %v with args %v", binary, DefaultTimeout, args)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("executing %s: %v", binary, err)
		}
	}

	return binResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}
}

// applyNormalizers applies each NormalizeFunc in order to the data.
//
// prd001-testutils R3.1, R4.1, R4.3.
func applyNormalizers(data []byte, normalizers []NormalizeFunc) []byte {
	for _, fn := range normalizers {
		data = fn(data)
	}
	return data
}

// truncateStdin returns a displayable representation of stdin, truncated to
// 256 bytes if longer.
//
// prd001-testutils R3.5.
func truncateStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	const maxLen = 256
	if len(stdin) > maxLen {
		return fmt.Sprintf("%s...(truncated, %d bytes total)", stdin[:maxLen], len(stdin))
	}
	return string(stdin)
}

// timestampPattern matches common strftime-formatted timestamps:
// "Mon DD HH:MM:SS", "YYYY-MM-DD HH:MM:SS", and ISO 8601 variants.
var timestampPattern = regexp.MustCompile(
	`(?:` +
		// "Feb 19 12:34:56" (ls -l, ts default)
		`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// "2026-02-19 12:34:56" or "2026-02-19T12:34:56"
		`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}` +
		`|` +
		// "12:34:56" (HH:MM:SS standalone)
		`\d{2}:\d{2}:\d{2}` +
		`)`,
)

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime-formatted timestamp patterns
// with a fixed placeholder string. Used by cmd/ts tests to eliminate
// wall-clock differences between the Go and reference binary outputs.
//
// prd001-testutils R4.2.
func TimestampNormalizer(data []byte) []byte {
	return timestampPattern.ReplaceAll(data, []byte(timestampPlaceholder))
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. This is a convenience for cmd/ test files that combine
// multiple normalizers into a single value for the DiffTest.Normalize field.
//
// prd001-testutils R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}
