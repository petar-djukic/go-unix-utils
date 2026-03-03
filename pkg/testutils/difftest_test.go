// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for RunDiffTests core comparison: stdout, stderr, and exit code
// matching and divergence detection (prd001-testutils R2.1, R3.2, R3.3, R3.4, R3.6).
package testutils_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// Helper program source code constants. Each produces controlled stdout,
// stderr, and exit code for use as goBinary/refBinary inputs to RunDiffTests.

// noopSource produces no output and exits 0.
const noopSource = `package main

func main() {}
`

// stdoutHelloSource writes "hello\n" to stdout and exits 0.
const stdoutHelloSource = `package main

import "fmt"

func main() { fmt.Println("hello") }
`

// stdoutWorldSource writes "world\n" to stdout and exits 0.
const stdoutWorldSource = `package main

import "fmt"

func main() { fmt.Println("world") }
`

// stderrHelloSource writes "hello\n" to stderr and exits 0.
const stderrHelloSource = `package main

import (
	"fmt"
	"os"
)

func main() { fmt.Fprintln(os.Stderr, "hello") }
`

// stderrWorldSource writes "world\n" to stderr and exits 0.
const stderrWorldSource = `package main

import (
	"fmt"
	"os"
)

func main() { fmt.Fprintln(os.Stderr, "world") }
`

// exit1Source exits with code 1 and produces no output.
const exit1Source = `package main

import "os"

func main() { os.Exit(1) }
`

// binDir holds the path to compiled test helper binaries. Set by TestMain.
var binDir string

// helperBin returns the absolute path to a compiled test helper binary.
func helperBin(name string) string {
	return filepath.Join(binDir, name)
}

// buildHelper compiles a Go source string into a named binary under binDir.
func buildHelper(name, source string) error {
	srcDir, err := os.MkdirTemp("", "src-"+name+"-*")
	if err != nil {
		return fmt.Errorf("creating source dir: %w", err)
	}
	defer os.RemoveAll(srcDir) // best-effort cleanup

	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(source), 0o644); err != nil {
		return fmt.Errorf("writing source: %w", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte("module helper\n\ngo 1.21\n"), 0o644); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	outPath := filepath.Join(binDir, name)
	cmd := exec.Command("go", "build", "-o", outPath, ".")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compiling %s: %w\n%s", name, err, out)
	}

	return nil
}

// TestMain compiles test helper binaries before running tests. In subprocess
// mode (HELPER_BIN_DIR set), it reuses pre-built binaries from the parent.
func TestMain(m *testing.M) {
	// Subprocess mode: reuse pre-built binaries from parent process.
	if dir := os.Getenv("HELPER_BIN_DIR"); dir != "" {
		binDir = dir
		os.Exit(m.Run())
	}

	var err error
	binDir, err = os.MkdirTemp("", "difftest-helpers-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	helpers := []struct {
		name   string
		source string
	}{
		{"noop", noopSource},
		{"stdout-hello", stdoutHelloSource},
		{"stdout-world", stdoutWorldSource},
		{"stderr-hello", stderrHelloSource},
		{"stderr-world", stderrWorldSource},
		{"exit1", exit1Source},
	}

	for _, h := range helpers {
		if err := buildHelper(h.name, h.source); err != nil {
			fmt.Fprintf(os.Stderr, "building helper %s: %v\n", h.name, err)
			os.RemoveAll(binDir) // best-effort cleanup
			os.Exit(1)
		}
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// TestRunDiffTests_Matching verifies R2.1, R3.2, R3.3, R3.4: RunDiffTests
// passes when both binaries produce identical stdout, stderr, and exit code.
func TestRunDiffTests_Matching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		goBin  string
		refBin string
	}{
		{"no-output", helperBin("noop"), helperBin("noop")},
		{"matching-stdout", helperBin("stdout-hello"), helperBin("stdout-hello")},
		{"matching-stderr", helperBin("stderr-hello"), helperBin("stderr-hello")},
		{"matching-exit-code", helperBin("exit1"), helperBin("exit1")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testutils.RunDiffTests(t, tc.goBin, tc.refBin, []testutils.DiffTest{
				{Name: "match"},
			})
		})
	}
}

// TestRunDiffTests_StdoutDivergence verifies R3.2: RunDiffTests detects and
// reports stdout differences between the go and reference binaries.
func TestRunDiffTests_StdoutDivergence(t *testing.T) {
	t.Parallel()

	if os.Getenv("EXPECT_FAIL") == "stdout" {
		testutils.RunDiffTests(t, helperBin("stdout-hello"), helperBin("stdout-world"), []testutils.DiffTest{
			{Name: "detect-stdout-diff"},
		})
		return
	}

	out := runExpectingFailure(t, "stdout", "TestRunDiffTests_StdoutDivergence")
	if !bytes.Contains(out, []byte("stdout differs")) {
		t.Fatalf("expected stdout divergence in failure output, got:\n%s", out)
	}
}

// TestRunDiffTests_StderrDivergence verifies R3.3: RunDiffTests detects and
// reports stderr differences between the go and reference binaries.
func TestRunDiffTests_StderrDivergence(t *testing.T) {
	t.Parallel()

	if os.Getenv("EXPECT_FAIL") == "stderr" {
		testutils.RunDiffTests(t, helperBin("stderr-hello"), helperBin("stderr-world"), []testutils.DiffTest{
			{Name: "detect-stderr-diff"},
		})
		return
	}

	out := runExpectingFailure(t, "stderr", "TestRunDiffTests_StderrDivergence")
	if !bytes.Contains(out, []byte("stderr differs")) {
		t.Fatalf("expected stderr divergence in failure output, got:\n%s", out)
	}
}

// TestRunDiffTests_ExitCodeDivergence verifies R3.4: RunDiffTests detects and
// reports exit code differences between the go and reference binaries.
func TestRunDiffTests_ExitCodeDivergence(t *testing.T) {
	t.Parallel()

	if os.Getenv("EXPECT_FAIL") == "exitcode" {
		testutils.RunDiffTests(t, helperBin("noop"), helperBin("exit1"), []testutils.DiffTest{
			{Name: "detect-exitcode-diff"},
		})
		return
	}

	out := runExpectingFailure(t, "exitcode", "TestRunDiffTests_ExitCodeDivergence")
	if !bytes.Contains(out, []byte("exit code:")) {
		t.Fatalf("expected exit code divergence in failure output, got:\n%s", out)
	}
}

// runExpectingFailure re-invokes the current test binary as a subprocess with
// EXPECT_FAIL set. It expects the subprocess to fail and returns its combined
// output. If the subprocess unexpectedly passes, the test fails.
func runExpectingFailure(t *testing.T, failType, testName string) []byte {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$", "-test.v")
	cmd.Env = append(os.Environ(),
		"EXPECT_FAIL="+failType,
		"HELPER_BIN_DIR="+binDir,
	)

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected subprocess to fail for %s divergence, but it passed:\n%s", failType, out)
	}

	return out
}
