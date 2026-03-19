// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd008-ls R1.1–R1.14, R2.1–R2.15, R3.1–R3.3:
// default listing, single-column output, C locale sorting, dotfile filtering,
// horizontal multi-column output, last-format-flag-wins, -a/-A show all,
// long format owner, group, size, date field rendering, link count,
// device major/minor, timestamps, total block count, inode display (-i),
// block count display (-s), numeric UID/GID (-n), combined -i -s prefix
// ordering, and --color flag support.
package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// R1.1 fixture: basic directory with known files.
	dirBasic := t.TempDir()
	makeFiles(t, dirBasic, "alpha", "beta", "gamma")

	// R1.2 fixture: directory with more entries for single-column verification.
	dirMulti := t.TempDir()
	makeFiles(t, dirMulti, "one", "two", "three", "four")

	// R1.2 fixture: a file argument.
	fileArgDir := t.TempDir()
	makeFiles(t, fileArgDir, "testfile")

	// R1.3 fixture: mixed-case names for C locale sort order.
	dirMixedCase := t.TempDir()
	makeFiles(t, dirMixedCase, "Z", "a", "B", "c")

	// R1.3 fixture: numeric names for bytewise (not natural) sorting.
	dirNumbers := t.TempDir()
	makeFiles(t, dirNumbers, "file10", "file2", "file1")

	// R1.4 fixture: directory with dotfiles and regular files.
	dirDot := t.TempDir()
	makeFiles(t, dirDot, ".hidden", "visible", ".secret", "public")

	// R1.13 fixture: many entries to exercise horizontal multi-column layout.
	dirHoriz := t.TempDir()
	makeFiles(t, dirHoriz, "aa", "bb", "cc", "dd", "ee", "ff", "gg", "hh")

	// R1.14 fixture: entries for format flag override verification.
	dirFmtOverride := t.TempDir()
	makeFiles(t, dirFmtOverride, "alpha", "beta", "gamma")

	tests := []testutils.DiffTest{
		// R1.1: default directory listing with no arguments.
		{
			Name:    "r1.1_no_args_lists_workdir",
			WorkDir: dirBasic,
		},
		// R1.1: default directory listing with an explicit directory argument.
		{
			Name: "r1.1_explicit_dir_arg",
			Args: []string{dirBasic},
		},
		// R1.1: listing an empty directory produces no output.
		{
			Name:    "r1.1_empty_dir",
			WorkDir: t.TempDir(),
		},

		// R1.2: single-column output when stdout is not a TTY (piped).
		{
			Name:    "r1.2_single_column_multiple_entries",
			WorkDir: dirMulti,
		},
		// R1.2: single file argument prints just the path.
		{
			Name: "r1.2_file_argument",
			Args: []string{filepath.Join(fileArgDir, "testfile")},
		},

		// R1.3: C locale sorts uppercase before lowercase (bytewise).
		{
			Name:    "r1.3_c_locale_mixed_case",
			WorkDir: dirMixedCase,
		},
		// R1.3: C locale sorts digit strings bytewise, not numerically.
		{
			Name:    "r1.3_c_locale_numbers",
			WorkDir: dirNumbers,
		},

		// R1.4: dotfiles are hidden by default.
		{
			Name:    "r1.4_default_hides_dotfiles",
			WorkDir: dirDot,
		},
		// R1.4: -a shows all entries including . and ..
		{
			Name:    "r1.4_show_all_with_a",
			Args:    []string{"-a"},
			WorkDir: dirDot,
		},
		// R1.4: -A shows dotfiles except . and ..
		{
			Name:    "r1.4_almost_all_with_A",
			Args:    []string{"-A"},
			WorkDir: dirDot,
		},

		// R1.13: -x produces horizontal multi-column output.
		{
			Name:    "r1.13_horizontal_x_flag",
			Args:    []string{"-x"},
			WorkDir: dirHoriz,
		},
		// R1.13: -x with explicit directory argument.
		{
			Name: "r1.13_horizontal_x_explicit_dir",
			Args: []string{"-x", dirHoriz},
		},
		// R1.13: -x on a directory with few entries.
		{
			Name:    "r1.13_horizontal_x_few_entries",
			Args:    []string{"-x"},
			WorkDir: dirBasic,
		},

		// R1.14: -l after -C overrides to long format (last flag wins).
		{
			Name:    "r1.14_C_then_l_long_wins",
			Args:    []string{"-Cl"},
			WorkDir: dirFmtOverride,
		},
		// R1.14: -C after -l overrides to columnar format (last flag wins).
		{
			Name:    "r1.14_l_then_C_columns_wins",
			Args:    []string{"-lC"},
			WorkDir: dirFmtOverride,
		},
		// R1.14: -1 after -x overrides to single-column (last flag wins).
		{
			Name:    "r1.14_x_then_1_single_wins",
			Args:    []string{"-x1"},
			WorkDir: dirFmtOverride,
		},
		// R1.14: -x after -1 overrides to horizontal columns (last flag wins).
		{
			Name:    "r1.14_1_then_x_horiz_wins",
			Args:    []string{"-1x"},
			WorkDir: dirFmtOverride,
		},

		// R2.1: -a includes entries starting with . including . and ..
		{
			Name:    "r2.1_show_all_includes_dot_dotdot",
			Args:    []string{"-a"},
			WorkDir: dirDot,
		},
		// R2.1: -a with -1 for clear single-column verification.
		{
			Name:    "r2.1_show_all_single_column",
			Args:    []string{"-a", "-1"},
			WorkDir: dirDot,
		},

		// R2.2: -A includes dotfiles but excludes . and ..
		{
			Name:    "r2.2_almost_all_excludes_dot_dotdot",
			Args:    []string{"-A"},
			WorkDir: dirDot,
		},
		// R2.2: -A with -1 for clear single-column verification.
		{
			Name:    "r2.2_almost_all_single_column",
			Args:    []string{"-A", "-1"},
			WorkDir: dirDot,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffLongFormat tests long format owner, group, size, and date fields.
// Implements prd008-ls R2.3 (owner), R2.4 (group), R2.5 (size), R2.6 (mtime).
func TestDiffLongFormat(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// Fixture: directory with files of varying sizes.
	dirSizes := t.TempDir()
	makeFileWithContent(t, dirSizes, "empty", "")
	makeFileWithContent(t, dirSizes, "small", "hello\n")
	makeFileWithContent(t, dirSizes, "medium", makePadding(1024))

	// Fixture: directory with a single file for simple long-format verification.
	dirSingle := t.TempDir()
	makeFiles(t, dirSingle, "onefile")

	// Fixture: directory with multiple files for column alignment verification.
	dirAlign := t.TempDir()
	makeFileWithContent(t, dirAlign, "tiny", "x")
	makeFileWithContent(t, dirAlign, "bigger", makePadding(10000))

	// Fixture: directory with a symlink for symlink display in long format.
	dirSymlink := t.TempDir()
	makeFiles(t, dirSymlink, "target")
	if err := os.Symlink("target", filepath.Join(dirSymlink, "link")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Fixture: directory with an old file (mtime > 6 months ago) for year display.
	dirOldFile := t.TempDir()
	makeFiles(t, dirOldFile, "oldfile")
	oldTime := time.Now().Add(-8 * 30 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dirOldFile, "oldfile"), oldTime, oldTime); err != nil {
		t.Fatalf("setting old mtime: %v", err)
	}

	// Fixture: mixed recent and old files for date format variation.
	dirMixed := t.TempDir()
	makeFiles(t, dirMixed, "recent", "ancient")
	ancientTime := time.Now().Add(-400 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dirMixed, "ancient"), ancientTime, ancientTime); err != nil {
		t.Fatalf("setting ancient mtime: %v", err)
	}

	// Fixture: directory with dotfiles for -la combination.
	dirDotLong := t.TempDir()
	makeFiles(t, dirDotLong, ".hidden", "visible")

	tests := []testutils.DiffTest{
		// R2.3/R2.4: -l shows owner and group fields.
		{
			Name:    "long_format_basic",
			Args:    []string{"-l"},
			WorkDir: dirSingle,
		},
		// R2.5: -l shows correct file sizes for varying file sizes.
		{
			Name:    "long_format_sizes",
			Args:    []string{"-l"},
			WorkDir: dirSizes,
		},
		// R2.5: size column alignment with different-width sizes.
		{
			Name:    "long_format_size_alignment",
			Args:    []string{"-l"},
			WorkDir: dirAlign,
		},
		// R2.6: recent file shows HH:MM format.
		{
			Name:    "long_format_recent_mtime",
			Args:    []string{"-l"},
			WorkDir: dirSingle,
		},
		// R2.6: old file shows year format.
		{
			Name:    "long_format_old_mtime",
			Args:    []string{"-l"},
			WorkDir: dirOldFile,
		},
		// R2.6: mixed recent and old files show different date formats.
		{
			Name:    "long_format_mixed_dates",
			Args:    []string{"-l"},
			WorkDir: dirMixed,
		},
		// Long format with symlink display.
		{
			Name:    "long_format_symlink",
			Args:    []string{"-l"},
			WorkDir: dirSymlink,
		},
		// Long format with -a flag shows . and .. with correct fields.
		{
			Name:    "long_format_with_all",
			Args:    []string{"-la"},
			WorkDir: dirDotLong,
		},
		// Long format with -A flag shows dotfiles except . and ..
		{
			Name:    "long_format_with_almost_all",
			Args:    []string{"-lA"},
			WorkDir: dirDotLong,
		},
		// Long format file argument (no total line).
		{
			Name: "long_format_file_arg",
			Args: []string{"-l", filepath.Join(dirSingle, "onefile")},
		},
		// Long format with explicit directory argument.
		{
			Name: "long_format_explicit_dir",
			Args: []string{"-l", dirSizes},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffLongFormatExtended tests long format link count, device major/minor,
// timestamps, and total block count.
func TestDiffLongFormatExtended(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// R2.7: hard link count display in long format.
	dirLinks := t.TempDir()
	makeFiles(t, dirLinks, "original")
	src := filepath.Join(dirLinks, "original")
	dst := filepath.Join(dirLinks, "hardlink")
	if err := os.Link(src, dst); err != nil {
		t.Fatalf("creating hard link: %v", err)
	}

	// R2.10: total block count header line.
	dirTotal := t.TempDir()
	makeFileWithContent(t, dirTotal, "file1", makePadding(512))
	makeFileWithContent(t, dirTotal, "file2", makePadding(1024))

	// R2.9: recent vs old timestamp formatting.
	dirTimestamps := t.TempDir()
	makeFiles(t, dirTimestamps, "recent_file", "old_file")
	oldTime := time.Now().Add(-400 * 24 * time.Hour)
	oldPath := filepath.Join(dirTimestamps, "old_file")
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("setting old mtime: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.7: link count shows nlink=2 for hard-linked files.
		{
			Name:    "long_format_hardlink_nlink",
			Args:    []string{"-l"},
			WorkDir: dirLinks,
		},
		// R2.8: device file shows major,minor instead of size.
		{
			Name: "long_format_device_null",
			Args: []string{"-l", "/dev/null"},
		},
		// R2.8: multiple device files in one listing.
		{
			Name: "long_format_device_multiple",
			Args: []string{"-l", "/dev/null", "/dev/zero"},
		},
		// R2.9: mixed recent and old timestamps.
		{
			Name:    "long_format_timestamp_mixed",
			Args:    []string{"-l"},
			WorkDir: dirTimestamps,
		},
		// R2.10: total block count header line.
		{
			Name:    "long_format_total_blocks",
			Args:    []string{"-l"},
			WorkDir: dirTotal,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffInodeBlocksNumeric tests inode display (-i), block count display (-s),
// and numeric UID/GID (-n).
func TestDiffInodeBlocksNumeric(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// Fixture: directory with files for inode/block tests.
	dirFiles := t.TempDir()
	makeFiles(t, dirFiles, "alpha", "beta")
	makeFileWithContent(t, dirFiles, "larger", makePadding(4096))

	// Fixture: single file for file-argument tests.
	dirSingle := t.TempDir()
	makeFiles(t, dirSingle, "onefile")

	// Fixture: directory with symlink for -il combination.
	dirSymlink := t.TempDir()
	makeFiles(t, dirSymlink, "target")
	if err := os.Symlink("target", filepath.Join(dirSymlink, "link")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.11: -i shows inode in single-column output.
		{
			Name:    "inode_single_column",
			Args:    []string{"-i"},
			WorkDir: dirFiles,
		},
		// R2.11: -i with -l shows inode before permissions in long format.
		{
			Name:    "inode_long_format",
			Args:    []string{"-il"},
			WorkDir: dirFiles,
		},
		// R2.11: -i with file argument (no total line).
		{
			Name: "inode_file_arg",
			Args: []string{"-i", filepath.Join(dirSingle, "onefile")},
		},
		// R2.11: -i with symlink in long format.
		{
			Name:    "inode_long_symlink",
			Args:    []string{"-il"},
			WorkDir: dirSymlink,
		},
		// R2.12: -s shows block count in single-column output with total.
		{
			Name:    "blocks_single_column",
			Args:    []string{"-s"},
			WorkDir: dirFiles,
		},
		// R2.12: -s with -l shows blocks before permissions.
		{
			Name:    "blocks_long_format",
			Args:    []string{"-sl"},
			WorkDir: dirFiles,
		},
		// R2.12: -s with file argument (no total line).
		{
			Name: "blocks_file_arg",
			Args: []string{"-s", filepath.Join(dirSingle, "onefile")},
		},
		// R2.13: -s -l total line includes block counts.
		{
			Name:    "blocks_long_total",
			Args:    []string{"-sl"},
			WorkDir: dirFiles,
		},
		// R2.11+R2.12: -i -s combined shows both inode and blocks.
		{
			Name:    "inode_and_blocks_combined",
			Args:    []string{"-is"},
			WorkDir: dirFiles,
		},
		// R2.11+R2.12: -i -s -l combined in long format.
		{
			Name:    "inode_and_blocks_long",
			Args:    []string{"-isl"},
			WorkDir: dirFiles,
		},
		// R2.14: -n shows numeric UID/GID in long format.
		{
			Name:    "numeric_ids",
			Args:    []string{"-n"},
			WorkDir: dirFiles,
		},
		// R2.14: -n with file argument.
		{
			Name: "numeric_ids_file_arg",
			Args: []string{"-n", filepath.Join(dirSingle, "onefile")},
		},
		// R2.14: -n with -i shows inode and numeric IDs.
		{
			Name:    "numeric_ids_with_inode",
			Args:    []string{"-ni"},
			WorkDir: dirFiles,
		},
		// R2.14: -n with -s shows blocks and numeric IDs.
		{
			Name:    "numeric_ids_with_blocks",
			Args:    []string{"-ns"},
			WorkDir: dirFiles,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffColorFlags tests --color flag support and combined -i -s ordering.
// Implements prd008-ls R2.15, R3.1, R3.2, R3.3.
func TestDiffColorFlags(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// Fixture: directory with varied file types for color tests.
	dirColor := t.TempDir()
	makeFiles(t, dirColor, "plain.txt")
	if err := os.Mkdir(filepath.Join(dirColor, "subdir"), 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	makeFiles(t, dirColor, "runme")
	if err := os.Chmod(filepath.Join(dirColor, "runme"), 0o755); err != nil {
		t.Fatalf("chmod runme: %v", err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(dirColor, "link")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Fixture: directory for combined -i -s ordering test.
	dirCombined := t.TempDir()
	makeFiles(t, dirCombined, "aaa", "bbb")
	makeFileWithContent(t, dirCombined, "ccc", makePadding(2048))

	tests := []testutils.DiffTest{
		// R2.15: -i and -s combined, inode first then blocks.
		{
			Name:    "r2.15_combined_is",
			Args:    []string{"-is"},
			WorkDir: dirCombined,
		},
		// R2.15: -i -s -l combined in long format.
		{
			Name:    "r2.15_combined_isl",
			Args:    []string{"-isl"},
			WorkDir: dirCombined,
		},
		// R2.15: -s -i order (same output as -is, both accepted).
		{
			Name:    "r2.15_combined_si",
			Args:    []string{"-si"},
			WorkDir: dirCombined,
		},
		// R3.1: --color=never produces no ANSI sequences.
		{
			Name:    "r3.1_color_never",
			Args:    []string{"-1", "--color=never"},
			WorkDir: dirColor,
		},
		// R3.1: --color=never with -l produces no ANSI sequences.
		{
			Name:    "r3.1_color_never_long",
			Args:    []string{"-l", "--color=never"},
			WorkDir: dirColor,
		},
		// R3.2: --color=auto piped (no TTY) produces no ANSI.
		{
			Name:    "r3.2_color_auto_piped",
			Args:    []string{"-1", "--color=auto"},
			WorkDir: dirColor,
		},
		// R3.2: --color=auto with -l piped produces no ANSI.
		{
			Name:    "r3.2_color_auto_piped_long",
			Args:    []string{"-l", "--color=auto"},
			WorkDir: dirColor,
		},
		// R3.3: --color=always with -1 (ANSI stripped for comparison).
		{
			Name:      "r3.3_color_always_single",
			Args:      []string{"-1", "--color=always"},
			WorkDir:   dirColor,
			Normalize: []testutils.NormalizeFunc{stripANSI},
		},
		// R3.3: --color=always with -l (ANSI stripped for comparison).
		{
			Name:      "r3.3_color_always_long",
			Args:      []string{"-l", "--color=always"},
			WorkDir:   dirColor,
			Normalize: []testutils.NormalizeFunc{stripANSI},
		},
		// R3.1: bare --color defaults to always (ANSI stripped for comparison).
		{
			Name:      "r3.1_bare_color_flag",
			Args:      []string{"-1", "--color"},
			WorkDir:   dirColor,
			Normalize: []testutils.NormalizeFunc{stripANSI},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestColorAlwaysProducesANSI verifies that --color=always emits ANSI codes.
// This is a unit test (not differential) to verify R3.3 color output presence.
func TestColorAlwaysProducesANSI(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	cmd := exec.Command(goBin, "-1", "--color=always", dir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running ls: %v", err)
	}
	if !bytes.Contains(out, []byte("\033[")) {
		t.Errorf("--color=always should contain ANSI escape sequences, got: %q", out)
	}
}

// TestColorNeverNoANSI verifies that --color=never emits no ANSI codes.
// R3.1/R3.4: when color is suppressed, no ANSI sequences in output.
func TestColorNeverNoANSI(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	makeFiles(t, dir, "runme")
	if err := os.Chmod(filepath.Join(dir, "runme"), 0o755); err != nil {
		t.Fatalf("chmod runme: %v", err)
	}

	cmd := exec.Command(goBin, "-1", "--color=never", dir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running ls: %v", err)
	}
	if bytes.Contains(out, []byte("\033[")) {
		t.Errorf("--color=never should not contain ANSI sequences, got: %q", out)
	}
}

// stripANSI removes ANSI escape sequences for differential test normalization.
// Used with --color=always tests where exact color codes may differ between
// our implementation and gls due to LS_COLORS settings.
func stripANSI(b []byte) []byte {
	var result []byte
	i := 0
	for i < len(b) {
		if i+1 < len(b) && b[i] == '\033' && b[i+1] == '[' {
			j := i + 2
			for j < len(b) && !isANSITerminator(b[j]) {
				j++
			}
			if j < len(b) {
				i = j + 1
				continue
			}
		}
		result = append(result, b[i])
		i++
	}
	return result
}

// isANSITerminator returns true for the final byte of an ANSI CSI sequence.
func isANSITerminator(b byte) bool {
	return b >= 0x40 && b <= 0x7E
}

// makeFiles creates empty regular files in dir.
func makeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatalf("creating fixture file %s: %v", n, err)
		}
	}
}

// makeFileWithContent creates a file with the given content.
func makeFileWithContent(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("creating fixture file %s: %v", name, err)
	}
}

// makePadding returns a string of n 'x' characters.
func makePadding(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
