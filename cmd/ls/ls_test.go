// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls differential tests for prd008 R2.7-R2.15, R3.1-R3.7.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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
