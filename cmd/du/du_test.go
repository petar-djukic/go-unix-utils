// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/du against the GNU reference binary gdu.
//
// Implements prd009-du R1, R2, R3, R4 via differential testing
// using pkg/testutils.RunDiffTests.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the Go du binary built in TestMain.
// refBinaryPath is the path to the GNU reference binary (gdu).
var (
	goBinaryPath  string
	refBinaryPath string
)

func TestMain(m *testing.M) {
	// Locate GNU reference binary gdu (Homebrew coreutils).
	refPath, err := exec.LookPath("gdu")
	if err != nil {
		fmt.Println("gdu not found on PATH; skipping du differential tests")
		os.Exit(0)
	}
	refBinaryPath = refPath

	// Build the Go du binary from the current package.
	tmpDir, err := os.MkdirTemp("", "du-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinaryPath = filepath.Join(tmpDir, "du")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building du: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// buildTree creates a two-level directory tree with known files for du tests.
// Layout:
//
//	root/
//	  file1.txt    (1024 bytes)
//	  sub1/
//	    file2.txt  (2048 bytes)
//	    sub1a/
//	      file3.txt (512 bytes)
//	  sub2/
//	    file4.txt  (4096 bytes)
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, "sub1", "sub1a"), 0o755)
	os.MkdirAll(filepath.Join(root, "sub2"), 0o755)

	os.WriteFile(filepath.Join(root, "file1.txt"), make([]byte, 1024), 0o644)
	os.WriteFile(filepath.Join(root, "sub1", "file2.txt"), make([]byte, 2048), 0o644)
	os.WriteFile(filepath.Join(root, "sub1", "sub1a", "file3.txt"), make([]byte, 512), 0o644)
	os.WriteFile(filepath.Join(root, "sub2", "file4.txt"), make([]byte, 4096), 0o644)

	return root
}

// ---------------------------------------------------------------------------
// R1: Default recursive disk usage behavior (prd009-du R1)
// ---------------------------------------------------------------------------

func TestDU_DefaultBehavior(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "single_directory_argument",
			Args: []string{root},
		},
		{
			Name: "single_regular_file",
			Args: []string{filepath.Join(root, "file1.txt")},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_MultipleArguments(t *testing.T) {
	root := buildTree(t)

	sub1 := filepath.Join(root, "sub1")
	sub2 := filepath.Join(root, "sub2")

	tests := []testutils.DiffTest{
		{
			Name: "multiple_directory_arguments_in_order",
			Args: []string{sub1, sub2},
		},
		{
			Name: "directory_and_file_arguments",
			Args: []string{sub1, filepath.Join(root, "file1.txt")},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R2: Size mode and output format flags (prd009-du R2)
// ---------------------------------------------------------------------------

func TestDU_SizeModeFlags(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "flag_h_human_readable_binary",
			Args: []string{"-h", root},
		},
		{
			Name: "long_flag_human_readable",
			Args: []string{"--human-readable", root},
		},
		{
			Name: "flag_si_human_readable",
			Args: []string{"--si", root},
		},
		{
			Name: "flag_k_kilobyte_blocks",
			Args: []string{"-k", root},
		},
		{
			Name: "flag_m_megabyte_blocks",
			Args: []string{"-m", root},
		},
		{
			Name: "flag_b_apparent_size",
			Args: []string{"-b", root},
		},
		{
			Name: "long_flag_apparent_size",
			Args: []string{"--apparent-size", root},
		},
		{
			Name: "flag_block_size_512",
			Args: []string{"--block-size=512", root},
		},
		{
			Name: "flag_block_size_4096",
			Args: []string{"--block-size=4096", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_SummarizeFlag(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "flag_s_summarize",
			Args: []string{"-s", root},
		},
		{
			Name: "long_flag_summarize",
			Args: []string{"--summarize", root},
		},
		{
			Name: "summarize_multiple_args",
			Args: []string{"-s", filepath.Join(root, "sub1"), filepath.Join(root, "sub2")},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_AllFilesFlag(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "flag_a_all_files",
			Args: []string{"-a", root},
		},
		{
			Name: "long_flag_all",
			Args: []string{"--all", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_TotalFlag(t *testing.T) {
	root := buildTree(t)
	sub1 := filepath.Join(root, "sub1")
	sub2 := filepath.Join(root, "sub2")

	tests := []testutils.DiffTest{
		{
			Name: "flag_c_total_single_arg",
			Args: []string{"-c", root},
		},
		{
			Name: "long_flag_total_multiple_args",
			Args: []string{"--total", sub1, sub2},
		},
		{
			Name: "flag_c_total_multiple_args",
			Args: []string{"-c", sub1, sub2},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_MaxDepthFlag(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "flag_d_0_depth_zero",
			Args: []string{"-d", "0", root},
		},
		{
			Name: "flag_d_1_depth_one",
			Args: []string{"-d", "1", root},
		},
		{
			Name: "flag_d_2_depth_two",
			Args: []string{"-d", "2", root},
		},
		{
			Name: "long_flag_max_depth_1",
			Args: []string{"--max-depth=1", root},
		},
		{
			Name: "long_flag_max_depth_0",
			Args: []string{"--max-depth=0", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_NullTerminator(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "flag_0_null_terminated",
			Args: []string{"-0", root},
		},
		{
			Name: "long_flag_null",
			Args: []string{"--null", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R3: Traversal behavior (prd009-du R3)
// ---------------------------------------------------------------------------

func TestDU_HardLinkDeduplication(t *testing.T) {
	root := t.TempDir()

	os.MkdirAll(filepath.Join(root, "dirA"), 0o755)
	os.MkdirAll(filepath.Join(root, "dirB"), 0o755)

	// Create a file and hard-link it into a second directory.
	original := filepath.Join(root, "dirA", "original.txt")
	os.WriteFile(original, make([]byte, 8192), 0o644)

	hardlink := filepath.Join(root, "dirB", "linked.txt")
	if err := os.Link(original, hardlink); err != nil {
		t.Skipf("os.Link not supported: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name: "hardlink_dedup_counted_once",
			Args: []string{"-a", root},
		},
		{
			Name: "hardlink_dedup_with_total",
			Args: []string{"-a", "-c", root},
		},
		{
			Name: "hardlink_dedup_separate_args",
			Args: []string{"-a", filepath.Join(root, "dirA"), filepath.Join(root, "dirB")},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_SymlinkHandling(t *testing.T) {
	root := t.TempDir()

	targetDir := filepath.Join(root, "target")
	os.MkdirAll(targetDir, 0o755)
	os.WriteFile(filepath.Join(targetDir, "data.txt"), make([]byte, 4096), 0o644)

	symDir := filepath.Join(root, "link_to_target")
	if err := os.Symlink(targetDir, symDir); err != nil {
		t.Skipf("os.Symlink not supported: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name: "symlink_default_no_follow",
			Args: []string{root},
		},
		{
			Name: "flag_L_dereference_symlinks",
			Args: []string{"-L", root},
		},
		{
			Name: "long_flag_dereference",
			Args: []string{"--dereference", root},
		},
		{
			Name: "flag_P_no_dereference_explicit",
			Args: []string{"-P", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_CombinedShortFlags(t *testing.T) {
	root := buildTree(t)

	tests := []testutils.DiffTest{
		{
			Name: "combined_sh",
			Args: []string{"-sh", root},
		},
		{
			Name: "combined_sc",
			Args: []string{"-sc", root},
		},
		{
			Name: "combined_ah",
			Args: []string{"-ah", root},
		},
		{
			Name: "combined_ac",
			Args: []string{"-ac", root},
		},
		{
			Name: "combined_sch",
			Args: []string{"-sch", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R4: Error handling and exit codes (prd009-du R4)
// ---------------------------------------------------------------------------

func TestDU_ErrorHandling(t *testing.T) {
	root := buildTree(t)
	nonexistent := filepath.Join(root, "nonexistent")

	tests := []testutils.DiffTest{
		{
			Name: "nonexistent_path_exit_1",
			Args: []string{nonexistent},
		},
		{
			Name: "nonexistent_with_valid_path",
			Args: []string{nonexistent, filepath.Join(root, "sub1")},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

func TestDU_FlagTerminator(t *testing.T) {
	root := t.TempDir()

	// Create a file whose name resembles a flag.
	dashFile := filepath.Join(root, "-s")
	os.WriteFile(dashFile, []byte("flag-like name\n"), 0o644)

	tests := []testutils.DiffTest{
		{
			Name: "double_dash_flag_like_filename",
			Args: []string{"--", dashFile},
		},
		{
			Name: "double_dash_with_directory",
			Args: []string{"--", root},
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}
