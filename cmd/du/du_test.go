// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests: prd009-du R1.1–R1.5, R2.1–R2.8, R3.1–R3.3 via differential testing against gdu (Homebrew GNU du).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

	// Create fixture directory structure.
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	nested := filepath.Join(subdir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFixture(t, dir, "file1.txt", "hello world content\n")
	writeFixture(t, subdir, "file2.txt", "subdir file content here\n")
	writeFixture(t, nested, "file3.txt", "nested file content\n")

	// Create an empty directory.
	emptyDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	// R1.4: create a symlink to verify it is not followed during traversal.
	if err := os.Symlink(subdir, filepath.Join(dir, "link_to_subdir")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	// Create a large file for human-readable size testing.
	largeDir := filepath.Join(dir, "largedir")
	if err := os.Mkdir(largeDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	writeFixture(t, largeDir, "big.bin", strings.Repeat("X", 100*1024))

	// R3.1–R3.3: create hard-link fixture for deduplication testing.
	hardlinkDir := filepath.Join(dir, "hardlinks")
	hlSubA := filepath.Join(hardlinkDir, "a")
	hlSubB := filepath.Join(hardlinkDir, "b")
	if err := os.MkdirAll(hlSubA, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(hlSubB, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Create original file and hard-link it into sibling directory.
	origFile := filepath.Join(hlSubA, "original.txt")
	writeFixture(t, hlSubA, "original.txt", strings.Repeat("D", 4096))
	if err := os.Link(origFile, filepath.Join(hlSubB, "hardlink.txt")); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// R3.3: create a second fixture for cross-argument hard-link deduplication.
	crossArgDir1 := filepath.Join(dir, "cross1")
	crossArgDir2 := filepath.Join(dir, "cross2")
	if err := os.Mkdir(crossArgDir1, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Mkdir(crossArgDir2, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	crossOrigFile := filepath.Join(crossArgDir1, "shared.dat")
	writeFixture(t, crossArgDir1, "shared.dat", strings.Repeat("Z", 8192))
	if err := os.Link(crossOrigFile, filepath.Join(crossArgDir2, "shared.dat")); err != nil {
		t.Fatalf("Link: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: recursive directory traversal including nested dirs and symlink.
		{
			Name: "du_recursive_traversal",
			Args: []string{dir},
		},
		// R1.1: no arguments defaults to current directory.
		{
			Name:    "du_no_args",
			WorkDir: dir,
		},
		// R1.2: single file argument prints only that file's block count.
		{
			Name: "du_single_file",
			Args: []string{filepath.Join(dir, "file1.txt")},
		},
		// R1.2, R1.5: multiple arguments processed in order.
		{
			Name: "du_multiple_args",
			Args: []string{
				filepath.Join(dir, "file1.txt"),
				subdir,
			},
		},
		// R1.1: empty directory reports only its own blocks.
		{
			Name: "du_empty_dir",
			Args: []string{emptyDir},
		},
		// R4.2: nonexistent path exits 1 with diagnostic to stderr.
		{
			Name:      "du_nonexistent_path",
			Args:      []string{filepath.Join(dir, "nonexistent")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeTryLine},
		},
		// R1.5: multiple directory arguments.
		{
			Name: "du_multiple_dirs",
			Args: []string{subdir, emptyDir},
		},

		// R2.1: human-readable output.
		{
			Name: "du_human_readable",
			Args: []string{"-h", dir},
		},
		// R2.1: -h on a single file.
		{
			Name: "du_human_readable_file",
			Args: []string{"-h", filepath.Join(dir, "file1.txt")},
		},
		// R2.1: -h on directory with a larger file.
		{
			Name: "du_human_readable_large",
			Args: []string{"-h", largeDir},
		},

		// R2.2: summary mode.
		{
			Name: "du_summary",
			Args: []string{"-s", dir},
		},
		// R2.2: summary with multiple args.
		{
			Name: "du_summary_multiple",
			Args: []string{"-s", subdir, emptyDir},
		},
		// R2.2: summary on a single file.
		{
			Name: "du_summary_file",
			Args: []string{"-s", filepath.Join(dir, "file1.txt")},
		},
		// R2.2: -s with -h combined.
		{
			Name: "du_summary_human",
			Args: []string{"-sh", dir},
		},

		// R2.3: all-files mode.
		{
			Name: "du_all_files",
			Args: []string{"-a", dir},
		},
		// R2.3: -a on empty directory.
		{
			Name: "du_all_files_empty",
			Args: []string{"-a", emptyDir},
		},
		// R2.3: -a with -h combined.
		{
			Name: "du_all_files_human",
			Args: []string{"-ah", dir},
		},
		// R2.3: -a with -s is an error (GNU du rejects this combination).
		{
			Name:      "du_all_files_summary",
			Args:      []string{"-as", dir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeTryLine},
		},

		// R1.5: multiple directory arguments with -h.
		{
			Name: "du_multiple_dirs_human",
			Args: []string{"-h", subdir, emptyDir},
		},
		// R1.5: multiple directory arguments with -a.
		{
			Name: "du_multiple_dirs_all",
			Args: []string{"-a", subdir, emptyDir},
		},

		// R2.4: depth limiting with -d.
		{
			Name: "du_max_depth_0",
			Args: []string{"-d", "0", dir},
		},
		{
			Name: "du_max_depth_1",
			Args: []string{"-d", "1", dir},
		},
		{
			Name: "du_max_depth_2",
			Args: []string{"-d", "2", dir},
		},
		// R2.4: --max-depth=N long form.
		{
			Name: "du_max_depth_long",
			Args: []string{"--max-depth=1", dir},
		},
		// R2.4: -d 0 is equivalent to -s.
		{
			Name: "du_max_depth_0_vs_summary",
			Args: []string{"-d", "0", dir},
		},
		// R2.4: -d with -a shows files within depth.
		{
			Name: "du_max_depth_with_all",
			Args: []string{"-d", "1", "-a", dir},
		},
		// R2.4: -d with -h combined.
		{
			Name: "du_max_depth_human",
			Args: []string{"-d", "1", "-h", dir},
		},

		// R2.5: -k accepted without error (default is already 1K blocks).
		{
			Name: "du_k_flag",
			Args: []string{"-k", dir},
		},
		// R2.5: -k combined with other flags.
		{
			Name: "du_k_with_summary",
			Args: []string{"-ks", dir},
		},

		// R2.6: -m reports in 1M blocks.
		{
			Name: "du_m_flag",
			Args: []string{"-m", dir},
		},
		// R2.6: -m on a single file.
		{
			Name: "du_m_file",
			Args: []string{"-m", filepath.Join(dir, "file1.txt")},
		},
		// R2.6: -m with summary.
		{
			Name: "du_m_summary",
			Args: []string{"-ms", dir},
		},

		// R2.7: grand total with -c.
		{
			Name: "du_grand_total",
			Args: []string{"-c", dir},
		},
		// R2.7: -c with multiple arguments.
		{
			Name: "du_grand_total_multiple",
			Args: []string{"-c", subdir, emptyDir},
		},
		// R2.7: -c with -s.
		{
			Name: "du_grand_total_summary",
			Args: []string{"-cs", subdir, emptyDir},
		},
		// R2.7: -c with -h.
		{
			Name: "du_grand_total_human",
			Args: []string{"-ch", dir},
		},
		// R2.7: -c with -d.
		{
			Name: "du_grand_total_depth",
			Args: []string{"-c", "-d", "1", dir},
		},

		// R2.8: --apparent-size reports st_size instead of st_blocks.
		{
			Name: "du_apparent_size",
			Args: []string{"--apparent-size", dir},
		},
		// R2.8: --apparent-size on a single file.
		{
			Name: "du_apparent_size_file",
			Args: []string{"--apparent-size", filepath.Join(dir, "file1.txt")},
		},
		// R2.8: --apparent-size with -h. Uses largeDir (exact 100K file) to
		// avoid rounding divergence between format.HumanSize (math.Round) and
		// GNU human_readable (ceiling) on fractional totals.
		{
			Name: "du_apparent_size_human",
			Args: []string{"--apparent-size", "-h", largeDir},
		},
		// R2.8: --apparent-size with -s.
		{
			Name: "du_apparent_size_summary",
			Args: []string{"--apparent-size", "-s", dir},
		},
		// R2.8: --apparent-size with -c.
		{
			Name: "du_apparent_size_grand_total",
			Args: []string{"--apparent-size", "-c", subdir, emptyDir},
		},
		// R2.8: --apparent-size with -a to see individual file sizes.
		{
			Name: "du_apparent_size_all",
			Args: []string{"--apparent-size", "-a", subdir},
		},

		// R3.1, R3.2: hard-link deduplication within a single directory tree.
		{
			Name: "du_hardlink_dedup",
			Args: []string{hardlinkDir},
		},
		// R3.1: hard-link dedup with -a shows the file counted once.
		{
			Name: "du_hardlink_dedup_all",
			Args: []string{"-a", hardlinkDir},
		},
		// R3.1: hard-link dedup with -c.
		{
			Name: "du_hardlink_dedup_total",
			Args: []string{"-c", hardlinkDir},
		},
		// R3.3: cross-argument hard-link deduplication.
		{
			Name: "du_hardlink_cross_arg",
			Args: []string{crossArgDir1, crossArgDir2},
		},
		// R3.3: cross-argument hard-link dedup with -c shows correct total.
		{
			Name: "du_hardlink_cross_arg_total",
			Args: []string{"-c", crossArgDir1, crossArgDir2},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeBinaryName replaces "gdu:" with "du:" in output so stderr from
// the reference binary matches our binary's error prefix.
func normalizeBinaryName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gdu:"), []byte("du:"))
}

// normalizeTryLine normalizes the "Try '...' for more information" line
// so the reference binary path matches our binary name.
func normalizeTryLine(data []byte) []byte {
	var result []byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("Try '")) {
			result = append(result, []byte("Try 'du --help' for more information.")...)
			result = append(result, '\n')
		} else {
			result = append(result, line...)
			result = append(result, '\n')
		}
	}
	// bytes.Split adds an extra trailing newline; trim to match original.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		result = result[:len(result)-1]
	} else if len(result) > 1 {
		result = result[:len(result)-1] // remove extra newline from Split
	}
	return result
}

// writeFixture creates a file in dir with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFixture %s: %v", name, err)
	}
}
