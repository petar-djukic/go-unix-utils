// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

// defaultTimeout is the maximum duration each binary invocation may run.
// R2.3: configurable default of 10 seconds.
const defaultTimeout = 10 * time.Second

// timestampPlaceholder replaces matched timestamp patterns during normalization.
const timestampPlaceholder = "<TIMESTAMP>"

// maxStdinDisplay is the maximum bytes of stdin shown in failure messages.
// R3.5: stdin truncated to 256 bytes in divergence reports.
const maxStdinDisplay = 256

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: type alias so callers can use func([]byte) []byte interchangeably.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
// R1.1: all fields match the prd001-testutils contract.
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (EOF); empty = open stdin closed immediately
	Env           []string          // nil = defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout/stderr before comparison
	ExpectedFiles map[string][]byte // path -> expected content after execution
}

// runResult holds captured output from a single binary execution.
type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// timestampPatterns matches common strftime-formatted timestamps.
// R4.2: used by TimestampNormalizer.
var timestampPatterns = []*regexp.Regexp{
	// ISO 8601: 2024-02-19 12:34:56 or 2024-02-19T12:34:56 with optional fractional seconds
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?`),
	// ctime-style: Feb 19 12:34:56
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// Unix epoch seconds with fractional part
	regexp.MustCompile(`\d{10,}\.\d+`),
	// Time only: 12:34:56 with optional fractional seconds
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}(\.\d+)?`),
}

// TimestampNormalizer replaces common timestamp patterns with a fixed placeholder.
// R4.2: used by cmd/ts tests to normalize wall-clock differences.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	for _, re := range timestampPatterns {
		b = re.ReplaceAll(b, []byte(timestampPlaceholder))
	}
	return b
}

// RunDiffTests runs each DiffTest as a named subtest, executing both binaries
// and comparing their stdout, stderr, and exit code.
// R2.1: accepts Go binary and reference binary paths.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleTest(t, goBinary, refBinary, tc)
		})
	}
}

// ComposeNormalizers returns a single NormalizeFunc that applies fns in order.
// R4.4: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// BuildBinary compiles a cmd/ package and returns the path to the built binary.
// Uses go build within the module boundary per shared_protocols.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "binary")
	cmd := exec.Command("go", "build", "-o", binPath, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", dir, err, out)
	}
	return binPath
}

// runSingleTest executes both binaries and compares their outputs.
func runSingleTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()
	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}
	env := buildEnv(tc.Env)
	refResult := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
	goResult := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)
	refStdout, goStdout := applyNormalizers(tc.Normalize, refResult.stdout, goResult.stdout)
	refStderr, goStderr := applyNormalizers(tc.Normalize, refResult.stderr, goResult.stderr)
	compareResults(t, tc, refStdout, goStdout, refStderr, goStderr, refResult.exitCode, goResult.exitCode)
	checkExpectedFiles(t, tc, workDir)
}

// buildEnv constructs the environment for binary execution.
// R2.6: sets LC_ALL=C by default, then merges DiffTest.Env overrides.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")
	for _, kv := range testEnv {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env = setEnvVar(env, k, v)
		}
	}
	return env
}

// setEnvVar sets or overrides a key=value pair in an environment slice.
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

// runBinary executes a binary and captures its output.
// R2.2: captures stdout, stderr, and exit code independently.
// R2.3: enforces defaultTimeout.
func runBinary(
	t *testing.T, binary string, args []string,
	stdin []byte, env []string, workDir string,
) runResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
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
	return handleRunResult(t, binary, err, ctx, &stdoutBuf, &stderrBuf)
}

// handleRunResult extracts the exit code from a completed command.
func handleRunResult(
	t *testing.T, binary string, err error,
	ctx context.Context, stdoutBuf, stderrBuf *bytes.Buffer,
) runResult {
	t.Helper()
	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("binary %s timed out after %v", binary, defaultTimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}
	return runResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}
}

// applyNormalizers runs all normalizers on both reference and Go output.
func applyNormalizers(fns []NormalizeFunc, ref, got []byte) ([]byte, []byte) {
	for _, fn := range fns {
		ref = fn(ref)
		got = fn(got)
	}
	return ref, got
}

// compareResults checks stdout, stderr, and exit code for divergence.
// R3.2-R3.5: reports differences with full context.
func compareResults(
	t *testing.T, tc DiffTest,
	refStdout, goStdout, refStderr, goStderr []byte,
	refExit, goExit int,
) {
	t.Helper()
	if bytes.Equal(refStdout, goStdout) &&
		bytes.Equal(refStderr, goStderr) &&
		refExit == goExit {
		return
	}
	stdinDisplay := truncateBytes(tc.Stdin, maxStdinDisplay)
	t.Errorf("divergence in %s\n"+
		"args: %v\nstdin: %q\n"+
		"ref stdout: %q\n go stdout: %q\n"+
		"ref stderr: %q\n go stderr: %q\n"+
		"ref exit: %d\n go exit: %d",
		tc.Name, tc.Args, stdinDisplay,
		refStdout, goStdout,
		refStderr, goStderr,
		refExit, goExit,
	)
}

// truncateBytes returns b truncated to maxLen bytes.
func truncateBytes(b []byte, maxLen int) []byte {
	if len(b) <= maxLen {
		return b
	}
	return b[:maxLen]
}

// checkExpectedFiles verifies file content after binary execution.
// R5.1-R5.2: compares expected file content byte-for-byte.
func checkExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()
	if tc.ExpectedFiles == nil {
		return
	}
	for relPath, expected := range tc.ExpectedFiles {
		checkSingleFile(t, workDir, relPath, expected)
	}
}

// checkSingleFile compares a single expected file against disk content.
func checkSingleFile(t *testing.T, workDir, relPath string, expected []byte) {
	t.Helper()
	fullPath := filepath.Join(workDir, relPath)
	actual, err := os.ReadFile(fullPath)
	if err != nil {
		t.Errorf("ExpectedFiles[%q]: %v", relPath, err)
		return
	}
	if !bytes.Equal(expected, actual) {
		t.Errorf("ExpectedFiles[%q] divergence:\n"+
			"expected: %q\n  actual: %q",
			relPath, expected, actual,
		)
	}
}
