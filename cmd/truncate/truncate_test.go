// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/truncate against gtruncate (GNU coreutils).
//
// Covers prd083-truncate R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2, R3.3.
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

// TestDiff runs differential tests for non-file-modifying cases.
// Covers --help, --version, and error conditions (R3.1, R3.2).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	workDir := t.TempDir()

	// R1.4: create a file for -c tests; a nonexistent path is tested below.
	noCreateTarget := filepath.Join(workDir, "no-such-file")

	tests := []testutils.DiffTest{
		// R3.2: missing --size and --reference
		{
			Name:      "no_size_or_reference",
			Args:      []string{"somefile"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: missing file operand
		{
			Name:      "missing_file_operand",
			Args:      []string{"-s", "100"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R3.2: invalid size value
		{
			Name:      "invalid_size",
			Args:      []string{"-s", "xyz", "somefile"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: -c with nonexistent file — no error, no file created
		{
			Name:    "no_create_nonexistent",
			Args:    []string{"-c", "-s", "100", noCreateTarget},
			WorkDir: workDir,
		},
		// R2.2: reference to nonexistent file — exit 1
		{
			Name:      "reference_nonexistent",
			Args:      []string{"-r", filepath.Join(workDir, "nonexistent-ref"), "somefile"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies --help prints output and exits 0.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--help produced no output")
	}
}

// TestVersion verifies --version prints output and exits 0.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--version produced no output")
	}
}

// TestAbsoluteSize verifies -s with absolute byte value (R1.1).
func TestAbsoluteSize(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "target", 0)
	setupFile(t, refDir, "target", 0)

	_, _, refExit := execBin(t, refBin, []string{"-s", "100", "target"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"-s", "100", "target"}, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "target"), 100)
	assertFileSize(t, filepath.Join(refDir, "target"), 100)
}

// TestRelativeGrow verifies -s +N grows file (R1.2).
func TestRelativeGrow(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "target", 50)
	setupFile(t, refDir, "target", 50)

	_, _, refExit := execBin(t, refBin, []string{"-s", "+50", "target"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"-s", "+50", "target"}, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "target"), 100)
	assertFileSize(t, filepath.Join(refDir, "target"), 100)
}

// TestRelativeShrink verifies -s -N shrinks file (R1.2).
func TestRelativeShrink(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "target", 100)
	setupFile(t, refDir, "target", 100)

	_, _, refExit := execBin(t, refBin, []string{"-s", "-30", "target"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"-s", "-30", "target"}, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "target"), 70)
	assertFileSize(t, filepath.Join(refDir, "target"), 70)
}

// TestReferenceFile verifies -r uses reference file size (R2.1).
func TestReferenceFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "ref", 200)
	setupFile(t, goDir, "target", 0)
	setupFile(t, refDir, "ref", 200)
	setupFile(t, refDir, "target", 0)

	_, _, refExit := execBin(t, refBin, []string{"-r", "ref", "target"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"-r", "ref", "target"}, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "target"), 200)
	assertFileSize(t, filepath.Join(refDir, "target"), 200)
}

// TestReferenceWithRelative verifies -r combined with -s (R2.1 + R1.2).
func TestReferenceWithRelative(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "ref", 100)
	setupFile(t, goDir, "target", 0)
	setupFile(t, refDir, "ref", 100)
	setupFile(t, refDir, "target", 0)

	_, _, refExit := execBin(t, refBin, []string{"-r", "ref", "-s", "+25", "target"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"-r", "ref", "-s", "+25", "target"}, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "target"), 125)
	assertFileSize(t, filepath.Join(refDir, "target"), 125)
}

// TestMultipleFiles verifies multiple FILE operands (R1.3).
func TestMultipleFiles(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		setupFile(t, goDir, name, 0)
		setupFile(t, refDir, name, 0)
	}

	args := []string{"-s", "64", "a", "b", "c"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	for _, name := range []string{"a", "b", "c"} {
		assertFileSize(t, filepath.Join(goDir, name), 64)
		assertFileSize(t, filepath.Join(refDir, name), 64)
	}
}

// TestNoCreateExisting verifies -c does not affect existing files (R1.4).
func TestNoCreateExisting(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "exists", 50)
	setupFile(t, refDir, "exists", 50)

	args := []string{"-c", "-s", "200", "exists"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "exists"), 200)
	assertFileSize(t, filepath.Join(refDir, "exists"), 200)
}

// TestCreateMissing verifies missing files are created without -c (R1.4).
func TestCreateMissing(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	args := []string{"-s", "80", "newfile"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "newfile"), 80)
	assertFileSize(t, filepath.Join(refDir, "newfile"), 80)
}

// TestAtMost verifies -s '<N' sets at most N bytes (R1.2).
func TestAtMost(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "big", 200)
	setupFile(t, refDir, "big", 200)

	args := []string{"-s", "<100", "big"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "big"), 100)
	assertFileSize(t, filepath.Join(refDir, "big"), 100)
}

// TestAtLeast verifies -s '>N' sets at least N bytes (R1.2).
func TestAtLeast(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "small", 30)
	setupFile(t, refDir, "small", 30)

	args := []string{"-s", ">100", "small"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "small"), 100)
	assertFileSize(t, filepath.Join(refDir, "small"), 100)
}

// TestRoundDown verifies -s '/N' rounds down to multiple (R1.2).
func TestRoundDown(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "target", 150)
	setupFile(t, refDir, "target", 150)

	args := []string{"-s", "/64", "target"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	// 150 / 64 = 2 * 64 = 128
	assertFileSize(t, filepath.Join(goDir, "target"), 128)
	assertFileSize(t, filepath.Join(refDir, "target"), 128)
}

// TestRoundUp verifies -s '%N' rounds up to multiple (R1.2).
func TestRoundUp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "target", 150)
	setupFile(t, refDir, "target", 150)

	args := []string{"-s", "%64", "target"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	// ceil(150/64)*64 = 3*64 = 192
	assertFileSize(t, filepath.Join(goDir, "target"), 192)
	assertFileSize(t, filepath.Join(refDir, "target"), 192)
}

// TestSizeWithSuffix verifies unit suffix parsing (R1.1).
func TestSizeWithSuffix(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtruncate")
	if err != nil {
		t.Skip("reference binary gtruncate not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	setupFile(t, goDir, "target", 0)
	setupFile(t, refDir, "target", 0)

	args := []string{"-s", "1K", "target"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	assertExitMatch(t, refExit, goExit)
	assertFileSize(t, filepath.Join(goDir, "target"), 1024)
	assertFileSize(t, filepath.Join(refDir, "target"), 1024)
}

// setupFile creates a file of the given size in dir.
func setupFile(t *testing.T, dir, name string, size int64) {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		if err := f.Truncate(size); err != nil {
			f.Close() // best-effort close
			t.Fatal(err)
		}
	}
	f.Close() // best-effort close
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

// assertExitMatch verifies ref and Go exit codes are equal.
func assertExitMatch(t *testing.T, refExit, goExit int) {
	t.Helper()
	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d", refExit, goExit)
	}
}

// assertFileSize verifies the file at path has the expected size.
func assertFileSize(t *testing.T, path string, expected int64) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if fi.Size() != expected {
		t.Errorf("file %q size = %d, want %d", filepath.Base(path), fi.Size(), expected)
	}
}
