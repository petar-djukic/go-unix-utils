// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mv against gmv (GNU coreutils).
//
// Covers prd057-mv R1.1-R1.4, R2.1-R2.4, R3.1-R3.3, R4.1-R4.4.
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

// mvTestCase describes a differential mv test with per-run setup.
type mvTestCase struct {
	name     string
	args     []string
	stdin    []byte
	exitCode int // expected exit code, 0 if omitted
	setup    func(t *testing.T, dir string)
	check    func(t *testing.T, dir string)
}

// TestDiffErrors runs error case tests using RunDiffTests (no file mutation).
// R1.1-R1.4: error handling for missing arguments and source.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

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

// TestDiffMoves runs mv tests that mutate the filesystem.
// Each test creates separate temp directories for ref and Go binaries.
// R1.1: single file rename.
// R1.2: multi-source move into directory.
// R1.3: directory move without -r.
// R1.4: move into existing directory.
func TestDiffMoves(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name: "rename_file",
			args: []string{"src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "hello\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "hello\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		{
			name: "move_into_directory",
			args: []string{"file.txt", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "content\n")
				mkdirAll(t, filepath.Join(dir, "destdir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "destdir/file.txt", "content\n")
				assertFileAbsent(t, dir, "file.txt")
			},
		},
		{
			name: "multi_file_into_dir",
			args: []string{"a.txt", "b.txt", "dest"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/a.txt", "aaa\n")
				assertFileContent(t, dir, "dest/b.txt", "bbb\n")
				assertFileAbsent(t, dir, "a.txt")
				assertFileAbsent(t, dir, "b.txt")
			},
		},
		{
			name: "directory_move_no_r",
			args: []string{"srcdir", "destdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "srcdir", "sub"))
				writeFile(t, filepath.Join(dir, "srcdir", "top.txt"), "top\n")
				writeFile(t, filepath.Join(dir, "srcdir", "sub", "deep.txt"), "deep\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "destdir/top.txt", "top\n")
				assertFileContent(t, dir, "destdir/sub/deep.txt", "deep\n")
				assertFileAbsent(t, dir, "srcdir")
			},
		},
		{
			name: "auto_detect_dest_dir",
			args: []string{"file.txt", "existing_dir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "file.txt"), "auto\n")
				mkdirAll(t, filepath.Join(dir, "existing_dir"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "existing_dir/file.txt", "auto\n")
				assertFileAbsent(t, dir, "file.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffPartialFailure verifies that multi-source moves continue
// after a failure and exit 1.
func TestDiffPartialFailure(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	tc := mvTestCase{
		name:     "partial_failure_exit_1",
		args:     []string{"good.txt", "nonexistent.txt", "dest"},
		exitCode: 1,
		setup: func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "good.txt"), "good\n")
			mkdirAll(t, filepath.Join(dir, "dest"))
		},
		check: func(t *testing.T, dir string) {
			t.Helper()
			assertFileContent(t, dir, "dest/good.txt", "good\n")
		},
	}

	runMvDiffTest(t, goBin, refBin, tc)
}

// TestDiffOverwriteControl runs overwrite control tests.
// R2.1: interactive prompt with -i.
// R2.2: force with -f, last-flag-wins precedence.
// R2.3: no-clobber with -n.
func TestDiffOverwriteControl(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		// R2.1: interactive prompt, user answers "y".
		{
			name:  "interactive_yes",
			args:  []string{"-i", "src.txt", "dst.txt"},
			stdin: []byte("y\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "new\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		// R2.1: interactive prompt, user answers "n".
		{
			name:     "interactive_no",
			args:     []string{"-i", "src.txt", "dst.txt"},
			stdin:    []byte("n\n"),
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "src.txt", "new\n")
				assertFileContent(t, dir, "dst.txt", "old\n")
			},
		},
		// R2.1: interactive with no conflict — no prompt needed.
		{
			name: "interactive_no_conflict",
			args: []string{"-i", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "data\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		// R2.2: force overwrite without prompt.
		{
			name: "force_overwrite",
			args: []string{"-f", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "new\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		// R2.2: -i then -f, last flag (-f) wins — force overwrites.
		{
			name: "interactive_then_force",
			args: []string{"-i", "-f", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "new\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		// R2.2: -f then -i, last flag (-i) wins — user declines.
		{
			name:     "force_then_interactive",
			args:     []string{"-f", "-i", "src.txt", "dst.txt"},
			stdin:    []byte("n\n"),
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "src.txt", "new\n")
				assertFileContent(t, dir, "dst.txt", "old\n")
			},
		},
		// R2.3: no-clobber skips existing destination.
		{
			name: "no_clobber_exists",
			args: []string{"-n", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "src.txt", "new\n")
				assertFileContent(t, dir, "dst.txt", "old\n")
			},
		},
		// R2.3: no-clobber with no conflict — move succeeds.
		{
			name: "no_clobber_no_dest",
			args: []string{"-n", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "data\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffPermissionDenied verifies error output when destination
// directory is not writable.
// R2.4: permission error prints to stderr and exits 1.
func TestDiffPermissionDenied(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	tc := mvTestCase{
		name:     "dest_dir_not_writable",
		args:     []string{"src.txt", "nowrite/target.txt"},
		exitCode: 1,
		setup: func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
			nowrite := filepath.Join(dir, "nowrite")
			mkdirAll(t, nowrite)
			writeFile(t, filepath.Join(nowrite, "target.txt"), "old\n")
			if err := os.Chmod(nowrite, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				os.Chmod(nowrite, 0o755) //nolint:errcheck // best-effort restore
			})
		},
		check: func(t *testing.T, dir string) {
			t.Helper()
			assertFileContent(t, dir, "src.txt", "data\n")
		},
	}

	runMvDiffTest(t, goBin, refBin, tc)
}

// TestDiffVerbose verifies -v/--verbose output.
// R3.1: verbose prints 'renamed SOURCE -> DEST'.
func TestDiffVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name: "verbose_short",
			args: []string{"-v", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "hello\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "hello\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		{
			name: "verbose_long",
			args: []string{"--verbose", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "hello\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "hello\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		{
			name: "verbose_overwrite",
			args: []string{"-v", "-f", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "new\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffTargetDir verifies -t/--target-directory.
// R3.2: -t DIRECTORY moves all SOURCEs into DIRECTORY.
func TestDiffTargetDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name: "target_dir_short",
			args: []string{"-t", "dest", "a.txt", "b.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/a.txt", "aaa\n")
				assertFileContent(t, dir, "dest/b.txt", "bbb\n")
				assertFileAbsent(t, dir, "a.txt")
				assertFileAbsent(t, dir, "b.txt")
			},
		},
		{
			name: "target_dir_long_eq",
			args: []string{"--target-directory=dest", "f.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "f.txt"), "data\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/f.txt", "data\n")
				assertFileAbsent(t, dir, "f.txt")
			},
		},
		{
			name: "target_dir_long_sep",
			args: []string{"--target-directory", "dest", "f.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "f.txt"), "data\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/f.txt", "data\n")
				assertFileAbsent(t, dir, "f.txt")
			},
		},
		{
			name:     "target_dir_not_a_dir",
			args:     []string{"-t", "notadir", "src.txt"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
				writeFile(t, filepath.Join(dir, "notadir"), "file\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "src.txt", "data\n")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffNoTargetDir verifies -T/--no-target-directory.
// R3.3: -T treats DEST as a normal file, not a directory.
func TestDiffNoTargetDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name: "no_target_dir_into_existing_dir",
			args: []string{"-T", "src.txt", "dstdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
				mkdirAll(t, filepath.Join(dir, "dstdir"))
			},
			exitCode: 1,
			check: func(t *testing.T, dir string) {
				t.Helper()
				// -T prevents moving into dstdir; rename file over dir fails.
				assertFileContent(t, dir, "src.txt", "data\n")
			},
		},
		{
			name: "no_target_dir_file_rename",
			args: []string{"-T", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "data\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffExitZeroSuccess verifies exit code 0 on successful moves.
// R4.1: exit 0 when all files are moved successfully.
func TestDiffExitZeroSuccess(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name:     "single_rename_exit_0",
			args:     []string{"src.txt", "dst.txt"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "hello\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "hello\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		{
			name:     "multi_move_exit_0",
			args:     []string{"a.txt", "b.txt", "dest"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/a.txt", "aaa\n")
				assertFileContent(t, dir, "dest/b.txt", "bbb\n")
			},
		},
		{
			name:     "verbose_move_exit_0",
			args:     []string{"-v", "src.txt", "dst.txt"},
			exitCode: 0,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "data\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffExitOneFail verifies exit code 1 on move failures.
// R4.2: exit 1 when any move operation fails (permission denied, source not found).
func TestDiffExitOneFail(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name:     "source_not_found_exit_1",
			args:     []string{"nosuchfile.txt", "dest.txt"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				// no source file created
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "dest.txt")
			},
		},
		{
			name:     "source_not_found_into_dir_exit_1",
			args:     []string{"nosuchfile.txt", "dest"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileAbsent(t, dir, "dest/nosuchfile.txt")
			},
		},
		{
			name:     "move_file_over_nonempty_dir_exit_1",
			args:     []string{"-T", "src.txt", "dstdir"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "data\n")
				mkdirAll(t, filepath.Join(dir, "dstdir"))
				writeFile(t, filepath.Join(dir, "dstdir", "child.txt"), "x\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "src.txt", "data\n")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffContinueAfterFailure verifies that multi-source moves continue
// processing after individual failures and exit 1.
// R4.3: when moving multiple files and one fails, continue and exit 1.
func TestDiffContinueAfterFailure(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name:     "middle_source_missing",
			args:     []string{"first.txt", "missing.txt", "last.txt", "dest"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "first.txt"), "one\n")
				writeFile(t, filepath.Join(dir, "last.txt"), "three\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/first.txt", "one\n")
				assertFileContent(t, dir, "dest/last.txt", "three\n")
				assertFileAbsent(t, dir, "first.txt")
				assertFileAbsent(t, dir, "last.txt")
			},
		},
		{
			name:     "first_source_missing",
			args:     []string{"missing.txt", "good.txt", "dest"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "good.txt"), "ok\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/good.txt", "ok\n")
				assertFileAbsent(t, dir, "good.txt")
			},
		},
		{
			name:     "target_dir_partial_failure",
			args:     []string{"-t", "dest", "ok.txt", "nope.txt", "also_ok.txt"},
			exitCode: 1,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "ok.txt"), "a\n")
				writeFile(t, filepath.Join(dir, "also_ok.txt"), "b\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/ok.txt", "a\n")
				assertFileContent(t, dir, "dest/also_ok.txt", "b\n")
				assertFileAbsent(t, dir, "ok.txt")
				assertFileAbsent(t, dir, "also_ok.txt")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// TestDiffComprehensive covers the full R4.4 test matrix: single file rename,
// move into directory, multi-file move, -i, -f, -n, -v, -t, directory move,
// and error cases. This test combines flag combinations not covered individually.
// R4.4: comprehensive differential testing.
func TestDiffComprehensive(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}

	cases := []mvTestCase{
		{
			name: "verbose_multi_file_move",
			args: []string{"-v", "a.txt", "b.txt", "dest"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
				writeFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/a.txt", "aaa\n")
				assertFileContent(t, dir, "dest/b.txt", "bbb\n")
			},
		},
		{
			name: "verbose_target_dir",
			args: []string{"-v", "-t", "dest", "f.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "f.txt"), "vt\n")
				mkdirAll(t, filepath.Join(dir, "dest"))
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dest/f.txt", "vt\n")
				assertFileAbsent(t, dir, "f.txt")
			},
		},
		{
			name: "no_clobber_verbose",
			args: []string{"-n", "-v", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "src.txt", "new\n")
				assertFileContent(t, dir, "dst.txt", "old\n")
			},
		},
		{
			name: "force_verbose",
			args: []string{"-f", "-v", "src.txt", "dst.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "src.txt"), "new\n")
				writeFile(t, filepath.Join(dir, "dst.txt"), "old\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dst.txt", "new\n")
				assertFileAbsent(t, dir, "src.txt")
			},
		},
		{
			name: "directory_move_verbose",
			args: []string{"-v", "srcdir", "dstdir"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				mkdirAll(t, filepath.Join(dir, "srcdir"))
				writeFile(t, filepath.Join(dir, "srcdir", "f.txt"), "in dir\n")
			},
			check: func(t *testing.T, dir string) {
				t.Helper()
				assertFileContent(t, dir, "dstdir/f.txt", "in dir\n")
				assertFileAbsent(t, dir, "srcdir")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runMvDiffTest(t, goBin, refBin, tc)
		})
	}
}

// runMvDiffTest runs a mv test case against both binaries in separate dirs,
// comparing exit codes and stdout, then checking filesystem state.
func runMvDiffTest(t *testing.T, goBin, refBin string, tc mvTestCase) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	env := append(os.Environ(), "LC_ALL=C")

	refOut, refErr, refExit := runBin(t, refBin, tc.args, env, refDir, tc.stdin)
	goOut, goErr, goExit := runBin(t, goBin, tc.args, env, goDir, tc.stdin)

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

	// Verify filesystem state for Go binary output.
	if tc.check != nil {
		tc.check(t, goDir)
	}
}

// runBin executes a binary and returns stdout, stderr, and exit code.
func runBin(t *testing.T, bin string, args []string, env []string, dir string, stdin []byte) ([]byte, []byte, int) {
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

// assertFileContent checks that a file exists with expected content.
func assertFileContent(t *testing.T, dir, relPath, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		t.Errorf("expected file %s: %v", relPath, err)
		return
	}
	if string(data) != want {
		t.Errorf("file %s content = %q, want %q", relPath, data, want)
	}
}

// assertFileAbsent checks that a file does not exist.
func assertFileAbsent(t *testing.T, dir, relPath string) {
	t.Helper()
	path := filepath.Join(dir, relPath)
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("expected %s to not exist, but it does", relPath)
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
