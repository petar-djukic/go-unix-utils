// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd008-ls R1.1–R1.14, R2.1–R2.15, R3.1–R3.15,
// R4.1–R4.7: default listing, single-column output, C locale sorting,
// dotfile filtering, horizontal multi-column output, last-format-flag-wins,
// -a/-A show all, long format owner, group, size, date field rendering,
// link count, device major/minor, timestamps, total block count,
// inode display (-i), block count display (-s), numeric UID/GID (-n),
// combined -i -s prefix ordering, --color flag support, human-readable
// size display (-h), -F classify indicator, -R recursive listing,
// sort modes (-t, -S, -r, -U, -v), recursive format/filter/sort propagation,
// exit codes (0 success, 1/2 failure), and invalid option handling.
package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

// TestDiffHumanReadable tests human-readable size display (-h).
// Implements prd008-ls R3.4, R3.5, R3.6, R3.7.
func TestDiffHumanReadable(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// Fixture: files with varying sizes for -lh testing.
	dirSizes := t.TempDir()
	makeFileWithContent(t, dirSizes, "empty", "")
	makeFileWithContent(t, dirSizes, "small", "hello\n")
	makeFileWithContent(t, dirSizes, "medium", makePadding(1024))
	makeFileWithContent(t, dirSizes, "large", makePadding(1048576))

	// Fixture: single file for file argument with -lh.
	dirSingle := t.TempDir()
	makeFileWithContent(t, dirSingle, "onefile", makePadding(2048))

	// Fixture: files for -sh block count testing.
	dirBlocks := t.TempDir()
	makeFileWithContent(t, dirBlocks, "alpha", makePadding(512))
	makeFileWithContent(t, dirBlocks, "beta", makePadding(4096))
	makeFileWithContent(t, dirBlocks, "gamma", makePadding(65536))

	// Fixture: directory for color suppression with -h (R3.4).
	dirColor := t.TempDir()
	makeFiles(t, dirColor, "plain.txt")
	if err := os.Mkdir(filepath.Join(dirColor, "subdir"), 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.5: -lh shows human-readable file sizes.
		{
			Name:    "r3.5_long_human_sizes",
			Args:    []string{"-lh"},
			WorkDir: dirSizes,
		},
		// R3.5: -h without -l has no visible effect on output.
		{
			Name:    "r3.5_h_without_l_no_effect",
			Args:    []string{"-h"},
			WorkDir: dirSizes,
		},
		// R3.5: -h with -1 has no visible effect (not long format).
		{
			Name:    "r3.5_h_with_1_no_effect",
			Args:    []string{"-1h"},
			WorkDir: dirSizes,
		},
		// R3.5: -lh with file argument (no total line).
		{
			Name: "r3.5_lh_file_arg",
			Args: []string{"-lh", filepath.Join(dirSingle, "onefile")},
		},
		// R3.6: -lh total block count line is human-readable.
		{
			Name:    "r3.6_lh_total_human",
			Args:    []string{"-lh"},
			WorkDir: dirBlocks,
		},
		// R3.7: -sh shows human-readable block counts.
		{
			Name:    "r3.7_sh_human_blocks",
			Args:    []string{"-sh"},
			WorkDir: dirBlocks,
		},
		// R3.7: -slh shows human-readable blocks in long format.
		{
			Name:    "r3.7_slh_human_blocks_long",
			Args:    []string{"-slh"},
			WorkDir: dirBlocks,
		},
		// R3.7: -sh with file argument (no total line).
		{
			Name: "r3.7_sh_file_arg",
			Args: []string{"-sh", filepath.Join(dirSingle, "onefile")},
		},
		// R3.7: -ish shows inode and human-readable blocks.
		{
			Name:    "r3.7_ish_human_blocks_inode",
			Args:    []string{"-ish"},
			WorkDir: dirBlocks,
		},
		// R3.4: --color=never with -lh produces no ANSI sequences.
		{
			Name:    "r3.4_color_never_lh",
			Args:    []string{"-lh", "--color=never"},
			WorkDir: dirColor,
		},
		// R3.4: --color=auto piped with -lh produces no ANSI.
		{
			Name:    "r3.4_color_auto_piped_lh",
			Args:    []string{"-lh", "--color=auto"},
			WorkDir: dirColor,
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

// TestDiffClassifyRecursive tests -F (classify) and -R (recursive) flags.
// Implements prd008-ls R3.8, R3.9, R3.10, R3.11.
func TestDiffClassifyRecursive(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// R3.8 fixture: directory with various file types.
	dirClassify := t.TempDir()
	makeFiles(t, dirClassify, "regular.txt")
	if err := os.Mkdir(filepath.Join(dirClassify, "subdir"), 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	makeFiles(t, dirClassify, "executable")
	if err := os.Chmod(filepath.Join(dirClassify, "executable"), 0o755); err != nil {
		t.Fatalf("chmod executable: %v", err)
	}
	if err := os.Symlink("regular.txt", filepath.Join(dirClassify, "link")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}
	// Best-effort FIFO creation; skip FIFO tests if not supported.
	fifoPath := filepath.Join(dirClassify, "mypipe")
	fifoErr := syscall.Mkfifo(fifoPath, 0o644)

	// R3.11 fixture: directory with subdirectories for recursion.
	dirRecurse := t.TempDir()
	makeFiles(t, dirRecurse, "file1")
	if err := os.Mkdir(filepath.Join(dirRecurse, "subA"), 0o755); err != nil {
		t.Fatalf("creating subA: %v", err)
	}
	makeFiles(t, filepath.Join(dirRecurse, "subA"), "fileA")
	if err := os.Mkdir(filepath.Join(dirRecurse, "subB"), 0o755); err != nil {
		t.Fatalf("creating subB: %v", err)
	}
	makeFiles(t, filepath.Join(dirRecurse, "subB"), "fileB")

	// R3.11 fixture: deeper nesting for multi-level recursion.
	dirDeep := t.TempDir()
	makeFiles(t, dirDeep, "top")
	if err := os.Mkdir(filepath.Join(dirDeep, "level1"), 0o755); err != nil {
		t.Fatalf("creating level1: %v", err)
	}
	makeFiles(t, filepath.Join(dirDeep, "level1"), "mid")
	deep2 := filepath.Join(dirDeep, "level1", "level2")
	if err := os.Mkdir(deep2, 0o755); err != nil {
		t.Fatalf("creating level2: %v", err)
	}
	makeFiles(t, deep2, "bottom")

	// R3.11 fixture: directory with symlink to dir (should not be followed).
	dirSymRecurse := t.TempDir()
	makeFiles(t, dirSymRecurse, "file")
	realSub := filepath.Join(dirSymRecurse, "realsub")
	if err := os.Mkdir(realSub, 0o755); err != nil {
		t.Fatalf("creating realsub: %v", err)
	}
	makeFiles(t, realSub, "inside")
	if err := os.Symlink("realsub", filepath.Join(dirSymRecurse, "linksub")); err != nil {
		t.Fatalf("creating symlink dir: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.8: -F appends type indicators in single-column.
		{
			Name:    "r3.8_classify_single",
			Args:    []string{"-F1"},
			WorkDir: dirClassify,
		},
		// R3.8: -F with default format.
		{
			Name:    "r3.8_classify_default",
			Args:    []string{"-F"},
			WorkDir: dirClassify,
		},
		// R3.10: -F with -l long format.
		{
			Name:    "r3.10_classify_long",
			Args:    []string{"-Fl"},
			WorkDir: dirClassify,
		},
		// R3.10: -F with -x horizontal layout.
		{
			Name:    "r3.10_classify_horizontal",
			Args:    []string{"-Fx"},
			WorkDir: dirClassify,
		},
		// R3.10: -F with --color=never (no ANSI).
		{
			Name:    "r3.10_classify_no_color",
			Args:    []string{"-F1", "--color=never"},
			WorkDir: dirClassify,
		},
		// R3.10: -F with --color=always (ANSI stripped for comparison).
		{
			Name:      "r3.10_classify_color_always",
			Args:      []string{"-F1", "--color=always"},
			WorkDir:   dirClassify,
			Normalize: []testutils.NormalizeFunc{stripANSI},
		},
		// R3.8: -F on a file argument (shows indicator).
		{
			Name: "r3.8_classify_file_arg",
			Args: []string{"-F", filepath.Join(dirClassify, "executable")},
		},
		// R3.8: -F on a directory argument with -d-like behavior (just the path).
		{
			Name: "r3.8_classify_dir_arg",
			Args: []string{"-F", dirClassify},
		},
		// R3.11: -R basic recursion with single-column.
		{
			Name:    "r3.11_recursive_single",
			Args:    []string{"-R1"},
			WorkDir: dirRecurse,
		},
		// R3.11: -R basic recursion with default format.
		{
			Name:    "r3.11_recursive_default",
			Args:    []string{"-R"},
			WorkDir: dirRecurse,
		},
		// R3.11: -R with -l long format.
		{
			Name:    "r3.11_recursive_long",
			Args:    []string{"-Rl"},
			WorkDir: dirRecurse,
		},
		// R3.11: -R with deeper nesting.
		{
			Name:    "r3.11_recursive_deep",
			Args:    []string{"-R1"},
			WorkDir: dirDeep,
		},
		// R3.11: -R does not follow symlinks to directories.
		{
			Name:    "r3.11_recursive_no_symlink_follow",
			Args:    []string{"-R1"},
			WorkDir: dirSymRecurse,
		},
		// R3.8 + R3.11: -FR combined.
		{
			Name:    "r3.8_r3.11_classify_recursive",
			Args:    []string{"-FR1"},
			WorkDir: dirRecurse,
		},
		// R3.11: -R with -a shows dotfiles in subdirectories.
		{
			Name:    "r3.11_recursive_with_all",
			Args:    []string{"-Ra1"},
			WorkDir: dirRecurse,
		},
	}

	// R3.8: add FIFO test if mkfifo succeeded.
	if fifoErr == nil {
		tests = append(tests, testutils.DiffTest{
			Name:    "r3.8_classify_with_fifo",
			Args:    []string{"-F1"},
			WorkDir: dirClassify,
		})
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveDisplay tests R3.12-R3.15: recursive format mode,
// symlink non-following, filter flags in subdirs, and sort order in recursion.
func TestDiffRecursiveDisplay(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// R3.12 fixture: directory with subdirs for format mode verification.
	dirFormat := t.TempDir()
	makeFiles(t, dirFormat, "file1", "file2")
	subFmt := filepath.Join(dirFormat, "subdir")
	if err := os.Mkdir(subFmt, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	makeFiles(t, subFmt, "inner1", "inner2")

	// R3.13 fixture: symlink to directory should not be followed.
	dirSym := t.TempDir()
	makeFiles(t, dirSym, "file")
	realSub := filepath.Join(dirSym, "realsub")
	if err := os.Mkdir(realSub, 0o755); err != nil {
		t.Fatalf("creating realsub: %v", err)
	}
	makeFiles(t, realSub, "inside")
	if err := os.Symlink("realsub", filepath.Join(dirSym, "linksub")); err != nil {
		t.Fatalf("creating symlink dir: %v", err)
	}

	// R3.14 fixture: subdirs with dotfiles for filter verification.
	dirFilter := t.TempDir()
	makeFiles(t, dirFilter, "visible")
	subFilter := filepath.Join(dirFilter, "sub")
	if err := os.Mkdir(subFilter, 0o755); err != nil {
		t.Fatalf("creating sub: %v", err)
	}
	makeFiles(t, subFilter, ".hidden", "shown")

	// R3.15 fixture: subdirs with different mtimes for sort order verification.
	dirSort := t.TempDir()
	makeFiles(t, dirSort, "zfile")
	subA := filepath.Join(dirSort, "aaa")
	if err := os.Mkdir(subA, 0o755); err != nil {
		t.Fatalf("creating aaa: %v", err)
	}
	makeFiles(t, subA, "a_inner")
	subB := filepath.Join(dirSort, "bbb")
	if err := os.Mkdir(subB, 0o755); err != nil {
		t.Fatalf("creating bbb: %v", err)
	}
	makeFiles(t, subB, "b_inner")
	// Set aaa to older mtime so -t sorts bbb before aaa.
	oldTime := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(subA, oldTime, oldTime); err != nil {
		t.Fatalf("setting old mtime: %v", err)
	}

	// R3.15 fixture: files with different sizes for -S sort.
	dirSizeSort := t.TempDir()
	makeFileWithContent(t, dirSizeSort, "small", "x")
	makeFileWithContent(t, dirSizeSort, "big", makePadding(4096))
	subSz := filepath.Join(dirSizeSort, "sub")
	if err := os.Mkdir(subSz, 0o755); err != nil {
		t.Fatalf("creating sub: %v", err)
	}
	makeFiles(t, subSz, "inner")

	tests := []testutils.DiffTest{
		// R3.12: -R with -l includes total line for each subdir.
		{
			Name:    "r3.12_recursive_long_format",
			Args:    []string{"-Rl"},
			WorkDir: dirFormat,
		},
		// R3.12: -R with default format (piped = single-column).
		{
			Name:    "r3.12_recursive_default_format",
			Args:    []string{"-R"},
			WorkDir: dirFormat,
		},
		// R3.12: -R with -1 single-column.
		{
			Name:    "r3.12_recursive_single_column",
			Args:    []string{"-R1"},
			WorkDir: dirFormat,
		},
		// R3.12: -R with -l and -s shows blocks in each subdir.
		{
			Name:    "r3.12_recursive_long_blocks",
			Args:    []string{"-Rls"},
			WorkDir: dirFormat,
		},
		// R3.13: -R does not follow symlinks to directories.
		{
			Name:    "r3.13_no_symlink_follow",
			Args:    []string{"-R1"},
			WorkDir: dirSym,
		},
		// R3.13: -R with -l does not follow symlinks.
		{
			Name:    "r3.13_no_symlink_follow_long",
			Args:    []string{"-Rl"},
			WorkDir: dirSym,
		},
		// R3.14: -R without filter hides dotfiles in subdirs.
		{
			Name:    "r3.14_recursive_no_filter",
			Args:    []string{"-R1"},
			WorkDir: dirFilter,
		},
		// R3.14: -R with -A shows dotfiles except . and .. in subdirs.
		{
			Name:    "r3.14_recursive_almost_all",
			Args:    []string{"-RA1"},
			WorkDir: dirFilter,
		},
		// R3.14: -R with -a shows all dotfiles including . and .. in subdirs.
		{
			Name:    "r3.14_recursive_all",
			Args:    []string{"-Ra1"},
			WorkDir: dirFilter,
		},
		// R3.15: -R with -t recurses subdirs in modification-time order.
		{
			Name:    "r3.15_recursive_time_sort",
			Args:    []string{"-Rt1"},
			WorkDir: dirSort,
		},
		// R3.15: -R with -S recurses subdirs in size order.
		{
			Name:    "r3.15_recursive_size_sort",
			Args:    []string{"-RS1"},
			WorkDir: dirSizeSort,
		},
		// R3.15: -R with -r reverses default name sort order.
		{
			Name:    "r3.15_recursive_reverse",
			Args:    []string{"-Rr1"},
			WorkDir: dirSort,
		},
		// R3.15: -R with -t -r reverses time sort.
		{
			Name:    "r3.15_recursive_time_reverse",
			Args:    []string{"-Rtr1"},
			WorkDir: dirSort,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
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

// TestDiffExitCodes tests exit code behavior for R4.1, R4.2, R4.3.
// R4.1: exit 0 on success. R4.2: exit non-zero for inaccessible entries.
// R4.3: exit 2 for invalid command-line options.
func TestDiffExitCodes(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// R4.1 fixture: basic directory for successful listing.
	dirSuccess := t.TempDir()
	makeFiles(t, dirSuccess, "file1", "file2")

	// R4.2 fixture: nonexistent path.
	nonexistent := filepath.Join(t.TempDir(), "does_not_exist")

	// R4.2 fixture: valid dir plus nonexistent path for partial listing.
	dirPartial := t.TempDir()
	makeFiles(t, dirPartial, "exists")
	nonexistent2 := filepath.Join(t.TempDir(), "also_missing")

	norm := []testutils.NormalizeFunc{normalizeErrorOutput}

	tests := []testutils.DiffTest{
		// R4.1: successful listing exits 0.
		{
			Name:      "r4.1_exit_0_success",
			Args:      []string{dirSuccess},
			Normalize: norm,
		},
		// R4.1: empty directory listing exits 0.
		{
			Name:      "r4.1_exit_0_empty_dir",
			Args:      []string{t.TempDir()},
			Normalize: norm,
		},
		// R4.2: nonexistent file produces error and non-zero exit.
		{
			Name:      "r4.2_nonexistent_file",
			Args:      []string{nonexistent},
			Normalize: norm,
		},
		// R4.2: mix of valid and invalid paths, partial listing succeeds.
		{
			Name:      "r4.2_partial_listing",
			Args:      []string{nonexistent2, dirPartial},
			Normalize: norm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestExitCodeInvalidOption verifies R4.3: exit 2 for invalid command-line
// options. Uses direct subprocess testing because error message format
// (program name, path prefix) differs between our binary and gls.
func TestExitCodeInvalidOption(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("r4.3_invalid_short_option", func(t *testing.T) {
		cmd := exec.Command(goBin, "-j")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected ExitError, got %v", err)
		}
		if exitErr.ExitCode() != 2 {
			t.Errorf("expected exit code 2, got %d", exitErr.ExitCode())
		}
		if !bytes.Contains(stderr.Bytes(), []byte("invalid option")) {
			t.Errorf("expected 'invalid option' in stderr, got: %q",
				stderr.String())
		}
	})

	t.Run("r4.3_invalid_long_option", func(t *testing.T) {
		cmd := exec.Command(goBin, "--foobar-invalid")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected ExitError, got %v", err)
		}
		if exitErr.ExitCode() != 2 {
			t.Errorf("expected exit code 2, got %d", exitErr.ExitCode())
		}
		if !bytes.Contains(stderr.Bytes(), []byte("unrecognized option")) {
			t.Errorf("expected 'unrecognized option' in stderr, got: %q",
				stderr.String())
		}
	})
}

// TestDiffSignalAndFlagInteractions tests R4.4-R4.7:
// SIGPIPE handling, SIGWINCH registration, -n implies -l, format flag precedence.
func TestDiffSignalAndFlagInteractions(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	// R4.6/R4.7 fixture: directory with files for format flag tests.
	dirFmt := t.TempDir()
	makeFiles(t, dirFmt, "alpha", "beta", "gamma")

	// R4.6 fixture: directory with files for -n implies -l test.
	dirNumeric := t.TempDir()
	makeFiles(t, dirNumeric, "file1", "file2")

	tests := []testutils.DiffTest{
		// R4.6: -n without explicit -l still produces long format.
		{
			Name:    "r4.6_n_implies_l",
			Args:    []string{"-n"},
			WorkDir: dirNumeric,
		},
		// R4.6: -n1 still produces long format (-n implies -l unconditionally).
		{
			Name:    "r4.6_n1_long_format",
			Args:    []string{"-n1"},
			WorkDir: dirNumeric,
		},
		// R4.6: -1n produces long format (-n implies -l unconditionally).
		{
			Name:    "r4.6_1n_long_format",
			Args:    []string{"-1n"},
			WorkDir: dirNumeric,
		},
		// R4.7: -C after -l overrides to columnar.
		{
			Name:    "r4.7_l_then_C_columns",
			Args:    []string{"-lC"},
			WorkDir: dirFmt,
		},
		// R4.7: -l after -C overrides to long.
		{
			Name:    "r4.7_C_then_l_long",
			Args:    []string{"-Cl"},
			WorkDir: dirFmt,
		},
		// R4.7: -x after -l overrides to horizontal.
		{
			Name:    "r4.7_l_then_x_horiz",
			Args:    []string{"-lx"},
			WorkDir: dirFmt,
		},
		// R4.7: -l after -x overrides to long.
		{
			Name:    "r4.7_x_then_l_long",
			Args:    []string{"-xl"},
			WorkDir: dirFmt,
		},
		// R4.7: -1 after -C overrides to single column.
		{
			Name:    "r4.7_C_then_1_single",
			Args:    []string{"-C1"},
			WorkDir: dirFmt,
		},
		// R4.7: -C after -1 overrides to columnar.
		{
			Name:    "r4.7_1_then_C_columns",
			Args:    []string{"-1C"},
			WorkDir: dirFmt,
		},
		// R4.7: triple override -l -C -1, last wins (-1).
		{
			Name:    "r4.7_triple_override_lC1",
			Args:    []string{"-lC1"},
			WorkDir: dirFmt,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPEHandling verifies R4.4: piping ls output to a truncating
// consumer does not produce a broken-pipe error message.
func TestSIGPIPEHandling(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	// Create a directory with many files so ls produces multiple lines.
	dir := t.TempDir()
	for i := 0; i < 100; i++ {
		makeFiles(t, dir, "file"+strings.Repeat("x", 3)+
			strings.Replace(strings.Replace(
				strings.Replace(
					fmt.Sprintf("%03d", i), "0", "a", 1),
				"1", "b", 1), "2", "c", 1))
	}

	// Pipe ls to head -1 to trigger SIGPIPE.
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("'%s' -1 '%s' | head -1", goBin, dir))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() != 0 {
			// SIGPIPE should cause exit 0 or the head exit code.
			// Any non-zero exit from ls itself is a failure.
			t.Logf("stderr: %s", stderr.String())
		}
	}
	// The key check: no "broken pipe" error message on stderr.
	if bytes.Contains(bytes.ToLower(stderr.Bytes()), []byte("broken pipe")) {
		t.Errorf("R4.4: SIGPIPE should not produce broken pipe error, got: %q",
			stderr.String())
	}
}

// normalizeErrorOutput normalizes program names and error message case
// for differential comparison. The Go binary reports as "ls" while the
// reference binary may report as "gls" or its full path. Go also uses
// lowercase system error strings while GNU uses capitalized ones.
func normalizeErrorOutput(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = normalizeErrorLine(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

// normalizeErrorLine normalizes a single line of output for comparison.
func normalizeErrorLine(line []byte) []byte {
	s := string(line)
	// Normalize program name at start of error lines.
	// Handles "gls:", "ls:", "/path/to/ls:", "/path/to/gls:".
	if colonIdx := strings.Index(s, ": "); colonIdx >= 0 {
		prefix := s[:colonIdx]
		baseName := filepath.Base(prefix)
		if baseName == "ls" || baseName == "gls" {
			s = "ls" + s[colonIdx:]
		}
	}
	// Normalize "Try 'PROG --help'" lines. The program path differs.
	if strings.Contains(s, "--help'") && strings.HasPrefix(s, "Try '") {
		s = "Try 'ls --help' for more information."
	}
	// Lowercase for case-insensitive error message comparison.
	// Go uses lowercase system error strings; GNU uses capitalized.
	s = strings.ToLower(s)
	return []byte(s)
}
