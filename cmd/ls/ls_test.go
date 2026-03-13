// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd008-ls R1.5-R1.14, R2.1-R2.6 (differential tests)
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
