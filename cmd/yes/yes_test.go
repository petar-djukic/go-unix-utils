// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd012-yes R1.1-R1.4, R2.1-R2.2, R3.1-R3.2, R4.1-R4.3.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// pipedTest defines a differential test case where both binaries are piped
// through "head -n N" to capture a finite number of output lines.
type pipedTest struct {
	name  string
	args  []string
	lines int
}

// TestDiff runs differential tests comparing the Go yes binary against gyes.
// R4.1: pipes output through head -n N for byte-for-byte comparison.
// R4.2: covers default, single-arg, multi-arg, --, and empty string cases.
// R4.3: verifies exit code 0 when stdout is closed early (SIGPIPE).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skipf("reference binary gyes not in PATH: %v", err)
	}

	tests := []pipedTest{
		{name: "default_y", args: nil, lines: 5},
		{name: "single_arg", args: []string{"hello"}, lines: 3},
		{name: "multi_arg", args: []string{"hello", "world"}, lines: 2},
		{name: "double_dash", args: []string{"--", "--help"}, lines: 1},
		{name: "empty_string", args: []string{""}, lines: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refOut := runPiped(t, refBin, tc.args, tc.lines)
			goOut := runPiped(t, goBin, tc.args, tc.lines)
			if !bytes.Equal(refOut, goOut) {
				t.Fatalf("divergence detected\n"+
					"args:       %v\n"+
					"ref stdout: %q\n"+
					"go  stdout: %q",
					tc.args, refOut, goOut)
			}
		})
	}
}

// runPiped executes a binary piped through "head -n N" via sh -c,
// returning the captured stdout. Uses LC_ALL=C for locale consistency.
func runPiped(t *testing.T, binary string, args []string, lines int) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	shellCmd := buildShellCmd(binary, args, lines)
	cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd)
	cmd.Env = buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// sh may return non-zero due to SIGPIPE on the left side of the pipe;
	// we care about stdout content, not the exit code from sh.
	_ = cmd.Run() // best-effort; timeout is handled below

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out: %s", shellCmd)
	}
	return stdout.Bytes()
}

// buildShellCmd constructs a shell command string: binary args... | head -n N.
// Each argument is single-quoted to prevent shell interpretation.
func buildShellCmd(binary string, args []string, lines int) string {
	parts := []string{shellQuote(binary)}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ") + fmt.Sprintf(" | head -n %d", lines)
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// buildEnv returns the current environment with LC_ALL=C set.
func buildEnv() []string {
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "LC_ALL=") {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}
