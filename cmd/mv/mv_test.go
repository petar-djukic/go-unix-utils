// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mv against gmv (GNU coreutils).
//
// Covers prd057-mv R1.1-R1.4.
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
	name  string
	args  []string
	setup func(t *testing.T, dir string)
	check func(t *testing.T, dir string)
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
		name: "partial_failure_exit_1",
		args: []string{"good.txt", "nonexistent.txt", "dest"},
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

	runMvDiffTestExitCode(t, goBin, refBin, tc, 1)
}

// runMvDiffTest runs a mv test case against both binaries in separate dirs,
// comparing exit codes and stdout, then checking filesystem state.
func runMvDiffTest(t *testing.T, goBin, refBin string, tc mvTestCase) {
	t.Helper()
	runMvDiffTestExitCode(t, goBin, refBin, tc, 0)
}

// runMvDiffTestExitCode runs a mv test with expected exit code.
func runMvDiffTestExitCode(t *testing.T, goBin, refBin string, tc mvTestCase, expectedExit int) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	tc.setup(t, refDir)
	tc.setup(t, goDir)

	env := append(os.Environ(), "LC_ALL=C")

	refOut, refErr, refExit := runBin(t, refBin, tc.args, env, refDir)
	goOut, goErr, goExit := runBin(t, goBin, tc.args, env, goDir)

	// Normalize stderr (binary name differs).
	refErr = discardAll(refErr)
	goErr = discardAll(goErr)

	if !bytes.Equal(refOut, goOut) || refExit != goExit {
		t.Errorf("divergence for args=%v\n"+
			"  stdout ref: %q\n  stdout  go: %q\n"+
			"  exit   ref: %d\n  exit    go: %d",
			tc.args, refOut, goOut, refExit, goExit)
	}
	if goExit != expectedExit {
		t.Errorf("go binary exit code %d, expected %d (args=%v)",
			goExit, expectedExit, tc.args)
	}

	// Verify filesystem state for Go binary output.
	if tc.check != nil {
		tc.check(t, goDir)
	}
}

// runBin executes a binary and returns stdout, stderr, and exit code.
func runBin(t *testing.T, bin string, args []string, env []string, dir string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = env

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
