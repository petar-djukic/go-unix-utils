// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd051-pwd R2.1-R2.2, R3.1-R3.3: differential tests for cmd/pwd
// against gpwd covering all flags, error handling, --version, --help, and edge cases.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpwd")
	if err != nil {
		t.Skipf("reference binary gpwd not in PATH: %v", err)
	}

	// Create a symlinked directory for logical vs physical divergence tests.
	// R3.2: tests must cover symlinked working directory.
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "realdir")
	symDir := filepath.Join(tmpDir, "symdir")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("creating realdir: %v", err)
	}
	if err := os.Symlink(realDir, symDir); err != nil {
		t.Fatalf("creating symdir: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: default invocation (physical mode).
		{
			Name: "default_no_flags",
			Args: []string{},
		},
		// R1.2: logical mode.
		{
			Name: "logical_flag",
			Args: []string{"-L"},
		},
		// R1.3: physical mode.
		{
			Name: "physical_flag",
			Args: []string{"-P"},
		},
		// R1.4: -L -P last wins → physical.
		{
			Name: "logical_then_physical",
			Args: []string{"-L", "-P"},
		},
		// R1.4: -P -L last wins → logical.
		{
			Name: "physical_then_logical",
			Args: []string{"-P", "-L"},
		},
		// R1.4: combined short flags, last wins → physical.
		{
			Name: "combined_LP",
			Args: []string{"-LP"},
		},
		// R1.4: combined short flags, last wins → logical.
		{
			Name: "combined_PL",
			Args: []string{"-PL"},
		},
		// R1.2/R1.3: symlinked directory with -L should print symlink path,
		// -P should print resolved path.
		{
			Name:    "symlink_dir_logical",
			Args:    []string{"-L"},
			WorkDir: symDir,
			Env:     []string{"PWD=" + symDir},
		},
		// R1.3: physical mode in symlinked directory.
		{
			Name:    "symlink_dir_physical",
			Args:    []string{"-P"},
			WorkDir: symDir,
			Env:     []string{"PWD=" + symDir},
		},
		// R1.2: --logical long form.
		{
			Name: "logical_long_flag",
			Args: []string{"--logical"},
		},
		// R1.3: --physical long form.
		{
			Name: "physical_long_flag",
			Args: []string{"--physical"},
		},
		// R1.4: --logical --physical last wins → physical.
		{
			Name: "logical_long_then_physical_long",
			Args: []string{"--logical", "--physical"},
		},
		// R1.4: --physical --logical last wins → logical.
		{
			Name: "physical_long_then_logical_long",
			Args: []string{"--physical", "--logical"},
		},
		// R1.4: mixed short and long flags.
		{
			Name: "short_L_then_long_physical",
			Args: []string{"-L", "--physical"},
		},
		// R1.4: mixed long and short flags.
		{
			Name: "long_logical_then_short_P",
			Args: []string{"--logical", "-P"},
		},
		// R3.1/R3.3: --version prints version info to stdout and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeVersion},
		},
		// R3.2/R3.3: --help prints usage info to stdout and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelp},
		},
		// R2.1: GNU pwd ignores extra operands with a warning, still exits 0.
		{
			Name:      "R2.1_extra_operand",
			Args:      []string{"extra"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.1: positional operand after valid flags — still prints cwd, exits 0.
		{
			Name:      "R2.1_operand_after_flags",
			Args:      []string{"-P", "extra"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.1: positional operand after -- — still prints cwd, exits 0.
		{
			Name:      "R2.1_operand_after_double_dash",
			Args:      []string{"--", "extra"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.2: unknown short flag.
		{
			Name:      "R2.2_unknown_short_flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R2.2: unknown long flag.
		{
			Name:      "R2.2_unknown_long_flag",
			Args:      []string{"--foobar"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeVersion reduces version output to just the program name so different
// version strings (dev vs GNU) don't cause divergence. Both binaries must
// produce output starting with the program name.
func normalizeVersion(b []byte) []byte {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[:i+1]
	}
	// Keep only the program name portion before the first space.
	if i := bytes.IndexByte(b, ' '); i >= 0 {
		return append(b[:i], '\n')
	}
	return b
}

// normalizeHelp reduces help output to a fixed token since the exact help text
// differs between implementations. Both must exit 0 and produce some stdout.
func normalizeHelp(b []byte) []byte {
	if len(b) > 0 {
		return []byte("help\n")
	}
	return b
}

// normalizeStderr clears stderr content since error message format may differ
// between implementations. Both must exit with the same code.
func normalizeStderr(b []byte) []byte {
	return nil
}
