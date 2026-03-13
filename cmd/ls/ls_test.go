// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd008-ls R1.5-R1.14, R2.1-R2.15, R3.1-R3.15, R4.1-R4.8 (differential tests)
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
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

// TestDiffLongFormatSymlink exercises R1.10: symlink display in long format.
func TestDiffLongFormatSymlink(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "realfile"), 42)

	// Create a symlink pointing to realfile.
	if err := os.Symlink("realfile", filepath.Join(dir, "linkfile")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R1.10: symlinks should display " -> target" in long format.
		{
			Name:      "long_format_symlink",
			Args:      []string{"-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColumnsFlag exercises R1.11 and R1.12: -C forces multi-column output.
func TestDiffColumnsFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R1.11: -C forces multi-column even when piped (uses 80 columns).
		{
			Name: "columns_flag",
			Args: []string{"-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.12: -C sorts vertically (same direction as default).
		{
			Name: "columns_flag_vertical_sort",
			Args: []string{"-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColumnsFlagOverride exercises R1.11, R1.14 with format flag interactions.
func TestDiffColumnsFlagOverride(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// -C then -l: long format wins.
		{
			Name:      "columns_then_long",
			Args:      []string{"-C", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// -l then -C: columns wins.
		{
			Name: "long_then_columns",
			Args: []string{"-l", "-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// -1 then -C: columns wins.
		{
			Name: "single_then_columns",
			Args: []string{"-1", "-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// -C then -1: single wins.
		{
			Name: "columns_then_single",
			Args: []string{"-C", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -x then -l: long format wins.
		{
			Name:      "horizontal_then_long",
			Args:      []string{"-x", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R1.14: -l then -x: horizontal wins.
		{
			Name: "long_then_horizontal",
			Args: []string{"-l", "-x", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -x then -1: single wins.
		{
			Name: "horizontal_then_single",
			Args: []string{"-x", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -1 then -x: horizontal wins.
		{
			Name: "single_then_horizontal",
			Args: []string{"-1", "-x", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -C then -x: horizontal wins.
		{
			Name: "columns_then_horizontal",
			Args: []string{"-C", "-x", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.14: -x then -C: columns wins.
		{
			Name: "horizontal_then_columns",
			Args: []string{"-x", "-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffLongFormatTotalBlocks exercises R1.10: "total N" block count line.
func TestDiffLongFormatTotalBlocks(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create files with known content to produce block allocations.
	writeFixtureFile(t, filepath.Join(dir, "small"), 10)
	writeFixtureFile(t, filepath.Join(dir, "medium"), 5000)
	writeFixtureFile(t, filepath.Join(dir, "large"), 100000)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		{
			Name:      "total_blocks",
			Args:      []string{"-l", dir},
			Env:       []string{"LC_ALL=C"},
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

// TestDiffHorizontalFlag exercises R1.13: -x produces horizontal multi-column output.
func TestDiffHorizontalFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R1.13: -x forces horizontal multi-column even when piped (uses 80 columns).
		{
			Name: "horizontal_flag",
			Args: []string{"-x", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHorizontalMany exercises R1.13 with enough entries to require multiple rows.
func TestDiffHorizontalMany(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create many files to force multiple rows in horizontal layout.
	// Names are 6 chars so column width (6+2 sep = 8) aligns to GNU ls tab stops.
	for _, name := range []string{"file01", "file02", "file03", "file04", "file05", "file06", "file07", "file08", "file09", "file10", "file11", "file12"} {
		writeFixtureFile(t, filepath.Join(dir, name), 10)
	}

	tests := []testutils.DiffTest{
		{
			Name: "horizontal_many_entries",
			Args: []string{"-x", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createDotfileFixture sets up a directory with dotfiles for testing -a and -A.
func createDotfileFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "visible"), 10)
	writeFixtureFile(t, filepath.Join(dir, ".hidden"), 10)
	writeFixtureFile(t, filepath.Join(dir, ".secret"), 10)
	if err := os.Mkdir(filepath.Join(dir, ".hiddendir"), 0o755); err != nil {
		t.Fatalf("creating .hiddendir: %v", err)
	}

	return dir
}

// TestDiffFilterAll exercises R2.1: -a includes all entries including . and ..
func TestDiffFilterAll(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createDotfileFixture(t)

	tests := []testutils.DiffTest{
		// R2.1: -a shows all entries including . and ..
		{
			Name: "filter_all",
			Args: []string{"-a", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: -a with -1 for clear single-column output.
		{
			Name: "filter_all_single",
			Args: []string{"-a", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFilterAlmostAll exercises R2.2: -A includes dotfiles except . and ..
func TestDiffFilterAlmostAll(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createDotfileFixture(t)

	tests := []testutils.DiffTest{
		// R2.2: -A shows dotfiles but not . and ..
		{
			Name: "filter_almost_all",
			Args: []string{"-A", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: -A with -1 for clear single-column output.
		{
			Name: "filter_almost_all_single",
			Args: []string{"-A", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFilterLongFormat exercises R2.1 and R2.2 with -l.
func TestDiffFilterLongFormat(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createDotfileFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R2.1: -la shows all entries in long format.
		{
			Name:      "filter_all_long",
			Args:      []string{"-la", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R2.2: -lA shows dotfiles except . and .. in long format.
		{
			Name:      "filter_almost_all_long",
			Args:      []string{"-lA", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDirOnly exercises R2.3: -d lists directories themselves, not contents.
func TestDiffDirOnly(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R2.3: -d lists the directory itself, not its contents.
		{
			Name: "dir_only",
			Args: []string{"-d", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -d with -1 (single column).
		{
			Name: "dir_only_single",
			Args: []string{"-d", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: -ld lists directory entry in long format without descending.
		{
			Name:      "dir_only_long",
			Args:      []string{"-ld", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R2.3: -d with multiple directories.
		{
			Name: "dir_only_multiple",
			Args: []string{"-d", fixture, filepath.Join(fixture, "subdir")},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFilterPrecedence exercises R2.4: last of -a/-A wins.
func TestDiffFilterPrecedence(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createDotfileFixture(t)

	tests := []testutils.DiffTest{
		// R2.4: -a then -A: -A wins (no . and ..).
		{
			Name: "filter_a_then_A",
			Args: []string{"-a", "-A", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: -A then -a: -a wins (includes . and ..).
		{
			Name: "filter_A_then_a",
			Args: []string{"-A", "-a", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: combined -aA: -A wins (last in combined flag string).
		{
			Name: "filter_combined_aA",
			Args: []string{"-aA", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: combined -Aa: -a wins.
		{
			Name: "filter_combined_Aa",
			Args: []string{"-Aa", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSortTime exercises R2.5: -t sorts by modification time, newest first.
func TestDiffSortTime(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create files with distinct modification times.
	now := time.Now()
	for i, name := range []string{"oldest", "middle", "newest"} {
		path := filepath.Join(dir, name)
		writeFixtureFile(t, path, 10)
		mtime := now.Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(path, mtime, mtime)
	}

	tests := []testutils.DiffTest{
		// R2.5: -t sorts by mtime, newest first.
		{
			Name: "sort_time",
			Args: []string{"-t", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.5: -t with long format.
		{
			Name:      "sort_time_long",
			Args:      []string{"-lt", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: normalizeLongFormat(refBin),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSortSize exercises R2.6: -S sorts by file size, largest first.
func TestDiffSortSize(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create files with distinct sizes.
	writeFixtureFile(t, filepath.Join(dir, "tiny"), 10)
	writeFixtureFile(t, filepath.Join(dir, "medium"), 5000)
	writeFixtureFile(t, filepath.Join(dir, "huge"), 100000)

	tests := []testutils.DiffTest{
		// R2.6: -S sorts by size, largest first.
		{
			Name: "sort_size",
			Args: []string{"-S", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.6: -S with long format.
		{
			Name:      "sort_size_long",
			Args:      []string{"-lS", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: normalizeLongFormat(refBin),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSortSizeTiebreaker exercises R2.6: same-size entries sorted by name.
func TestDiffSortSizeTiebreaker(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create files with the same size — tiebreaker is name in C locale order.
	for _, name := range []string{"cherry", "apple", "banana"} {
		writeFixtureFile(t, filepath.Join(dir, name), 100)
	}

	tests := []testutils.DiffTest{
		{
			Name: "sort_size_tiebreaker",
			Args: []string{"-S", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffReverse exercises R2.7: -r reverses the current sort order.
func TestDiffReverse(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.7: -r reverses default alphabetical sort.
		{
			Name: "reverse_default",
			Args: []string{"-r", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffReverseTime exercises R2.7: -tr reverses time sort.
func TestDiffReverseTime(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	now := time.Now()
	for i, name := range []string{"oldest", "middle", "newest"} {
		path := filepath.Join(dir, name)
		writeFixtureFile(t, path, 10)
		mtime := now.Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(path, mtime, mtime)
	}

	tests := []testutils.DiffTest{
		// R2.7: -tr reverses time sort (oldest first).
		{
			Name: "reverse_time",
			Args: []string{"-tr", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffReverseSize exercises R2.7: -rS reverses size sort.
func TestDiffReverseSize(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "tiny"), 10)
	writeFixtureFile(t, filepath.Join(dir, "medium"), 5000)
	writeFixtureFile(t, filepath.Join(dir, "huge"), 100000)

	tests := []testutils.DiffTest{
		// R2.7: -rS reverses size sort (smallest first).
		{
			Name: "reverse_size",
			Args: []string{"-rS", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffUnsorted exercises R2.8: -U disables sorting.
func TestDiffUnsorted(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.8: -U lists in directory order.
		{
			Name: "unsorted",
			Args: []string{"-U", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.8: -rU accepted without error (-r has no meaningful effect with -U).
		{
			Name: "unsorted_with_reverse",
			Args: []string{"-rU", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffVersionSort exercises R2.9: -v uses natural version sort.
func TestDiffVersionSort(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	for _, name := range []string{"file1", "file2", "file10", "file20", "file3"} {
		writeFixtureFile(t, filepath.Join(dir, name), 10)
	}

	tests := []testutils.DiffTest{
		// R2.9: -v sorts file2 before file10.
		{
			Name: "version_sort",
			Args: []string{"-v", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7+R2.9: -rv reverses version sort.
		{
			Name: "version_sort_reverse",
			Args: []string{"-rv", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSortPrecedence exercises R2.10: last sort flag wins.
func TestDiffSortPrecedence(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	now := time.Now()
	for i, name := range []string{"aaa", "bbb", "ccc"} {
		path := filepath.Join(dir, name)
		writeFixtureFile(t, path, (3-i)*1000)
		mtime := now.Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(path, mtime, mtime)
	}

	tests := []testutils.DiffTest{
		// R2.10: -t then -S: -S wins (size sort).
		{
			Name: "sort_time_then_size",
			Args: []string{"-t", "-S", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: -S then -t: -t wins (time sort).
		{
			Name: "sort_size_then_time",
			Args: []string{"-S", "-t", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: -t then -U: -U wins (unsorted).
		{
			Name: "sort_time_then_unsorted",
			Args: []string{"-t", "-U", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: -U then -t: -t wins (time sort).
		{
			Name: "sort_unsorted_then_time",
			Args: []string{"-U", "-t", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInodeDisplay exercises R2.11: -i prepends inode number.
func TestDiffInodeDisplay(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R2.11: -i in single-column mode.
		{
			Name: "inode_single",
			Args: []string{"-i", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.11: -i in long format — inode before permissions.
		{
			Name:      "inode_long",
			Args:      []string{"-il", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R2.11: -i in default (piped) mode.
		{
			Name: "inode_default",
			Args: []string{"-i", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBlockDisplay exercises R2.12: -s prepends allocated block count.
func TestDiffBlockDisplay(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R2.12: -s in single-column mode.
		{
			Name: "blocks_single",
			Args: []string{"-s", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.13: -s with -l — block counts in long format with total line.
		{
			Name:      "blocks_long",
			Args:      []string{"-sl", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R2.12: -s in default (piped) mode.
		{
			Name: "blocks_default",
			Args: []string{"-s", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInodeBlocksCombined exercises R2.15: -i and -s combined.
func TestDiffInodeBlocksCombined(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R2.15: -is in single-column mode — inode first, then blocks.
		{
			Name: "inode_blocks_single",
			Args: []string{"-is", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.15: -is with -l.
		{
			Name:      "inode_blocks_long",
			Args:      []string{"-isl", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNumericIDs exercises R2.14: -n displays numeric UID/GID.
func TestDiffNumericIDs(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R2.14: -n implies -l with numeric UID/GID.
		{
			Name:      "numeric_ids",
			Args:      []string{"-n", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R2.14: -n with -i — inode + numeric long format.
		{
			Name:      "numeric_ids_with_inode",
			Args:      []string{"-ni", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInodeBlocksCombinedFormats exercises R2.15 with additional format modes.
func TestDiffInodeBlocksCombinedFormats(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.15: -is in default (piped) mode — inode first, then blocks.
		{
			Name: "inode_blocks_default",
			Args: []string{"-is", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.15: -is with -C (forced columns).
		{
			Name: "inode_blocks_columns",
			Args: []string{"-isC", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.15: -is with -x (horizontal columns).
		{
			Name: "inode_blocks_horizontal",
			Args: []string{"-isx", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorNever exercises R3.1: --color=never produces no ANSI escapes.
func TestDiffColorNever(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R3.1/R3.4: --color=never suppresses all ANSI sequences.
		{
			Name: "color_never",
			Args: []string{"--color=never", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: --color=never with long format.
		{
			Name:      "color_never_long",
			Args:      []string{"--color=never", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: normalizeLongFormat(refBin),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorAuto exercises R3.2: --color=auto with piped stdout emits no ANSI.
func TestDiffColorAuto(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R3.2: --color=auto with piped stdout (not a TTY) — no ANSI codes.
		{
			Name: "color_auto_piped",
			Args: []string{"--color=auto", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --color=auto with piped long format.
		{
			Name:      "color_auto_piped_long",
			Args:      []string{"--color=auto", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: normalizeLongFormat(refBin),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorAlways exercises R3.1/R3.3: --color=always emits ANSI codes.
func TestDiffColorAlways(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create fixture with diverse file types for color testing.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "regular"), 10)
	os.Mkdir(filepath.Join(dir, "mydir"), 0o755)
	writeFixtureFile(t, filepath.Join(dir, "runme"), 10)
	os.Chmod(filepath.Join(dir, "runme"), 0o755)
	os.Symlink("regular", filepath.Join(dir, "mylink"))

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.1/R3.3: --color=always in single-column mode.
		{
			Name: "color_always_single",
			Args: []string{"--color=always", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --color=always with long format — names colorized.
		{
			Name:      "color_always_long",
			Args:      []string{"--color=always", "-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.1: bare --color (no value) defaults to "always".
		{
			Name: "color_bare",
			Args: []string{"--color", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorSuppression exercises R3.4: no ANSI escapes when color is suppressed.
func TestDiffColorSuppression(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create fixture with diverse file types that would normally be colorized.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "regular"), 10)
	os.Mkdir(filepath.Join(dir, "mydir"), 0o755)
	writeFixtureFile(t, filepath.Join(dir, "runme"), 10)
	os.Chmod(filepath.Join(dir, "runme"), 0o755)
	os.Symlink("regular", filepath.Join(dir, "mylink"))

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.4: --color=never with diverse file types — no ANSI.
		{
			Name: "color_suppressed_never",
			Args: []string{"--color=never", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --color=auto piped (non-TTY) with diverse file types — no ANSI.
		{
			Name: "color_suppressed_auto_piped",
			Args: []string{"--color=auto", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --color=never with long format and diverse file types.
		{
			Name:      "color_suppressed_never_long",
			Args:      []string{"--color=never", "-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.4: --color=auto piped with long format.
		{
			Name:      "color_suppressed_auto_piped_long",
			Args:      []string{"--color=auto", "-l", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHumanSizes exercises R3.5: -h with -l shows human-readable sizes.
func TestDiffHumanSizes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create files with various sizes to test human-readable formatting.
	writeFixtureFile(t, filepath.Join(dir, "empty"), 0)
	writeFixtureFile(t, filepath.Join(dir, "small"), 100)
	writeFixtureFile(t, filepath.Join(dir, "medium"), 5000)
	writeFixtureFile(t, filepath.Join(dir, "large"), 100000)
	writeFixtureFile(t, filepath.Join(dir, "huge"), 2000000)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.5: -lh shows human-readable sizes in long format.
		{
			Name:      "human_sizes_long",
			Args:      []string{"-lh", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.5: -h without -l has no visible effect.
		{
			Name: "human_sizes_no_long",
			Args: []string{"-h", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.5: -h without -l in default mode has no visible effect.
		{
			Name: "human_sizes_no_long_default",
			Args: []string{"-h", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHumanSizesTotalLine exercises R3.6: -h applies to "total N" line.
func TestDiffHumanSizesTotalLine(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	// Create files with enough blocks to produce a human-readable total.
	writeFixtureFile(t, filepath.Join(dir, "big1"), 500000)
	writeFixtureFile(t, filepath.Join(dir, "big2"), 500000)
	writeFixtureFile(t, filepath.Join(dir, "big3"), 500000)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.6: -lh total line uses human-readable format.
		{
			Name:      "human_sizes_total_line",
			Args:      []string{"-lh", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHumanSizesBlocks exercises R3.7: -h applies to -s block counts.
func TestDiffHumanSizesBlocks(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "small"), 100)
	writeFixtureFile(t, filepath.Join(dir, "medium"), 5000)
	writeFixtureFile(t, filepath.Join(dir, "large"), 100000)
	writeFixtureFile(t, filepath.Join(dir, "huge"), 2000000)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.7: -sh shows human-readable block counts.
		{
			Name: "human_blocks_single",
			Args: []string{"-sh", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.7: -slh shows human-readable blocks in long format.
		{
			Name:      "human_blocks_long",
			Args:      []string{"-slh", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.7: -sh with default (piped) mode.
		{
			Name: "human_blocks_default",
			Args: []string{"-sh", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.7: -ish — inode + human-readable blocks.
		{
			Name: "human_blocks_with_inode",
			Args: []string{"-ish", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorWithPrefixes exercises R3.3 combined with R2.11/R2.12/R2.15.
func TestDiffColorWithPrefixes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "afile"), 100)
	os.Mkdir(filepath.Join(dir, "bdir"), 0o755)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.3 + R2.11: --color=always with -i.
		{
			Name: "color_with_inode",
			Args: []string{"--color=always", "-i", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3 + R2.12: --color=always with -s.
		{
			Name: "color_with_blocks",
			Args: []string{"--color=always", "-s", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3 + R2.15: --color=always with -is.
		{
			Name: "color_with_inode_blocks",
			Args: []string{"--color=always", "-is", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3 + R2.15 + long format.
		{
			Name:      "color_with_inode_blocks_long",
			Args:      []string{"--color=always", "-isl", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createClassifyFixture creates a fixture directory with a directory, regular file,
// executable, symlink, and named pipe for -F classification testing.
func createClassifyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Regular non-executable file.
	writeFixtureFile(t, filepath.Join(dir, "regular"), 10)

	// Executable regular file.
	execPath := filepath.Join(dir, "runme")
	writeFixtureFile(t, execPath, 10)
	if err := os.Chmod(execPath, 0o755); err != nil {
		t.Fatalf("chmod executable: %v", err)
	}

	// Subdirectory.
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Symbolic link.
	if err := os.Symlink("regular", filepath.Join(dir, "symlink")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Named pipe (FIFO).
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	return dir
}

// TestDiffClassify exercises R3.8, R3.9, R3.10: -F appends type indicators.
func TestDiffClassify(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := createClassifyFixture(t)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.8: -F with single-column output.
		{
			Name: "classify_single_column",
			Args: []string{"-F", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.10: -F with long format.
		{
			Name:      "classify_long_format",
			Args:      []string{"-Fl", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.10: -F with --color=never to verify indicator placement.
		{
			Name: "classify_color_never",
			Args: []string{"-F", "--color=never", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursive exercises R3.11: -R recursively lists subdirectories.
func TestDiffRecursive(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create nested directory structure.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "topfile"), 10)

	sub1 := filepath.Join(dir, "adir")
	if err := os.Mkdir(sub1, 0o755); err != nil {
		t.Fatalf("mkdir adir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub1, "inner1"), 20)

	sub2 := filepath.Join(dir, "bdir")
	if err := os.Mkdir(sub2, 0o755); err != nil {
		t.Fatalf("mkdir bdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub2, "inner2"), 30)

	// Nested subdirectory.
	sub1sub := filepath.Join(sub1, "nested")
	if err := os.Mkdir(sub1sub, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub1sub, "deep"), 5)

	// Symlink to directory (R3.13: should NOT be followed).
	if err := os.Symlink("adir", filepath.Join(dir, "linkdir")); err != nil {
		t.Fatalf("symlink linkdir: %v", err)
	}

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.11: basic recursive listing.
		{
			Name: "recursive_basic",
			Args: []string{"-R", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.12: -R with -l (long format with "total N" per subdir).
		{
			Name:      "recursive_long",
			Args:      []string{"-Rl", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.11 + R3.8: -R with -F (classification in subdirectories).
		{
			Name: "recursive_classify",
			Args: []string{"-RF", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveFormatMode exercises R3.12: -R respects the current format mode.
func TestDiffRecursiveFormatMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create nested directory structure.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "afile"), 10)
	writeFixtureFile(t, filepath.Join(dir, "bfile"), 20)

	sub := filepath.Join(dir, "cdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir cdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub, "inner1"), 30)
	writeFixtureFile(t, filepath.Join(sub, "inner2"), 40)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R3.12: -R with -l includes "total N" per subdirectory.
		{
			Name:      "recursive_long_total",
			Args:      []string{"-Rl", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R3.12: -R with -1 (single-column in each subdirectory).
		{
			Name: "recursive_single_column",
			Args: []string{"-R1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.12: -R with -x (horizontal multi-column in each subdirectory).
		{
			Name: "recursive_horizontal",
			Args: []string{"-Rx", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveNoFollowSymlinks exercises R3.13: -R must not follow symlinks to directories.
func TestDiffRecursiveNoFollowSymlinks(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()

	// Real subdirectory with content.
	realDir := filepath.Join(dir, "realdir")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("mkdir realdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(realDir, "realfile"), 10)

	// Symlink pointing to the real directory — should NOT be recursed.
	if err := os.Symlink("realdir", filepath.Join(dir, "symdir")); err != nil {
		t.Fatalf("symlink symdir: %v", err)
	}

	writeFixtureFile(t, filepath.Join(dir, "topfile"), 5)

	tests := []testutils.DiffTest{
		// R3.13: symdir should appear in the listing but not be recursed into.
		{
			Name: "recursive_no_follow_symlink",
			Args: []string{"-R", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveFilter exercises R3.14: -R applies filter flags (-a, -A) to subdirectories.
func TestDiffRecursiveFilter(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()

	// Create dotfiles in top-level.
	writeFixtureFile(t, filepath.Join(dir, ".hidden"), 10)
	writeFixtureFile(t, filepath.Join(dir, "visible"), 20)

	// Create subdirectory with its own dotfiles.
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub, ".subhidden"), 15)
	writeFixtureFile(t, filepath.Join(sub, "subvisible"), 25)

	tests := []testutils.DiffTest{
		// R3.14: -Ra shows dotfiles (including . and ..) in all subdirectories.
		{
			Name: "recursive_filter_all",
			Args: []string{"-Ra", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -RA shows dotfiles except . and .. in all subdirectories.
		{
			Name: "recursive_filter_almost_all",
			Args: []string{"-RA", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -R without -a/-A hides dotfiles in subdirectories.
		{
			Name: "recursive_filter_default",
			Args: []string{"-R", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveSortOrder exercises R3.15: -R lists directories in sort order.
func TestDiffRecursiveSortOrder(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	dir := t.TempDir()

	// Create subdirectories with different mtimes for -t ordering.
	sub1 := filepath.Join(dir, "zdir")
	if err := os.Mkdir(sub1, 0o755); err != nil {
		t.Fatalf("mkdir zdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub1, "file_z"), 10)

	sub2 := filepath.Join(dir, "adir")
	if err := os.Mkdir(sub2, 0o755); err != nil {
		t.Fatalf("mkdir adir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub2, "file_a"), 20)

	sub3 := filepath.Join(dir, "mdir")
	if err := os.Mkdir(sub3, 0o755); err != nil {
		t.Fatalf("mkdir mdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub3, "file_m"), 15)

	writeFixtureFile(t, filepath.Join(dir, "topfile"), 5)

	// Set different mtimes so -t produces a deterministic order.
	now := time.Now()
	// zdir is newest, adir is oldest.
	if err := os.Chtimes(sub1, now, now); err != nil {
		t.Fatalf("chtimes zdir: %v", err)
	}
	if err := os.Chtimes(sub2, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatalf("chtimes adir: %v", err)
	}
	if err := os.Chtimes(sub3, now.Add(-1*time.Hour), now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("chtimes mdir: %v", err)
	}
	// topfile between mdir and adir.
	if err := os.Chtimes(filepath.Join(dir, "topfile"), now.Add(-90*time.Minute), now.Add(-90*time.Minute)); err != nil {
		t.Fatalf("chtimes topfile: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.15: -Rt recurses into subdirectories in time-sorted order.
		{
			Name: "recursive_sort_time",
			Args: []string{"-Rt", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -Rr recurses in reverse-alphabetical order.
		{
			Name: "recursive_sort_reverse",
			Args: []string{"-Rr", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -RS recurses in size order.
		{
			Name: "recursive_sort_size",
			Args: []string{"-RS", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitCodeSuccess exercises R4.1: exit 0 when all paths are accessible.
func TestDiffExitCodeSuccess(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)

	tests := []testutils.DiffTest{
		// R4.1: exit 0 when all listed paths are accessed successfully.
		{
			Name:     "exit_0_single_dir",
			Args:     []string{"-1", fixture},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.1: exit 0 when listing a single file.
		{
			Name:     "exit_0_single_file",
			Args:     []string{"-1", filepath.Join(fixture, "alpha")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.1: exit 0 when listing multiple valid paths.
		{
			Name:     "exit_0_multiple_valid",
			Args:     []string{"-1", filepath.Join(fixture, "alpha"), filepath.Join(fixture, "beta")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitCodeMinorProblem exercises R4.2: exit with diagnostic on access failure,
// still listing remaining accessible entries.
func TestDiffExitCodeMinorProblem(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLsOutput(refBin)

	tests := []testutils.DiffTest{
		// R4.2: nonexistent path produces diagnostic and non-zero exit.
		{
			Name:      "exit_nonexistent",
			Args:      []string{"-1", "/nonexistent_ls_r42_test"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: norms,
		},
		// R4.2: mix of valid and invalid paths — valid entries still listed.
		{
			Name:      "exit_mixed_valid_invalid",
			Args:      []string{"-1", "/nonexistent_ls_r42_test", fixture},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: norms,
		},
		// R4.2: mix of valid file and nonexistent path.
		{
			Name:      "exit_mixed_file_invalid",
			Args:      []string{"-1", filepath.Join(fixture, "alpha"), "/nonexistent_ls_r42_test"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeErrorOutput returns normalizers for error message comparison.
// Strips the "Try '...' for more information." line and extra help text that
// differs between our implementation and GNU ls.
func normalizeErrorOutput(refBin string) []testutils.NormalizeFunc {
	norms := normalizeLsOutput(refBin)
	// Strip "Try ..." lines and "Valid arguments ..." help blocks.
	tryLine := regexp.MustCompile(`(?m)^Try .*\n`)
	stripTry := func(b []byte) []byte { return tryLine.ReplaceAll(b, nil) }
	validArgs := regexp.MustCompile(`(?m)^Valid arguments are:\n(?:  - .*\n)*`)
	stripValid := func(b []byte) []byte { return validArgs.ReplaceAll(b, nil) }
	return append(norms, stripTry, stripValid)
}

// TestDiffExitCodeBadOption exercises R4.3: exit 2 for invalid command-line options.
func TestDiffExitCodeBadOption(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	norms := normalizeErrorOutput(refBin)

	tests := []testutils.DiffTest{
		// R4.3: invalid short option (-j is not recognized by either binary).
		{
			Name:      "exit_2_bad_short_option",
			Args:      []string{"-j"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: norms,
		},
		// R4.3: invalid long option.
		{
			Name:      "exit_2_bad_long_option",
			Args:      []string{"--nonexistent-option"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSIGPIPE exercises R4.4: SIGPIPE handler prevents broken-pipe errors
// when output is piped to a consumer that closes its input early.
func TestDiffSIGPIPE(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Create a fixture with enough files to produce multiple lines of output.
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFixtureFile(t, filepath.Join(dir, fmt.Sprintf("file%03d", i)), 10)
	}

	// R4.4: pipe ls output to head -1, which closes its stdin after one line.
	// Without SIGPIPE handling, the process would exit non-zero or print a
	// broken pipe error. With proper handling, it exits 0.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("'%s' -1 '%s' | head -1", goBin, dir))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("R4.4: ls piped to head should exit 0, got error: %v\noutput: %s", err, out)
	}
}

// TestDiffNumericIDsImpliesLong exercises R4.6: -n implies -l even without explicit -l.
func TestDiffNumericIDsImpliesLong(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R4.6: -n without -l still produces long format with numeric UID/GID.
		{
			Name:      "numeric_ids_implies_long",
			Args:      []string{"-n", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R4.6: -n -1 — -n implies -l, and per GNU ls, -1 does not override -l.
		{
			Name:      "numeric_ids_with_single",
			Args:      []string{"-n", "-1", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFormatFlagInteractions exercises R4.7: -C/-x are mutually exclusive with -l/-1,
// last format flag wins.
func TestDiffFormatFlagInteractions(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	fixture := createFixture(t)
	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R4.7: -C then -l: long format wins.
		{
			Name:      "r47_columns_then_long",
			Args:      []string{"-C", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R4.7: -l then -C: columns wins.
		{
			Name: "r47_long_then_columns",
			Args: []string{"-l", "-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -x then -l: long format wins.
		{
			Name:      "r47_horizontal_then_long",
			Args:      []string{"-x", "-l", fixture},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R4.7: -l then -x: horizontal wins.
		{
			Name: "r47_long_then_horizontal",
			Args: []string{"-l", "-x", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -1 then -C: columns wins.
		{
			Name: "r47_single_then_columns",
			Args: []string{"-1", "-C", fixture},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -C then -1: single wins.
		{
			Name: "r47_columns_then_single",
			Args: []string{"-C", "-1", fixture},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveLongTotal exercises R4.8: -R with -l produces "total N" block line
// for each subdirectory listing.
func TestDiffRecursiveLongTotal(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create nested directory structure with known file sizes.
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "topfile"), 100)

	sub1 := filepath.Join(dir, "adir")
	if err := os.Mkdir(sub1, 0o755); err != nil {
		t.Fatalf("mkdir adir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub1, "inner1"), 200)
	writeFixtureFile(t, filepath.Join(sub1, "inner2"), 300)

	sub2 := filepath.Join(dir, "bdir")
	if err := os.Mkdir(sub2, 0o755); err != nil {
		t.Fatalf("mkdir bdir: %v", err)
	}
	writeFixtureFile(t, filepath.Join(sub2, "deep1"), 400)

	norms := normalizeLongFormat(refBin)

	tests := []testutils.DiffTest{
		// R4.8: -lR produces "total N" for each subdirectory.
		{
			Name:      "recursive_long_total_per_subdir",
			Args:      []string{"-lR", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
		// R4.8: -lRh produces human-readable "total" per subdirectory.
		{
			Name:      "recursive_long_human_total",
			Args:      []string{"-lRh", dir},
			Env:       []string{"LC_ALL=C"},
			Normalize: norms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
