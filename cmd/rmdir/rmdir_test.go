// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against grmdir (GNU coreutils).
//
// Covers prd035-rmdir R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where the binary name prefix differs
// between grmdir and the Go binary.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for rmdir error cases where both
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
		if _, err := os.Stat(filepath.Join(goDir, d)); !os.IsNotExist(err) {
			t.Errorf("directory %q still exists after rmdir", d)
		}
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
	if _, err := os.Stat(filepath.Join(goDir, "ok")); !os.IsNotExist(err) {
		t.Error("directory 'ok' still exists after rmdir")
	}
	if _, err := os.Stat(filepath.Join(goDir, "bad")); err != nil {
		t.Error("directory 'bad' should still exist (non-empty)")
	}
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
