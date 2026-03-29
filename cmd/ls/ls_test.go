// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls differential tests for prd008 R2.7-R2.14.
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

// setMtime sets the modification time of a file.
func setMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}
