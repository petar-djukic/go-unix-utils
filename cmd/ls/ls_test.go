// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd008-ls R1.1–R1.14, R2.1–R2.6: default listing,
// single-column output, C locale sorting, dotfile filtering, horizontal
// multi-column output, last-format-flag-wins, -a/-A show all, and long format
// owner, group, size, and date field rendering.
package main_test

import (
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
