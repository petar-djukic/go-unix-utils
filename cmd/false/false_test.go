// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for false core exit behavior and argument ignoring.
//
// Implements prd013-false R1.1, R1.2, R1.3, R1.5.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the compiled Go false binary. Set by TestMain.
var goBinary string

// refBinary is the path to the GNU gfalse reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go false binary and locates the gfalse reference binary.
// D1: skip all tests if gfalse is not on PATH.
// D1: build Go false binary into a temporary directory.
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gfalse")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gfalse not found on PATH; skipping false differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "false-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "false")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go false binary: %v\n%s", err, out)
		os.RemoveAll(binDir) // best-effort cleanup
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// TestFalseNoArgs verifies R1.1, R1.2, R1.3: false exits with code 1 and
// produces no output to stdout or stderr when invoked with no arguments.
func TestFalseNoArgs(t *testing.T) {
	t.Parallel()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "no-args-exits-1",
			ExitCode: 1,
		},
	})
}

// TestFalseIgnoresArguments verifies R1.5: false silently ignores non-flag
// arguments and still exits with code 1 with no output.
func TestFalseIgnoresArguments(t *testing.T) {
	t.Parallel()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "single-arg-ignored",
			Args:     []string{"foo"},
			ExitCode: 1,
		},
		{
			Name:     "multiple-args-ignored",
			Args:     []string{"foo", "bar", "baz"},
			ExitCode: 1,
		},
		{
			Name:     "unknown-flag-ignored",
			Args:     []string{"--unknown"},
			ExitCode: 1,
		},
	})
}
