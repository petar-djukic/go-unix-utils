// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rm against grm (GNU coreutils).
//
// Covers prd058-rm R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code and file state.
func discardAll(data []byte) []byte {
	return nil
}

// rmTestCase describes a differential rm test with per-run setup.
type rmTestCase struct {
	name     string
	args     []string
	stdin    []byte
	exitCode int
	setup    func(t *testing.T, dir string)
	check    func(t *testing.T, dir string)
}

// TestDiffErrors runs error case tests using RunDiffTests (no file mutation).
// R1.1-R1.4: error handling for missing arguments.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSingleFile verifies single file removal.
// R1.1: remove a file using unlink(2).
func TestDiffSingleFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name: "remove_single_file",
			args: []string{"file.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "hello\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffMultiFile verifies multi-file removal.
// R1.1: remove multiple files.
// R1.4: continue after failure.
func TestDiffMultiFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name: "remove_multiple_files",
			args: []string{"a.txt", "b.txt", "c.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
				writeFile(t, filepath.Join(dir, "c.txt"), "ccc\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "a.txt")
				assertFileAbsent(t, dir, "b.txt")
				assertFileAbsent(t, dir, "c.txt")
			},
		},
		{
			name:     "partial_failure_continues",
			args:     []string{"good.txt", "nonexistent.txt", "also_good.txt"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "good.txt"), "ok\n")
				writeFile(t, filepath.Join(dir, "also_good.txt"), "ok\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "good.txt")
				assertFileAbsent(t, dir, "also_good.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffDirectoryWithoutR verifies that directories are refused without -r.
// R1.2: without -r, refuse directories.
func TestDiffDirectoryWithoutR(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "directory_without_r",
			args:     []string{"mydir"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "mydir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertDirExists(t, dir, "mydir")
			},
		},
		{
			name:     "file_and_dir_mixed_no_r",
			args:     []string{"file.txt", "mydir"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
				mkdirAll(t, filepath.Join(dir, "mydir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
				assertDirExists(t, dir, "mydir")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffDotRefusal verifies that '.' and '..' are refused.
// R1.3: refuse removal of '.' and '..'.
func TestDiffDotRefusal(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "refuse_dot",
			args:     []string{"-rf", "."},
			exitCode: 1,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "refuse_dotdot",
			args:     []string{"-rf", ".."},
			exitCode: 1,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "refuse_path_ending_dot",
			args:     []string{"-rf", "subdir/."},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "subdir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertDirExists(t, dir, "subdir")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffPermissionDenied verifies error on permission denied.
// R1.4: print error and continue.
func TestDiffPermissionDenied(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	tc := rmTestCase{
		name:     "permission_denied_continues",
		args:     []string{"protected/file.txt", "ok.txt"},
		exitCode: 1,
		setup: func(t *testing.T, dir string) {
			t.Helper()
			pdir := filepath.Join(dir, "protected")
			mkdirAll(t, pdir)
			writeFile(t, filepath.Join(pdir, "file.txt"), "secret\n")
			if err := os.Chmod(pdir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				os.Chmod(pdir, 0o755) //nolint:errcheck // best-effort restore
			})
			writeFile(t, filepath.Join(dir, "ok.txt"), "ok\n")
		},
		check: func(t *testing.T, dir string) {
			t.Helper()
			assertFileAbsent(t, dir, "ok.txt")
		},
	}

	runRmDiffTest(t, goBin, refBin, tc)
}

// TestDiffNonexistentFile verifies error on non-existent file.
// R1.4: print error to stderr and exit 1.
func TestDiffNonexistentFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"nosuchfile.txt"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursive verifies recursive directory removal.
// R2.1: -r removes directories and their contents recursively.
func TestDiffRecursive(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name: "recursive_single_dir",
			args: []string{"-r", "mydir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "mydir", "sub"))
				writeFile(t, filepath.Join(dir, "mydir", "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "mydir", "sub", "b.txt"), "bbb\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "mydir")
			},
		},
		{
			name: "recursive_with_R_flag",
			args: []string{"-R", "d"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "d"))
				writeFile(t, filepath.Join(dir, "d", "f.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "d")
			},
		},
		{
			name: "recursive_long_flag",
			args: []string{"--recursive", "tree"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "tree", "a", "b"))
				writeFile(t, filepath.Join(dir, "tree", "a", "b", "c.txt"), "deep\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "tree")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffForce verifies force mode.
// R2.2: -f ignores non-existent files, never prompts.
func TestDiffForce(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "force_nonexistent_file",
			args:     []string{"-f", "nosuch.txt"},
			exitCode: 0,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "force_no_operands",
			args:     []string{"-f"},
			exitCode: 0,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "force_long_flag",
			args:     []string{"--force", "missing.txt"},
			exitCode: 0,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "force_removes_existing_file",
			args:     []string{"-f", "exists.txt"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "exists.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "exists.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffRecursiveForce verifies -rf combined behavior.
// R2.3: silently removes directory trees without prompting.
func TestDiffRecursiveForce(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "rf_directory_tree",
			args:     []string{"-rf", "tree"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "tree", "sub"))
				writeFile(t, filepath.Join(dir, "tree", "f1.txt"), "one\n")
				writeFile(t, filepath.Join(dir, "tree", "sub", "f2.txt"), "two\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "tree")
			},
		},
		{
			name:     "rf_nonexistent_dir",
			args:     []string{"-rf", "nosuchdir"},
			exitCode: 0,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "rf_mixed_existing_nonexistent",
			args:     []string{"-rf", "real", "fake"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "real"))
				writeFile(t, filepath.Join(dir, "real", "x.txt"), "x\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "real")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffEmptyDir verifies -d flag for empty directory removal.
// R2.4: -d removes empty directories; without -d or -r, directory removal fails.
func TestDiffEmptyDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "d_removes_empty_dir",
			args:     []string{"-d", "emptydir"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "emptydir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "emptydir")
			},
		},
		{
			name:     "d_long_flag",
			args:     []string{"--dir", "emptydir"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "emptydir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "emptydir")
			},
		},
		{
			name:     "d_nonempty_dir_fails",
			args:     []string{"-d", "notempty"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "notempty"))
				writeFile(t, filepath.Join(dir, "notempty", "f.txt"), "x\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertDirExists(t, dir, "notempty")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffRecursiveVerbose verifies verbose output during recursive removal.
// R2.1 + R3.3: verbose output matches GNU rm format for directory removal.
func TestDiffRecursiveVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	tc := rmTestCase{
		name:     "recursive_verbose",
		args:     []string{"-rv", "d"},
		exitCode: 0,
		setup: func(t *testing.T, dir string) {
			t.Helper()
			mkdirAll(t, filepath.Join(dir, "d"))
			writeFile(t, filepath.Join(dir, "d", "f.txt"), "data\n")
		},
		check: func(t *testing.T, dir string) {
			t.Helper()
			assertFileAbsent(t, dir, "d")
		},
	}

	runRmDiffTest(t, goBin, refBin, tc)
}

// TestDiffInteractiveAlways verifies -i prompts before every removal.
// R3.1: -i must prompt before every removal.
func TestDiffInteractiveAlways(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "i_single_file_yes",
			args:     []string{"-i", "file.txt"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
		{
			name:     "i_single_file_no",
			args:     []string{"-i", "file.txt"},
			stdin:    []byte("n\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileExists(t, dir, "file.txt")
			},
		},
		{
			name:     "i_two_files_yes",
			args:     []string{"-i", "a.txt", "b.txt"},
			stdin:    []byte("y\ny\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "a.txt")
				assertFileAbsent(t, dir, "b.txt")
			},
		},
		{
			name:     "i_recursive_dir_yes",
			args:     []string{"-ri", "d"},
			stdin:    []byte("y\ny\ny\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "d"))
				writeFile(t, filepath.Join(dir, "d", "f.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "d")
			},
		},
		{
			name:     "i_empty_dir_yes",
			args:     []string{"-di", "emptydir"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "emptydir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "emptydir")
			},
		},
		{
			name:     "i_empty_dir_no",
			args:     []string{"-di", "emptydir"},
			stdin:    []byte("n\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "emptydir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertDirExists(t, dir, "emptydir")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffInteractiveOnce verifies -I prompts once for bulk removal.
// R3.2: -I must prompt once before removing >3 files or recursively.
func TestDiffInteractiveOnce(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "I_four_files_yes",
			args:     []string{"-I", "a", "b", "c", "d"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a"), "1\n")
				writeFile(t, filepath.Join(dir, "b"), "2\n")
				writeFile(t, filepath.Join(dir, "c"), "3\n")
				writeFile(t, filepath.Join(dir, "d"), "4\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "a")
				assertFileAbsent(t, dir, "b")
				assertFileAbsent(t, dir, "c")
				assertFileAbsent(t, dir, "d")
			},
		},
		{
			name:     "I_four_files_no",
			args:     []string{"-I", "a", "b", "c", "d"},
			stdin:    []byte("n\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a"), "1\n")
				writeFile(t, filepath.Join(dir, "b"), "2\n")
				writeFile(t, filepath.Join(dir, "c"), "3\n")
				writeFile(t, filepath.Join(dir, "d"), "4\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileExists(t, dir, "a")
				assertFileExists(t, dir, "b")
				assertFileExists(t, dir, "c")
				assertFileExists(t, dir, "d")
			},
		},
		{
			name:     "I_two_files_no_prompt",
			args:     []string{"-I", "a", "b"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a"), "1\n")
				writeFile(t, filepath.Join(dir, "b"), "2\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "a")
				assertFileAbsent(t, dir, "b")
			},
		},
		{
			name:     "I_recursive_dir_yes",
			args:     []string{"-rI", "d"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "d"))
				writeFile(t, filepath.Join(dir, "d", "f.txt"), "x\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "d")
			},
		},
		{
			name:     "I_recursive_dir_no",
			args:     []string{"-rI", "d"},
			stdin:    []byte("n\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "d"))
				writeFile(t, filepath.Join(dir, "d", "f.txt"), "x\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertDirExists(t, dir, "d")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffVerboseFile verifies verbose output for single file removal.
// R3.3: -v prints the name of each file as it is removed.
func TestDiffVerboseFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "verbose_single_file",
			args:     []string{"-v", "file.txt"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
		{
			name:     "verbose_long_flag",
			args:     []string{"--verbose", "file.txt"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffInteractiveWhen verifies --interactive=WHEN flag.
// R3.4: --interactive=WHEN controls prompting mode.
func TestDiffInteractiveWhen(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
	}

	cases := []rmTestCase{
		{
			name:     "interactive_never_nonexistent",
			args:     []string{"--interactive=never", "nosuch.txt"},
			exitCode: 1,
			setup:    func(t *testing.T, dir string) { t.Helper() },
			check:    func(t *testing.T, dir string) { t.Helper() },
		},
		{
			name:     "interactive_always_yes",
			args:     []string{"--interactive=always", "file.txt"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
		{
			name:     "interactive_always_no",
			args:     []string{"--interactive=always", "file.txt"},
			stdin:    []byte("n\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileExists(t, dir, "file.txt")
			},
		},
		{
			name:     "interactive_once_four_yes",
			args:     []string{"--interactive=once", "a", "b", "c", "d"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a"), "1\n")
				writeFile(t, filepath.Join(dir, "b"), "2\n")
				writeFile(t, filepath.Join(dir, "c"), "3\n")
				writeFile(t, filepath.Join(dir, "d"), "4\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "a")
				assertFileAbsent(t, dir, "b")
				assertFileAbsent(t, dir, "c")
				assertFileAbsent(t, dir, "d")
			},
		},
		{
			name:     "interactive_no_value_yes",
			args:     []string{"--interactive", "file.txt"},
			stdin:    []byte("y\n"),
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
		{
			name:     "force_overrides_i",
			args:     []string{"-if", "file.txt"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "file.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runRmDiffTest(t, goBin, refBin, tc)
		})
	}
}

// runRmDiffTest runs an rm test case against both binaries in separate dirs,
// comparing exit codes and stdout, then checking filesystem state.
func runRmDiffTest(t *testing.T, goBin, refBin string, tc rmTestCase) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	env := append(os.Environ(), "LC_ALL=C")

	refOut, refErr, refExit := runBin(t, refBin, tc.args, tc.stdin, env, refDir)
	goOut, goErr, goExit := runBin(t, goBin, tc.args, tc.stdin, env, goDir)

	// Normalize stderr (binary name differs).
	refErr = discardAll(refErr)
	goErr = discardAll(goErr)

	if !bytes.Equal(refOut, goOut) || refExit != goExit {
		t.Errorf("divergence for args=%v\n"+
			"  stdout ref: %q\n  stdout  go: %q\n"+
			"  exit   ref: %d\n  exit    go: %d",
			tc.args, refOut, goOut, refExit, goExit)
	}
	if goExit != tc.exitCode {
		t.Errorf("go binary exit code %d, expected %d (args=%v)",
			goExit, tc.exitCode, tc.args)
	}

	if tc.check != nil {
		tc.check(t, goDir)
	}
}

// runBin executes a binary and returns stdout, stderr, and exit code.
func runBin(
	t *testing.T, bin string, args []string, stdin []byte, env []string, dir string,
) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode(err)
}

// exitCode extracts the exit code from an exec error.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// assertFileAbsent checks that a file does not exist.
func assertFileAbsent(t *testing.T, dir, relPath string) {
	t.Helper()
	path := filepath.Join(dir, relPath)
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("expected %s to not exist, but it does", relPath)
	}
}

// assertFileExists checks that a file exists.
func assertFileExists(t *testing.T, dir, relPath string) {
	t.Helper()
	path := filepath.Join(dir, relPath)
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("expected %s to exist: %v", relPath, err)
	}
}

// assertDirExists checks that a directory exists.
func assertDirExists(t *testing.T, dir, relPath string) {
	t.Helper()
	path := filepath.Join(dir, relPath)
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("expected directory %s to exist: %v", relPath, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", relPath)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mkdirAll creates a directory and all parents.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
