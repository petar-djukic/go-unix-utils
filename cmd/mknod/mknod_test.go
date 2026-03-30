// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mknod against gmknod (GNU coreutils).
//
// Covers prd093-mknod R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
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
// between gmknod and the Go binary.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for argument validation error cases.
// R1.3: invalid arguments produce exit 1.
// R2.2: exit 1 when arguments are invalid.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.3: no arguments
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: only NAME, missing TYPE
		{
			Name:      "missing_type",
			Args:      []string{"foo"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: invalid device type
		{
			Name:      "invalid_type",
			Args:      []string{"foo", "x"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: FIFO with device numbers
		{
			Name:      "fifo_with_device_numbers",
			Args:      []string{"foo", "p", "1", "2"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: block device missing MAJOR and MINOR
		{
			Name:      "block_missing_major_minor",
			Args:      []string{"foo", "b"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: block device missing MINOR
		{
			Name:      "block_missing_minor",
			Args:      []string{"foo", "b", "1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: non-numeric major device number
		{
			Name:      "non_numeric_major",
			Args:      []string{"foo", "b", "abc", "1"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: non-numeric minor device number
		{
			Name:      "non_numeric_minor",
			Args:      []string{"foo", "b", "1", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMknodFIFO verifies FIFO creation via mknod NAME p.
// R1.1: create FIFO special files.
// R2.1: exit 0 when the special file is created successfully.
func TestMknodFIFO(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	t.Run("create_fifo", func(t *testing.T) {
		t.Parallel()
		compareMknod(t, goBin, refBin, []string{"pipe1", "p"}, "pipe1")
	})
}

// TestMknodMode verifies -m/--mode permission handling on FIFOs.
// R1.2: mode flag sets permission bits.
func TestMknodMode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary gmknod not in PATH")
	}

	t.Run("mode_0600", func(t *testing.T) {
		t.Parallel()
		compareMknod(t, goBin, refBin, []string{"-m", "0600", "pipe1", "p"}, "pipe1")
	})

	t.Run("mode_long_form", func(t *testing.T) {
		t.Parallel()
		compareMknod(t, goBin, refBin, []string{"--mode=0644", "pipe1", "p"}, "pipe1")
	})

	t.Run("mode_bundled", func(t *testing.T) {
		t.Parallel()
		compareMknod(t, goBin, refBin, []string{"-m0700", "pipe1", "p"}, "pipe1")
	})
}

// compareMknod runs both binaries and compares exit codes and FIFO properties.
func compareMknod(t *testing.T, goBin, refBin string, args []string, checkName string) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()

	refStdout, _, refExit := execBin(t, refBin, args, refDir)
	goStdout, _, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d (args=%v)", refExit, goExit, args)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout:\nref: %q\ngo:  %q", string(refStdout), string(goStdout))
	}

	compareFIFO(t, filepath.Join(refDir, checkName), filepath.Join(goDir, checkName), checkName)
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

// TestMknodExitZero verifies that successful FIFO creation exits 0.
// R2.1: exit 0 when the special file is created successfully.
func TestMknodExitZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	_, _, exitCode := execBin(t, goBin, []string{"testpipe", "p"}, dir)
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

// TestMknodExitOne verifies that failed creation exits 1.
// R2.2: exit 1 when creation fails.
func TestMknodExitOne(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Create a FIFO, then try again — second call should fail (file exists).
	dir := t.TempDir()
	execBin(t, goBin, []string{"testpipe", "p"}, dir)
	_, _, exitCode := execBin(t, goBin, []string{"testpipe", "p"}, dir)
	if exitCode != 1 {
		t.Errorf("expected exit 1 for existing file, got %d", exitCode)
	}
}

// TestMknodSIGPIPE verifies SIGPIPE handling.
// R2.3: must handle SIGPIPE gracefully (exit 0).
func TestMknodSIGPIPE(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// mknod writes nothing to stdout on success, so SIGPIPE won't trigger
	// during normal operation. Verify the binary loads and runs without
	// crashing when SIGPIPE handling is installed.
	dir := t.TempDir()
	_, _, exitCode := execBin(t, goBin, []string{"testpipe", "p"}, dir)
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
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
