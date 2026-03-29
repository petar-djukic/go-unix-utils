// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// defaultTimeout is the per-binary execution timeout (R2.3).
const defaultTimeout = 10 * time.Second

// maxStdinDisplay is the truncation limit for stdin in failure messages (R3.5).
const maxStdinDisplay = 256

// NormalizeFunc transforms raw output bytes before comparison.
// R1.2: type alias for output normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case comparing a Go binary
// against a GNU reference binary.
//
// R1.1: fields cover arguments, stdin, environment, working directory,
// expected exit code, output normalizers, and expected file-system state.
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

// TimestampNormalizer replaces timestamps in output with a fixed placeholder
// so time-dependent output can be compared deterministically.
//
// R4.2: stub — not yet implemented.
var TimestampNormalizer NormalizeFunc

// RunDiffTests executes each DiffTest against both the Go binary and the
// reference binary, comparing stdout, stderr, and exit code.
//
// R2.1: iterates tests as subtests via t.Run.
// R2.2: captures stdout, stderr, exit code from each binary.
// R2.3: 10-second default timeout per binary.
// R2.4: no output on passing tests.
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
			normalize := ComposeNormalizers(tc.Normalize...)

			refOut, refErr, refExit := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
			goOut, goErr, goExit := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

			refOut = normalize(refOut)
			goOut = normalize(goOut)
			refErr = normalize(refErr)
			goErr = normalize(goErr)

			reportDivergence(t, tc, refOut, goOut, refErr, goErr, refExit, goExit)
		})
	}
}

// buildEnv constructs the environment for binary invocation.
// R2.6: sets LC_ALL=C unless overridden by testEnv.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = append(env, "LC_ALL=C")
	env = append(env, testEnv...)
	return env
}

// runBinary executes a binary and returns stdout, stderr, and exit code.
// R2.2, R2.3: captures output with timeout.
func runBinary(t *testing.T, bin string, args []string, stdin []byte, env []string, workDir string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v", bin, defaultTimeout)
	}

	return stdout.Bytes(), stderr.Bytes(), exitCodeFromErr(err)
}

// exitCodeFromErr extracts the exit code from an exec error.
func exitCodeFromErr(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// reportDivergence checks for differences and reports them via t.Errorf.
// R3.2, R3.3, R3.4, R3.5: byte-for-byte comparison with detailed output.
// D3: uses t.Errorf so all subtests run even when some fail.
func reportDivergence(t *testing.T, tc DiffTest, refOut, goOut, refErr, goErr []byte, refExit, goExit int) {
	t.Helper()
	if bytes.Equal(refOut, goOut) && bytes.Equal(refErr, goErr) && refExit == goExit {
		return
	}

	stdinDisplay := tc.Stdin
	if len(stdinDisplay) > maxStdinDisplay {
		stdinDisplay = stdinDisplay[:maxStdinDisplay]
	}

	t.Errorf("divergence for args=%v stdin=%q\n"+
		"  stdout ref: %q\n"+
		"  stdout  go: %q\n"+
		"  stderr ref: %q\n"+
		"  stderr  go: %q\n"+
		"  exit   ref: %d\n"+
		"  exit    go: %d",
		tc.Args, stdinDisplay,
		refOut, goOut,
		refErr, goErr,
		refExit, goExit)
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single
// NormalizeFunc that applies each in order.
//
// R4.3, R4.4: composition of normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			if fn != nil {
				data = fn(data)
			}
		}
		return data
	}
}

// BuildBinary compiles the Go package in dir and returns the path to the
// resulting binary. It calls t.Fatal on build failure.
//
// D4: uses go build -o with t.TempDir() for output.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	binName := filepath.Base(dir)
	if binName == "." {
		abs, err := filepath.Abs(dir)
		if err != nil {
			t.Fatalf("BuildBinary: resolve dir %q: %v", dir, err)
		}
		binName = filepath.Base(abs)
	}

	outPath := filepath.Join(t.TempDir(), binName)

	var stderr bytes.Buffer
	cmd := exec.Command("go", "build", "-o", outPath, dir)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("BuildBinary: go build %s: %v\n%s", dir, err, stderr.String())
	}

	return outPath
}
