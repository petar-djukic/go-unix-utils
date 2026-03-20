// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pwd against gpwd (GNU coreutils).
// Implements prd051-pwd R3.1 (differential tests), R3.2 (coverage),
// R3.3 (LC_ALL=C environment).
package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer returns a NormalizeFunc that replaces
// occurrences of the Go and reference binary paths with "pwd"
// so stderr messages can be compared regardless of install path.
func binaryNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	goBase := filepath.Base(goBin)
	refBase := filepath.Base(refBin)
	return func(b []byte) []byte {
		// Replace full paths first, then basenames.
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("pwd"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("pwd"))
		b = bytes.ReplaceAll(b, []byte(refBase), []byte("pwd"))
		b = bytes.ReplaceAll(b, []byte(goBase), []byte("pwd"))
		return b
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skipf("reference binary gpwd not in PATH: %v", err)
	}
	normalize := binaryNameNormalizer(goBin, refBin)
	// R3.3: all differential tests set LC_ALL=C explicitly.
	lcEnv := []string{"LC_ALL=C"}
	tests := []testutils.DiffTest{
		{
			Name:     "no_args",
			Args:     nil,
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			Name:     "physical_flag",
			Args:     []string{"-P"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			Name:     "logical_flag",
			Args:     []string{"-L"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			Name:     "physical_long",
			Args:     []string{"--physical"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			Name:     "logical_long",
			Args:     []string{"--logical"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			// R1.4: last flag wins — -L then -P → physical
			Name:     "logical_then_physical",
			Args:     []string{"-L", "-P"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			// R1.4: last flag wins — -P then -L → logical
			Name:     "physical_then_logical",
			Args:     []string{"-P", "-L"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			Name:     "repeated_physical",
			Args:     []string{"-P", "-P"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		{
			Name:     "repeated_logical",
			Args:     []string{"-L", "-L"},
			Env:      lcEnv,
			ExitCode: 0,
		},
		// R2.1: extra operands produce warning but exit 0.
		{
			Name:      "extra_operand",
			Args:      []string{"foo"},
			Env:       lcEnv,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalize},
		},
		{
			Name:      "extra_operand_after_flags",
			Args:      []string{"-P", "bar"},
			Env:       lcEnv,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalize},
		},
		{
			Name:      "extra_operand_after_double_dash",
			Args:      []string{"--", "baz"},
			Env:       lcEnv,
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalize},
		},
		// R2.2: unknown flags produce error exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-x"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalize},
		},
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--unknown"},
			Env:       lcEnv,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalize},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
