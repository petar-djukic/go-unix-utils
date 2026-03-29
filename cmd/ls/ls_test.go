// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls differential tests for prd008 R2.7-R2.15, R3.1-R3.15, R4.1-R4.8.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeLsName normalizes the binary name in error messages so
// gls and ls outputs can be compared. Handles both bare names and
// full paths like /opt/homebrew/bin/gls.
func normalizeLsName(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = normalizeLsLine(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

// normalizeLsLine normalizes a single line of output.
func normalizeLsLine(line []byte) []byte {
	// Normalize error prefix: "PROGPATH: msg" → "ls: msg"
	if colonIdx := bytes.Index(line, []byte(": ")); colonIdx >= 0 {
		prog := filepath.Base(string(line[:colonIdx]))
		if prog == "ls" || prog == "gls" {
			return append([]byte("ls"), line[colonIdx:]...)
		}
	}
	// Normalize Try line: "Try 'PROGPATH --help'..." → "Try 'ls --help'..."
	if bytes.HasPrefix(line, []byte("Try '")) {
		if spIdx := bytes.Index(line[5:], []byte(" ")); spIdx >= 0 {
			prog := filepath.Base(string(line[5 : 5+spIdx]))
			if prog == "ls" || prog == "gls" {
				return append([]byte("Try 'ls"), line[5+spIdx:]...)
			}
		}
	}
	return line
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skipf("reference binary gls not in PATH: %v", err)
	}

	sortDir := setupSortDir(t)
	versionDir := setupVersionDir(t)
	singleDir := setupSingleFileDir(t)
	humanDir := setupHumanDir(t)
	classifyDir := setupClassifyDir(t)
	recursiveDir := setupRecursiveDir(t)
	symlinkRecDir := setupSymlinkRecursiveDir(t)
	dotRecDir := setupDotRecursiveDir(t)
	timeRecDir := setupTimeRecursiveDir(t)

	tests := []testutils.DiffTest{
		// R2.7: -r reverses default (name) sort
		{
			Name: "R2.7_reverse_default",
			Args: []string{"-1", "-r", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7: -r with -S reverses size sort
		{
			Name: "R2.7_reverse_size",
			Args: []string{"-1", "-S", "-r", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.7: -r with -t reverses time sort
		{
			Name: "R2.7_reverse_time",
			Args: []string{"-1", "-t", "-r", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.8: -U accepted without error (single file avoids order divergence)
		{
			Name: "R2.8_unsorted_single",
			Args: []string{"-1", "-U", singleDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.8: -U with -r accepted without error
		{
			Name: "R2.8_unsorted_reverse",
			Args: []string{"-1", "-U", "-r", singleDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.9: -v version sort (file2 before file10)
		{
			Name: "R2.9_version_sort",
			Args: []string{"-1", "-v", versionDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.9: -v with -r reverses version sort
		{
			Name: "R2.9_version_reverse",
			Args: []string{"-1", "-v", "-r", versionDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: last sort flag wins — -t then -S produces size sort
		{
			Name: "R2.10_tS_size_wins",
			Args: []string{"-1", "-t", "-S", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: last sort flag wins — -S then -t produces time sort
		{
			Name: "R2.10_St_time_wins",
			Args: []string{"-1", "-S", "-t", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.10: last sort flag wins — -v then -t produces time sort
		{
			Name: "R2.10_vt_time_wins",
			Args: []string{"-1", "-v", "-t", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.11: -i shows inode numbers in single-column mode
		{
			Name: "R2.11_inode_single",
			Args: []string{"-1", "-i", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.11: -i with -l shows inode before permissions
		{
			Name: "R2.11_inode_long",
			Args: []string{"-l", "-i", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.12: -s shows block counts in single-column mode
		{
			Name: "R2.12_blocks_single",
			Args: []string{"-1", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.12: -s with -l shows block counts before permissions
		{
			Name: "R2.12_blocks_long",
			Args: []string{"-l", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.13: -s with -l shows total line
		{
			Name: "R2.13_blocks_total_long",
			Args: []string{"-l", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.13: -s without -l also shows total line
		{
			Name: "R2.13_blocks_total_single",
			Args: []string{"-1", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.14: -n shows numeric UID/GID in long format
		{
			Name: "R2.14_numeric_ids",
			Args: []string{"-n", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.14: -n implies -l
		{
			Name: "R2.14_numeric_implies_long",
			Args: []string{"-n", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.11+R2.12: -i and -s combined, inode first then blocks
		{
			Name: "R2.11_R2.12_inode_blocks_combined",
			Args: []string{"-1", "-i", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.11+R2.12+R2.14: -i -s -n combined in long format
		{
			Name: "R2.11_R2.12_R2.14_combined_long",
			Args: []string{"-n", "-i", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.15: -i and -s combined in single-column (inode first, then blocks)
		{
			Name: "R2.15_inode_blocks_combined_single",
			Args: []string{"-1", "-i", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.15: -i and -s combined in long format
		{
			Name: "R2.15_inode_blocks_combined_long",
			Args: []string{"-l", "-i", "-s", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: --color=always forces color even in a pipe
		{
			Name: "R3.1_color_always",
			Args: []string{"-1", "--color=always", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: --color=never suppresses color
		{
			Name: "R3.1_color_never",
			Args: []string{"-1", "--color=never", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: --color without value defaults to always
		{
			Name: "R3.1_color_bare",
			Args: []string{"-1", "--color", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: --color=auto in pipe produces no ANSI sequences
		{
			Name: "R3.2_color_auto_pipe",
			Args: []string{"-1", "--color=auto", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: --color=always with -l includes color in long format
		{
			Name: "R3.3_color_always_long",
			Args: []string{"-l", "--color=always", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --color=never with -l produces no ANSI sequences
		{
			Name: "R3.4_color_never_long",
			Args: []string{"-l", "--color=never", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: --color=auto in pipe produces no ANSI sequences
		{
			Name: "R3.4_color_auto_pipe_long",
			Args: []string{"-l", "--color=auto", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.5: -h with -l shows human-readable sizes
		{
			Name: "R3.5_human_long",
			Args: []string{"-l", "-h", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.5: -h without -l has no visible effect
		{
			Name: "R3.5_human_no_long",
			Args: []string{"-1", "-h", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.5+R3.6: -h -l with large files shows human-readable
		{
			Name: "R3.5_R3.6_human_long_large",
			Args: []string{"-l", "-h", humanDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.6: -h applies to total line in long format
		{
			Name: "R3.6_human_total",
			Args: []string{"-l", "-h", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.7: -h with -s shows human-readable block counts
		{
			Name: "R3.7_human_blocks_single",
			Args: []string{"-1", "-s", "-h", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.7: -h with -s and -l shows human-readable blocks in long format
		{
			Name: "R3.7_human_blocks_long",
			Args: []string{"-l", "-s", "-h", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.7: -h with -s and -l for larger files
		{
			Name: "R3.7_human_blocks_long_large",
			Args: []string{"-l", "-s", "-h", humanDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.8: -F appends type indicators in single-column mode
		{
			Name: "R3.8_classify_single",
			Args: []string{"-1", "-F", classifyDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.8+R3.10: -F with -l in long format
		{
			Name: "R3.8_R3.10_classify_long",
			Args: []string{"-l", "-F", classifyDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.10: -F with --color=never
		{
			Name: "R3.10_classify_no_color",
			Args: []string{"-1", "-F", "--color=never", classifyDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.11: -R recursive listing in single-column mode
		{
			Name: "R3.11_recursive_single",
			Args: []string{"-1", "-R", recursiveDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.11: -R recursive listing in long format
		{
			Name: "R3.11_recursive_long",
			Args: []string{"-l", "-R", recursiveDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.11+R3.8: -R with -F shows indicators in recursive listing
		{
			Name: "R3.11_R3.8_recursive_classify",
			Args: []string{"-1", "-R", "-F", recursiveDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.12: -R with -l produces long format in each subdirectory
		{
			Name: "R3.12_recursive_long_format",
			Args: []string{"-l", "-R", recursiveDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.12: -R with default (non-TTY) produces single-column
		{
			Name: "R3.12_recursive_default",
			Args: []string{"-R", recursiveDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.13: -R does not follow symlinks to directories
		{
			Name: "R3.13_recursive_no_symlink_follow",
			Args: []string{"-1", "-R", symlinkRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.13: -R with -l does not follow symlinks to dirs
		{
			Name: "R3.13_recursive_no_symlink_long",
			Args: []string{"-l", "-R", symlinkRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -R without -a hides dotfiles in subdirectories
		{
			Name: "R3.14_recursive_no_dotfiles",
			Args: []string{"-1", "-R", dotRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -R with -a shows dotfiles in subdirectories
		{
			Name: "R3.14_recursive_with_a",
			Args: []string{"-1", "-R", "-a", dotRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.14: -R with -A shows dotfiles except . and ..
		{
			Name: "R3.14_recursive_with_A",
			Args: []string{"-1", "-R", "-A", dotRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -R with -t visits subdirectories in time order
		{
			Name: "R3.15_recursive_time_sort",
			Args: []string{"-1", "-R", "-t", timeRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -R with -t -r visits subdirectories in reverse time order
		{
			Name: "R3.15_recursive_time_reverse",
			Args: []string{"-1", "-R", "-t", "-r", timeRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.15: -R with -S visits subdirectories in size order
		{
			Name: "R3.15_recursive_size_sort",
			Args: []string{"-1", "-R", "-S", timeRecDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: successful listing exits 0
		{
			Name: "R4.1_exit_success",
			Args: []string{"-1", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.2: nonexistent path produces stderr diagnostic
		{
			Name:      "R4.2_nonexistent_path",
			Args:      []string{"-1", "/nonexistent_path_xyzzy_99"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeLsName},
		},
		// R4.2: nonexistent + valid path still lists valid entries
		{
			Name:      "R4.2_partial_access",
			Args:      []string{"-1", "/nonexistent_path_xyzzy_99", sortDir},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeLsName},
		},
		// R4.3: invalid short option exits 2
		{
			Name:      "R4.3_invalid_short_option",
			Args:      []string{"-z"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeLsName},
		},
		// R4.3: invalid long option exits 2
		{
			Name:      "R4.3_invalid_long_option",
			Args:      []string{"--nonexistent-flag"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeLsName},
		},
		// R4.5: SIGWINCH handler is installed (ls produces valid output
		// with default format; the handler updates termWidth but cannot
		// be exercised differentially without a real TTY resize signal).
		{
			Name: "R4.5_sigwinch_basic_output",
			Args: []string{"-1", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.6: -n implies -l (long format with numeric UID/GID)
		{
			Name: "R4.6_n_implies_long",
			Args: []string{"-n", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.6: -n without explicit -l still produces long format
		{
			Name: "R4.6_n_alone_is_long",
			Args: []string{"-n", singleDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -l after -C overrides multi-column with long format
		{
			Name: "R4.7_C_then_l",
			Args: []string{"-C", "-l", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -C after -l overrides long format with multi-column
		{
			Name: "R4.7_l_then_C",
			Args: []string{"-l", "-C", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -1 after -C overrides multi-column with single-column
		{
			Name: "R4.7_C_then_1",
			Args: []string{"-C", "-1", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -x after -l overrides long format with across layout
		{
			Name: "R4.7_l_then_x",
			Args: []string{"-l", "-x", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -l after -x overrides across layout with long format
		{
			Name: "R4.7_x_then_l",
			Args: []string{"-x", "-l", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.7: -1 after -x overrides across with single-column
		{
			Name: "R4.7_x_then_1",
			Args: []string{"-x", "-1", sortDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.8: -R with -l produces "total N" block line per subdirectory
		{
			Name: "R4.8_recursive_long_total",
			Args: []string{"-l", "-R", recursiveDir},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.8: -R with -l on deeper nesting
		{
			Name: "R4.8_recursive_long_total_deep",
			Args: []string{"-l", "-R", timeRecDir},
			Env:  []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupSortDir creates a directory with files of different sizes and times.
func setupSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []struct {
		name string
		size int
		age  time.Duration
	}{
		{"aaa", 100, 3 * time.Hour},
		{"bbb", 300, 1 * time.Hour},
		{"ccc", 200, 2 * time.Hour},
	}
	for _, f := range files {
		writeSizedFile(t, filepath.Join(dir, f.name), f.size)
		mtime := time.Now().Add(-f.age)
		setMtime(t, filepath.Join(dir, f.name), mtime)
	}
	return dir
}

// setupVersionDir creates a directory with version-numbered files.
func setupVersionDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	names := []string{
		"file1", "file2", "file3", "file10", "file20",
	}
	for _, name := range names {
		writeSizedFile(t, filepath.Join(dir, name), 0)
	}
	return dir
}

// setupSingleFileDir creates a directory with one file for -U testing.
func setupSingleFileDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "only"), 0)
	return dir
}

// writeSizedFile creates a file with the given byte count.
func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// setupHumanDir creates a directory with files large enough for
// human-readable size formatting to produce suffixed output.
func setupHumanDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "small"), 512)
	writeSizedFile(t, filepath.Join(dir, "medium"), 10240)
	writeSizedFile(t, filepath.Join(dir, "large"), 1048576)
	return dir
}

// setMtime sets the modification time of a file.
func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// setupClassifyDir creates a directory with entries of different types
// for testing -F classification indicators.
// R3.8: directory, executable, symlink, FIFO, regular file.
func setupClassifyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Regular file (no indicator)
	writeSizedFile(t, filepath.Join(dir, "plain"), 0)
	// Directory (/)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	// Executable file (*)
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write run.sh: %v", err)
	}
	// Symlink (@)
	if err := os.Symlink("plain", filepath.Join(dir, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	// Named pipe / FIFO (|)
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	return dir
}

// setupRecursiveDir creates a directory with nested subdirectories
// for testing -R recursive listing.
// R3.11: nested structure with files at each level.
func setupRecursiveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "top.txt"), 10)
	sub := filepath.Join(dir, "alpha")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	writeSizedFile(t, filepath.Join(sub, "mid.txt"), 20)
	deep := filepath.Join(sub, "beta")
	if err := os.Mkdir(deep, 0o755); err != nil {
		t.Fatalf("mkdir beta: %v", err)
	}
	writeSizedFile(t, filepath.Join(deep, "deep.txt"), 30)
	return dir
}

// setupSymlinkRecursiveDir creates a directory with a real subdirectory
// and a symlink that points to a directory.
// R3.13: symlinks to directories must not be followed by -R.
func setupSymlinkRecursiveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "file.txt"), 10)
	realSub := filepath.Join(dir, "realdir")
	if err := os.Mkdir(realSub, 0o755); err != nil {
		t.Fatalf("mkdir realdir: %v", err)
	}
	writeSizedFile(t, filepath.Join(realSub, "inside.txt"), 20)
	// Symlink pointing to realdir — must not be recursed into
	if err := os.Symlink("realdir", filepath.Join(dir, "linkdir")); err != nil {
		t.Fatalf("symlink linkdir: %v", err)
	}
	return dir
}

// setupDotRecursiveDir creates a directory with dotfiles in a
// subdirectory for testing -R with filter flags.
// R3.14: dotfiles shown only when -a or -A is given.
func setupDotRecursiveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "visible"), 10)
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	writeSizedFile(t, filepath.Join(sub, "normal"), 20)
	writeSizedFile(t, filepath.Join(sub, ".hidden"), 30)
	return dir
}

// setupTimeRecursiveDir creates a directory with subdirectories that
// have different modification times and file sizes for testing
// -R with -t and -S sort ordering.
// R3.15: subdirectories recursed in current sort order.
func setupTimeRecursiveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSizedFile(t, filepath.Join(dir, "root.txt"), 10)
	// Create subdirs with different mtimes and sizes
	for _, s := range []struct {
		name string
		size int
		age  time.Duration
	}{
		{"zzz", 100, 1 * time.Hour},
		{"aaa", 300, 3 * time.Hour},
		{"mmm", 200, 2 * time.Hour},
	} {
		sub := filepath.Join(dir, s.name)
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", s.name, err)
		}
		writeSizedFile(t, filepath.Join(sub, "child.txt"), s.size)
		setMtime(t, sub, time.Now().Add(-s.age))
	}
	return dir
}
