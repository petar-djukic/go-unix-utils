// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ls against gls (GNU coreutils).
// Implements prd008-ls R1.1-R1.14, R2.5-R2.12 test coverage.
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

// normalizeLongFormat normalizes long-format output to handle expected
// differences between runs: mtime field varies with wall clock.
func normalizeLongFormat(data []byte) []byte {
	re := regexp.MustCompile(`(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d+\s+[\d: ]+\d`)
	data = re.ReplaceAll(data, []byte("MTIME_NORM"))
	return data
}

// normalizeTotalLine normalizes the "total N" line since block counts can
// differ across filesystems.
func normalizeTotalLine(data []byte) []byte {
	re := regexp.MustCompile(`total \d+`)
	data = re.ReplaceAll(data, []byte("total BLOCKS"))
	return data
}

// normalizeStderrProgName normalizes the program name prefix in stderr messages
// so "gls:" matches "ls:".
func normalizeStderrProgName(data []byte) []byte {
	re := regexp.MustCompile(`^gls:`)
	data = re.ReplaceAll(data, []byte("ls:"))
	return data
}

// normalizeErrorCase normalizes error message case differences between
// Go and GNU C runtime (e.g., "No such" vs "no such").
func normalizeErrorCase(data []byte) []byte {
	re := regexp.MustCompile(`(?i)(no such file or directory)`)
	data = re.ReplaceAll(data, []byte("No such file or directory"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// Create test fixture directory with known contents.
	tmpDir := t.TempDir()
	fixtureDir := filepath.Join(tmpDir, "fixture")
	if err := os.Mkdir(fixtureDir, 0o755); err != nil {
		t.Fatalf("creating fixture dir: %v", err)
	}

	// Create regular files.
	writeFile(t, filepath.Join(fixtureDir, "alpha.txt"), "alpha content\n")
	writeFile(t, filepath.Join(fixtureDir, "bravo.txt"), "bravo content here\n")
	writeFile(t, filepath.Join(fixtureDir, "charlie.txt"), "c\n")

	// Create a subdirectory.
	subDir := filepath.Join(fixtureDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	// Create a dotfile (hidden by default).
	writeFile(t, filepath.Join(fixtureDir, ".hidden"), "hidden\n")

	// Create a symlink.
	if err := os.Symlink("alpha.txt", filepath.Join(fixtureDir, "link-to-alpha")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Create an empty directory for edge cases.
	emptyDir := filepath.Join(tmpDir, "emptydir")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatalf("creating empty dir: %v", err)
	}

	// Create a nested subdirectory for -R recursive tests.
	nestedDir := filepath.Join(subDir, "nested")
	if err := os.Mkdir(nestedDir, 0o755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}
	writeFile(t, filepath.Join(subDir, "sub-file.txt"), "sub content\n")
	writeFile(t, filepath.Join(nestedDir, "deep-file.txt"), "deep content\n")

	// R2.5/R2.6: Create a fixture with files of known different sizes and mtimes.
	sortDir := filepath.Join(tmpDir, "sortfix")
	if err := os.Mkdir(sortDir, 0o755); err != nil {
		t.Fatalf("creating sort fixture dir: %v", err)
	}
	// Files with different sizes for -S testing.
	writeFile(t, filepath.Join(sortDir, "small.txt"), "a\n")
	writeFile(t, filepath.Join(sortDir, "medium.txt"), "medium content here\n")
	writeFile(t, filepath.Join(sortDir, "large.txt"), "this is a much larger file with significantly more content than the others for testing size sort\n")
	writeFile(t, filepath.Join(sortDir, "tiny.txt"), "x")
	// Set distinct modification times for -t testing.
	// oldest -> newest: tiny, small, medium, large
	now := time.Now()
	setMtime(t, filepath.Join(sortDir, "tiny.txt"), now.Add(-4*time.Hour))
	setMtime(t, filepath.Join(sortDir, "small.txt"), now.Add(-3*time.Hour))
	setMtime(t, filepath.Join(sortDir, "medium.txt"), now.Add(-2*time.Hour))
	setMtime(t, filepath.Join(sortDir, "large.txt"), now.Add(-1*time.Hour))

	// R2.9: Create a fixture for version sort testing.
	versionDir := filepath.Join(tmpDir, "versionfix")
	if err := os.Mkdir(versionDir, 0o755); err != nil {
		t.Fatalf("creating version fixture dir: %v", err)
	}
	writeFile(t, filepath.Join(versionDir, "file1"), "")
	writeFile(t, filepath.Join(versionDir, "file2"), "")
	writeFile(t, filepath.Join(versionDir, "file10"), "")
	writeFile(t, filepath.Join(versionDir, "file20"), "")
	writeFile(t, filepath.Join(versionDir, "file3"), "")

	longNorm := []testutils.NormalizeFunc{
		normalizeLongFormat,
		normalizeTotalLine,
	}

	stderrNorm := []testutils.NormalizeFunc{
		normalizeStderrProgName,
		normalizeErrorCase,
	}

	tests := []testutils.DiffTest{
		// R1.1/R1.2: Default single-column output (piped = non-TTY).
		{
			Name:    "R1.1_R1.2_default_single_column",
			Args:    []string{fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.4: Dotfiles hidden by default.
		{
			Name:    "R1.4_dotfiles_hidden",
			Args:    []string{fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.5: -1 forces single-column output.
		{
			Name:    "R1.5_single_column_flag",
			Args:    []string{"-1", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.5: -1 on empty directory.
		{
			Name:    "R1.5_single_column_empty_dir",
			Args:    []string{"-1", emptyDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.6/R1.7/R1.8: -l long format.
		{
			Name:      "R1.6_R1.7_R1.8_long_format",
			Args:      []string{"-l", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.6: -l on empty directory (just "total 0").
		{
			Name:      "R1.6_long_format_empty_dir",
			Args:      []string{"-l", emptyDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.5/R1.6: -l after -1 (long format wins in GNU ls).
		{
			Name:      "R1.5_R1.6_l_after_1",
			Args:      []string{"-1", "-l", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.5/R1.6: -1 after -l (GNU ls: -l still wins, -1 is one-per-line which -l already is).
		{
			Name:      "R1.5_R1.6_1_after_l",
			Args:      []string{"-l", "-1", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.6: Combined flags -1l (last char = -l).
		{
			Name:      "R1.6_combined_flags_1l",
			Args:      []string{"-1l", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.6: Combined flags -l1 (GNU ls: -l still active).
		{
			Name:      "R1.6_combined_flags_l1",
			Args:      []string{"-l1", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.7/R1.10: -l shows symlink with " -> target".
		{
			Name:      "R1.7_R1.10_long_format_symlink",
			Args:      []string{"-l", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.6: -l with single file argument (no total line).
		{
			Name:      "R1.6_long_format_single_file",
			Args:      []string{"-l", filepath.Join(fixtureDir, "alpha.txt")},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R4.2: Non-existent path exits 2 with diagnostic.
		{
			Name:      "R4.2_nonexistent_path",
			Args:      []string{filepath.Join(tmpDir, "no-such-file")},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			ExitCode:  2,
			Normalize: stderrNorm,
		},
		// R2.1: -a includes dotfiles including "." and "..".
		{
			Name:    "R2.1_show_all",
			Args:    []string{"-a", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.1: -a with -l (long format with dotfiles).
		{
			Name:      "R2.1_show_all_long",
			Args:      []string{"-la", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R2.7: -r reverses alphabetical sort.
		{
			Name:    "R2.7_reverse_sort",
			Args:    []string{"-r", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.7: -r with -l (long format reversed).
		{
			Name:      "R2.7_reverse_long",
			Args:      []string{"-lr", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R2.1/R2.7: -a and -r combined.
		{
			Name:    "R2.1_R2.7_all_reverse",
			Args:    []string{"-ar", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R3.11: -R recursive listing.
		{
			Name:    "R3.11_recursive",
			Args:    []string{"-R", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R3.11/R3.12: -R with -l (recursive long format).
		{
			Name:      "R3.11_R3.12_recursive_long",
			Args:      []string{"-lR", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R3.14: -R with -a (recursive including dotfiles).
		{
			Name:    "R3.14_recursive_all",
			Args:    []string{"-aR", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R3.11/R2.7: -R with -r (recursive reversed).
		{
			Name:    "R3.11_R2.7_recursive_reverse",
			Args:    []string{"-Rr", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.12: -p appends '/' to directories.
		{
			Name:    "R1.12_indicator_slash",
			Args:    []string{"-p", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.12: -p with -l (long format with indicator).
		{
			Name:      "R1.12_indicator_long",
			Args:      []string{"-lp", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.12/R2.1: -p with -a (all entries with indicator).
		{
			Name:    "R1.12_R2.1_indicator_all",
			Args:    []string{"-ap", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.12: -p on empty directory.
		{
			Name:    "R1.12_indicator_empty_dir",
			Args:    []string{"-p", emptyDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.13: -x horizontal multi-column output.
		{
			Name:    "R1.13_horizontal_columns",
			Args:    []string{"-x", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.13: -x on empty directory.
		{
			Name:    "R1.13_horizontal_empty_dir",
			Args:    []string{"-x", emptyDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.13: -x with -a (horizontal with dotfiles).
		{
			Name:    "R1.13_horizontal_all",
			Args:    []string{"-xa", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.13: -x with -r (horizontal reversed).
		{
			Name:    "R1.13_horizontal_reverse",
			Args:    []string{"-xr", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.13: -x with -p (horizontal with directory indicator).
		{
			Name:    "R1.13_horizontal_indicator",
			Args:    []string{"-xp", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -C multi-column vertical fill (piped).
		{
			Name:    "R1.14_C_vertical_columns",
			Args:    []string{"-C", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -l after -x (long format overrides horizontal).
		{
			Name:      "R1.14_l_after_x",
			Args:      []string{"-x", "-l", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.14: -x after -l (horizontal overrides long).
		{
			Name:    "R1.14_x_after_l",
			Args:    []string{"-l", "-x", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -1 after -x (single-column overrides horizontal).
		{
			Name:    "R1.14_1_after_x",
			Args:    []string{"-x", "-1", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -x after -1 (horizontal overrides single-column).
		{
			Name:    "R1.14_x_after_1",
			Args:    []string{"-1", "-x", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -C after -l (multi-column overrides long).
		{
			Name:    "R1.14_C_after_l",
			Args:    []string{"-l", "-C", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -l after -C (long overrides multi-column).
		{
			Name:      "R1.14_l_after_C",
			Args:      []string{"-C", "-l", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R1.14: -x after -C (horizontal overrides vertical).
		{
			Name:    "R1.14_x_after_C",
			Args:    []string{"-C", "-x", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R1.14: -C after -x (vertical overrides horizontal).
		{
			Name:    "R1.14_C_after_x",
			Args:    []string{"-x", "-C", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},

		// === R2.5: -t sorts by modification time, newest first ===
		{
			Name:    "R2.5_time_sort",
			Args:    []string{"-1", "-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.5: -t with -l (long format, time-sorted).
		{
			Name:      "R2.5_time_sort_long",
			Args:      []string{"-lt", sortDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},

		// === R2.6: -S sorts by file size, largest first ===
		{
			Name:    "R2.6_size_sort",
			Args:    []string{"-1", "-S", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.6: -S with -l (long format, size-sorted).
		{
			Name:      "R2.6_size_sort_long",
			Args:      []string{"-lS", sortDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},

		// === R2.7: -r reverses the current sort order ===
		// R2.7: -r with -t (reverse time sort = oldest first).
		{
			Name:    "R2.7_reverse_time_sort",
			Args:    []string{"-1", "-tr", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.7: -r with -S (reverse size sort = smallest first).
		{
			Name:    "R2.7_reverse_size_sort",
			Args:    []string{"-1", "-Sr", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.7: -r with -l -t (reverse time, long format).
		{
			Name:      "R2.7_reverse_time_long",
			Args:      []string{"-ltr", sortDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},

		// === R2.8: -U disables sorting (directory order) ===
		{
			Name:    "R2.8_unsorted",
			Args:    []string{"-1", "-U", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.8: -U overrides -t (last flag wins per R2.10, but -U also
		// explicitly overrides time/size sort).
		{
			Name:    "R2.8_unsorted_overrides_time",
			Args:    []string{"-1", "-t", "-U", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.8: -U overrides -S.
		{
			Name:    "R2.8_unsorted_overrides_size",
			Args:    []string{"-1", "-S", "-U", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.8: -r with -U is accepted without error.
		{
			Name:    "R2.8_unsorted_with_reverse",
			Args:    []string{"-1", "-Ur", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.10: -t after -U re-enables time sort (last sort flag wins).
		{
			Name:    "R2.10_time_after_unsorted",
			Args:    []string{"-1", "-U", "-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},

		// === R2.9: -v version sort ===
		{
			Name:    "R2.9_version_sort",
			Args:    []string{"-1", "-v", versionDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.9: -v with -r (reverse version sort).
		{
			Name:    "R2.9_version_sort_reverse",
			Args:    []string{"-1", "-vr", versionDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},

		// === R2.10: Last sort flag wins ===
		// R2.10: -v after -t (version sort overrides time sort).
		{
			Name:    "R2.10_version_after_time",
			Args:    []string{"-1", "-t", "-v", versionDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.10: -t after -v (time sort overrides version sort).
		{
			Name:    "R2.10_time_after_version",
			Args:    []string{"-1", "-v", "-t", sortDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},

		// === R2.11: -i inode display ===
		{
			Name:    "R2.11_inode_display",
			Args:    []string{"-1", "-i", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.11: -i with -l (long format with inode).
		{
			Name:      "R2.11_inode_long_format",
			Args:      []string{"-li", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},

		// === R2.12: -s block count display ===
		{
			Name:    "R2.12_blocks_display",
			Args:    []string{"-1", "-s", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
		// R2.12: -s with -l (long format with blocks).
		{
			Name:      "R2.12_blocks_long_format",
			Args:      []string{"-ls", fixtureDir},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   tmpDir,
			Normalize: longNorm,
		},
		// R2.15: -i and -s combined.
		{
			Name:    "R2.15_inode_and_blocks",
			Args:    []string{"-1", "-is", fixtureDir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: tmpDir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// setMtime is a test helper that sets the modification time of a file.
func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("setting mtime on %s: %v", path, err)
	}
}
