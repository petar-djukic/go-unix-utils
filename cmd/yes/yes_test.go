// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/yes against gyes (Homebrew GNU coreutils).
// Implements prd012-yes R4.
package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinaryName = "gyes"

// TestDiff runs differential tests for --help and --version using RunDiffTests.
// Stdout content is blanked because help/version text differs between
// implementations; only exit code parity is verified here.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Normalizer that blanks output so only exit codes are compared.
	blankOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		{
			Name:      "help flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
		{
			Name:      "version flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{blankOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestYesPiped runs differential tests for yes output piped through a line reader.
// Each test reads N lines from both the Go binary and the reference binary and
// compares the captured output byte-for-byte. (prd012-yes R4.1, R4.2)
func TestYesPiped(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []struct {
		name  string
		args  []string
		lines int
	}{
		{"default y", nil, 5},
		{"single arg", []string{"hello"}, 3},
		{"multi arg join", []string{"hello", "world"}, 3},
		{"dash-dash separator", []string{"--", "--help"}, 2},
		{"empty string arg", []string{""}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			goOut := readLines(t, goBin, tc.args, tc.lines)
			refOut := readLines(t, refBin, tc.args, tc.lines)

			if goOut != refOut {
				t.Errorf("output mismatch\nGo:  %q\nRef: %q", goOut, refOut)
			}
		})
	}
}

// readLines starts a binary with the given args, reads n lines from stdout,
// then closes the pipe and waits for the process to exit. The captured lines
// are returned as a single string with trailing newlines.
func readLines(t *testing.T, binary string, args []string, n int) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe for %s: %v", binary, err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
	}

	scanner := bufio.NewScanner(stdout)
	var b strings.Builder
	for i := 0; i < n && scanner.Scan(); i++ {
		b.WriteString(scanner.Text())
		b.WriteByte('\n')
	}

	// Close stdout to trigger SIGPIPE in the child process.
	stdout.Close()
	// Wait for process exit; ignore error since SIGPIPE or context cancellation is expected.
	_ = cmd.Wait() // SIGPIPE exit expected

	return b.String()
}
