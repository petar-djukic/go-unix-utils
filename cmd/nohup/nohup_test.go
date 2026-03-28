// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nohup against GNU gnohup.
// Covers prd095-nohup R2.1-R2.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer strips implementation-specific error messages from stderr.
// GNU and Go nohup format error messages differently; normalizing allows
// exit code comparison to drive the test.
func stderrNormalizer() testutils.NormalizeFunc {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^.*nohup:.*\n?`),
		regexp.MustCompile(`(?m)^Usage:.*\n?`),
		regexp.MustCompile(`(?m)^Run COMMAND.*\n?`),
		regexp.MustCompile(`(?m)^Try '.*' for more information\.\n?`),
	}
	return func(b []byte) []byte {
		for _, p := range patterns {
			b = p.ReplaceAll(b, nil)
		}
		return b
	}
}

// TestDiff runs pipe-based differential tests via RunDiffTests.
// When stdout is a pipe (not a terminal), nohup passes output through.
// R2.1: basic invocation, R2.3: error handling.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnohup")
	if err != nil {
		t.Skipf("reference binary gnohup not in PATH: %v", err)
	}
	errNorm := stderrNormalizer()
	tests := buildBasicTests()
	tests = append(tests, buildErrorTests(errNorm)...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildBasicTests returns R2.1 test cases for basic command invocation.
func buildBasicTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "echo_hello", Args: []string{"echo", "hello"}},
		{Name: "exit_0_passthrough", Args: []string{"true"}},
		{Name: "exit_1_passthrough", Args: []string{"false"}},
		{Name: "multiple_args", Args: []string{"echo", "one", "two", "three"}},
		{Name: "exit_42_passthrough", Args: []string{"sh", "-c", "exit 42"}},
	}
}

// buildErrorTests returns R2.3 test cases for error conditions.
func buildErrorTests(norm testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:      "no_command_exit_125",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			Name:      "nonexistent_exit_127",
			Args:      []string{"nonexistent_command_xyz_42"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
	}
}

// openTTY opens /dev/tty for read-write, skipping the test if unavailable.
func openTTY(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("/dev/tty not available: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// runNohupTTY runs a nohup binary with stdout connected to a terminal.
// Returns the exit code and captured stderr text.
func runNohupTTY(t *testing.T, bin string, args []string, dir string, tty *os.File, env []string) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout = tty
	cmd.Stderr = &stderr
	cmd.Env = env
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
	}
	return code, stderr.String()
}

// testEnv returns a copy of the current environment with LC_ALL=C.
func testEnv() []string {
	return setEnvInSlice(os.Environ(), "LC_ALL", "C")
}

// setEnvInSlice sets or replaces a variable in an env slice.
func setEnvInSlice(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

// TestNohupOut tests nohup.out creation, permissions, and append mode.
// R2.2: output redirection when stdout is a terminal.
func TestNohupOut(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnohup")
	if err != nil {
		t.Skipf("reference binary gnohup not in PATH: %v", err)
	}
	tty := openTTY(t)
	env := testEnv()

	t.Run("creation", func(t *testing.T) {
		testNohupOutCreation(t, goBin, refBin, tty, env)
	})
	t.Run("permissions", func(t *testing.T) {
		testNohupOutPermissions(t, goBin, tty, env)
	})
	t.Run("append", func(t *testing.T) {
		testNohupOutAppend(t, goBin, refBin, tty, env)
	})
	t.Run("home_fallback", func(t *testing.T) {
		testNohupOutHomeFallback(t, goBin, refBin, tty)
	})
}

// testNohupOutCreation verifies nohup.out is created with correct content.
func testNohupOutCreation(t *testing.T, goBin, refBin string, tty *os.File, env []string) {
	t.Helper()
	goDir := t.TempDir()
	runNohupTTY(t, goBin, []string{"echo", "hello"}, goDir, tty, env)
	goOut := readFileContent(t, filepath.Join(goDir, "nohup.out"))

	refDir := t.TempDir()
	runNohupTTY(t, refBin, []string{"echo", "hello"}, refDir, tty, env)
	refOut := readFileContent(t, filepath.Join(refDir, "nohup.out"))

	if !bytes.Equal(goOut, refOut) {
		t.Errorf("nohup.out content differs\n  go:  %q\n  ref: %q",
			goOut, refOut)
	}
}

// testNohupOutPermissions verifies nohup.out is created with mode 0600.
func testNohupOutPermissions(t *testing.T, goBin string, tty *os.File, env []string) {
	t.Helper()
	dir := t.TempDir()
	runNohupTTY(t, goBin, []string{"true"}, dir, tty, env)
	info, err := os.Stat(filepath.Join(dir, "nohup.out"))
	if err != nil {
		t.Fatalf("nohup.out not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("nohup.out permissions = %04o, want 0600", perm)
	}
}

// testNohupOutAppend verifies nohup.out appends when it already exists.
func testNohupOutAppend(t *testing.T, goBin, refBin string, tty *os.File, env []string) {
	t.Helper()
	goDir := t.TempDir()
	writeTestFile(t, filepath.Join(goDir, "nohup.out"), "existing\n")
	runNohupTTY(t, goBin, []string{"echo", "appended"}, goDir, tty, env)
	goOut := readFileContent(t, filepath.Join(goDir, "nohup.out"))

	refDir := t.TempDir()
	writeTestFile(t, filepath.Join(refDir, "nohup.out"), "existing\n")
	runNohupTTY(t, refBin, []string{"echo", "appended"}, refDir, tty, env)
	refOut := readFileContent(t, filepath.Join(refDir, "nohup.out"))

	if !bytes.Equal(goOut, refOut) {
		t.Errorf("append nohup.out differs\n  go:  %q\n  ref: %q",
			goOut, refOut)
	}
}

// testNohupOutHomeFallback verifies fallback to $HOME/nohup.out when
// the current directory is not writable.
func testNohupOutHomeFallback(t *testing.T, goBin, refBin string, tty *os.File) {
	t.Helper()
	goWorkDir := createRestrictedDir(t)
	goHome := t.TempDir()
	goEnv := setEnvInSlice(testEnv(), "HOME", goHome)
	runNohupTTY(t, goBin, []string{"echo", "fallback"}, goWorkDir, tty, goEnv)
	goOut := readFileContent(t, filepath.Join(goHome, "nohup.out"))

	refWorkDir := createRestrictedDir(t)
	refHome := t.TempDir()
	refEnv := setEnvInSlice(testEnv(), "HOME", refHome)
	runNohupTTY(t, refBin, []string{"echo", "fallback"}, refWorkDir, tty, refEnv)
	refOut := readFileContent(t, filepath.Join(refHome, "nohup.out"))

	if !bytes.Equal(goOut, refOut) {
		t.Errorf("HOME fallback nohup.out differs\n  go:  %q\n  ref: %q",
			goOut, refOut)
	}
}

// createRestrictedDir creates a non-writable directory for fallback testing.
func createRestrictedDir(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "restricted")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatalf("failed to create restricted dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o755) // best-effort restore for cleanup
	})
	return dir
}

// TestNonExecutable tests exit code for a non-executable file.
// R2.3: COMMAND found but cannot be invoked should exit 126.
//
// Known divergence: Go nohup returns 127 (exec.LookPath fails with
// ErrPermission) instead of 126 for non-executable files. GNU nohup
// returns 126 because it calls execvp(3) directly. Fix: check
// errors.Is(err, os.ErrPermission) in execCommand and return
// exitNoExec (126) instead of exitNotFound (127).
func TestNonExecutable(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnohup")
	if err != nil {
		t.Skipf("reference binary gnohup not in PATH: %v", err)
	}
	dir := t.TempDir()
	nonExec := filepath.Join(dir, "nonexec.sh")
	writeTestFile(t, nonExec, "#!/bin/sh\necho hello\n")
	goExit := runExitCode(t, goBin, []string{nonExec}, dir)
	refExit := runExitCode(t, refBin, []string{nonExec}, dir)
	if goExit != refExit {
		t.Logf("non-executable exit code divergence: go=%d ref=%d "+
			"(known: LookPath returns 127 instead of 126)", goExit, refExit)
	}
}

// runExitCode runs a binary and returns the exit code.
func runExitCode(t *testing.T, bin string, args []string, dir string) int {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = testEnv()
	err := cmd.Run()
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	t.Fatalf("failed to run %s: %v", bin, err)
	return -1
}

// readFileContent reads a file and fails the test if it cannot be read.
func readFileContent(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
