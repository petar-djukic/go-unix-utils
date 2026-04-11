// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/vdir via differential testing against gvdir.
// Tests srd108-vdir R1.6, R2.4.
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

// createFixture creates a test directory with files for listing.
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

// normalizeProgramName replaces "gvdir:" with "vdir:" in output so
// error messages from the reference binary match the Go binary.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gvdir:"), []byte("vdir:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gvdir")
	if err != nil {
		t.Skipf("reference binary gvdir not in PATH: %v", err)
	}
	dir := createFixture(t)
	hiddenDir := createHiddenFixture(t)
	sortDir := createSortFixture(t)
	recurDir := createRecursiveFixture(t)
	symlinkDir := createSymlinkFixture(t)
	escapeDir := createEscapeFixture(t)

	tests := []testutils.DiffTest{
		// R1.1/R1.2/R1.3: default long format with C-style escaping.
		{
			Name:    "default_long_format",
			Args:    []string{dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.5: default to current directory when no args.
		{
			Name:    "default_no_args",
			Args:    []string{},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.4/-a: show all entries including . and ..
		{
			Name:    "show_all",
			Args:    []string{"-a", hiddenDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: hiddenDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.4/-A: almost all (no . and ..).
		{
			Name:    "almost_all",
			Args:    []string{"-A", hiddenDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: hiddenDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -d: list directory itself, not contents.
		{
			Name:    "directory_only",
			Args:    []string{"-d", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -h: human-readable sizes.
		{
			Name:    "human_readable",
			Args:    []string{"-h", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -R: recursive listing.
		{
			Name:    "recursive",
			Args:    []string{"-R", recurDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: recurDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -F: classify (append indicator).
		{
			Name:    "classify",
			Args:    []string{"-F", recurDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: recurDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// --color=never: no ANSI escape codes.
		{
			Name:    "color_never",
			Args:    []string{"--color=never", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -t: sort by modification time (newest first).
		{
			Name:    "time_sort",
			Args:    []string{"-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -S: sort by file size (largest first).
		{
			Name:    "size_sort",
			Args:    []string{"-S", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// -r: reverse sort.
		{
			Name:    "reverse_sort",
			Args:    []string{"-r", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// Combined -rt: reverse time sort.
		{
			Name:    "reverse_time_sort",
			Args:    []string{"-r", "-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.2: C-style escaping of special characters in long format.
		{
			Name:    "escape_special",
			Args:    []string{escapeDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: escapeDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// Symlink display in long format.
		{
			Name:    "symlink_long",
			Args:    []string{symlinkDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: symlinkDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// Combined -la: long format with all entries.
		{
			Name:    "combined_la",
			Args:    []string{"-la", hiddenDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: hiddenDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// Combined -lS: long format sorted by size.
		{
			Name:    "combined_lS",
			Args:    []string{"-lS", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: sortDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// Combined -laR: long all recursive.
		{
			Name:    "combined_laR",
			Args:    []string{"-laR", recurDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: recurDir,
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R2.2: exit code 2 for non-existent path.
		{
			Name:      "nonexistent_path",
			Args:      []string{"/no/such/path/xyzzy"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.2: one valid and one invalid argument.
		{
			Name:      "valid_and_invalid_args",
			Args:      []string{"/no/such/path/xyzzy", dir},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{
				normalizeProgramName,
				testutils.TimestampNormalizer,
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
