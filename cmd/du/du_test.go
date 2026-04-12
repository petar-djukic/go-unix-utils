// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestDiffGrandTotal verifies -c flag produces a grand total line.
// R2.7: prints "SIZE\ttotal\n" after all arguments.
func TestDiffGrandTotal(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name: "grand_total_single_arg",
			Args: []string{"-c", filepath.Join(basic, "sub1")},
		},
		{
			Name: "grand_total_multiple_args",
			Args: []string{"-c",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
		},
		{
			Name: "grand_total_with_human",
			Args: []string{"-ch", filepath.Join(basic, "sub1")},
		},
		{
			Name: "grand_total_with_summary",
			Args: []string{"-cs",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMaxDepth verifies -d/--max-depth flag limits output depth.
// R2.4: only entries at depth <= N are printed.
func TestDiffMaxDepth(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	dir := createNestedFixture(t)

	tests := []testutils.DiffTest{
		{
			Name: "max_depth_0",
			Args: []string{"-d", "0", dir},
		},
		{
			Name: "max_depth_1",
			Args: []string{"-d", "1", dir},
		},
		{
			Name: "max_depth_2",
			Args: []string{"-d", "2", dir},
		},
		{
			Name: "max_depth_long_form",
			Args: []string{"--max-depth=1", dir},
		},
		{
			Name: "max_depth_with_all",
			Args: []string{"-d", "1", "-a", dir},
		},
		{
			Name: "max_depth_with_total",
			Args: []string{"-d", "1", "-c", dir},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBytes verifies -b flag shows apparent size in bytes.
func TestDiffBytes(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name: "bytes_dir",
			Args: []string{"-b", filepath.Join(basic, "sub1")},
		},
		{
			Name: "bytes_all_files",
			Args: []string{"-ba", filepath.Join(basic, "sub1")},
		},
		{
			Name: "bytes_with_total",
			Args: []string{"-bc",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMBlocks verifies -m flag shows sizes in 1M blocks.
// R2.6: each size is converted to 1048576-byte blocks, rounding up.
func TestDiffMBlocks(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)

	tests := []testutils.DiffTest{
		{
			Name: "mblocks_dir",
			Args: []string{"-m", filepath.Join(basic, "sub1")},
		},
		{
			Name: "mblocks_summary",
			Args: []string{"-ms", filepath.Join(basic, "sub1")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorPaths verifies exit code and error handling.
// R4.1: exit 0 on success. R4.2: exit 1 on error, diagnostic to stderr,
// continue processing remaining arguments.
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
			// R4.2: nonexistent path produces exit 1 and diagnostic.
			Name:      "nonexistent_path",
			Args:      []string{"/nonexistent/path/xyz_du_test"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgramNameNormalizer},
		},
		{
			// R4.2: continue processing after error; exit 1.
			Name: "mixed_valid_and_invalid",
			Args: []string{
				filepath.Join(basic, "sub1"),
				"/nonexistent/path/xyz_du_test",
			},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrProgramNameNormalizer},
		},
		{
			// R4.1: successful traversal exits 0. ExitCode defaults to 0.
			Name: "success_exits_zero",
			Args: []string{"-k", filepath.Join(basic, "sub1")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffPermissionDenied verifies that a permission-denied directory
// produces exit 1 with a "cannot read directory" diagnostic. R4.2.
func TestDiffPermissionDenied(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("permission test requires unix")
	}
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	dir := t.TempDir()
	noRead := filepath.Join(dir, "noperm")
	if err := os.Mkdir(noRead, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(noRead, "secret.txt"), "hidden\n")
	// Remove read permission so ReadDir fails.
	if err := os.Chmod(noRead, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Restore permission so TempDir cleanup can remove it.
		os.Chmod(noRead, 0o755) // best-effort cleanup
	})

	tests := []testutils.DiffTest{
		{
			Name:      "permission_denied_dir",
			Args:      []string{noRead},
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

// TestDiffFlagCombinations verifies combined flags work correctly.
func TestDiffFlagCombinations(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdu")
	if err != nil {
		t.Skipf("reference binary gdu not in PATH: %v", err)
	}

	basic := createBasicFixture(t)
	nested := createNestedFixture(t)

	tests := []testutils.DiffTest{
		{
			Name: "combined_bytes_total",
			Args: []string{"-bc",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
		},
		{
			Name: "depth_total_combined",
			Args: []string{"-cd", "1", nested},
		},
		{
			Name: "combined_short_flags_ack",
			Args: []string{"-ack",
				filepath.Join(basic, "sub1"),
				filepath.Join(basic, "sub2")},
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

// createNestedFixture creates a deeply nested directory for max-depth tests.
func createNestedFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	ab := filepath.Join(a, "b")
	abc := filepath.Join(ab, "c")
	if err := os.MkdirAll(abc, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(a, "f1.txt"), "level one\n")
	writeFixtureFile(t, filepath.Join(ab, "f2.txt"), "level two\n")
	writeFixtureFile(t, filepath.Join(abc, "f3.txt"), "level three\n")
	return dir
}

// writeFixtureFile writes a test file with the given content.
func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
