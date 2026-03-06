// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/yes against gyes reference binary.
// Implements prd012-yes R4.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gyes"

// runPiped executes "binary [args...] | head -n N" and returns stdout.
func runPiped(t *testing.T, binary string, args []string, headN int) []byte {
	t.Helper()
	// Build shell command: binary arg1 arg2 ... | head -n N
	parts := make([]string, 0, len(args)+2)
	parts = append(parts, shellQuote(binary))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	shellCmd := fmt.Sprintf("%s | head -n %d", strings.Join(parts, " "), headN)

	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// yes exits with SIGPIPE when head closes the pipe; ignore the error.
	_ = cmd.Run() // best-effort: SIGPIPE causes non-zero exit
	return stdout.Bytes()
}

// shellQuote wraps s in single quotes for safe shell interpolation.
func shellQuote(s string) string {
	// Replace each ' with '\'' (end quote, escaped quote, start quote).
	escaped := bytes.ReplaceAll([]byte(s), []byte("'"), []byte("'\\''"))
	return "'" + string(escaped) + "'"
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}

	tests := []struct {
		name  string
		args  []string
		headN int
	}{
		{
			name:  "default_output",
			args:  []string{},
			headN: 5,
		},
		{
			name:  "single_arg",
			args:  []string{"hello"},
			headN: 3,
		},
		{
			name:  "multi_arg",
			args:  []string{"hello", "world"},
			headN: 2,
		},
		{
			name:  "double_dash_separator",
			args:  []string{"--", "--help"},
			headN: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			goOut := runPiped(t, goBin, tc.args, tc.headN)
			refOut := runPiped(t, refBin, tc.args, tc.headN)
			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goOut, refOut)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("--help produced no output")
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("--version produced no output")
	}
}
