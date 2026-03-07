// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

// defaultTimeout is the per-binary execution timeout. R2.3: default 10 seconds.
var defaultTimeout = 10 * time.Second

// RunDiffTests runs differential tests comparing goBinary against refBinary.
// Each DiffTest is executed as a named subtest via t.Run. R2.1.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// BuildBinary compiles a Go package to a temporary binary and returns its path.
// The binary is cleaned up automatically when the test completes. R4 (task).
func BuildBinary(t *testing.T, pkgPath string) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "testbin")

	var cmd *exec.Cmd
	fi, err := os.Stat(pkgPath)
	if err == nil && fi.IsDir() {
		cmd = exec.Command("go", "build", "-o", binPath, ".")
		cmd.Dir = pkgPath
	} else {
		cmd = exec.Command("go", "build", "-o", binPath, pkgPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build %s: %v\n%s", pkgPath, err, output)
	}
	return binPath
}

func runSingleDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	workDir := tc.WorkDir
	if workDir == "" {
		workDir = t.TempDir()
	}

	env := buildEnv(tc.Env)

	refStdout, refStderr, refExit := runBinary(t, refBinary, tc.Args, tc.Stdin, env, workDir)
	goStdout, goStderr, goExit := runBinary(t, goBinary, tc.Args, tc.Stdin, env, workDir)

	// R3.1: Apply normalizers before comparison.
	if len(tc.Normalize) > 0 {
		composed := ComposeNormalizers(tc.Normalize...)
		refStdout = composed(refStdout)
		refStderr = composed(refStderr)
		goStdout = composed(goStdout)
		goStderr = composed(goStderr)
	}

	// R3.2, R3.3, R3.4: Compare stdout, stderr, and exit code.
	failed := !bytes.Equal(refStdout, goStdout) ||
		!bytes.Equal(refStderr, goStderr) ||
		refExit != goExit

	if failed {
		// R3.5: Report divergence with args, stdin (truncated), both outputs, both exit codes.
		stdinDisplay := tc.Stdin
		if len(stdinDisplay) > 256 {
			stdinDisplay = stdinDisplay[:256]
		}
		t.Errorf("divergence detected\n"+
			"args:       %v\n"+
			"stdin:      %q\n"+
			"ref stdout: %q\n"+
			"go  stdout: %q\n"+
			"ref stderr: %q\n"+
			"go  stderr: %q\n"+
			"ref exit:   %d\n"+
			"go  exit:   %d",
			tc.Args, stdinDisplay,
			refStdout, goStdout,
			refStderr, goStderr,
			refExit, goExit)
	}

	// R5.1, R5.2: Check expected files.
	if tc.ExpectedFiles != nil {
		for path, expected := range tc.ExpectedFiles {
			fullPath := filepath.Join(workDir, path)
			actual, err := os.ReadFile(fullPath)
			if err != nil {
				t.Errorf("expected file %s: %v", path, err)
				continue
			}
			if !bytes.Equal(expected, actual) {
				t.Errorf("file %s divergence\nexpected: %q\nactual:   %q", path, expected, actual)
			}
		}
	}
}

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

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out after %v", binary, defaultTimeout)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}

	return stdout, stderr, exitCode
}

// buildEnv constructs the environment for binary execution. R2.6: LC_ALL=C
// is set by default; DiffTest.Env entries are merged on top.
func buildEnv(testEnv []string) []string {
	env := os.Environ()
	env = setEnvVar(env, "LC_ALL", "C")

	for _, entry := range testEnv {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			env = setEnvVar(env, parts[0], parts[1])
		}
	}

	return env
}

// setEnvVar sets or overrides a KEY=VALUE pair in an environment slice.
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
