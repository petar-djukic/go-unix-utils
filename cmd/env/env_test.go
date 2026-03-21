// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd039-env R1.1, R1.2, R1.3, R2.1, R2.2, R2.3,
// R3.1, R3.2, R3.3, R4.1, R4.2, R4.3: compares stdout, stderr, exit codes
// via pkg/testutils.
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
		// R2.3: NAME=VALUE without command, adds to current env.
		{Name: "set_var_no_cmd",
			Args: []string{"TESTVAR_ENV_XYZ=hello"}},
		// R1.2: execute command.
		{Name: "run_echo",
			Args: []string{echoBin, "hello", "world"}},
		// R1.2/R3.2: command exit code passthrough.
		{Name: "exit_code_false",
			Args: []string{falseBin}, ExitCode: 1},
		// R2.1 + R2.3 + command: set var and verify via printenv.
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
		// R2.2: -u unsets a variable, printenv exits 1 when not found.
		{Name: "unset_var",
			Args:     []string{"-u", "HOME", printenvBin, "HOME"},
			ExitCode: 1},
		// R2.2: --unset=NAME long form.
		{Name: "unset_long_form",
			Args:     []string{"--unset=HOME", printenvBin, "HOME"},
			ExitCode: 1},
		// R2.2: -uNAME short form without space.
		{Name: "unset_short_no_space",
			Args:     []string{"-uHOME", printenvBin, "HOME"},
			ExitCode: 1},
		// R2.2: multiple -u flags.
		{Name: "unset_multiple",
			Args:     []string{"-u", "HOME", "-u", "USER", printenvBin, "HOME"},
			ExitCode: 1},
		// R2.2: -u with -i (no-op since env is already empty).
		{Name: "unset_with_ignore",
			Args: []string{"-i", "-u", "HOME", "FOO=bar"}},
		// R2.3: NAME=VALUE overrides existing variable.
		{Name: "override_existing_var",
			Args: []string{"HOME=/tmp/override_test", printenvBin, "HOME"}},
		// R2.3: multiple NAME=VALUE pairs with command.
		{Name: "multiple_assignments",
			Args: []string{"-i", "A=1", "B=2", "C=3", printenvBin, "A"}},
		// R3.1: -0 NUL-delimited output.
		{Name: "null_output",
			Args: []string{"-i", "-0", "A=1", "B=2"}},
		// R3.1: --null long form.
		{Name: "null_long_form",
			Args: []string{"-i", "--null", "X=y"}},
		// R3.2: exit code passthrough with specific code via sh -c.
		{Name: "exit_code_passthrough_42",
			Args:     []string{"sh", "-c", "exit 42"},
			ExitCode: 42},
		// R3.3/R4.3: invalid long option exits 125.
		{Name: "invalid_long_option",
			Args:      []string{"--invalid-xyz"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R3.3/R4.3: invalid short option exits 125.
		{Name: "invalid_short_option",
			Args:      []string{"-x"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R3.3: combined flags with invalid char exits 125.
		{Name: "combined_flags_invalid",
			Args:      []string{"-ix"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{normBin}},
		// R4.2: combined short flags -i0 work together.
		{Name: "combined_flags_i0",
			Args: []string{"-i0", "A=1", "B=2"}},
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
