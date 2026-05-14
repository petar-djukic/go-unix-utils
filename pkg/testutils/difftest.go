// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code.
// Implements srd001-testutils.
package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const defaultTimeout = 10 * time.Second

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: named type alias for normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
//
// R1.1: Name identifies the subtest (used with t.Run). Args holds command-line
// arguments passed to both binaries.
//
// R1.2: Stdin nil means both binaries receive EOF immediately. An empty non-nil
// slice ([]byte{}) also produces no bytes but represents an open stdin that is
// immediately closed. ExitCode defaults to 0 when left as the zero value.
//
// R1.3: Env nil means both binaries inherit the test process environment with
// LC_ALL=C applied as a default override. When non-nil, KEY=VALUE pairs are
// merged into the inherited environment: matching keys are overridden, new keys
// are added.
//
// R1.4: Normalize holds a slice of NormalizeFunc applied in order to stdout and
// stderr of both binaries before comparison. Nil or empty means no normalization.
type DiffTest struct {
	Name          string
	Args          []string
	Stdin         []byte
	Env           []string
	WorkDir       string
	ExitCode      int
	Normalize     []NormalizeFunc
	ExpectedFiles map[string][]byte
}

var tsPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`),
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string.
// R4.2: built-in normalizer for cmd/ts tests.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	result := b
	for _, pat := range tsPatterns {
		result = pat.ReplaceAll(result, []byte("<TIMESTAMP>"))
	}
	return result
}

type binaryResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary and
// refBinary with identical inputs and comparing their outputs.
// R2.1: accepts paths to Go and reference binaries plus test cases.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			runSubtest(t, goBinary, refBinary, tc)
		})
	}
}

func runSubtest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()
	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}
	env := buildEnv(tc.Env)
	goRes := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)
	refRes := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
	goRes = applyNormalizers(goRes, tc.Normalize)
	refRes = applyNormalizers(refRes, tc.Normalize)
	compareOutputs(t, tc, goRes, refRes)
	checkExpectedFiles(t, workDir, tc.ExpectedFiles)
}

func runBinary(
	t *testing.T, binary string, args []string,
	stdin []byte, env []string, workDir string,
) binaryResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v", binary, defaultTimeout)
	}
	return extractResult(t, binary, err, stdout.Bytes(), stderr.Bytes())
}

func extractResult(
	t *testing.T, binary string, err error,
	stdout, stderr []byte,
) binaryResult {
	t.Helper()
	if err == nil {
		return binaryResult{stdout: stdout, stderr: stderr, exitCode: 0}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return binaryResult{
			stdout:   stdout,
			stderr:   stderr,
			exitCode: exitErr.ExitCode(),
		}
	}
	t.Fatalf("failed to start binary %s: %v", binary, err)
	return binaryResult{}
}

func buildEnv(testEnv []string) []string {
	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		key, val, _ := strings.Cut(entry, "=")
		envMap[key] = val
	}
	envMap["LC_ALL"] = "C"
	for _, entry := range testEnv {
		key, val, _ := strings.Cut(entry, "=")
		envMap[key] = val
	}
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

func applyNormalizers(res binaryResult, fns []NormalizeFunc) binaryResult {
	for _, fn := range fns {
		res.stdout = fn(res.stdout)
		res.stderr = fn(res.stderr)
	}
	return res
}

func compareOutputs(t *testing.T, tc DiffTest, goRes, refRes binaryResult) {
	t.Helper()
	stdoutMatch := bytes.Equal(goRes.stdout, refRes.stdout)
	stderrMatch := bytes.Equal(goRes.stderr, refRes.stderr)
	exitMatch := goRes.exitCode == refRes.exitCode
	if stdoutMatch && stderrMatch && exitMatch {
		return
	}
	t.Fatalf("%s", formatDivergence(tc, goRes, refRes))
}

func formatDivergence(tc DiffTest, goRes, refRes binaryResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "divergence detected\n")
	fmt.Fprintf(&b, "  args:        %v\n", tc.Args)
	fmt.Fprintf(&b, "  stdin:       %s\n", formatStdin(tc.Stdin))
	fmt.Fprintf(&b, "  ref stdout:  %q\n", refRes.stdout)
	fmt.Fprintf(&b, "  go  stdout:  %q\n", goRes.stdout)
	fmt.Fprintf(&b, "  ref stderr:  %q\n", refRes.stderr)
	fmt.Fprintf(&b, "  go  stderr:  %q\n", goRes.stderr)
	fmt.Fprintf(&b, "  ref exit:    %d\n", refRes.exitCode)
	fmt.Fprintf(&b, "  go  exit:    %d\n", goRes.exitCode)
	return b.String()
}

func formatStdin(stdin []byte) string {
	if stdin == nil {
		return "<nil>"
	}
	if len(stdin) > 256 {
		return fmt.Sprintf("%q... (truncated from %d bytes)", stdin[:256], len(stdin))
	}
	return fmt.Sprintf("%q", stdin)
}

func checkExpectedFiles(t *testing.T, workDir string, expected map[string][]byte) {
	t.Helper()
	if expected == nil {
		return
	}
	for relPath, want := range expected {
		fullPath := filepath.Join(workDir, relPath)
		got, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("expected file %s: %v", relPath, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("file %s divergence\n  expected: %q\n  actual:   %q",
				relPath, want, got)
		}
	}
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order.
// R4.4: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

