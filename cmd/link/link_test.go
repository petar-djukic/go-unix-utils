// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/link against glink (GNU coreutils).
//
// Covers prd084-link R1.1, R1.2, R1.3, R1.4, R2.1, R2.2.
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

// TestDiff runs differential tests using the testutils harness.
// R1.3: covers missing-operand and extra-operand errors.
// R1.4: covers nonexistent source file error.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skip("reference binary glink not in PATH")
	}

	workDir := t.TempDir()

	tests := []testutils.DiffTest{
		// R1.3: no arguments — exit 1.
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: one argument — missing destination operand — exit 1.
		{
			Name:      "missing_destination",
			Args:      []string{"file1"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: three arguments — extra operand — exit 1.
		{
			Name:      "extra_operand",
			Args:      []string{"a", "b", "c"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: nonexistent source file — exit 1.
		{
			Name:      "nonexistent_source",
			Args:      []string{"nonexistent.txt", "dest.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestLinkCreate verifies R1.1 and R1.2: successful hard link creation.
// R2.1: exit 0 on success.
func TestLinkCreate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skip("reference binary glink not in PATH")
	}

	t.Run("create_hard_link", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "source.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"source.txt", "dest.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"source.txt", "dest.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}

		// R1.1: dest.txt must exist and be a hard link to source.txt.
		assertHardLink(t, goDir, "source.txt", "dest.txt")
		assertHardLink(t, refDir, "source.txt", "dest.txt")
	})
}

// TestLinkDestExists verifies R1.4: destination already exists — exit 1.
func TestLinkDestExists(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skip("reference binary glink not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, base := range []string{goDir, refDir} {
		writeFile(t, filepath.Join(base, "source.txt"), "data")
		writeFile(t, filepath.Join(base, "dest.txt"), "exists")
	}

	_, _, refExit := execBin(t, refBin, []string{"source.txt", "dest.txt"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"source.txt", "dest.txt"}, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if goExit == 0 {
		t.Error("expected non-zero exit when destination exists")
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

// assertHardLink verifies two files share the same inode.
func assertHardLink(t *testing.T, base, name1, name2 string) {
	t.Helper()

	info1, err := os.Stat(filepath.Join(base, name1))
	if err != nil {
		t.Fatalf("%q should exist: %v", name1, err)
	}
	info2, err := os.Stat(filepath.Join(base, name2))
	if err != nil {
		t.Fatalf("%q should exist: %v", name2, err)
	}

	if !os.SameFile(info1, info2) {
		t.Errorf("%q and %q should be hard links (same inode)", name1, name2)
	}
}
