// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against the GNU reference binary gtrue.
//
// Implements prd020-true R1, R2, R3, R5 via differential testing
// using pkg/testutils.RunDiffTests.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the Go true binary built in TestMain.
// refBinaryPath is the path to the GNU reference binary (gtrue).
var (
	goBinaryPath  string
	refBinaryPath string
)

func TestMain(m *testing.M) {
	// Locate GNU reference binary gtrue (Homebrew coreutils).
	refPath, err := exec.LookPath("gtrue")
	if err != nil {
		fmt.Println("gtrue not found on PATH; skipping true differential tests")
		os.Exit(0)
	}
	refBinaryPath = refPath

	// Build the Go true binary from the current package.
	tmpDir, err := os.MkdirTemp("", "true-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinaryPath = filepath.Join(tmpDir, "true")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building true: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// clearStdout returns a NormalizeFunc that replaces stdout with an empty slice.
// Used for --help and --version tests where output text differs between Go and
// GNU implementations but exit code and stderr must match.
func clearStdout(b []byte) []byte {
	return nil
}

// ---------------------------------------------------------------------------
// R1: Default behavior and exit status (prd020-true R1)
// ---------------------------------------------------------------------------

func TestTrue_DefaultBehavior(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name: "no_args_exit_0",
			Args: nil,
		},
		{
			Name: "single_arbitrary_arg",
			Args: []string{"foo"},
		},
		{
			Name: "multiple_arbitrary_args",
			Args: []string{"foo", "bar", "--baz"},
		},
		{
			Name: "double_dash_ignored",
			Args: []string{"--"},
		},
		{
			Name: "args_after_double_dash",
			Args: []string{"--", "something"},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R2: --help and --version as first argument (prd020-true R2)
// ---------------------------------------------------------------------------

func TestTrue_HelpVersion(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name: "help_first_arg",
			Args: []string{"--help"},
		},
		{
			Name: "version_first_arg",
			Args: []string{"--version"},
		},
		{
			Name: "help_with_trailing_args",
			Args: []string{"--help", "extra", "args"},
		},
		{
			Name: "version_with_trailing_args",
			Args: []string{"--version", "extra"},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, []testutils.NormalizeFunc{clearStdout}, tests)
}

// ---------------------------------------------------------------------------
// R3: Ignored arguments (prd020-true R3)
// ---------------------------------------------------------------------------

func TestTrue_IgnoredArgs(t *testing.T) {
	tests := []testutils.DiffTest{
		{
			Name: "help_as_second_arg_ignored",
			Args: []string{"something", "--help"},
		},
		{
			Name: "version_as_second_arg_ignored",
			Args: []string{"something", "--version"},
		},
		{
			Name: "short_flag_n_ignored",
			Args: []string{"-n"},
		},
		{
			Name: "short_flag_v_ignored",
			Args: []string{"-v"},
		},
		{
			Name: "mixed_flags_and_args_ignored",
			Args: []string{"-x", "foo", "--unknown", "bar"},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}
