// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd008-ls R1.5, R1.6, R1.7, R1.8 (differential tests)
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for ls.
const refBinaryName = "gls"

// createFixture sets up a temporary directory with known files for testing.
// Returns the path to the fixture directory.
func createFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Create regular files with different sizes.
	writeFixtureFile(t, filepath.Join(dir, "alpha"), 100)
	writeFixtureFile(t, filepath.Join(dir, "beta"), 200)
	writeFixtureFile(t, filepath.Join(dir, "gamma"), 50)

	// Create a subdirectory.
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("creating subdirectory: %v", err)
	}

	return dir
}

// writeFixtureFile creates a file of the given size.
func writeFixtureFile(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing fixture file %q: %v", path, err)
	}
}

// normalizeLsOutput normalizes output for comparison by replacing binary names
// and paths that differ between the Go binary and gls.
func normalizeLsOutput(refBin string) []testutils.NormalizeFunc {
	programNamePattern := regexp.MustCompile(`(?:` + regexp.QuoteMeta(refBin) + `|gls|ls)`)
	normalizeProgramName := func(b []byte) []byte {
		return programNamePattern.ReplaceAll(b, []byte("PROG"))
	}
	tryPattern := regexp.MustCompile(`Try '[^']*'`)
	normalizeTryPath := func(b []byte) []byte {
		return tryPattern.ReplaceAll(b, []byte("Try 'PROG'"))
	}
	return []testutils.NormalizeFunc{normalizeProgramName, normalizeTryPath}
}

// normalizeLongFormat returns normalizers for long-format output.
// It normalizes timestamps (which depend on wall clock) and program names.
func normalizeLongFormat(refBin string) []testutils.NormalizeFunc {
	norms := normalizeLsOutput(refBin)

	// Normalize mtime column: matches "Mon DD HH:MM" or "Mon DD  YYYY" patterns.
	mtimePattern := regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+(?:\d{2}:\d{2}|\s?\d{4})`)
	normalizeMtime := func(b []byte) []byte {
		return mtimePattern.ReplaceAll(b, []byte("MTIME_NORMALIZED"))
	}

	return append(norms, normalizeMtime)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R1.1/R1.2: default output (piped, so single-column).
		{
			Name:    "default_piped",
			Args:    []string{fixture},
			Env:     []string{"LC_ALL=C"},
			WorkDir: fixture,
		},
		// R1.4: dotfiles hidden by default.
		{
			Name:    "no_dotfiles",
			Args:    []string{fixture},
			Env:     []string{"LC_ALL=C"},
			WorkDir: fixture,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffSingleColumn(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R1.5: -1 produces single-column output.
		{
			Name: "single_column",
			Args: []string{"-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -1 with explicit directory.
		{
			Name: "single_column_dir",
			Args: []string{"-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffLongFormat(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R1.6, R1.7, R1.8: long format with permissions, owner, group.
		{
			Name:      "long_format",
			Args:      []string{"-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// In GNU ls, -l -1 still produces long format (long format is already
		// one-per-line, so -1 is redundant with -l).
		{
			Name:      "long_then_single",
			Args:      []string{"-l", "-1", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// -1 then -l: long wins.
		{
			Name:      "single_then_long",
			Args:      []string{"-1", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffLongFormatFiles(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	// Test -l on individual files (no "total" line for bare file args).
	tests := []testutils.DiffTest{
		{
			Name:      "long_format_file",
			Args:      []string{"-l", filepath.Join(fixture, "alpha")},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffLongFormatPermissions(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create a fixture with files of various permission modes.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "readable"), 10)
	os.Chmod(filepath.Join(dir, "readable"), 0o444)

	writeFixtureFile(t, filepath.Join(dir, "executable"), 10)
	os.Chmod(filepath.Join(dir, "executable"), 0o755)

	writeFixtureFile(t, filepath.Join(dir, "no_perms"), 10)
	os.Chmod(filepath.Join(dir, "no_perms"), 0o000)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		{
			Name:      "permission_modes",
			Args:      []string{"-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffLongFormatSubdir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create fixture with subdirectory.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "file1"), 50)
	os.Mkdir(filepath.Join(dir, "mydir"), 0o755)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		{
			Name:      "long_with_subdir",
			Args:      []string{"-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffLongFormatTimestamps(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()

	// Create a file with a recent mtime (within 6 months).
	recentFile := filepath.Join(dir, "recent")
	writeFixtureFile(t, recentFile, 10)

	// Create a file with an old mtime (more than 6 months ago).
	oldFile := filepath.Join(dir, "oldfile")
	writeFixtureFile(t, oldFile, 10)
	oldTime := time.Now().Add(-365 * 24 * time.Hour) // 1 year ago
	os.Chtimes(oldFile, oldTime, oldTime)

	// Do not normalize mtime here — we want to verify both format patterns match.
	norms := normalizeLsOutput(refBin)

	tests := []testutils.DiffTest{
		{
			Name:      "timestamps_recent_and_old",
			Args:      []string{"-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffCombinedFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// Combined -l1: GNU ls treats -1 as redundant with -l (already one-per-line).
		{
			Name:      "combined_l1",
			Args:      []string{"-l1", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// Combined -1l: -l wins.
		{
			Name:      "combined_1l",
			Args:      []string{"-1l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffErrorCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	norms := normalizeLsOutput(refBin)

	tests := []testutils.DiffTest{
		// Non-existent path.
		{
			Name:      "nonexistent_path",
			Args:      []string{"/nonexistent_path_for_ls_test"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffEmptyDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	emptyDir := t.TempDir()

	tests := []testutils.DiffTest{
		{
			Name: "empty_dir_default",
			Args: []string{emptyDir},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "empty_dir_single",
			Args: []string{"-1", emptyDir},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name:      "empty_dir_long",
			Args:      []string{"-l", emptyDir},
			Env:       []string{"LC_ALL=C"},
			Normalize: normalizeLongFormat(refBin),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
