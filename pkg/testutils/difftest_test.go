// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// helperEnvKey controls stub binary behavior when the test binary is run
// as a subprocess by DiffTest.Run. When set, TestMain handles the named mode
// and exits immediately without running any tests.
const helperEnvKey = "GO_TESTUTILS_HELPER"

// innerTestEnvKey signals that TestRun_ExitCodeMismatch is running inside a
// subprocess spawned by the outer invocation of that same test function.
const innerTestEnvKey = "GO_TESTUTILS_INNER"

// TestMain intercepts runs where the test binary is used as a stub binary.
// When GO_TESTUTILS_HELPER is set, the binary acts as the named stub and
// exits immediately without running any tests.
func TestMain(m *testing.M) {
	if mode := os.Getenv(helperEnvKey); mode != "" {
		runHelper(mode)
	}
	os.Exit(m.Run())
}

// runHelper implements stub binary behavior for the named mode and always
// calls os.Exit, so it never returns to the caller.
func runHelper(mode string) {
	switch mode {
	case "noop":
		os.Exit(0)
	case "exit1":
		os.Exit(1)
	case "hello":
		_, _ = fmt.Fprint(os.Stdout, "hello\n") // best-effort; exit 0 regardless
		os.Exit(0)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown helper mode: %s\n", mode) // best-effort
		os.Exit(2)
	}
}

// TestRun_AllMatch verifies that Run does not call t.Errorf when exit code,
// stdout, and stderr all match the expected values (prd001-testutils R3, AC2).
func TestRun_AllMatch(t *testing.T) {
	t.Parallel()

	self := os.Args[0]
	tc := DiffTest{
		Env:        []string{helperEnvKey + "=hello"},
		WantExit:   0,
		WantStdout: []byte("hello\n"),
		WantStderr: nil,
	}
	tc.Run(t, self, self)
}

// TestRun_ExitCodeMismatch verifies that Run calls t.Errorf with the expected
// and actual exit codes when the Go binary exit code does not match WantExit
// (prd001-testutils R3, AC3).
//
// Because calling Run with a mismatch causes t.Errorf (and fails the test),
// the mismatch case is executed inside a subprocess. The outer test verifies
// the subprocess exits non-zero and that the failure message contains both the
// expected code (0) and the actual code (1).
func TestRun_ExitCodeMismatch(t *testing.T) {
	if os.Getenv(innerTestEnvKey) == "exit_mismatch" {
		// Inner invocation: exercise the exit code mismatch path.
		// t.Parallel is intentionally omitted here to avoid pausing a
		// subprocess test that has no sibling parallel tests.
		self := os.Args[0]
		tc := DiffTest{
			Env:        []string{helperEnvKey + "=exit1"},
			WantExit:   0, // binary exits 1, want 0 → t.Errorf expected
			WantStdout: nil,
			WantStderr: nil,
		}
		tc.Run(t, self, self)
		return
	}

	t.Parallel()

	// Outer invocation: run the inner test as a subprocess and verify it fails.
	cmd := exec.Command(os.Args[0], "-test.run=^TestRun_ExitCodeMismatch$", "-test.v")
	cmd.Env = append(os.Environ(), innerTestEnvKey+"=exit_mismatch")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected inner test to fail with exit code mismatch, but it passed:\n%s", out)
		return
	}
	output := string(out)
	if !strings.Contains(output, "exit code = 1") {
		t.Errorf("expected failure message to contain 'exit code = 1', got:\n%s", output)
	}
	if !strings.Contains(output, "want 0") {
		t.Errorf("expected failure message to contain 'want 0', got:\n%s", output)
	}
}

// TestRun_NormStdout verifies that a NormStdout hook that strips trailing
// newlines causes an otherwise-mismatched stdout comparison to pass (AC4).
// The Go binary writes "hello\n" but WantStdout is "hello"; NormStdout strips
// the trailing newline so the comparison succeeds without t.Errorf.
func TestRun_NormStdout(t *testing.T) {
	t.Parallel()

	self := os.Args[0]
	tc := DiffTest{
		Env:        []string{helperEnvKey + "=hello"},
		WantExit:   0,
		WantStdout: []byte("hello"), // no trailing newline
		WantStderr: nil,
		NormStdout: func(b []byte) []byte {
			return bytes.TrimRight(b, "\n")
		},
	}
	tc.Run(t, self, self)
}
