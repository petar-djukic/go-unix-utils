// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/env against genv (GNU coreutils).
//
// Covers prd039-env R1.1, R1.2, R1.3, R2.1.
package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// sortLines sorts output lines for deterministic comparison.
func sortLines(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	s := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// discardAll blanks output for exit-code-only comparison.
func discardAll(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skip("reference binary genv not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: no arguments prints all environment variables
		{
			Name:      "R1.1_print_all",
			Args:      []string{},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R1.2: execute a command
		{
			Name:     "R1.2_run_command",
			Args:     []string{"printenv", "HOME"},
			ExitCode: 0,
		},
		// R1.2: NAME=VALUE + command
		{
			Name:     "R1.2_set_and_run",
			Args:     []string{"MY_TEST_VAR=hello", "printenv", "MY_TEST_VAR"},
			ExitCode: 0,
		},
		// R1.3: command not found exits 127
		{
			Name:      "R1.3_not_found",
			Args:      []string{"nonexistent_command_xyz_42"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R2.1: -i starts with empty environment
		{
			Name:     "R2.1_ignore_env",
			Args:     []string{"-i", "MY_VAR=only", "env"},
			ExitCode: 0,
		},
		// R2.1: --ignore-environment long form
		{
			Name:     "R2.1_ignore_env_long",
			Args:     []string{"--ignore-environment", "SINGLE=val", "env"},
			ExitCode: 0,
		},
		// R2.1: -i with no command prints empty output
		{
			Name:     "R2.1_ignore_no_cmd",
			Args:     []string{"-i"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestExitCode126 verifies that a non-executable file produces exit code 126.
// R1.3: COMMAND found but cannot be executed must exit 126.
func TestExitCode126(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	notExec := filepath.Join(tmpDir, "notexec")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("creating non-executable file: %v", err)
	}

	cmd := exec.Command(goBin, notExec)
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.ExitCode() != 126 {
		t.Errorf("exit code = %d, want 126", exitErr.ExitCode())
	}
}
