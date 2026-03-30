// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkfifo against gmkfifo (GNU coreutils).
//
// Covers prd092-mkfifo R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where the binary name prefix differs
// between gmkfifo and the Go binary.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for error cases where both binaries
// can share a WorkDir without conflict.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	existDir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(existDir, "exists"), 0o666); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R1.4: no arguments — error exit 1
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: path already exists
		{
			Name:      "already_exists",
			Args:      []string{"exists"},
			WorkDir:   existDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: parent directory does not exist
		{
			Name:      "missing_parent",
			Args:      []string{"no/such/path/fifo"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMkfifoCreate verifies successful FIFO creation by running both
// binaries in separate temp dirs and comparing exit codes.
// R1.1: create a single FIFO.
func TestMkfifoCreate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	// R1.1: create a single FIFO
	t.Run("single_fifo", func(t *testing.T) {
		t.Parallel()
		compareMkfifo(t, goBin, refBin, []string{"pipe1"}, []string{"pipe1"})
	})

	// R1.2: create multiple FIFOs
	t.Run("multiple_fifos", func(t *testing.T) {
		t.Parallel()
		compareMkfifo(t, goBin, refBin,
			[]string{"p1", "p2", "p3"},
			[]string{"p1", "p2", "p3"})
	})
}

// TestMkfifoMode verifies -m/--mode permission handling.
// R1.3: explicit mode sets permission bits.
func TestMkfifoMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary gmkfifo not in PATH")
	}

	// R1.3: default permissions match gmkfifo
	t.Run("default_perms", func(t *testing.T) {
		t.Parallel()
		compareMkfifo(t, goBin, refBin,
			[]string{"pipe1"}, []string{"pipe1"})
	})

	// R1.3: -m with octal mode
	t.Run("mode_0600", func(t *testing.T) {
		t.Parallel()
		compareMkfifo(t, goBin, refBin,
			[]string{"-m", "0600", "pipe1"}, []string{"pipe1"})
	})

	// R1.3: --mode=VALUE long form
	t.Run("mode_long_form", func(t *testing.T) {
		t.Parallel()
		compareMkfifo(t, goBin, refBin,
			[]string{"--mode=0644", "pipe1"}, []string{"pipe1"})
	})

	// R1.3: -m with bundled value
	t.Run("mode_bundled", func(t *testing.T) {
		t.Parallel()
		compareMkfifo(t, goBin, refBin,
			[]string{"-m0700", "pipe1"}, []string{"pipe1"})
	})
}

// TestMkfifoContinuesOnError verifies R1.4: errors on one FIFO do not
// abort processing of remaining arguments.
func TestMkfifoContinuesOnError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	// Pre-create a file so the first argument fails
	if err := os.WriteFile(filepath.Join(dir, "existing"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(goBin, "existing", "newpipe")
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

	// R1.4: "newpipe" must still be created despite "existing" failure
	info, err := os.Stat(filepath.Join(dir, "newpipe"))
	if err != nil {
		t.Fatalf("newpipe not created: %v", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Error("newpipe is not a FIFO")
	}

	// Stderr should mention the failed path
	if !bytes.Contains(stderr.Bytes(), []byte("existing")) {
		t.Errorf("stderr should mention 'existing', got: %q", stderr.String())
	}
}

// compareMkfifo runs both binaries in separate temp dirs and compares
// exit codes and verifies FIFOs were created with matching permissions.
func compareMkfifo(t *testing.T, goBin, refBin string, args, checkNames []string) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	refStdout, refStderr, refExit := execBin(t, refBin, args, refDir)
	goStdout, goStderr, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d (args=%v)", refExit, goExit, args)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout:\nref: %q\ngo:  %q", string(refStdout), string(goStdout))
	}
	// Normalize stderr by discarding (binary name differs)
	_ = refStderr
	_ = goStderr

	for _, name := range checkNames {
		compareFIFO(t, filepath.Join(refDir, name), filepath.Join(goDir, name), name)
	}
}

// compareFIFO checks that both paths are FIFOs with matching permissions.
func compareFIFO(t *testing.T, refPath, goPath, name string) {
	t.Helper()
	refInfo, err := os.Stat(refPath)
	if err != nil {
		t.Fatalf("ref fifo %s: %v", name, err)
	}
	goInfo, err := os.Stat(goPath)
	if err != nil {
		t.Fatalf("go fifo %s: %v", name, err)
	}
	if refInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("ref %s is not a FIFO", name)
	}
	if goInfo.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("go %s is not a FIFO", name)
	}
	if refInfo.Mode().Perm() != goInfo.Mode().Perm() {
		t.Errorf("perm %s: ref=%o go=%o",
			name, refInfo.Mode().Perm(), goInfo.Mode().Perm())
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

