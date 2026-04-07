// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgramNameNormalizer replaces "gdu:" with "du:" in stderr
// so the differential test ignores the binary name difference.
var stderrProgramNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gdu:"), []byte("du:"))
}

// TestDiff runs differential tests comparing the Go du binary against
// the GNU reference binary (gdu) for core traversal behavior.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name:    "no_args_default_dir",
			Args:    []string{"-k"},
			WorkDir: basic,
		},
		{
			Name: "explicit_subdir",
			Args: []string{"-k", filepath.Join(basic, "sub1")},
		},
		{
			Name: "multiple_args",
			Args: []string{"-k",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
		},
		{
			Name: "file_argument",
			Args: []string{"-k",
				filepath.Join(basic, "sub1", "file1.txt")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHumanReadable verifies -h flag produces human-readable output
// matching gdu -h. R2.1: uses binary units (1024-based).
func TestDiffHumanReadable(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name:    "human_readable_dir",
			Args:    []string{"-h"},
			WorkDir: basic,
		},
		{
			Name: "human_readable_subdir",
			Args: []string{"-h", filepath.Join(basic, "sub1")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSummary verifies -s flag prints only the total per argument.
// R2.2: suppresses per-subdirectory output.
func TestDiffSummary(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name:    "summary_default_dir",
			Args:    []string{"-s"},
			WorkDir: basic,
		},
		{
			Name: "summary_explicit_dir",
			Args: []string{"-s", filepath.Join(basic, "sub1")},
		},
		{
			Name: "summary_multiple_args",
			Args: []string{"-s",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
		},
		{
			Name: "summary_human_readable",
			Args: []string{"-sh",
				filepath.Join(basic, "sub1")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffAllFiles verifies -a flag includes all files in output.
// R2.3: writes counts for every file, not just directories.
func TestDiffAllFiles(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name:    "all_files_default_dir",
			Args:    []string{"-a"},
			WorkDir: basic,
		},
		{
			Name: "all_files_subdir",
			Args: []string{"-a", filepath.Join(basic, "sub1")},
		},
		{
			Name: "all_files_human_readable",
			Args: []string{"-ah", filepath.Join(basic, "sub1")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorPaths verifies exit code and error handling.
// R1.5: exits 1 when a path does not exist, continues processing.
// R4.2: prints diagnostic to stderr.
func TestDiffErrorPaths(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_path",
			Args:      []string{"/nonexistent/path/xyz_du_test"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgramNameNormalizer},
		},
		{
			Name: "mixed_valid_and_invalid",
			Args: []string{
				filepath.Join(basic, "sub1"),
				"/nonexistent/path/xyz_du_test",
			},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgramNameNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHardLinks verifies that hard-linked files are counted only
// once per traversal. R3.1: deduplication by dev+ino.
func TestDiffHardLinks(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	dir := t.TempDir()
	orig := filepath.Join(dir, "original.txt")
	if err := os.WriteFile(orig, []byte("hard link test content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "hardlink.txt")
	if err := os.Link(orig, link); err != nil {
		t.Skipf("hard links not supported: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "hard_links_counted_once",
			Args:    []string{"-k"},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// createBasicFixture creates a test directory with subdirectories and files.
func createBasicFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sub1 := filepath.Join(dir, "sub1")
	sub2 := filepath.Join(dir, "sub2")
	if err := os.MkdirAll(sub1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub2, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(sub1, "file1.txt"), "hello world\n")
	writeFixtureFile(t, filepath.Join(sub2, "file2.txt"), "foo bar baz\n")
	return dir
}

// writeFixtureFile writes a test file with the given content.
func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
