// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/ls via differential testing against gls.
// Tests srd008-ls R1.13, R1.14, R2.1-R2.6.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

// createSortFixture creates files with distinct sizes and modification times.
// Sizes: aaa=3000, bbb=1000, ccc=2000.
// Mtimes: bbb newest (1h ago), ccc middle (2h ago), aaa oldest (3h ago).
func createSortFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "aaa"), 3000)
	writeFixtureFile(t, filepath.Join(dir, "bbb"), 1000)
	writeFixtureFile(t, filepath.Join(dir, "ccc"), 2000)
	now := time.Now()
	setMtime(t, filepath.Join(dir, "aaa"), now.Add(-3*time.Hour))
	setMtime(t, filepath.Join(dir, "bbb"), now.Add(-1*time.Hour))
	setMtime(t, filepath.Join(dir, "ccc"), now.Add(-2*time.Hour))
	return dir
}

// writeFixtureFile creates a file with the given byte size.
func writeFixtureFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setMtime sets the modification time of a file.
func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
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
		// R2.3: -d lists directory itself, not contents.
		{
			Name: "d_directory_itself",
			Args: []string{"-d", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -d with multiple directories.
		{
			Name: "d_multiple_dirs",
			Args: []string{"-d", "-1", "--color=never", dir, dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: -aA combined, -A wins (last flag).
		{
			Name: "aA_A_wins",
			Args: []string{"-aA", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: -Aa combined, -a wins (last flag).
		{
			Name: "Aa_a_wins",
			Args: []string{"-Aa", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSort tests sort flags with a fixture that has distinct sizes and mtimes.
func TestDiffSort(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createSortFixture(t)

	tests := []testutils.DiffTest{
		// R2.5: -t sorts by modification time, newest first.
		{
			Name: "t_time_sort",
			Args: []string{"-t", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.5: -t with -l shows time-sorted long format.
		{
			Name: "t_long_format",
			Args: []string{"-t", "-l", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.6: -S sorts by size, largest first.
		{
			Name: "S_size_sort",
			Args: []string{"-S", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.6: -S with -l shows size-sorted long format.
		{
			Name: "S_long_format",
			Args: []string{"-S", "-l", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
