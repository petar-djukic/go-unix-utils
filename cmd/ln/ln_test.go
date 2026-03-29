// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln against gln (GNU coreutils).
//
// Covers prd037-ln R1.1, R1.2, R1.3, R1.4.
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
// Used for error messages where the binary name prefix differs.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for error cases via RunDiffTests.
// R1.3: hard link to directory fails.
// R1.4: existing destination without -f fails.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	workDir := t.TempDir()

	// Create a directory for R1.3 test.
	if err := os.Mkdir(filepath.Join(workDir, "somedir"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create files for R1.4 test (existing destination).
	writeFile(t, filepath.Join(workDir, "src.txt"), "source")
	writeFile(t, filepath.Join(workDir, "existing.txt"), "existing")

	tests := []testutils.DiffTest{
		// No arguments — exit 1.
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: hard link to directory — exit 1.
		{
			Name:      "hard_link_directory",
			Args:      []string{"somedir", "newlink"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: existing destination without -f — exit 1.
		{
			Name:      "existing_destination",
			Args:      []string{"src.txt", "existing.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHardLinkCreation verifies R1.1: ln TARGET LINK_NAME creates a hard link.
func TestHardLinkCreation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("single_hard_link", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"target.txt", "link.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"target.txt", "link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "target.txt", "link.txt")
		assertHardLink(t, refDir, "target.txt", "link.txt")
	})
}

// TestHardLinkIntoDirectory verifies R1.2: ln TARGET... DIRECTORY.
func TestHardLinkIntoDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("multiple_targets_into_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "a.txt"), "aaa")
			writeFile(t, filepath.Join(base, "b.txt"), "bbb")
			if err := os.Mkdir(filepath.Join(base, "dest"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"a.txt", "b.txt", "dest"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"a.txt", "b.txt", "dest"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "a.txt", filepath.Join("dest", "a.txt"))
		assertHardLink(t, goDir, "b.txt", filepath.Join("dest", "b.txt"))
		assertHardLink(t, refDir, "a.txt", filepath.Join("dest", "a.txt"))
		assertHardLink(t, refDir, "b.txt", filepath.Join("dest", "b.txt"))
	})

	t.Run("single_target_into_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "src.txt"), "data")
			if err := os.Mkdir(filepath.Join(base, "outdir"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"src.txt", "outdir"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"src.txt", "outdir"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "src.txt", filepath.Join("outdir", "src.txt"))
		assertHardLink(t, refDir, "src.txt", filepath.Join("outdir", "src.txt"))
	})
}

// TestNonExistentTarget verifies error on non-existent target.
func TestNonExistentTarget(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	_, _, refExit := execBin(t, refBin, []string{"nonexistent", "newlink"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"nonexistent", "newlink"}, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if goExit == 0 {
		t.Error("expected non-zero exit for non-existent target")
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertHardLink checks that two paths share the same inode (are hard links).
func assertHardLink(t *testing.T, base, file1, file2 string) {
	t.Helper()
	info1, err := os.Stat(filepath.Join(base, file1))
	if err != nil {
		t.Fatalf("stat %s: %v", file1, err)
	}
	info2, err := os.Stat(filepath.Join(base, file2))
	if err != nil {
		t.Fatalf("stat %s: %v", file2, err)
	}
	if !os.SameFile(info1, info2) {
		t.Errorf("%s and %s are not hard links (different inodes)", file1, file2)
	}
}
