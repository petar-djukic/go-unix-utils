// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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

// TestVersionFlag verifies R1.4: --version prints the same output.
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

// TestVersionUnknownFlag verifies R1.4: unknown flag prints usage to stderr, exits 2.
func TestVersionUnknownFlag(t *testing.T) {
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

	// Build with ldflags to inject a version string.
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
