// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/cut implementing prd026-cut R3.1, R3.2, R3.3, R4.1.
package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput normalizes output by discarding all content.
// Used for error tests where stderr messages differ between Go and GNU binaries
// but exit codes must match.
func clearOutput(b []byte) []byte {
	return nil
}

// TestDiff runs differential tests against the gcut reference binary.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcut")
	if err != nil {
		t.Skip("reference binary gcut not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			// R3.1 (task R1): no mode flag prints error to stderr and exits 1.
			Name:      "no_mode_flag",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R3.1 (task R1): mutually exclusive -b and -f flags.
			Name:      "conflicting_b_and_f",
			Args:      []string{"-b1", "-f1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R3.1 (task R1): mutually exclusive -c and -f flags.
			Name:      "conflicting_c_and_f",
			Args:      []string{"-c1", "-f1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			// R4.1 (task R4): '-' filename reads stdin.
			Name:     "dash_reads_stdin",
			Args:     []string{"-d:", "-f2", "-"},
			Stdin:    []byte("a:b:c\n"),
			ExitCode: 0,
		},
		{
			// R4.1: exit 0 on successful processing.
			Name:     "exit_zero_on_success",
			Args:     []string{"-b2-4"},
			Stdin:    []byte("abcdef\n"),
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies that --version prints output and exits 0.
// Not a differential test because version strings differ between implementations.
func TestVersion(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("--version exited with error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "cut") {
		t.Errorf("--version output does not contain 'cut': %q", out)
	}
}

// TestHelp verifies that --help prints usage and exits 0.
// Not a differential test because help text differs between implementations.
func TestHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("--help exited with error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("--help output does not contain 'Usage:': %q", out)
	}
}
