// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	fixture := createFixture(t)
	hlFixture := createHardLinkFixture(t)
	deepFixture := createDeepFixture(t)

	tests := []testutils.DiffTest{
		{
			Name:      "default_traversal",
			Args:      []string{fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "all_files_short",
			Args:      []string{"-a", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "all_files_long",
			Args:      []string{"--all", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "hard_link_dedup",
			Args:      []string{"-a", hlFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "multiple_args",
			Args:      []string{fixture, hlFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name: "file_argument",
			Args: []string{filepath.Join(fixture, "root.txt")},
		},
		{
			Name:      "current_dir_default",
			Args:      []string{},
			WorkDir:   fixture,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.1: -h/--human-readable with binary (1024-based) suffixes.
		{
			Name:      "human_readable_short",
			Args:      []string{"-h", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "human_readable_long",
			Args:      []string{"--human-readable", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "human_readable_all",
			Args:      []string{"-h", "-a", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.5: -k is equivalent to default (1024-byte blocks).
		{
			Name:      "kilo_blocks",
			Args:      []string{"-k", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "kilo_blocks_all",
			Args:      []string{"-k", "-a", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.6: -m reports sizes in 1048576-byte blocks.
		{
			Name:      "mega_blocks",
			Args:      []string{"-m", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "mega_blocks_all",
			Args:      []string{"-m", "-a", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.2: -s/--summarize displays only total per argument.
		{
			Name: "summarize_short",
			Args: []string{"-s", fixture},
		},
		{
			Name: "summarize_long",
			Args: []string{"--summarize", fixture},
		},
		{
			Name:      "summarize_multiple_args",
			Args:      []string{"-s", fixture, hlFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name: "summarize_human",
			Args: []string{"-s", "-h", fixture},
		},
		// R2.7: -c/--total prints grand total line.
		{
			Name:      "total_short",
			Args:      []string{"-c", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "total_long",
			Args:      []string{"--total", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "total_multiple_args",
			Args:      []string{"-c", fixture, hlFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name: "summarize_total",
			Args: []string{"-s", "-c", fixture, hlFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// R2.4: -d/--max-depth limits output depth.
		{
			Name: "max_depth_0",
			Args: []string{"-d", "0", deepFixture},
		},
		{
			Name:      "max_depth_1",
			Args:      []string{"--max-depth=1", deepFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "max_depth_2",
			Args:      []string{"-d", "2", deepFixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name: "max_depth_combined_d0",
			Args: []string{"-d0", fixture},
		},
		{
			Name:      "max_depth_all",
			Args:      []string{"-d", "1", "-a", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		// -x/--one-file-system on a normal tree (no mount points).
		{
			Name:      "one_file_system",
			Args:      []string{"-x", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "one_file_system_long",
			Args:      []string{"--one-file-system", fixture},
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// sortLines sorts output lines for order-independent comparison.
// GNU du and Go os.ReadDir may traverse entries in different order.
func sortLines(b []byte) []byte {
	s := string(b)
	if s == "" {
		return b
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// createFixture builds a directory tree for du testing.
func createFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	sub1 := filepath.Join(dir, "sub1")
	sub2 := filepath.Join(dir, "sub2")
	mustMkdir(t, sub1)
	mustMkdir(t, sub2)

	writeTestFile(t, filepath.Join(sub1, "file1.txt"), 4096)
	writeTestFile(t, filepath.Join(sub2, "file2.txt"), 8192)
	writeTestFile(t, filepath.Join(dir, "root.txt"), 1024)

	return dir
}

// createHardLinkFixture builds a directory with hard-linked files.
// R3.1: same file linked twice should be counted only once.
func createHardLinkFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	sub1 := filepath.Join(dir, "a")
	sub2 := filepath.Join(dir, "b")
	mustMkdir(t, sub1)
	mustMkdir(t, sub2)

	original := filepath.Join(sub1, "original.txt")
	writeTestFile(t, original, 4096)

	link := filepath.Join(sub2, "hardlink.txt")
	if err := os.Link(original, link); err != nil {
		t.Fatalf("cannot create hard link: %v", err)
	}

	return dir
}

// createDeepFixture builds a deeper directory tree for max-depth testing.
func createDeepFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	sub1 := filepath.Join(dir, "sub1")
	deep := filepath.Join(sub1, "deep")
	sub2 := filepath.Join(dir, "sub2")
	mustMkdir(t, sub1)
	mustMkdir(t, deep)
	mustMkdir(t, sub2)

	writeTestFile(t, filepath.Join(dir, "root.txt"), 1024)
	writeTestFile(t, filepath.Join(sub1, "file1.txt"), 4096)
	writeTestFile(t, filepath.Join(deep, "deep.txt"), 2048)
	writeTestFile(t, filepath.Join(sub2, "file2.txt"), 8192)

	return dir
}

// mustMkdir creates a directory or fails the test.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// writeTestFile creates a file with the given number of zero bytes.
func writeTestFile(t *testing.T, path string, size int) {
	t.Helper()
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
