// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/chronic (prd112-chronic R2.1, R2.2, R2.3).
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests against the reference chronic binary.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("chronic")
	if err != nil {
		t.Skip("reference binary chronic not in PATH")
	}
	tests := []testutils.DiffTest{
		{
			Name:     "suppress_output_on_success",
			Args:     []string{"echo", "hello"},
			ExitCode: 0,
		},
		{
			Name:     "show_output_on_failure",
			Args:     []string{"sh", "-c", "echo out; echo err >&2; exit 1"},
			ExitCode: 1,
		},
		{
			Name:     "exit_code_passthrough",
			Args:     []string{"sh", "-c", "exit 42"},
			ExitCode: 42,
		},
		{
			Name:     "false_exits_nonzero",
			Args:     []string{"false"},
			ExitCode: 1,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestStderrFlag verifies that -e triggers output display when stderr is non-empty.
// R1.2: tested standalone because reference binary version may differ.
func TestStderrFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "-e", "sh", "-c", "echo err >&2")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "err") {
		t.Errorf("stderr = %q, want it to contain 'err'", stderr.String())
	}
}

// TestVerboseFlag verifies that -v prints the command to stderr.
// R1.3: tested standalone because reference binary version may differ.
func TestVerboseFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "-v", "echo", "hello")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "echo hello") {
		t.Errorf("stderr = %q, want it to contain 'echo hello'", stderr.String())
	}
}

// TestCommandNotFound verifies that chronic exits 127 when the command is not found.
// R2.1: command not found produces exit code 127 with error on stderr.
func TestCommandNotFound(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "nonexistent_cmd_xyz_12345")
	out, err := cmd.CombinedOutput()
	requireExitCode(t, err, exitCmdNotFound)
	if !strings.Contains(string(out), "command not found") {
		t.Errorf("output = %q, want it to contain 'command not found'", out)
	}
}

// TestCommandNotExecutable verifies that chronic exits 126 when the command
// exists but is not executable.
// R2.2: permission denied produces exit code 126.
func TestCommandNotExecutable(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	noExec := filepath.Join(dir, "noexec.sh")
	if err := os.WriteFile(noExec, []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(goBin, noExec)
	_, err := cmd.CombinedOutput()
	requireExitCode(t, err, exitCmdNoExec)
}

// TestHelp verifies that --help prints usage and exits 0.
// R2.3: --help flag per GNU convention.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("--help output missing 'Usage:': %q", out)
	}
}

// TestVersion verifies that --version prints version info and exits 0.
// R2.3: --version flag per GNU convention.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(string(out), "chronic") {
		t.Errorf("--version output missing 'chronic': %q", out)
	}
}

const (
	exitCmdNotFound = 127
	exitCmdNoExec   = 126
)

// requireExitCode asserts that the error represents a process exit with the given code.
func requireExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if want == 0 {
		if err != nil {
			t.Fatalf("expected exit 0, got error: %v", err)
		}
		return
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError with code %d, got %v", want, err)
	}
	if exitErr.ExitCode() != want {
		t.Errorf("exit code = %d, want %d", exitErr.ExitCode(), want)
	}
}
