// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unlink against gunlink (GNU coreutils).
//
// Covers prd038-unlink R1.1, R1.2, R1.3, R2.1.
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
// between gunlink and the Go binary.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests using the testutils harness.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skip("reference binary gunlink not in PATH")
	}

	workDir := t.TempDir()

	tests := []testutils.DiffTest{
		// R2.1: no arguments — error exit 1.
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Extra operand — error exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"a", "b"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// Non-existent file — error exit 1.
		{
			Name:      "nonexistent_file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestUnlinkFile verifies R1.1, R1.2, R1.3: successful removal of a regular file.
func TestUnlinkFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skip("reference binary gunlink not in PATH")
	}

	t.Run("regular_file", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "content")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"target.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"target.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		// R1.1: file must no longer exist.
		assertRemoved(t, goDir, "target.txt")
	})

	t.Run("symbolic_link", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "real.txt"), "data")
			if err := os.Symlink("real.txt", filepath.Join(base, "link.txt")); err != nil {
				t.Fatal(err)
			}
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"link.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		// Symlink removed, target remains.
		assertRemoved(t, goDir, "link.txt")
		assertExists(t, goDir, "real.txt")
	})
}

// TestUnlinkDirectory verifies that unlinking a directory fails.
func TestUnlinkDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skip("reference binary gunlink not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, base := range []string{goDir, refDir} {
		if err := os.Mkdir(filepath.Join(base, "somedir"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	_, _, refExit := execBin(t, refBin, []string{"somedir"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"somedir"}, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if goExit == 0 {
		t.Error("expected non-zero exit for directory argument")
	}
	// Directory must still exist.
	assertExists(t, goDir, "somedir")
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRemoved(t *testing.T, base, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(base, name)); !os.IsNotExist(err) {
		t.Errorf("%q should have been removed", name)
	}
}

func assertExists(t *testing.T, base, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(base, name)); err != nil {
		t.Errorf("%q should still exist: %v", name, err)
	}
}
