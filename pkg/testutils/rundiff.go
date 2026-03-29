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

// RunDiffTests executes each DiffTest against both the Go binary and the
// reference binary, comparing stdout, stderr, and exit code.
//
// R2.1: iterates tests as subtests via t.Run.
// R2.2: captures stdout, stderr, exit code from each binary.
// R2.3: 10-second default timeout per binary.
// R2.4: no output on passing tests.
// R3.1, R3.2: applies normalizers before byte-for-byte comparison.
// R3.3, R3.4: compares stderr and exit codes.
// R3.6: expected exit code verification against DiffTest.ExitCode.
// R5.1, R5.2: ExpectedFiles verification after execution.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runSingleDiffTest runs one DiffTest case against both binaries.
func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
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
	reportExitCodeMismatch(t, tc, refExit, goExit)
	verifyExpectedFiles(t, tc, workDir)
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
func reportDivergence(t *testing.T, tc DiffTest, refOut, goOut, refErr, goErr []byte, refExit, goExit int) {
	t.Helper()
	if bytes.Equal(refOut, goOut) && bytes.Equal(refErr, goErr) && refExit == goExit {
		return
	}

	stdinDisplay := truncateStdin(tc.Stdin)

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

// truncateStdin returns stdin truncated to maxStdinDisplay bytes.
func truncateStdin(stdin []byte) []byte {
	if len(stdin) > maxStdinDisplay {
		return stdin[:maxStdinDisplay]
	}
	return stdin
}

// reportExitCodeMismatch checks both binaries' exit codes against the expected
// DiffTest.ExitCode and reports if either differs.
// R3.6: exit code comparison against expected value.
func reportExitCodeMismatch(t *testing.T, tc DiffTest, refExit, goExit int) {
	t.Helper()
	if refExit != tc.ExitCode {
		t.Errorf("ref binary exit code %d, expected %d (args=%v)",
			refExit, tc.ExitCode, tc.Args)
	}
	if goExit != tc.ExitCode {
		t.Errorf("go binary exit code %d, expected %d (args=%v)",
			goExit, tc.ExitCode, tc.Args)
	}
}

// verifyExpectedFiles checks that files listed in DiffTest.ExpectedFiles match
// expected contents after both binaries have run.
// R5.1, R5.2: file-state comparison for file-output utilities.
func verifyExpectedFiles(t *testing.T, tc DiffTest, workDir string) {
	t.Helper()
	if len(tc.ExpectedFiles) == 0 {
		return
	}
	for relPath, expected := range tc.ExpectedFiles {
		absPath := resolvePath(relPath, workDir)
		actual, err := os.ReadFile(absPath)
		if err != nil {
			t.Errorf("ExpectedFiles[%q]: %v (args=%v)", relPath, err, tc.Args)
			continue
		}
		if !bytes.Equal(expected, actual) {
			t.Errorf("ExpectedFiles[%q] mismatch (args=%v)\n"+
				"  expected: %q\n"+
				"  actual:   %q",
				relPath, tc.Args, expected, actual)
		}
	}
}

// resolvePath resolves a file path, making relative paths relative to workDir.
func resolvePath(path, workDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}
