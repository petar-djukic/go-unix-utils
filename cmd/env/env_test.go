// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd039-env R1.1, R1.2, R1.3, R2.1:
// compares stdout, stderr, exit codes via pkg/testutils.
package main_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests for prd039-env comparing the Go
// binary against the GNU reference binary (genv).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("genv")
	if err != nil {
		t.Skipf("reference binary genv not in PATH: %v", err)
	}
	normBin := makeBinaryNormalizer(refBin)
	echoBin := findBinaryOrSkip(t, "echo")
	falseBin := findBinaryOrSkip(t, "false")
	printenvBin := findBinaryOrSkip(t, "printenv")

	tests := []testutils.DiffTest{
		// R1.1: no args, print full environment.
		{Name: "no_args"},
		// R2.1: -i with no command, empty output.
		{Name: "ignore_env_empty", Args: []string{"-i"}},
		// R2.1: -i with NAME=VALUE, print only assignments.
		{Name: "ignore_env_with_vars",
			Args: []string{"-i", "FOO=bar", "BAZ=qux"}},
		// R2.1: bare dash implies -i.
		{Name: "dash_implies_i", Args: []string{"-", "X=1"}},
		// R2.1: --ignore-environment long form.
		{Name: "ignore_env_long",
			Args: []string{"--ignore-environment", "A=1"}},
		// NAME=VALUE without command, adds to current env.
		{Name: "set_var_no_cmd",
			Args: []string{"TESTVAR_ENV_XYZ=hello"}},
		// R1.2: execute command.
		{Name: "run_echo",
			Args: []string{echoBin, "hello", "world"}},
		// R1.2/R1.3: command exit code passthrough.
		{Name: "exit_code_false",
			Args: []string{falseBin}, ExitCode: 1},
		// R2.1 + NAME=VALUE + command: set var and verify via printenv.
		{Name: "set_var_with_cmd",
			Args: []string{"-i", "MYVAR=test_value", printenvBin, "MYVAR"}},
		// R1.3: command not found, exit 127.
		{Name: "command_not_found",
			Args:      []string{"nonexistent_command_xyz_99"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// --version prints version info.
		{Name: "version", Args: []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNormalizer, normBin}},
		// --help prints usage info.
		{Name: "help", Args: []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNormalizer}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// findBinaryOrSkip locates a binary in PATH or skips the test.
func findBinaryOrSkip(t *testing.T, name string) string {
	t.Helper()
	bin, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s not found in PATH: %v", name, err)
	}
	return bin
}

// makeBinaryNormalizer returns a normalizer that replaces the reference
// binary's path and name with "env", then lowercases everything.
func makeBinaryNormalizer(refBin string) testutils.NormalizeFunc {
	refDir := filepath.Dir(refBin)
	return func(data []byte) []byte {
		data = bytes.ReplaceAll(data, []byte(refBin), []byte("env"))
		if refDir != "" {
			data = bytes.ReplaceAll(data, []byte(refDir+"/env"), []byte("env"))
		}
		data = bytes.ReplaceAll(data, []byte("genv"), []byte("env"))
		return bytes.ToLower(data)
	}
}

// versionNormalizer reduces version output to just the program name.
func versionNormalizer(data []byte) []byte {
	if idx := bytes.IndexByte(data, ' '); idx >= 0 {
		return data[:idx]
	}
	return data
}

// helpNormalizer replaces all output with nil so --help tests compare
// only exit codes, not implementation-specific text.
func helpNormalizer(data []byte) []byte {
	return nil
}
