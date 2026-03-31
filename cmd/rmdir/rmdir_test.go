// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against grmdir (GNU coreutils).
//
// Covers prd035-rmdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where the binary name prefix differs
// between grmdir and the Go binary.
func discardAll(data []byte) []byte {
	return nil
}

// normProgName replaces the binary name prefix (e.g. "/opt/homebrew/bin/grmdir")
// with "rmdir" so verbose output can be compared across binaries.
var progNameRe = regexp.MustCompile(`(?m)^[^\s:]+`)

func normProgName(data []byte) []byte {
	return progNameRe.ReplaceAll(data, []byte("rmdir"))
}

// TestDiff runs differential tests for rmdir cases where both
// binaries can share a WorkDir without conflict.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	// Set up a non-empty directory for error tests.
	workDir := t.TempDir()
	nonEmpty := filepath.Join(workDir, "nonempty")
	if err := os.Mkdir(nonEmpty, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonEmpty, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R1.3: non-empty directory — error exit 1
		{
			Name:      "non_empty_directory",
			Args:      []string{"nonempty"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: non-existent directory — error exit 1
		{
			Name:      "nonexistent_directory",
			Args:      []string{"doesnotexist"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: target is a file, not a directory — error exit 1
		{
			Name:      "target_is_file",
			Args:      []string{filepath.Join("nonempty", "file.txt")},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.2: no arguments — error exit 1
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.1: --ignore-fail-on-non-empty on non-empty dir — exits 0
		{
			Name:     "ignore_fail_nonempty",
			Args:     []string{"--ignore-fail-on-non-empty", "nonempty"},
			WorkDir:  workDir,
			ExitCode: 0,
		},
		// R3.2: --ignore-fail-on-non-empty does NOT suppress non-existent target.
		{
			Name:      "ignore_fail_nonexistent",
			Args:      []string{"--ignore-fail-on-non-empty", "doesnotexist"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: --ignore-fail-on-non-empty does NOT suppress file target error.
		{
			Name:      "ignore_fail_file_target",
			Args:      []string{"--ignore-fail-on-non-empty", filepath.Join("nonempty", "file.txt")},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestRmdirSingle verifies R1.1: removing a single empty directory.
func TestRmdirSingle(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	t.Run("single_empty_dir", func(t *testing.T) {
		t.Parallel()
		compareRmdir(t, goBin, refBin, []string{"emptydir"})
	})
}

// TestRmdirMultiple verifies R1.2: removing multiple directories independently.
func TestRmdirMultiple(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	t.Run("multiple_empty_dirs", func(t *testing.T) {
		t.Parallel()
		compareRmdir(t, goBin, refBin, []string{"d1", "d2", "d3"})
	})

	// R1.2 + R1.3: mixed valid and invalid targets — continues on error
	t.Run("mixed_valid_invalid", func(t *testing.T) {
		t.Parallel()
		compareRmdirMixed(t, goBin, refBin)
	})
}

// TestRmdirParents verifies R2.1, R2.2, R2.3: -p flag behavior.
func TestRmdirParents(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	// R2.1: -p removes target and all empty ancestors.
	t.Run("nested_empty", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "a", "b", "c"))
		}
		refStdout, _, refExit := execBin(t, refBin, []string{"-p", "a/b/c"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"-p", "a/b/c"}, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		for _, d := range []string{"a/b/c", "a/b", "a"} {
			assertRemoved(t, goDir, d)
		}
	})

	// R2.1: --parents long form.
	t.Run("long_flag_parents", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "x", "y"))
		}
		_, _, refExit := execBin(t, refBin, []string{"--parents", "x/y"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"--parents", "x/y"}, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertRemoved(t, goDir, "x/y")
		assertRemoved(t, goDir, "x")
	})

	// R2.2: -p stops when parent is not empty.
	t.Run("stops_on_nonempty_parent", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "a", "b", "c"))
			writeTestFile(t, filepath.Join(base, "a", "keep.txt"), "x")
		}
		_, _, refExit := execBin(t, refBin, []string{"-p", "a/b/c"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-p", "a/b/c"}, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertRemoved(t, goDir, "a/b/c")
		assertRemoved(t, goDir, "a/b")
		assertExists(t, goDir, "a")
	})

	// R2.3: -p with multiple arguments processed independently.
	t.Run("multiple_args", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "x", "y"))
			mkdirAll(t, filepath.Join(base, "m", "n"))
		}
		args := []string{"-p", "x/y", "m/n"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		for _, d := range []string{"x/y", "x", "m/n", "m"} {
			assertRemoved(t, goDir, d)
		}
	})
}

// TestRmdirIgnoreNonEmpty verifies R3.1: --ignore-fail-on-non-empty.
func TestRmdirIgnoreNonEmpty(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	// R3.1: suppresses non-empty error, directory remains.
	t.Run("suppresses_nonempty_error", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "nonempty"))
			writeTestFile(t, filepath.Join(base, "nonempty", "file.txt"), "x")
		}
		args := []string{"--ignore-fail-on-non-empty", "nonempty"}
		refStdout, refStderr, refExit := execBin(t, refBin, args, refDir)
		goStdout, goStderr, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		if len(refStderr) == 0 && len(goStderr) != 0 {
			t.Errorf("go stderr should be empty but got: %q", goStderr)
		}
		assertExists(t, goDir, "nonempty")
	})

	// R3.1 + R2.1: --ignore-fail-on-non-empty with -p stops silently.
	t.Run("with_parents_stops_silently", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "a", "b", "c"))
			writeTestFile(t, filepath.Join(base, "a", "keep.txt"), "x")
		}
		args := []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertRemoved(t, goDir, "a/b/c")
		assertRemoved(t, goDir, "a/b")
		assertExists(t, goDir, "a")
	})

	// R3.2: --ignore-fail-on-non-empty does NOT suppress non-existent error.
	t.Run("does_not_suppress_nonexistent", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		args := []string{"--ignore-fail-on-non-empty", "nosuchdir"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, goStderr, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if goExit != 1 {
			t.Errorf("expected exit 1 for non-existent dir, got %d", goExit)
		}
		if len(goStderr) == 0 {
			t.Error("expected stderr output for non-existent dir")
		}
	})
}

// TestRmdirVerbose verifies R3.3: -v/--verbose prints removal messages.
func TestRmdirVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	// R3.3: -v prints a message for each removed directory.
	t.Run("verbose_single", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "emptydir"))
		}
		args := []string{"-v", "emptydir"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(normProgName(refStdout), normProgName(goStdout)) {
			t.Errorf("stdout:\nref: %q\ngo:  %q", refStdout, goStdout)
		}
		assertRemoved(t, goDir, "emptydir")
	})

	// R3.3: --verbose long form.
	t.Run("verbose_long_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "d1"))
		}
		args := []string{"--verbose", "d1"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(normProgName(refStdout), normProgName(goStdout)) {
			t.Errorf("stdout:\nref: %q\ngo:  %q", refStdout, goStdout)
		}
	})

	// R3.3: -v with -p prints message for each removed directory.
	t.Run("verbose_with_parents", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "a", "b", "c"))
		}
		args := []string{"-pv", "a/b/c"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(normProgName(refStdout), normProgName(goStdout)) {
			t.Errorf("stdout:\nref: %q\ngo:  %q", refStdout, goStdout)
		}
		for _, d := range []string{"a/b/c", "a/b", "a"} {
			assertRemoved(t, goDir, d)
		}
	})

	// R3.3: -v with multiple dirs.
	t.Run("verbose_multiple", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "x"))
			mkdirAll(t, filepath.Join(base, "y"))
		}
		args := []string{"-v", "x", "y"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(normProgName(refStdout), normProgName(goStdout)) {
			t.Errorf("stdout:\nref: %q\ngo:  %q", refStdout, goStdout)
		}
	})
}

// TestRmdirParentStopsCorrectly verifies R4.3: -p stops ascending
// at the correct level when a parent is not empty, matching grmdir.
func TestRmdirParentStopsCorrectly(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	// R4.3: deep nesting — stops at grandparent that contains a file.
	t.Run("stops_at_grandparent", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "a", "b", "c", "d"))
			writeTestFile(t, filepath.Join(base, "a", "b", "keep.txt"), "x")
		}
		args := []string{"-p", "a/b/c/d"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertRemoved(t, goDir, "a/b/c/d")
		assertRemoved(t, goDir, "a/b/c")
		assertExists(t, goDir, "a/b")
		assertExists(t, goDir, "a")
	})

	// R4.3: -p with --ignore-fail-on-non-empty stops silently at blocker.
	t.Run("parents_ignore_stops_silently", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "x", "y", "z"))
			writeTestFile(t, filepath.Join(base, "x", "blocker.txt"), "x")
		}
		args := []string{"-p", "--ignore-fail-on-non-empty", "x/y/z"}
		refStdout, refStderr, refExit := execBin(t, refBin, args, refDir)
		goStdout, goStderr, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		if len(refStderr) == 0 && len(goStderr) != 0 {
			t.Errorf("go stderr should be empty but got: %q", goStderr)
		}
		assertRemoved(t, goDir, "x/y/z")
		assertRemoved(t, goDir, "x/y")
		assertExists(t, goDir, "x")
	})

	// R4.3: -pv stops and verbose shows only removed dirs.
	t.Run("parents_verbose_stops", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "m", "n", "o"))
			writeTestFile(t, filepath.Join(base, "m", "file.txt"), "x")
		}
		args := []string{"-pv", "m/n/o"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(normProgName(refStdout), normProgName(goStdout)) {
			t.Errorf("stdout:\nref: %q\ngo:  %q", refStdout, goStdout)
		}
		assertRemoved(t, goDir, "m/n/o")
		assertRemoved(t, goDir, "m/n")
		assertExists(t, goDir, "m")
	})
}

// TestRmdirExitCodes verifies R3.4: exit code behavior.
func TestRmdirExitCodes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary grmdir not in PATH")
	}

	// R3.4: exit 0 when all directories successfully removed.
	t.Run("all_success_exit_0", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "a"))
			mkdirAll(t, filepath.Join(base, "b"))
		}
		args := []string{"a", "b"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if goExit != 0 {
			t.Errorf("expected exit 0, got %d", goExit)
		}
	})

	// R3.4: exit non-zero when any removal fails.
	t.Run("partial_failure_exit_nonzero", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "ok"))
			mkdirAll(t, filepath.Join(base, "bad"))
			writeTestFile(t, filepath.Join(base, "bad", "f"), "x")
		}
		args := []string{"ok", "bad"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if goExit != 1 {
			t.Errorf("expected exit 1, got %d", goExit)
		}
	})

	// R3.4 + R3.1: suppressed failure yields exit 0.
	t.Run("suppressed_failure_exit_0", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()
		for _, base := range []string{goDir, refDir} {
			mkdirAll(t, filepath.Join(base, "bad"))
			writeTestFile(t, filepath.Join(base, "bad", "f"), "x")
		}
		args := []string{"--ignore-fail-on-non-empty", "bad"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)
		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if goExit != 0 {
			t.Errorf("expected exit 0, got %d", goExit)
		}
	})
}

// compareRmdir sets up identical empty directories in two temp dirs,
// runs both binaries, and compares exit codes and stdout.
func compareRmdir(t *testing.T, goBin, refBin string, dirs []string) {
	t.Helper()

	goDir := t.TempDir()
	refDir := t.TempDir()
	for _, d := range dirs {
		if err := os.Mkdir(filepath.Join(goDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(refDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	refStdout, _, refExit := execBin(t, refBin, dirs, refDir)
	goStdout, _, goExit := execBin(t, goBin, dirs, goDir)

	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d (args=%v)",
			refExit, goExit, dirs)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout divergence:\nref: %q\ngo:  %q",
			string(refStdout), string(goStdout))
	}

	// Verify directories were actually removed.
	for _, d := range dirs {
		assertRemoved(t, goDir, d)
	}
}

// compareRmdirMixed tests R1.2 + R1.3: one valid and one non-empty dir.
// Both binaries should remove the empty one and fail on the non-empty one.
func compareRmdirMixed(t *testing.T, goBin, refBin string) {
	t.Helper()

	goDir := t.TempDir()
	refDir := t.TempDir()

	// Create empty directory "ok" and non-empty "bad" in both.
	for _, base := range []string{goDir, refDir} {
		if err := os.Mkdir(filepath.Join(base, "ok"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(base, "bad"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "bad", "f"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	args := []string{"ok", "bad"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d", refExit, goExit)
	}

	// "ok" should be removed; "bad" should remain.
	assertRemoved(t, goDir, "ok")
	assertExists(t, goDir, "bad")
}

// execBin runs a binary and returns stdout, stderr, and exit code.
func execBin(t *testing.T, bin string, args []string, workDir string) ([]byte, []byte, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRemoved(t *testing.T, base, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(base, dir)); !os.IsNotExist(err) {
		t.Errorf("directory %q should have been removed", dir)
	}
}

func assertExists(t *testing.T, base, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(base, dir)); err != nil {
		t.Errorf("directory %q should still exist: %v", dir, err)
	}
}
