// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unlink against gunlink (GNU coreutils).
//
// Covers prd038-unlink R1.1, R1.2, R1.3, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
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
// R3.2: covers zero-argument error, multi-argument error, non-existent file error,
// and directory argument error via RunDiffTests.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skip("reference binary gunlink not in PATH")
	}

	workDir := t.TempDir()

	// R3.2: directory argument error — create a directory in workDir.
	if err := os.Mkdir(filepath.Join(workDir, "somedir"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R3.2: zero-argument error — exit 1.
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: multi-argument error — exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"a", "b"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: non-existent file error — exit 1.
		{
			Name:      "nonexistent_file",
			Args:      []string{"nonexistent.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: directory argument error — exit 1.
		{
			Name:      "directory_argument",
			Args:      []string{"somedir"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestUnlinkFile verifies R3.2 (regular file and symbolic link removal) and
// R3.3 (file no longer exists after successful invocation).
func TestUnlinkFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skip("reference binary gunlink not in PATH")
	}

	// R3.2: successful removal of a regular file.
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
		// R3.3: file must no longer exist after successful invocation.
		assertRemoved(t, goDir, "target.txt")
		assertRemoved(t, refDir, "target.txt")
	})

	// R3.2: successful removal of a symbolic link.
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
		// R3.3: symlink must no longer exist after successful invocation.
		assertRemoved(t, goDir, "link.txt")
		assertRemoved(t, refDir, "link.txt")
		// Symlink target must still exist.
		assertExists(t, goDir, "real.txt")
		assertExists(t, refDir, "real.txt")
	})
}

// TestUnlinkDirectory verifies R2.4 and R3.2: unlinking a directory fails.
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
