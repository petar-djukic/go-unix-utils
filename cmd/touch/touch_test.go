// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/touch against gtouch (GNU coreutils).
//
// Covers prd062-touch R1.1, R1.2, R1.3, R1.4.
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

// TestDiff runs differential tests for cases where both binaries can
// share a WorkDir without conflict (error cases, no-create).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R1.3: -c with nonexistent file — no file created, exit 0
	t.Run("no_create_nonexistent", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:     "no_create_short",
				Args:     []string{"-c", "nonexistent"},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.3: --no-create long form
	t.Run("no_create_long", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:     "no_create_long",
				Args:     []string{"--no-create", "nonexistent"},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: touch existing file updates timestamps, exit 0
	t.Run("update_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		existing := filepath.Join(dir, "existing")
		if err := os.WriteFile(existing, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		tests := []testutils.DiffTest{
			{
				Name:     "update_existing",
				Args:     []string{existing},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// Missing operand — error exit 1
	t.Run("missing_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "no_args",
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{discardAll},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestTouchCreate verifies file creation by running both binaries
// in separate temp dirs and comparing exit codes and file existence.
// R1.2: create file when it does not exist.
func TestTouchCreate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary gtouch not in PATH")
	}

	// R1.2: create a single new file
	t.Run("create_single", func(t *testing.T) {
		t.Parallel()
		compareTouch(t, goBin, refBin, []string{"newfile"})
		verifyFilesCreated(t, goBin, []string{"newfile"})
	})

	// R1.4: create multiple files
	t.Run("create_multiple", func(t *testing.T) {
		t.Parallel()
		compareTouch(t, goBin, refBin, []string{"a", "b", "c"})
		verifyFilesCreated(t, goBin, []string{"a", "b", "c"})
	})
}

// TestTouchNoCreateVerify confirms -c does not create files.
// R1.3: -c suppresses creation without error.
func TestTouchNoCreateVerify(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	target := filepath.Join(dir, "should_not_exist")

	_, stderr, exitCode := execBin(t, goBin, []string{"-c", target}, dir)
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d (stderr=%q)", exitCode, stderr)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("file should not have been created with -c")
	}
}

// compareTouch runs both binaries in separate temp dirs and compares
// exit codes and stdout/stderr.
func compareTouch(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	refStdout, refStderr, refExit := execBin(t, refBin, args, refDir)
	goStdout, goStderr, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d (args=%v)",
			refExit, goExit, args)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout divergence:\nref: %q\ngo:  %q", refStdout, goStdout)
	}
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr divergence:\nref: %q\ngo:  %q", refStderr, goStderr)
	}
}

// verifyFilesCreated runs the Go binary and checks files were created.
func verifyFilesCreated(t *testing.T, goBin string, names []string) {
	t.Helper()
	dir := t.TempDir()

	_, stderr, exitCode := execBin(t, goBin, names, dir)
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d (stderr=%q)", exitCode, stderr)
	}

	for _, name := range names {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("file %q not created: %v", name, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("%q should be a file, not a directory", name)
		}
		if info.Size() != 0 {
			t.Errorf("%q should be empty, got size %d", name, info.Size())
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
