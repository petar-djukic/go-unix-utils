// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the name of the Homebrew GNU ls reference binary.
const refBinaryName = "gls"

// stderrNormalizer normalizes stderr by replacing the reference binary path
// with "ls" and removing the "Try ... --help" line so comparison ignores
// binary name and path differences.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	// Replace full binary paths (e.g., "/opt/homebrew/bin/gls:" or "/opt/homebrew/bin/ls:")
	re := regexp.MustCompile(`[^\s]*(?:gls|/ls):`)
	result := re.ReplaceAll(b, []byte("ls:"))
	// Remove "Try ... --help" lines.
	reHelp := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	result = reHelp.ReplaceAll(result, nil)
	return result
}

// lsMtimeNormalizer replaces modification time columns in -l output with
// a fixed placeholder to eliminate wall-clock divergence between runs.
// Matches patterns like "Jan  1 12:34" or "Jan  1  2025".
var lsMtimeNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	// Match "Mon DD HH:MM" or "Mon DD  YYYY" (the mtime field in ls -l output).
	re := regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+(\d{2}:\d{2}|\s?\d{4})`)
	return re.ReplaceAll(b, []byte("MTIME"))
}

// setupFixture creates a test fixture directory with known structure:
//
//	ls-fixture/
//	  .hidden       (dotfile, 0 bytes)
//	  file_a.txt    (1024 bytes)
//	  file_b.txt    (2048 bytes)
//	  file_c.txt    (512 bytes)
//	  subdir/       (directory)
//	    nested.txt  (256 bytes)
//	  link_to_a -> file_a.txt (symlink)
//	  exec_file    (executable, 100 bytes)
func setupFixture(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "ls-fixture")

	sub := filepath.Join(base, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(base, ".hidden"), 0)
	writeFile(t, filepath.Join(base, "file_a.txt"), 1024)
	writeFile(t, filepath.Join(base, "file_b.txt"), 2048)
	writeFile(t, filepath.Join(base, "file_c.txt"), 512)
	writeFile(t, filepath.Join(sub, "nested.txt"), 256)

	// Symlink.
	if err := os.Symlink("file_a.txt", filepath.Join(base, "link_to_a")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Executable file.
	writeFile(t, filepath.Join(base, "exec_file"), 100)
	if err := os.Chmod(filepath.Join(base, "exec_file"), 0o755); err != nil {
		t.Fatal(err)
	}

	return base
}

// writeFile writes n zero bytes to the given path.
func writeFile(t *testing.T, path string, n int) {
	t.Helper()
	data := make([]byte, n)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// longNorm combines mtime normalization and stderr normalization.
var longNorm = []testutils.NormalizeFunc{lsMtimeNormalizer, stderrNormalizer}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := setupFixture(t)

	tests := []testutils.DiffTest{
		// R1.2: Default single-column when stdout is redirected (not a TTY).
		{
			Name: "ls_default_single_col",
			Args: []string{"--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -1 forces single-column output.
		{
			Name: "ls_single_col_flag",
			Args: []string{"-1", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.6-R1.10: -l long format.
		{
			Name:      "ls_long_format",
			Args:      []string{"-l", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// R3.5, R3.6: -lh human-readable sizes.
		{
			Name:      "ls_long_human",
			Args:      []string{"-lh", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// R2.1: -a includes all entries including . and ..
		{
			Name: "ls_filter_all",
			Args: []string{"-a", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -A includes dotfiles but excludes . and ..
		{
			Name: "ls_filter_almost_all",
			Args: []string{"-A", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -d shows directory itself.
		{
			Name: "ls_directory_itself",
			Args: []string{"-d", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --color=never suppresses ANSI.
		{
			Name: "ls_color_never",
			Args: []string{"--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.5: -t sort by time.
		{
			Name: "ls_sort_time",
			Args: []string{"-1t", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.6: -S sort by size.
		{
			Name: "ls_sort_size",
			Args: []string{"-1S", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7: -r reverse sort.
		{
			Name: "ls_reverse",
			Args: []string{"-1r", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7: -tr reverse time sort.
		{
			Name: "ls_reverse_time",
			Args: []string{"-1tr", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.11: -R recursive listing.
		{
			Name: "ls_recursive",
			Args: []string{"-R", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.8: -F classify.
		{
			Name: "ls_classify",
			Args: []string{"-1F", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.8: -p append / to directories.
		{
			Name: "ls_append_slash",
			Args: []string{"-1p", "--color=never", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// Combined -la.
		{
			Name:      "ls_long_all",
			Args:      []string{"-la", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// Combined -lt.
		{
			Name:      "ls_long_time",
			Args:      []string{"-lt", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// File argument.
		{
			Name:      "ls_file_arg",
			Args:      []string{"-l", "--color=never", filepath.Join(fixture, "file_a.txt")},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// Multiple directory arguments.
		{
			Name: "ls_multiple_dirs",
			Args: []string{"-1", "--color=never", fixture, filepath.Join(fixture, "subdir")},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2/R4.3: Nonexistent path (only arg) exits 2.
		{
			Name:      "ls_missing_file",
			Args:      []string{"--color=never", "nonexistent_path_xyzzy"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.3: Invalid option exits 2.
		{
			Name:      "ls_bad_option",
			Args:      []string{"--invalid-flag"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R2.14: -n numeric IDs.
		{
			Name:      "ls_numeric_ids",
			Args:      []string{"-n", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// R2.3: -ld shows directory long format.
		{
			Name:      "ls_directory_long",
			Args:      []string{"-ld", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
		// -lR recursive long format.
		{
			Name:      "ls_recursive_long",
			Args:      []string{"-lR", "--color=never", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: longNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
