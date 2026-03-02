// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary under test and a GNU reference binary with identical
// inputs (args, stdin, environment) and compares their stdout, stderr, and exit
// codes byte-for-byte. Normalization hooks accommodate known non-deterministic
// output fields such as timestamps.
//
// Implements: prd001-testutils R1-R5
// Architecture: docs/ARCHITECTURE.yaml (pkg/testutils/ component)
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

// Timeout is the maximum duration allowed for each binary invocation.
// Callers may override this for test suites that require longer or shorter
// timeouts. The default is 10 seconds. (prd001-testutils R2.3)
var Timeout = 10 * time.Second

// stdinTruncateLen is the maximum stdin bytes included in divergence reports.
// (prd001-testutils R3.5)
const stdinTruncateLen = 256

// timestampPlaceholder is the fixed string substituted by TimestampNormalizer.
const timestampPlaceholder = "TIMESTAMP"

// NormalizeFunc is a function that transforms output bytes before comparison.
// Applied to both stdout and stderr of both binaries before any byte-for-byte
// comparison takes place. (prd001-testutils R1.4)
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. The zero value is a valid
// test case: no args, nil stdin (EOF), nil env (defaults with LC_ALL=C), empty
// WorkDir (per-test t.TempDir()), exit code 0, no normalization, and no
// expected files. (prd001-testutils R1.1)
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

// RunDiffTests runs each DiffTest as a named subtest via t.Run. Both goBinary
// and refBinary are invoked with identical Args, Stdin, and Env for each case.
// Any divergence in stdout, stderr, exit code, or ExpectedFiles causes the
// subtest to fail via t.Fatal. A passing run produces no output.
// (prd001-testutils R2.1, R2.4, R3.6)
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		tc := tc // capture loop variable
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runOneTest(t, goBinary, refBinary, tc)
		})
	}
}

// runOneTest executes a single DiffTest: invokes both binaries with identical
// inputs, applies normalization, compares outputs, and fails the subtest on
// any divergence.
func runOneTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(tc.Env)

	refStdout, refStderr, refCode, err := runBinary(refBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("reference binary %q: %v", refBinary, err)
	}

	goStdout, goStderr, goCode, err := runBinary(goBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("go binary %q: %v", goBinary, err)
	}

	// Apply normalization hooks before comparison (prd001-testutils R3.1).
	refStdoutN := applyNormalizers(tc.Normalize, refStdout)
	goStdoutN := applyNormalizers(tc.Normalize, goStdout)
	refStderrN := applyNormalizers(tc.Normalize, refStderr)
	goStderrN := applyNormalizers(tc.Normalize, goStderr)

	// Evaluate all divergences before failing so the full report is emitted.
	stdoutDiverged := !bytes.Equal(refStdoutN, goStdoutN)
	stderrDiverged := !bytes.Equal(refStderrN, goStderrN)
	exitDiverged := refCode != goCode || refCode != tc.ExitCode

	if stdoutDiverged || stderrDiverged || exitDiverged {
		stdinDisplay := tc.Stdin
		if len(stdinDisplay) > stdinTruncateLen {
			stdinDisplay = stdinDisplay[:stdinTruncateLen]
		}
		t.Fatalf(
			"output divergence:\n"+
				"  args:          %v\n"+
				"  stdin:         %q\n"+
				"  ref stdout:    %q\n"+
				"   go stdout:    %q\n"+
				"  ref stderr:    %q\n"+
				"   go stderr:    %q\n"+
				"  ref exit:      %d\n"+
				"   go exit:      %d\n"+
				"  expected exit: %d\n",
			tc.Args,
			stdinDisplay,
			refStdoutN,
			goStdoutN,
			refStderrN,
			goStderrN,
			refCode,
			goCode,
			tc.ExitCode,
		)
	}

	// File-state comparison runs only when stream outputs agree.
	// (prd001-testutils R5.1, R5.2)
	checkExpectedFiles(t, tc.ExpectedFiles, workDir)
}

// runBinary executes binary with the given args, stdin, env, and working
// directory. It captures stdout, stderr, and exit code. Returns an error when
// the binary exceeds Timeout or cannot be started.
// (prd001-testutils R2.2, R2.3)
func runBinary(binary string, args []string, stdin []byte, env []string, workDir string) (stdout, stderr []byte, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // binary path is test-controlled
	cmd.Env = env
	cmd.Dir = workDir

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, -1, fmt.Errorf("binary %q exceeded timeout of %s", binary, Timeout)
	}

	if runErr != nil {
		exitErr, ok := runErr.(*exec.ExitError)
		if !ok {
			return nil, nil, -1, fmt.Errorf("exec %q: %w", binary, runErr)
		}
		return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode(), nil
	}

	return outBuf.Bytes(), errBuf.Bytes(), 0, nil
}

// buildEnv constructs the environment slice for binary invocations. The
// inherited process environment is used as the base. LC_ALL=C is applied as
// the default before merging any override entries so callers can still
// override LC_ALL when needed. (prd001-testutils R1.3, R2.6)
func buildEnv(override []string) []string {
	base := os.Environ()
	m := make(map[string]string, len(base)+1)
	for _, kv := range base {
		k, v := splitEnvEntry(kv)
		m[k] = v
	}

	// LC_ALL=C default: applied before caller overrides so callers may
	// override it with an explicit LC_ALL entry. (prd001-testutils R2.6)
	m["LC_ALL"] = "C"

	for _, kv := range override {
		k, v := splitEnvEntry(kv)
		m[k] = v
	}

	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}

// splitEnvEntry splits a KEY=VALUE string at the first '='.
// Returns (kv, "") when no '=' is present.
func splitEnvEntry(kv string) (string, string) {
	idx := strings.IndexByte(kv, '=')
	if idx < 0 {
		return kv, ""
	}
	return kv[:idx], kv[idx+1:]
}

// applyNormalizers applies each fn in fns to b in order and returns the
// result. Returns b unchanged when fns is nil or empty.
// (prd001-testutils R4.3)
func applyNormalizers(fns []NormalizeFunc, b []byte) []byte {
	for _, fn := range fns {
		b = fn(b)
	}
	return b
}

// checkExpectedFiles compares expected file contents against files written by
// the Go binary in workDir. Missing files and content mismatches are reported
// via t.Errorf. (prd001-testutils R5.1, R5.2)
func checkExpectedFiles(t *testing.T, expected map[string][]byte, workDir string) {
	t.Helper()
	for relPath, want := range expected {
		fullPath := filepath.Join(workDir, relPath)
		got, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("expected file %q: %v", relPath, err)
			continue
		}
		if !bytes.Equal(want, got) {
			t.Errorf("file %q content mismatch:\n  expected: %q\n    actual: %q",
				relPath, want, got)
		}
	}
}

// timestampRe matches common strftime timestamp patterns produced by ts and
// similar utilities:
//   - "Feb 19 12:34:56" (ts default format: %b %e %T)
//   - "2026-02-19T12:34:56" and "2026-02-19 12:34:56" (ISO-like formats)
//   - "12:34:56" (time-only: %T / %H:%M:%S)
//
// (prd001-testutils R4.2)
var timestampRe = regexp.MustCompile(
	`\b(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\b` +
		`|\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}` +
		`|\b\d{2}:\d{2}:\d{2}\b`,
)

// TimestampNormalizer replaces strftime timestamp patterns with the fixed
// placeholder "TIMESTAMP". Use this in DiffTest.Normalize when testing
// utilities such as ts whose output includes wall-clock timestamps.
// (prd001-testutils R4.2; AC3)
func TimestampNormalizer(b []byte) []byte {
	return timestampRe.ReplaceAll(b, []byte(timestampPlaceholder))
}

// ComposeNormalizers returns a single NormalizeFunc that applies fns in order.
// When called with no arguments it returns a no-op normalizer. Use this to
// combine multiple normalizers for the DiffTest.Normalize slice.
// (prd001-testutils R4.4; AC7)
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
