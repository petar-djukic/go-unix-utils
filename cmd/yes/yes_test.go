// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/yes against gyes (GNU coreutils).
// Implements prd012-yes R4.1-R4.3 test coverage.
//
// Because yes produces infinite output, tests pipe both binaries through
// "head -n N" via a shell pipeline and compare the captured output.
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// headLines is the number of lines captured via head for output comparison.
const headLines = 100

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skipf("reference binary gyes not in PATH: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		// R4.2: no arguments — default "y" output.
		{
			name: "R1.1_default_y",
			args: nil,
		},
		// R4.2: single argument.
		{
			name: "R1.2_single_arg",
			args: []string{"hello"},
		},
		// R4.2: multiple arguments joined with spaces.
		{
			name: "R1.2_multi_arg",
			args: []string{"hello", "world"},
		},
		// R4.2: arguments after "--" separator.
		{
			name: "R1.3_double_dash",
			args: []string{"--", "--help"},
		},
		// R4.2: empty string argument.
		{
			name: "R4.2_empty_string",
			args: []string{""},
		},
		// Additional: single space argument.
		{
			name: "single_space_arg",
			args: []string{" "},
		},
		// Additional: multiple words with special chars.
		{
			name: "special_chars",
			args: []string{"a", "b", "c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refOut := runPipedHead(t, refBin, tc.args, headLines)
			goOut := runPipedHead(t, goBin, tc.args, headLines)

			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout mismatch\nargs: %v\nreference stdout:\n%s\ngo binary stdout:\n%s",
					tc.args, refOut, goOut)
			}
		})
	}
}

// TestSIGPIPEExitCode verifies R4.3: yes exits cleanly (exit 0) when stdout
// is closed early by a pipe consumer.
func TestSIGPIPEExitCode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Pipe through head -n 1 to trigger SIGPIPE.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s | head -n 1", goBin))
	err := cmd.Run()

	// R3.1: exit 0 on SIGPIPE.
	if err != nil {
		t.Errorf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

// runPipedHead runs "binary [args...] | head -n N" via sh and returns stdout.
// R4.1: pipe output through head to capture finite output for comparison.
func runPipedHead(t *testing.T, binary string, args []string, n int) []byte {
	t.Helper()

	// Build the shell command: binary [args...] | head -n N
	var cmdStr strings.Builder
	cmdStr.WriteString(shellQuote(binary))
	for _, a := range args {
		cmdStr.WriteByte(' ')
		cmdStr.WriteString(shellQuote(a))
	}
	fmt.Fprintf(&cmdStr, " | head -n %d", n)

	cmd := exec.Command("sh", "-c", cmdStr.String())
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to run %s: %v", cmdStr.String(), err)
	}

	return out
}

// shellQuote wraps a string in single quotes for safe shell interpolation.
func shellQuote(s string) string {
	// Replace each single quote with '\'' (end quote, escaped quote, start quote).
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
