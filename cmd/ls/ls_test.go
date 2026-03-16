// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ls against gls (GNU coreutils).
// Implements prd008-ls R1.1-R1.8 test coverage.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

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
