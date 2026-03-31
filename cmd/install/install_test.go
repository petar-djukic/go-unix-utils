// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/install against ginstall (GNU coreutils).
//
// Covers prd101-install R1.1-R1.4, R2.1-R2.4, R3.1-R3.3.
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

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ginstall")
	if err != nil {
		t.Skip("reference binary ginstall not in PATH")
	}

	t.Run("error_cases", func(t *testing.T) {
		t.Parallel()
		runErrorTests(t, goBin, refBin)
	})

	t.Run("basic_copy", func(t *testing.T) {
		t.Parallel()
		runBasicCopyTest(t, goBin, refBin)
	})

	t.Run("copy_mode", func(t *testing.T) {
		t.Parallel()
		runCopyModeTest(t, goBin, refBin)
	})

	t.Run("directory_mode", func(t *testing.T) {
		t.Parallel()
		runDirectoryModeTest(t, goBin, refBin)
	})

	t.Run("create_leading", func(t *testing.T) {
		t.Parallel()
		runCreateLeadingTest(t, goBin, refBin)
	})

	t.Run("backup_simple", func(t *testing.T) {
		t.Parallel()
		runBackupSimpleTest(t, goBin, refBin)
	})

	t.Run("verbose_output", func(t *testing.T) {
		t.Parallel()
		runVerboseTest(t, goBin, refBin)
	})

	t.Run("compare_identical", func(t *testing.T) {
		t.Parallel()
		runCompareIdenticalTest(t, goBin, refBin)
	})

	t.Run("compare_different_content", func(t *testing.T) {
		t.Parallel()
		runCompareDifferentContentTest(t, goBin, refBin)
	})

	t.Run("compare_different_mode", func(t *testing.T) {
		t.Parallel()
		runCompareDifferentModeTest(t, goBin, refBin)
	})

	t.Run("compare_dest_missing", func(t *testing.T) {
		t.Parallel()
		runCompareDestMissingTest(t, goBin, refBin)
	})

	t.Run("compare_long_flag", func(t *testing.T) {
		t.Parallel()
		runCompareLongFlagTest(t, goBin, refBin)
	})

	t.Run("exit_code_error", func(t *testing.T) {
		t.Parallel()
		runExitCodeErrorTest(t, goBin, refBin)
	})

	t.Run("multi_file_into_dir", func(t *testing.T) {
		t.Parallel()
		runMultiFileIntoDirTest(t, goBin, refBin)
	})
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// mkdirAll creates a directory tree.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdirAll %s: %v", path, err)
	}
}

// runErrorTests verifies R3.2: exit 1 on errors.
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

// runBasicCopyTest verifies R1.1: basic file copy with default 755 mode.
func runBasicCopyTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "hello install\n")

	tests := []testutils.DiffTest{
		{
			Name:     "basic_copy",
			Args:     []string{"src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("hello install\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCopyModeTest verifies R1.2: -m sets permissions.
func runCopyModeTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "mode test\n")

	tests := []testutils.DiffTest{
		{
			Name:     "mode_644",
			Args:     []string{"-m", "644", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("mode test\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runDirectoryModeTest verifies R2.1: -d creates directories.
func runDirectoryModeTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()

	tests := []testutils.DiffTest{
		{
			Name:     "create_directory",
			Args:     []string{"-d", "newdir/subdir"},
			WorkDir:  workDir,
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCreateLeadingTest verifies R2.2: -D creates leading directories.
func runCreateLeadingTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "leading test\n")

	tests := []testutils.DiffTest{
		{
			Name:     "create_leading_dirs",
			Args:     []string{"-D", "src.txt", "a/b/dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"a/b/dst.txt": []byte("leading test\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runBackupSimpleTest verifies R2.3: -b creates simple backups.
func runBackupSimpleTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "new content\n")
	writeFile(t, filepath.Join(workDir, "dst.txt"), "old content\n")

	tests := []testutils.DiffTest{
		{
			Name:     "backup_simple",
			Args:     []string{"-b", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("new content\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runVerboseTest verifies R2.4: -v prints installed file names.
func runVerboseTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "verbose\n")

	tests := []testutils.DiffTest{
		{
			Name:     "verbose_copy",
			Args:     []string{"-v", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("verbose\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCompareIdenticalTest verifies R3.1: -C skips install when identical.
func runCompareIdenticalTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "same content\n")
	// Pre-create dest with same content and default install mode (755).
	destPath := filepath.Join(workDir, "dst.txt")
	writeFile(t, destPath, "same content\n")
	if err := os.Chmod(destPath, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "compare_skip_identical",
			Args:     []string{"-C", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("same content\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCompareDifferentContentTest verifies R3.1: -C installs when content differs.
func runCompareDifferentContentTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "new content\n")
	destPath := filepath.Join(workDir, "dst.txt")
	writeFile(t, destPath, "old content\n")
	if err := os.Chmod(destPath, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "compare_install_different",
			Args:     []string{"-C", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("new content\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCompareDifferentModeTest verifies R3.1: -C installs when mode differs.
func runCompareDifferentModeTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "mode diff\n")
	destPath := filepath.Join(workDir, "dst.txt")
	writeFile(t, destPath, "mode diff\n")
	// Set to 644 but install with default 755 — should trigger install.
	if err := os.Chmod(destPath, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "compare_install_mode_diff",
			Args:     []string{"-C", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("mode diff\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCompareDestMissingTest verifies R3.1: -C installs when dest doesn't exist.
func runCompareDestMissingTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "no dest yet\n")

	tests := []testutils.DiffTest{
		{
			Name:     "compare_install_no_dest",
			Args:     []string{"-C", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("no dest yet\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runCompareLongFlagTest verifies R3.1: --compare works the same as -C.
func runCompareLongFlagTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "src.txt"), "long flag\n")
	destPath := filepath.Join(workDir, "dst.txt")
	writeFile(t, destPath, "long flag\n")
	if err := os.Chmod(destPath, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "compare_long_flag",
			Args:     []string{"--compare", "src.txt", "dst.txt"},
			WorkDir:  workDir,
			ExitCode: 0,
			ExpectedFiles: map[string][]byte{
				"dst.txt": []byte("long flag\n"),
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runExitCodeErrorTest verifies R3.2: exit 1 when source doesn't exist.
func runExitCodeErrorTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()

	tests := []testutils.DiffTest{
		{
			Name:      "exit_1_missing_source",
			Args:      []string{"no_such_file", "dest"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runMultiFileIntoDirTest verifies multi-source install into directory.
func runMultiFileIntoDirTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "a.txt"), "aaa\n")
	writeFile(t, filepath.Join(workDir, "b.txt"), "bbb\n")
	mkdirAll(t, filepath.Join(workDir, "dest"))

	tests := []testutils.DiffTest{
		{
			Name:     "multi_into_dir",
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
