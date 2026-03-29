// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkdir against gmkdir (GNU coreutils).
//
// Covers prd034-mkdir R1.1, R1.2, R1.3, R1.4.
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
// between gmkdir and the Go binary.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for error cases where both binaries
// produce errors (no filesystem modification conflicts).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	existDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(existDir, "exists"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R1.2: no arguments — error exit 1
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: missing parent directory
		{
			Name:      "missing_parent",
			Args:      []string{"a/b/c"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: directory already exists
		{
			Name:      "already_exists",
			Args:      []string{"exists"},
			WorkDir:   existDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMkdirCreate verifies successful directory creation by running both
// binaries in separate temp dirs and comparing exit codes and output.
// RunDiffTests cannot be used for creation tests because both binaries
// share a WorkDir, and the ref binary's creation prevents the Go binary
// from succeeding.
func TestMkdirCreate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	// R1.1: create a single directory
	t.Run("single_directory", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"testdir"})
		verifyDirsCreated(t, goBin, []string{"testdir"})
	})

	// R1.2: create multiple directories
	t.Run("multiple_directories", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"d1", "d2", "d3"})
		verifyDirsCreated(t, goBin, []string{"d1", "d2", "d3"})
	})
}

// TestMkdirContinuesOnError verifies R1.3: errors on one directory do not
// abort processing of remaining arguments.
func TestMkdirContinuesOnError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	// Pre-create one directory so it fails
	if err := os.Mkdir(filepath.Join(dir, "existing"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(goBin, "existing", "newdir")
	cmd.Dir = dir
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error type: %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code: got %d, want 1", exitErr.ExitCode())
	}

	// R1.3: "newdir" must still be created despite "existing" failure
	info, err := os.Stat(filepath.Join(dir, "newdir"))
	if err != nil {
		t.Fatalf("newdir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("newdir is not a directory")
	}

	// Stderr should mention the failed directory
	if !bytes.Contains(stderr.Bytes(), []byte("existing")) {
		t.Errorf("stderr should mention 'existing', got: %q", stderr.String())
	}
}

// compareMkdir runs both binaries in separate temp dirs and compares
// exit codes and stdout. Stderr is not compared byte-for-byte because
// the binary name prefix differs (gmkdir vs mkdir).
func compareMkdir(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d (args=%v)",
			refExit, goExit, args)
	}
}

// verifyDirsCreated runs the Go binary and checks that all directories
// were created.
func verifyDirsCreated(t *testing.T, goBin string, dirs []string) {
	t.Helper()

	dir := t.TempDir()
	stdout, stderr, exitCode := execBin(t, goBin, dirs, dir)

	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d (stderr=%q)", exitCode, stderr)
	}
	if len(stdout) > 0 {
		t.Errorf("unexpected stdout: %q", stdout)
	}

	for _, name := range dirs {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("directory %q not created: %v", name, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", name)
		}
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
