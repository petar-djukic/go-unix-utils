// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkdir against gmkdir (GNU coreutils).
//
// Covers prd034-mkdir R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.4.
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

// normalizeProgName replaces any binary path prefix before ": " with
// "mkdir" so verbose messages from both binaries can be compared.
// GNU mkdir uses argv[0] (full path) as the prefix, e.g.,
// "/opt/homebrew/bin/gmkdir: created directory 'x'".
func normalizeProgName(data []byte) []byte {
	var result []byte
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if len(result) > 0 {
			result = append(result, '\n')
		}
		if idx := bytes.Index(line, []byte(": ")); idx >= 0 {
			result = append(result, []byte("mkdir")...)
			result = append(result, line[idx:]...)
		} else {
			result = append(result, line...)
		}
	}
	return result
}

// TestDiff runs differential tests for error cases and -p on existing dirs
// where both binaries can share a WorkDir without conflict.
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
		// R2.2: -p with existing directory — no error
		{
			Name:     "parents_existing_no_error",
			Args:     []string{"-p", "exists"},
			WorkDir:  existDir,
			ExitCode: 0,
		},
		// R2.2: --parents long form with existing directory
		{
			Name:     "parents_long_existing",
			Args:     []string{"--parents", "exists"},
			WorkDir:  existDir,
			ExitCode: 0,
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

// TestMkdirParents verifies -p/--parents directory creation.
func TestMkdirParents(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	// R2.1: create nested directory chain
	t.Run("nested_chain", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"-p", "a/b/c"})
		verifyNestedDirs(t, goBin, []string{"-p", "a/b/c"},
			[]string{"a", "a/b", "a/b/c"})
	})

	// R2.1: deep chain
	t.Run("deep_chain", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"-p", "x/y/z/w"})
		verifyNestedDirs(t, goBin, []string{"-p", "x/y/z/w"},
			[]string{"x", "x/y", "x/y/z", "x/y/z/w"})
	})

	// R2.2: existing target with -p is not an error
	t.Run("existing_target", func(t *testing.T) {
		t.Parallel()
		goDir, refDir := setupDirs(t, "exists")
		compareMkdirInDirs(t, goBin, refBin,
			[]string{"-p", "exists"}, goDir, refDir)
	})

	// R2.3: partial existing path — only missing dirs created
	t.Run("partial_existing", func(t *testing.T) {
		t.Parallel()
		goDir, refDir := setupDirs(t, "a")
		compareMkdirInDirs(t, goBin, refBin,
			[]string{"-p", "a/b/c"}, goDir, refDir)
	})

	// R2.1: multiple -p arguments
	t.Run("multiple_parents", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"-p", "p/q", "r/s"})
	})
}

// TestMkdirVerbose verifies -v/--verbose output.
func TestMkdirVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary gmkdir not in PATH")
	}

	// R3.4: verbose output for single directory
	t.Run("verbose_single", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"-v", "testdir"})
	})

	// R3.4: --verbose long form
	t.Run("verbose_long", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"--verbose", "testdir"})
	})

	// D3: -pv prints for each intermediate directory
	t.Run("parents_verbose", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"-pv", "a/b/c"})
	})

	// D3: -pv with partial existing path
	t.Run("parents_verbose_partial", func(t *testing.T) {
		t.Parallel()
		goDir, refDir := setupDirs(t, "a")
		compareMkdirInDirs(t, goBin, refBin,
			[]string{"-pv", "a/b/c"}, goDir, refDir)
	})

	// R2.2 + R3.4: -pv on existing dir produces no output
	t.Run("parents_verbose_existing", func(t *testing.T) {
		t.Parallel()
		goDir, refDir := setupDirs(t, "exists")
		compareMkdirInDirs(t, goBin, refBin,
			[]string{"-pv", "exists"}, goDir, refDir)
	})

	// R3.4: verbose with multiple directories
	t.Run("verbose_multiple", func(t *testing.T) {
		t.Parallel()
		compareMkdir(t, goBin, refBin, []string{"-v", "d1", "d2"})
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
// exit codes and normalized stdout.
func compareMkdir(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	compareMkdirInDirs(t, goBin, refBin, args, goDir, refDir)
}

// compareMkdirInDirs runs both binaries in given dirs and compares
// exit codes and normalized stdout.
func compareMkdirInDirs(t *testing.T, goBin, refBin string, args []string, goDir, refDir string) {
	t.Helper()

	refStdout, _, refExit := execBin(t, refBin, args, refDir)
	goStdout, _, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d (args=%v)",
			refExit, goExit, args)
	}

	refNorm := normalizeProgName(refStdout)
	goNorm := normalizeProgName(goStdout)
	if !bytes.Equal(refNorm, goNorm) {
		t.Errorf("stdout divergence:\nref: %q\ngo:  %q", string(refNorm), string(goNorm))
	}
}

// setupDirs creates pre-existing directories in both go and ref work dirs.
func setupDirs(t *testing.T, subdirs ...string) (string, string) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(goDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(refDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return goDir, refDir
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
		verifyIsDir(t, filepath.Join(dir, name))
	}
}

// verifyNestedDirs runs the Go binary with args and checks that all
// expected directories exist afterward.
func verifyNestedDirs(t *testing.T, goBin string, args, expectedDirs []string) {
	t.Helper()

	dir := t.TempDir()
	_, stderr, exitCode := execBin(t, goBin, args, dir)

	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d (stderr=%q)", exitCode, stderr)
	}

	for _, name := range expectedDirs {
		verifyIsDir(t, filepath.Join(dir, name))
	}
}

// verifyIsDir checks that the path exists and is a directory.
func verifyIsDir(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("directory %q not created: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", path)
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
