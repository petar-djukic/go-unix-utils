// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for true core exit behavior and argument ignoring.
//
// Implements prd012-true R1.1, R1.2, R1.3, R1.5.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinary is the path to the compiled Go true binary. Set by TestMain.
var goBinary string

// refBinary is the path to the GNU gtrue reference binary. Set by TestMain.
var refBinary string

// TestMain builds the Go true binary and locates the gtrue reference binary.
// D1: skip all tests if gtrue is not on PATH.
// D1: build Go true binary into a temporary directory.
func TestMain(m *testing.M) {
	ref, err := exec.LookPath("gtrue")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gtrue not found on PATH; skipping true differential tests")
		os.Exit(0)
	}
	refBinary = ref

	binDir, err := os.MkdirTemp("", "true-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating bin dir: %v\n", err)
		os.Exit(1)
	}

	goBinary = filepath.Join(binDir, "true")
	cmd := exec.Command("go", "build", "-o", goBinary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building Go true binary: %v\n%s", err, out)
		os.RemoveAll(binDir) // best-effort cleanup
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(binDir) // best-effort cleanup
	os.Exit(code)
}

// TestTrueNoArgs verifies R1.1, R1.2, R1.3: true exits with code 0 and
// produces no output to stdout or stderr when invoked with no arguments.
func TestTrueNoArgs(t *testing.T) {
	t.Parallel()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "no-args-exits-0",
			ExitCode: 0,
		},
	})
}

// TestTrueIgnoresArguments verifies R1.5: true silently ignores non-flag
// arguments and still exits with code 0 with no output.
func TestTrueIgnoresArguments(t *testing.T) {
	t.Parallel()

	testutils.RunDiffTests(t, goBinary, refBinary, []testutils.DiffTest{
		{
			Name:     "single-arg-ignored",
			Args:     []string{"foo"},
			ExitCode: 0,
		},
		{
			Name:     "multiple-args-ignored",
			Args:     []string{"foo", "bar", "baz"},
			ExitCode: 0,
		},
		{
			Name:     "unknown-flag-ignored",
			Args:     []string{"--unknown"},
			ExitCode: 0,
		},
	})
}
