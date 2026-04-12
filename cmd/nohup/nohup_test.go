// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/nohup against gnohup (GNU coreutils).
// Implements srd095 R2.1 (TestDiff with RunDiffTests), R2.2 (core behavior),
// R2.3 (error handling and exit code tests).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gnohup"

// makeNormalizer creates a NormalizeFunc that replaces binary names and
// normalizes syscall error message capitalization between GNU and Go.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(progName))
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(progName))
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
		{"No such file or directory", "no such file or directory"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/nohup against gnohup.
// R2.1: uses testutils.BuildBinary and testutils.RunDiffTests.
// R2.2: covers basic command execution with stdout passthrough.
// R2.3: covers missing command, non-existent command, non-executable errors.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	tests := []testutils.DiffTest{
		// R2.2: basic command execution. stdout is not a terminal in test,
		// so nohup passes output through directly. R1.1, R1.4.
		{
			Name:      "basic_echo",
			Args:      []string{"echo", "hello"},
			Normalize: norms,
		},
		// R2.2: command with multiple arguments. R1.4.
		{
			Name:      "command_with_args",
			Args:      []string{"echo", "one", "two", "three"},
			Normalize: norms,
		},
		// R2.2: command that exits non-zero. R2.1.
		{
			Name:      "command_exit_nonzero",
			Args:      []string{"sh", "-c", "exit 42"},
			ExitCode:  42,
			Normalize: norms,
		},
		// R2.3: missing operand exits 125.
		{
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  125,
			Normalize: norms,
		},
		// R2.3: non-existent command exits 127. R2.2.
		{
			Name:      "nonexistent_command",
			Args:      []string{"nonexistent_cmd_xyz_42"},
			ExitCode:  127,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonExecutable verifies exit code 126 for a non-executable file.
// R2.3: command found but not executable.
func TestDiffNonExecutable(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	tmpDir := t.TempDir()
	notExec := filepath.Join(tmpDir, "notexec")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("failed to create non-executable file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "not_executable",
			Args:      []string{notExec},
			ExitCode:  126,
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffStdinPassthrough verifies nohup passes stdin to the child.
// R1.4: all arguments and stdin are forwarded.
func TestDiffStdinPassthrough(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)
	norms := []testutils.NormalizeFunc{norm}

	tests := []testutils.DiffTest{
		{
			Name:      "stdin_passthrough",
			Args:      []string{"cat"},
			Stdin:     []byte("hello from stdin\n"),
			Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
