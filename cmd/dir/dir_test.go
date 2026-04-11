// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/dir via differential testing against gdir.
// Tests srd107-dir R1.1-R1.5, R2.1-R2.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// createFixture creates a test directory with files for column layout testing.
func createFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// createMixedCaseFixture creates a directory with mixed-case filenames
// to verify C locale sort order (uppercase before lowercase).
func createMixedCaseFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"Bravo", "alpha", "Charlie", "delta", "Echo",
		"FOXTROT", "golf", "Hotel", "india",
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// createHiddenFixture creates a directory with hidden and visible files.
func createHiddenFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"alpha", "bravo", ".hidden", ".secret", "charlie",
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// createSortFixture creates files with distinct sizes and modification
// times for -t and -S sort testing.
// Sizes: aaa=3000, bbb=1000, ccc=2000.
// Mtimes: bbb newest (1h ago), ccc middle (2h ago), aaa oldest (3h ago).
func createSortFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "aaa"), 3000)
	writeSizedFile(t, filepath.Join(dir, "bbb"), 1000)
	writeSizedFile(t, filepath.Join(dir, "ccc"), 2000)
	now := time.Now()
	setMtime(t, filepath.Join(dir, "aaa"), now.Add(-3*time.Hour))
	setMtime(t, filepath.Join(dir, "bbb"), now.Add(-1*time.Hour))
	setMtime(t, filepath.Join(dir, "ccc"), now.Add(-2*time.Hour))
	return dir
}

// writeSizedFile creates a file with the given byte count.
func writeSizedFile(t *testing.T, path string, size int) {
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

// createRecursiveFixture creates a directory tree with subdirectories.
func createRecursiveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSmallFile(t, filepath.Join(dir, "top.txt"))
	writeSmallFile(t, filepath.Join(dir, "sub1", "a.txt"))
	writeSmallFile(t, filepath.Join(dir, "sub1", "b.txt"))
	writeSmallFile(t, filepath.Join(dir, "sub2", "c.txt"))
	return dir
}

// createSymlinkFixture creates a directory with a regular file and
// a symlink pointing to it.
func createSymlinkFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSmallFile(t, filepath.Join(dir, "target"))
	err := os.Symlink("target", filepath.Join(dir, "link"))
	if err != nil {
		t.Fatal(err)
	}
	writeSmallFile(t, filepath.Join(dir, "other"))
	return dir
}

// createEscapeFixture creates a directory with filenames that require
// C-style backslash escaping (spaces, tabs, backslashes).
func createEscapeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	names := []string{
		"normal",
		"with space",
		"with\ttab",
		"back\\slash",
	}
	for _, n := range names {
		err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeSmallFile creates a small text file.
func writeSmallFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// normalizeProgramName replaces "gdir:" with "dir:" in output so
// error messages from the reference binary match the Go binary.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gdir:"), []byte("dir:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdir")
	if err != nil {
		t.Skipf("reference binary gdir not in PATH: %v", err)
	}
	dir := createFixture(t)
	mixedDir := createMixedCaseFixture(t)
	hiddenDir := createHiddenFixture(t)
	sortDir := createSortFixture(t)
	recurDir := createRecursiveFixture(t)
	symlinkDir := createSymlinkFixture(t)
	escapeDir := createEscapeFixture(t)

	tests := []testutils.DiffTest{
		// R1.1: multi-column output by default.
		{
			Name:    "default_multicolumn",
			Args:    []string{dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.4: default to current directory when no args.
		{
			Name:    "default_no_args",
			Args:    []string{},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.3: sort in C locale order.
		{
			Name:    "sorted_output",
			Args:    []string{dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: accepts -l flag.
		{
			Name: "long_format",
			Args: []string{"-l", dir},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.5: accepts -a flag (show all entries including . and ..).
		{
			Name:    "show_all",
			Args:    []string{"-a", hiddenDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: hiddenDir,
		},
		// R1.5: accepts -A flag (almost all, no . and ..).
		{
			Name:    "almost_all",
			Args:    []string{"-A", hiddenDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: hiddenDir,
		},
		// R1.5: accepts -1 flag (one per line).
		{
			Name:    "one_per_line",
			Args:    []string{"-1", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
		},
		// R1.5: accepts -r flag (reverse sort).
		{
			Name:    "reverse_sort",
			Args:    []string{"-r", dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: accepts --color=never flag.
		{
			Name:    "color_never",
			Args:    []string{"--color=never", dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: mixed-case filenames sorted in C locale order.
		{
			Name:    "mixed_case_sort",
			Args:    []string{"-1", mixedDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: mixedDir,
		},
		// R1.5: mixed-case sort with multi-column format.
		{
			Name:    "mixed_case_sort_columns",
			Args:    []string{mixedDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: mixedDir,
		},
		// R1.5: -t sort by modification time (newest first).
		{
			Name:    "time_sort",
			Args:    []string{"-1", "-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
		},
		// R1.5: -S sort by file size (largest first).
		{
			Name:    "size_sort",
			Args:    []string{"-1", "-S", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
		},
		// R1.5: -R recursive listing.
		{
			Name:    "recursive",
			Args:    []string{"-R", recurDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: recurDir,
		},
		// R1.5: -i show inode numbers.
		{
			Name:    "inode",
			Args:    []string{"-i", "-1", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
		},
		// R1.5: -s show allocated size in blocks.
		{
			Name:    "size_blocks",
			Args:    []string{"-s", dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: combined -la (long format with all entries).
		{
			Name:    "combined_la",
			Args:    []string{"-la", hiddenDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: hiddenDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.5: combined -lS (long format sorted by size).
		{
			Name:    "combined_lS",
			Args:    []string{"-lS", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.5: combined -rt (reverse time sort).
		{
			Name:    "reverse_time_sort",
			Args:    []string{"-1", "-r", "-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
		},
		// R1.5: combined -laR (long all recursive).
		{
			Name:    "combined_laR",
			Args:    []string{"-laR", recurDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: recurDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.5: symlink display in default mode.
		{
			Name:    "symlink_default",
			Args:    []string{symlinkDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: symlinkDir,
		},
		// R1.5: symlink display in long format.
		{
			Name:    "symlink_long",
			Args:    []string{"-l", symlinkDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: symlinkDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.2: C-style escaping of special characters.
		{
			Name:    "escape_special",
			Args:    []string{"-1", escapeDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: escapeDir,
		},
		// R1.2: escaping in multi-column format.
		{
			Name:    "escape_multicolumn",
			Args:    []string{escapeDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: escapeDir,
		},
		// R1.5: -F classify (append indicator).
		{
			Name:    "classify",
			Args:    []string{"-F", recurDir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: recurDir,
		},
		// R2.1/R2.2/R2.3: exit code for non-existent path.
		{
			Name:      "nonexistent_path",
			Args:      []string{"/no/such/path/xyzzy"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.2/R2.3: one valid and one invalid argument.
		{
			Name:      "valid_and_invalid_args",
			Args:      []string{"/no/such/path/xyzzy", dir},
			Env:       []string{"LC_ALL=C", "COLUMNS=80"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.2/R2.3: two invalid arguments continue processing both.
		{
			Name:      "two_invalid_args",
			Args:      []string{"/no/such/path/aaa", "/no/such/path/bbb"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
