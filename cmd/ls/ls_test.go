// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ls against gls (Homebrew GNU coreutils).
// Implements prd008-ls R1-R4 acceptance criteria.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// lsMtimeNormalizer replaces the mtime column in long-format output with a
// fixed placeholder so differential tests pass despite wall-clock differences.
var lsMtimeNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	// Match "Mon DD HH:MM" or "Mon DD  YYYY" patterns in -l output.
	// Example: "Jan  1 12:34" or "Jan  1  2025"
	re := regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d [ \d]\d:\d\d|[A-Z][a-z]{2} [ \d]\d  \d{4}`)
	return re.ReplaceAll(b, []byte("MTIME"))
}

// lsStderrNormalizer normalizes the program name in error messages so that
// both "ls:" and "gls:" match.
var lsStderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`^(g?ls): `)
	return re.ReplaceAll(b, []byte("ls: "))
}

// setupFixture creates a test fixture directory with known contents.
func setupFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ls-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Regular files.
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Hidden file.
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Subdirectory.
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Symlink.
	if err := os.Symlink("alpha", filepath.Join(dir, "link-to-alpha")); err != nil {
		t.Fatal(err)
	}

	// Executable file.
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	fixture := setupFixture(t)

	tests := []testutils.DiffTest{
		{
			// R1.2, R1.3, R1.4: Default single-column (stdout is pipe).
			Name: "ls_default_single_col_redirect",
			Args: []string{"--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R2.1: -1 forces single-column.
			Name: "ls_single_col_flag",
			Args: []string{"-1", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R2.2-R2.6: Long format.
			Name:      "ls_long_format",
			Args:      []string{"-l", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{lsMtimeNormalizer},
		},
		{
			// R2.6: Symlink display in long format.
			Name:      "ls_long_format_symlink",
			Args:      []string{"-l", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{lsMtimeNormalizer},
		},
		{
			// R3.1: -a includes . and .. and dotfiles.
			Name: "ls_filter_all",
			Args: []string{"-a", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R3.2: -A includes dotfiles but not . and ..
			Name: "ls_filter_almost_all",
			Args: []string{"-A", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R3.3: -d lists directory itself.
			Name: "ls_directory_itself",
			Args: []string{"-d", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R4.4: --color=never suppresses ANSI sequences.
			Name: "ls_color_suppressed",
			Args: []string{"--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		{
			// R5.1, R5.2: -lh human-readable sizes.
			Name:      "ls_long_human",
			Args:      []string{"-lh", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{lsMtimeNormalizer},
		},
		{
			// R6.2: Non-existent path exits 1.
			Name:      "ls_missing_file",
			Args:      []string{"--color=never", filepath.Join(fixture, "nonexistent")},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{lsStderrNormalizer},
		},
		{
			// R6.3: Invalid option exits 2.
			Name:      "ls_bad_option",
			Args:      []string{"--invalid-flag"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{lsStderrNormalizer},
		},
		{
			// R3: -la combines long format with all files.
			Name:      "ls_long_all",
			Args:      []string{"-la", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{lsMtimeNormalizer},
		},
		{
			// R3: -ld lists directory itself in long format.
			Name:      "ls_long_directory",
			Args:      []string{"-ld", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{lsMtimeNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
