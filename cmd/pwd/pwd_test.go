// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd051-pwd R3.1-R3.3: differential tests for cmd/pwd against gpwd.
package main

import (
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
		{
			// R1.1: default invocation (physical mode).
			Name: "default_no_flags",
			Args: []string{},
		},
		{
			// R1.2: logical mode.
			Name: "logical_flag",
			Args: []string{"-L"},
		},
		{
			// R1.3: physical mode.
			Name: "physical_flag",
			Args: []string{"-P"},
		},
		{
			// R1.4: -L -P last wins → physical.
			Name: "logical_then_physical",
			Args: []string{"-L", "-P"},
		},
		{
			// R1.4: -P -L last wins → logical.
			Name: "physical_then_logical",
			Args: []string{"-P", "-L"},
		},
		{
			// R1.4: combined short flags, last wins → physical.
			Name: "combined_LP",
			Args: []string{"-LP"},
		},
		{
			// R1.4: combined short flags, last wins → logical.
			Name: "combined_PL",
			Args: []string{"-PL"},
		},
		{
			// R1.2/R1.3: symlinked directory with -L should print symlink path,
			// -P should print resolved path.
			Name:    "symlink_dir_logical",
			Args:    []string{"-L"},
			WorkDir: symDir,
			Env:     []string{"PWD=" + symDir},
		},
		{
			// R1.3: physical mode in symlinked directory.
			Name:    "symlink_dir_physical",
			Args:    []string{"-P"},
			WorkDir: symDir,
			Env:     []string{"PWD=" + symDir},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
