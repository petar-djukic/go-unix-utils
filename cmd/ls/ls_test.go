// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/ls via differential testing against gls.
// Tests srd008-ls R1.13, R1.14, R2.1-R2.15, R3.1-R3.15, R4.1-R4.9.
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

// TestDiffRecursiveFormat tests -R respects format mode (R3.12).
func TestDiffRecursiveFormat(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createRecursiveFixture(t)

	tests := []testutils.DiffTest{
		// R3.12: -R with -C multi-column layout in subdirectories.
		{
			Name: "RC_columns",
			Args: []string{"-RC", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.12: -R with -x horizontal layout in subdirectories.
		{
			Name: "Rx_horizontal",
			Args: []string{"-Rx", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.12: -R with -l includes total line in subdirectories.
		{
			Name: "Rl_total_line",
			Args: []string{"-Rl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createSymlinkDirFixture creates a fixture with a symlink to a directory.
// R3.13: -R must not follow symlinks to directories.
func createSymlinkDirFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "file.txt"), 5)
	sub := filepath.Join(dir, "realdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(sub, "child.txt"), 10)
	if err := os.Symlink("realdir", filepath.Join(dir, "linkdir")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDiffRecursiveSymlink tests -R does not follow symlinks (R3.13).
func TestDiffRecursiveSymlink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createSymlinkDirFixture(t)

	tests := []testutils.DiffTest{
		// R3.13: -R must not recurse into symlink-to-directory.
		{
			Name: "R_no_follow_symlink",
			Args: []string{"-R", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.13: -R with -l does not follow symlink dirs.
		{
			Name: "Rl_no_follow_symlink",
			Args: []string{"-Rl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createRecursiveDotFixture creates a fixture with dotfiles in subdirs.
// R3.14: -R applies filter flags to each subdirectory.
func createRecursiveDotFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "visible.txt"), 5)
	writeFixtureFile(t, filepath.Join(dir, ".hidden"), 3)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(sub, "shown.txt"), 8)
	writeFixtureFile(t, filepath.Join(sub, ".dotfile"), 4)
	return dir
}

// TestDiffRecursiveFilter tests -R with filter flags (R3.14).
func TestDiffRecursiveFilter(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createRecursiveDotFixture(t)

	tests := []testutils.DiffTest{
		// R3.14: -R default hides dotfiles in subdirectories.
		{
			Name: "R_default_no_dots",
			Args: []string{"-R", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -Ra shows dotfiles including . and .. in subdirs.
		{
			Name: "Ra_all_in_subdirs",
			Args: []string{"-Ra", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -RA shows dotfiles except . and .. in subdirs.
		{
			Name: "RA_almost_all_in_subdirs",
			Args: []string{"-RA", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createRecursiveSortFixture creates subdirectories with distinct mtimes.
// R3.15: -R lists subdirs in the same sort order as entries.
func createRecursiveSortFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "file.txt"), 5)
	dirs := []string{"zzz", "aaa", "mmm"}
	now := time.Now()
	for i, name := range dirs {
		sub := filepath.Join(dir, name)
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFixtureFile(t, filepath.Join(sub, "child.txt"), 10)
		// zzz newest (1h), mmm middle (2h), aaa oldest (3h)
		setMtime(t, sub, now.Add(-time.Duration(i+1)*time.Hour))
	}
	return dir
}

// TestDiffRecursiveSort tests -R sort order (R3.15).
func TestDiffRecursiveSort(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createRecursiveSortFixture(t)

	tests := []testutils.DiffTest{
		// R3.15: -Rt subdirectories recursed in time order.
		{
			Name: "Rt_time_order",
			Args: []string{"-Rt", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -Rtr reverse time order for subdirectories.
		{
			Name: "Rtr_reverse_time",
			Args: []string{"-Rtr", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -R default (name sort) for subdirectories.
		{
			Name: "R_name_order",
			Args: []string{"-R", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stderrNormalizer normalizes stderr for differential testing of error cases.
// R4.2/R4.3: normalizes binary name in error messages and removes the
// "Try '...' --help" advice line that GNU ls appends after errors.
func stderrNormalizer(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	var result [][]byte
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("Try ")) {
			continue
		}
		line = normalizeBinaryName(line)
		result = append(result, line)
	}
	return bytes.Join(result, []byte("\n"))
}

// normalizeBinaryName replaces binary path/name prefix with "ls" in
// error lines. Handles full paths (/opt/homebrew/bin/gls:) and bare
// names (gls:, ls:) by checking the base name of the prefix.
func normalizeBinaryName(line []byte) []byte {
	idx := bytes.Index(line, []byte(": "))
	if idx <= 0 {
		return line
	}
	base := filepath.Base(string(line[:idx]))
	if base == "ls" || base == "gls" {
		return append([]byte("ls"), line[idx:]...)
	}
	return line
}

// TestDiffExitCodes tests exit code behavior (R4.1, R4.2, R4.3).
func TestDiffExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R4.1: exit 0 on successful listing.
		{
			Name: "exit_0_success",
			Args: []string{"-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2: exit 1 on nonexistent path, diagnostic on stderr.
		{
			Name: "exit_1_nonexistent",
			Args: []string{"--color=never", "/nonexistent_path_4556"},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.2: exit 1 with mixed valid/invalid, remaining listed.
		{
			Name: "exit_1_mixed_paths",
			Args: []string{"-1", "--color=never", "/nonexistent_path_4556", dir},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R4.3: exit 2 on invalid option.
		{
			Name: "exit_2_invalid_option",
			Args: []string{"-z"},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNImpliesLong tests -n implies -l (R4.6).
func TestDiffNImpliesLong(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R4.6: -n alone produces long format with numeric UID/GID.
		{
			Name: "n_implies_long",
			Args: []string{"-n", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.6: -n with -l is redundant but valid.
		{
			Name: "nl_redundant",
			Args: []string{"-nl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFormatMutex tests format flag mutual exclusivity (R4.7).
func TestDiffFormatMutex(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R4.7: -l after -C overrides to long format.
		{
			Name: "l_overrides_C",
			Args: []string{"-C", "-l", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -1 after -x overrides to single-column.
		{
			Name: "1_overrides_x",
			Args: []string{"-x", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -C after -1 overrides to multi-column.
		{
			Name: "C_overrides_1",
			Args: []string{"-1", "-C", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -x after -l overrides to horizontal multi-column.
		{
			Name: "x_overrides_l",
			Args: []string{"-l", "-x", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveCombined tests -R combined with -i, -s, -F (R4.9).
func TestDiffRecursiveCombined(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createRecursiveFixture(t)

	tests := []testutils.DiffTest{
		// R4.9: -Ri shows inode numbers in each subdirectory.
		{
			Name: "Ri_inode_recursive",
			Args: []string{"-Ri", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.9: -Rs shows block counts in each subdirectory.
		{
			Name: "Rs_blocks_recursive",
			Args: []string{"-Rs", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.9: -RisF combines inode, blocks, and classify in recursive.
		{
			Name: "RisF_all_combined",
			Args: []string{"-RisF", "-1", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.9: -Ril combines inode with long format in recursive.
		{
			Name: "Ril_inode_long_recursive",
			Args: []string{"-Ril", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.9: -Rsl combines blocks with long format in recursive.
		{
			Name: "Rsl_blocks_long_recursive",
			Args: []string{"-Rsl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveLongTotal tests -R with -l total block line (R4.8).
func TestDiffRecursiveLongTotal(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}
	dir := createRecursiveFixture(t)

	tests := []testutils.DiffTest{
		// R4.8: -Rl produces total block line for each subdirectory.
		{
			Name: "Rl_subdir_totals",
			Args: []string{"-Rl", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.8: -Rls combines recursive long format with block counts.
		{
			Name: "Rls_subdir_block_totals",
			Args: []string{"-Rls", "--color=never", dir},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
