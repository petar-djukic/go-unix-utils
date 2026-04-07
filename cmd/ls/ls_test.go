// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/ls via differential testing against gls.
// Tests srd008-ls R1.13, R1.14, R2.1-R2.15, R3.1-R3.11.
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

// createVersionFixture creates files for version sort testing.
// R2.9: "file2" must sort before "file10" in version sort.
func createVersionFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"file1", "file2", "file10", "file20", "file100",
		"abc", "xyz",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestDiffReverse tests -r reverse sort flag (R2.7).
func TestDiffReverse(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createSortFixture(t)

	tests := []testutils.DiffTest{
		// R2.7: -r reverses default alphabetical sort.
		{
			Name: "r_reverse_alpha",
			Args: []string{"-r", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7: -tr reverses time sort.
		{
			Name: "tr_reverse_time",
			Args: []string{"-tr", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7: -Sr reverses size sort.
		{
			Name: "Sr_reverse_size",
			Args: []string{"-Sr", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffUnsorted tests -U unsorted flag (R2.8).
func TestDiffUnsorted(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createSortFixture(t)

	tests := []testutils.DiffTest{
		// R2.8: -U lists in directory order (no sorting).
		{
			Name: "U_unsorted",
			Args: []string{"-U", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.8: -rU accepted without error.
		{
			Name: "rU_accepted",
			Args: []string{"-rU", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffVersionSort tests -v version sort flag (R2.9).
func TestDiffVersionSort(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createVersionFixture(t)

	tests := []testutils.DiffTest{
		// R2.9: -v sorts with natural version semantics.
		{
			Name: "v_version_sort",
			Args: []string{"-v", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7+R2.9: -vr reverses version sort.
		{
			Name: "vr_reverse_version",
			Args: []string{"-vr", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInode tests -i inode display (R2.11).
func TestDiffInode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.11: -i with single-column output.
		{
			Name: "i_single_column",
			Args: []string{"-i", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.11: -i with long format.
		{
			Name: "i_long_format",
			Args: []string{"-il", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBlocks tests -s block count display (R2.12, R2.13).
func TestDiffBlocks(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.12: -s with single-column output.
		{
			Name: "s_single_column",
			Args: []string{"-s", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.12/R2.13: -s with long format (includes total line).
		{
			Name: "s_long_format",
			Args: []string{"-sl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNumericIDs tests -n numeric UID/GID display (R2.14).
func TestDiffNumericIDs(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.14: -n shows numeric UID/GID in long format.
		{
			Name: "n_numeric_ids",
			Args: []string{"-n", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInodeBlocks tests -i and -s combined (R2.15).
func TestDiffInodeBlocks(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R2.15: -i and -s combined, single column.
		{
			Name: "is_single_column",
			Args: []string{"-is", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.15: -i and -s combined, long format.
		{
			Name: "is_long_format",
			Args: []string{"-isl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSortPrecedence tests last sort flag wins (R2.10).
func TestDiffSortPrecedence(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createSortFixture(t)

	tests := []testutils.DiffTest{
		// R2.10: -tS, -S wins (last sort flag).
		{
			Name: "tS_size_wins",
			Args: []string{"-tS", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: -St, -t wins (last sort flag).
		{
			Name: "St_time_wins",
			Args: []string{"-St", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: -tU, -U wins (unsorted overrides time).
		{
			Name: "tU_unsorted_wins",
			Args: []string{"-tU", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createColorFixture creates a directory with different file types for color testing.
// Includes a regular file, executable, subdirectory, and symlink.
func createColorFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "plain.txt"), 5)
	writeFixtureFile(t, filepath.Join(dir, "run.sh"), 10)
	if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDiffColor tests --color flag behavior (R3.1, R3.2, R3.3).
func TestDiffColor(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createColorFixture(t)

	tests := []testutils.DiffTest{
		// R3.1: --color=always forces ANSI color output.
		// TERM must be set for gls to initialize its color database.
		{
			Name: "color_always_single",
			Args: []string{"--color=always", "-1", dir},
			Env:  []string{"LC_ALL=C", "TERM=xterm-256color"},
		},
		// R3.2: --color=auto produces no ANSI when stdout is piped.
		{
			Name: "color_auto_piped",
			Args: []string{"--color=auto", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: --color=never suppresses all color output.
		{
			Name: "color_never_single",
			Args: []string{"--color=never", "-1", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: bare --color defaults to "always".
		{
			Name: "color_bare_flag",
			Args: []string{"--color", "-1", dir},
			Env:  []string{"LC_ALL=C", "TERM=xterm-256color"},
		},
		// R3.3: color with long format shows colored names.
		{
			Name: "color_always_long",
			Args: []string{"--color=always", "-l", dir},
			Env:  []string{"LC_ALL=C", "TERM=xterm-256color"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorNever tests --color=never suppresses all ANSI (R3.4).
func TestDiffColorNever(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createColorFixture(t)

	tests := []testutils.DiffTest{
		// R3.4: --color=never with long format produces no ANSI.
		{
			Name: "color_never_long",
			Args: []string{"--color=never", "-l", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --color=auto piped (non-TTY) suppresses ANSI.
		{
			Name: "color_auto_piped_long",
			Args: []string{"--color=auto", "-l", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createHumanFixture creates files with varying sizes for -h testing.
func createHumanFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "small"), 100)
	writeFixtureFile(t, filepath.Join(dir, "medium"), 5000)
	writeFixtureFile(t, filepath.Join(dir, "large"), 50000)
	return dir
}

// TestDiffHumanReadable tests -h flag behavior (R3.5, R3.6).
func TestDiffHumanReadable(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createHumanFixture(t)

	tests := []testutils.DiffTest{
		// R3.5: -lh shows human-readable file sizes.
		{
			Name: "lh_human_sizes",
			Args: []string{"-lh", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.5: -h without -l has no visible effect.
		{
			Name: "h_without_l",
			Args: []string{"-h", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.6: -lh total line is human-readable.
		{
			Name: "lh_total_line",
			Args: []string{"-lh", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHumanBlocks tests -h with -s block counts (R3.7).
func TestDiffHumanBlocks(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createHumanFixture(t)

	tests := []testutils.DiffTest{
		// R3.7: -sh shows human-readable block counts.
		{
			Name: "sh_human_blocks",
			Args: []string{"-sh", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.7: -slh combines long format with human block counts.
		{
			Name: "slh_long_human_blocks",
			Args: []string{"-slh", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createClassifyFixture creates a directory with different file types for -F testing.
// Includes a regular file, executable, subdirectory, symlink, and FIFO.
func createClassifyFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "plain.txt"), 5)
	writeFixtureFile(t, filepath.Join(dir, "run.sh"), 10)
	if err := os.Chmod(filepath.Join(dir, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDiffClassify tests -F type indicator flag (R3.8, R3.9, R3.10).
func TestDiffClassify(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createClassifyFixture(t)

	tests := []testutils.DiffTest{
		// R3.8: -F with single-column output.
		{
			Name: "F_single_column",
			Args: []string{"-F", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.10: -F with long format.
		{
			Name: "F_long_format",
			Args: []string{"-Fl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.10: -F with color output.
		{
			Name: "F_color_always",
			Args: []string{"-F", "-1", "--color=always", dir},
			Env:  []string{"LC_ALL=C", "TERM=xterm-256color"},
		},
		// R3.10: -F with -C multi-column.
		{
			Name: "F_columns",
			Args: []string{"-FC", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createRecursiveFixture creates nested directories for -R testing.
func createRecursiveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "top.txt"), 5)
	sub := filepath.Join(dir, "subA")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(sub, "child.txt"), 10)
	sub2 := filepath.Join(dir, "subB")
	if err := os.Mkdir(sub2, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(sub2, "other.txt"), 15)
	return dir
}

// TestDiffRecursive tests -R recursive listing (R3.11).
func TestDiffRecursive(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createRecursiveFixture(t)

	tests := []testutils.DiffTest{
		// R3.11: -R with single-column.
		{
			Name: "R_single_column",
			Args: []string{"-R", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.11: -R with long format.
		{
			Name: "R_long_format",
			Args: []string{"-Rl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.11+R3.8: -R with -F classify.
		{
			Name: "RF_classify_recursive",
			Args: []string{"-RF", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
