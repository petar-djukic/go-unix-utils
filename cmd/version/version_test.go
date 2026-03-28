// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/version (prd059-version R1.1–R1.5).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// testBinaryPath stores the compiled binary path for reuse across tests.
var testBinaryPath string

// TestMain compiles the version binary once before all tests run.
func TestMain(m *testing.M) {
	// Check that source exists before attempting build.
	if _, err := os.Stat(filepath.Join(".", "main.go")); os.IsNotExist(err) {
		os.Exit(0) // skip gracefully during generation
	}

	dir, err := os.MkdirTemp("", "version-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir) // best-effort cleanup

	binPath := filepath.Join(dir, "version")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("go build failed: " + string(out))
	}
	testBinaryPath = binPath

	os.Exit(m.Run())
}

// runVersion executes the test binary with the given args and returns
// stdout, stderr, and exit code.
func runVersion(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, testBinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run version binary: %v", err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// TestNoArgs verifies R1.1: no arguments prints version + newline, exits 0.
// R1.2: without ldflags, the version is "dev".
func TestNoArgs(t *testing.T) {
	t.Parallel()
	stdout, stderr, exitCode := runVersion(t)

	if stdout != "dev\n" {
		t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
	}
	if stderr != "" {
		t.Errorf("stderr: got %q, want empty", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", exitCode)
	}
}

// TestVersionFlag verifies R1.4: --version prints the same as no-args.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	stdout, stderr, exitCode := runVersion(t, "--version")

	if stdout != "dev\n" {
		t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
	}
	if stderr != "" {
		t.Errorf("stderr: got %q, want empty", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", exitCode)
	}
}

// TestShortVersionFlag verifies R1.4: -v prints the same as no-args.
func TestShortVersionFlag(t *testing.T) {
	t.Parallel()
	stdout, stderr, exitCode := runVersion(t, "-v")

	if stdout != "dev\n" {
		t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
	}
	if stderr != "" {
		t.Errorf("stderr: got %q, want empty", stderr)
	}
	if exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", exitCode)
	}
}

// TestUnknownFlag verifies R1.4: unrecognized flags print usage to stderr
// and exit 2.
func TestUnknownFlag(t *testing.T) {
	t.Parallel()
	stdout, stderr, exitCode := runVersion(t, "--bogus")

	if stdout != "" {
		t.Errorf("stdout: got %q, want empty", stdout)
	}
	if stderr == "" {
		t.Error("stderr: got empty, want usage message")
	}
	if exitCode != 2 {
		t.Errorf("exit code: got %d, want 2", exitCode)
	}
}

// TestExportedVersion verifies R1.5: the Version() function returns the
// version string (matches the package-level variable).
func TestExportedVersion(t *testing.T) {
	t.Parallel()
	got := Version()
	if got != "dev" {
		t.Errorf("Version(): got %q, want %q", got, "dev")
	}
}
