// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSingleDiffTest executes a single differential test case, running both
// binaries with identical inputs and comparing their outputs.
func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(tc.Env)

	// Run reference binary with its own timeout context. (prd001-testutils R2.1, R2.2, R2.3)
	refCtx, refCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer refCancel()
	refResult, err := runBinary(refCtx, refBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("reference binary: %v", err)
	}

	// Run Go binary with its own timeout context. (prd001-testutils R2.1, R2.2, R2.3)
	goCtx, goCancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer goCancel()
	goResult, err := runBinary(goCtx, goBinary, tc.Args, tc.Stdin, env, workDir)
	if err != nil {
		t.Fatalf("go binary: %v", err)
	}

	// Apply normalizers to stdout and stderr of both binaries. (prd001-testutils R3.1, R4.1, R4.3)
	refStdout := applyNormalizers(refResult.stdout, tc.Normalize)
	refStderr := applyNormalizers(refResult.stderr, tc.Normalize)
	goStdout := applyNormalizers(goResult.stdout, tc.Normalize)
	goStderr := applyNormalizers(goResult.stderr, tc.Normalize)

	// Compare stdout, stderr, and exit code. (prd001-testutils R3.2, R3.3, R3.4)
	if !bytes.Equal(refStdout, goStdout) || !bytes.Equal(refStderr, goStderr) || refResult.exitCode != goResult.exitCode {
		t.Fatal(formatDivergence(tc, refStdout, goStdout, refStderr, goStderr, refResult.exitCode, goResult.exitCode))
	}

	// Compare expected files. (prd001-testutils R5.1, R5.2)
	compareExpectedFiles(t, tc.ExpectedFiles, workDir)
}

// runBinary invokes a binary with the given arguments, stdin, environment,
// and working directory. It captures stdout, stderr, and exit code.
func runBinary(ctx context.Context, binary string, args []string, stdin []byte, env []string, workDir string) (diffResult, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ctx.Err() != nil {
			return diffResult{}, fmt.Errorf("binary %s timed out after %v", binary, DefaultTimeout)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return diffResult{}, fmt.Errorf("executing %s: %w", binary, err)
		}
	}

	return diffResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}, nil
}

// buildEnv constructs the environment for binary invocation. It starts with the
// current process environment, applies LC_ALL=C as the default, then merges any
// caller-provided KEY=VALUE pairs on top. (prd001-testutils R2.6)
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")
	for _, e := range testEnv {
		if k, v, ok := strings.Cut(e, "="); ok {
			env = setEnvVar(env, k, v)
		}
	}
	return env
}

// setEnvVar sets or overrides an environment variable in the given slice.
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

// applyNormalizers applies each normalizer function in order to the given bytes.
func applyNormalizers(b []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		b = fn(b)
	}
	return b
}

// maxStdinDisplay is the maximum number of stdin bytes shown in failure messages.
const maxStdinDisplay = 256

// formatDivergence builds a failure message that includes the triggering input
// and both binaries' outputs. (prd001-testutils R3.5)
func formatDivergence(tc DiffTest, refStdout, goStdout, refStderr, goStderr []byte, refExit, goExit int) string {
	return fmt.Sprintf("divergence detected\n"+
		"  args:       %v\n"+
		"  stdin:      %s\n"+
		"  ref stdout: %q\n"+
		"  go  stdout: %q\n"+
		"  ref stderr: %q\n"+
		"  go  stderr: %q\n"+
		"  ref exit:   %d\n"+
		"  go  exit:   %d",
		tc.Args,
		truncateBytes(tc.Stdin, maxStdinDisplay),
		refStdout,
		goStdout,
		refStderr,
		goStderr,
		refExit,
		goExit,
	)
}

// truncateBytes returns a quoted string representation of b, truncated to
// maxLen bytes if longer.
func truncateBytes(b []byte, maxLen int) string {
	if b == nil {
		return "<nil>"
	}
	if len(b) <= maxLen {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q...(truncated, %d bytes total)", b[:maxLen], len(b))
}

// compareExpectedFiles checks that each expected file exists and has the correct
// content after binary execution. (prd001-testutils R5.1, R5.2)
func compareExpectedFiles(t *testing.T, expected map[string][]byte, workDir string) {
	t.Helper()
	if expected == nil {
		return
	}
	for path, expectedContent := range expected {
		fullPath := filepath.Join(workDir, path)
		actual, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
		if !bytes.Equal(expectedContent, actual) {
			t.Fatalf("file content divergence\n"+
				"  file:     %s\n"+
				"  expected: %q\n"+
				"  actual:   %q",
				path,
				expectedContent,
				actual,
			)
		}
	}
}
