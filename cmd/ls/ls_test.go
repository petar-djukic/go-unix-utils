// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/ls via differential testing against gls.
// Tests srd008-ls R1.13, R1.14, R2.1, R2.2.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// createFixture creates a test directory with files and a dotfile.
func createFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
		".hidden", ".secret",
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R1.13: -x horizontal multi-column output.
		{
			Name: "x_horizontal",
			Args: []string{"-x", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.13: -x with single-column fallback (few entries).
		{
			Name: "x_single_entry",
			Args: []string{"-x", "--color=never", "-a", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -C after -l overrides to multi-column.
		{
			Name: "C_after_l",
			Args: []string{"-l", "-C", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -l after -C overrides to long format.
		{
			Name: "l_after_C",
			Args: []string{"-C", "-l", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -x after -1 overrides to horizontal multi-column.
		{
			Name: "x_after_1",
			Args: []string{"-1", "-x", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -1 after -x overrides to single-column.
		{
			Name: "1_after_x",
			Args: []string{"-x", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: combined flags in single arg, last wins (-lC → -C wins).
		{
			Name: "combined_lC",
			Args: []string{"-lC", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: combined flags in single arg (-Cl → -l wins).
		{
			Name: "combined_Cl",
			Args: []string{"-Cl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -a includes . and .. entries.
		{
			Name: "a_all",
			Args: []string{"-a", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -A includes dotfiles except . and ..
		{
			Name: "A_almost_all",
			Args: []string{"-A", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1/R2.2: default hides dotfiles.
		{
			Name: "default_no_dots",
			Args: []string{"-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -x after -l in combined flags (-lx → -x wins).
		{
			Name: "combined_lx",
			Args: []string{"-lx", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
