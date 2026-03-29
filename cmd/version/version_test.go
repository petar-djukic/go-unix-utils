// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential and unit tests for cmd/version per prd059 R1.1, R1.2, R1.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests against a reference binary.
// R5.1: version has no GNU reference binary; this test skips gracefully.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gversion")
	if err != nil {
		t.Skipf("reference binary gversion not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     []string{},
			ExitCode: 0,
		},
		{
			Name:     "version_flag",
			Args:     []string{"--version"},
			ExitCode: 0,
		},
		{
			Name:     "help_flag",
			Args:     []string{"--help"},
			ExitCode: 0,
		},
		{
			Name:     "unknown_flag",
			Args:     []string{"--bogus"},
			ExitCode: 2,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersionNoArgs verifies R1.1: no arguments prints version + newline, exits 0.
func TestVersionNoArgs(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(bin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// R1.2: development build (no ldflags) prints "dev".
	if got := string(out); got != "dev\n" {
		t.Errorf("got %q, want %q", got, "dev\n")
	}
}

// TestVersionFlag verifies R1.4: --version and -v print the same output.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(bin, flag)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("flag %s: unexpected error: %v", flag, err)
			}
			if got := string(out); got != "dev\n" {
				t.Errorf("flag %s: got %q, want %q", flag, got, "dev\n")
			}
		})
	}
}

// TestHelpFlag verifies --help prints usage and exits 0.
func TestHelpFlag(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	for _, flag := range []string{"--help", "-h"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(bin, flag)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("flag %s: unexpected error: %v", flag, err)
			}
			if len(out) == 0 {
				t.Errorf("flag %s: expected help output, got empty", flag)
			}
		})
	}
}

// TestUnknownFlag verifies R1.4: unknown flag prints usage to stderr, exits 2.
func TestUnknownFlag(t *testing.T) {
	t.Parallel()
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--bogus")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exitErr.ExitCode())
	}
	if stderr.Len() == 0 {
		t.Error("expected usage message on stderr, got nothing")
	}
}

// TestVersionLdflags verifies R1.2: version set via ldflags is printed.
func TestVersionLdflags(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	binPath := tmpDir + "/version-ldflags"

	buildCmd := exec.Command("go", "build",
		"-ldflags", "-X main.Version=v1.20260328.1",
		"-o", binPath, ".")
	buildCmd.Dir = "."
	buildCmd.Env = append(os.Environ(), "GOFLAGS=")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build with ldflags failed: %v\n%s", err, out)
	}

	cmd := exec.Command(binPath)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := string(out); got != "v1.20260328.1\n" {
		t.Errorf("got %q, want %q", got, "v1.20260328.1\n")
	}
}
