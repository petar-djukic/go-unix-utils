// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cp against gcp (GNU coreutils).
//
// Covers prd056-cp R1.1-R1.4.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code and file state.
// Used for error messages where the binary name prefix differs.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for cp against gcp.
// R1.1: single file copy, multi-file copy into directory.
// R1.2: -i interactive (skipped — requires tty).
// R1.3: -f force remove and retry.
// R1.4: -n no-clobber.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skip("reference binary gcp not in PATH")
	}

	t.Run("error_cases", func(t *testing.T) {
		t.Parallel()
		runErrorTests(t, goBin, refBin)
	})

	t.Run("single_copy", func(t *testing.T) {
		t.Parallel()
		runSingleCopyTest(t, goBin, refBin)
	})

	t.Run("multi_copy_into_dir", func(t *testing.T) {
		t.Parallel()
		runMultiCopyTest(t, goBin, refBin)
	})

	t.Run("no_clobber", func(t *testing.T) {
		t.Parallel()
		runNoClobberTest(t, goBin, refBin)
	})

	t.Run("force_overwrite", func(t *testing.T) {
		t.Parallel()
		runForceTest(t, goBin, refBin)
	})

	t.Run("verbose_output", func(t *testing.T) {
		t.Parallel()
		runVerboseTest(t, goBin, refBin)
	})

	t.Run("target_directory", func(t *testing.T) {
		t.Parallel()
		runTargetDirTest(t, goBin, refBin)
	})
}

// runErrorTests tests error cases with discarded stderr (binary name differs).
func runErrorTests(t *testing.T, goBin, refBin string) {
	t.Helper()

	tests := []testutils.DiffTest{
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "missing_dest",
			Args:      []string{"onlyfile"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		{
			Name:      "source_not_found",
			Args:      []string{"nonexistent", "dest"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runSingleCopyTest verifies R1.1: single file copy.
func runSingleCopyTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "hello world\n")

	tests := []testutils.DiffTest{
		{
			Name:     "single_file_copy",
			Args:     []string{"src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("hello world\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runMultiCopyTest verifies R1.1: multi-file copy into directory.
func runMultiCopyTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "a.txt"), "aaa\n")
	writeFile(t, filepath.Join(workDir, "b.txt"), "bbb\n")
	if err := os.Mkdir(filepath.Join(workDir, "dest"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "multi_file_into_dir",
			Args:     []string{"a.txt", "b.txt", "dest"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dest/a.txt": []byte("aaa\n"),
				"dest/b.txt": []byte("bbb\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runNoClobberTest verifies R1.4: -n does not overwrite existing files.
func runNoClobberTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "new content\n")
	writeFile(t, filepath.Join(workDir, "existing.txt"), "old content\n")

	tests := []testutils.DiffTest{
		{
			Name:     "no_clobber",
			Args:     []string{"-n", "src.txt", "existing.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"existing.txt": []byte("old content\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runForceTest verifies R1.3: -f removes dest and retries.
func runForceTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "forced content\n")
	writeFile(t, filepath.Join(workDir, "dst.txt"), "old\n")

	tests := []testutils.DiffTest{
		{
			Name:     "force_overwrite",
			Args:     []string{"-f", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("forced content\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runVerboseTest verifies verbose output matches between binaries.
func runVerboseTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "v.txt"), "verbose\n")

	tests := []testutils.DiffTest{
		{
			Name:     "verbose_copy",
			Args:     []string{"-v", "v.txt", "v_copy.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"v_copy.txt": []byte("verbose\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runTargetDirTest verifies -t DIRECTORY copies sources into directory.
func runTargetDirTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "t1.txt"), "target dir\n")
	if err := os.Mkdir(filepath.Join(workDir, "tdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "target_directory_flag",
			Args:     []string{"-t", "tdir", "t1.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"tdir/t1.txt": []byte("target dir\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDirWithoutR verifies R2.2 behavior (omitting directory without -r).
func TestDiffDirWithoutR(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skip("reference binary gcp not in PATH")
	}

	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "srcdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "dir_without_recursive",
			Args:      []string{"srcdir", "destdir"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
