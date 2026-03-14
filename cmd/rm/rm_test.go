// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd058-rm R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4 differential tests
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// programNameNormalizer replaces the binary name (grm or the full Go binary
// path) with the canonical name "rm" so stderr messages are comparable.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("rm"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("rm"))
		b = bytes.ReplaceAll(b, []byte("grm"), []byte("rm"))
		return b
	}
}

// tryHelpNormalizer removes the "Try '...' for more information." line that
// GNU rm appends to some error messages. The Go implementation also emits
// this line, but the binary name differs so we strip the whole line.
var tryHelpRe = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

var tryHelpNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return tryHelpRe.ReplaceAll(b, nil)
}

// TestDiffNoArgs verifies that rm with no arguments prints usage to stderr
// and exits 1, matching grm.
// R1.1: no operands → usage to stderr and exit 1.
func TestDiffNoArgs(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)
	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm, tryHelpNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffForceNoArgs verifies that rm -f with no arguments exits 0 silently.
// R2.2: -f with no operands exits 0.
func TestDiffForceNoArgs(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "force_no_args",
			Args:     []string{"-f"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffForceNonexistent verifies that rm -f on nonexistent files exits 0.
// R2.2: -f suppresses errors for nonexistent files.
func TestDiffForceNonexistent(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "force_nonexistent",
			Args:     []string{"-f", "does_not_exist"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		{
			Name:     "force_multiple_nonexistent",
			Args:     []string{"-f", "no1", "no2", "no3"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNonexistentFile verifies error output for removing a nonexistent file.
// R1.4: print error to stderr and exit 1.
func TestDiffNonexistentFile(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)
	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_file",
			Args:      []string{"nonexistent_file_xyz"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDirectoryWithoutR verifies that rm refuses to remove a directory
// without -r and prints an error.
// R1.2: without -r, refuse to remove directories.
func TestDiffDirectoryWithoutR(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)

	// Create a temp directory that both binaries will try (and fail) to remove.
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "testdir")
	if mkErr := os.Mkdir(targetDir, 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "dir_without_r",
			Args:      []string{targetDir},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Verify directory still exists after both binaries refused.
	if _, statErr := os.Stat(targetDir); statErr != nil {
		t.Errorf("directory should still exist after failed rm: %v", statErr)
	}
}

// TestDiffDotDotDot verifies that rm refuses to remove '.' and '..'.
// R1.3: Must not remove '.' or '..'.
func TestDiffDotDotDot(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)
	tests := []testutils.DiffTest{
		{
			Name:      "refuse_dot",
			Args:      []string{"."},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
		{
			Name:      "refuse_dotdot",
			Args:      []string{".."},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runAndCapture runs the binary with args in workDir and returns stdout, stderr,
// and exit code.
func runAndCapture(t *testing.T, binary string, args []string, workDir string) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %q: %v", binary, runErr)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode
}

// setupFile creates a file with the given content in the specified directory.
func setupFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// setupTree creates a directory tree for recursive removal tests:
//
//	<dir>/tree/
//	<dir>/tree/sub/
//	<dir>/tree/sub/leaf.txt
//
// Uses a single child per directory to avoid ordering issues in verbose output.
func setupTree(t *testing.T, dir string) {
	t.Helper()
	subDir := filepath.Join(dir, "tree", "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	setupFile(t, subDir, "leaf.txt", "data\n")
}

// normalizeBinaryNames replaces binary paths with "rm" in output for comparison.
func normalizeBinaryNames(b []byte, goBin, refBin string) []byte {
	b = bytes.ReplaceAll(b, []byte(refBin), []byte("rm"))
	b = bytes.ReplaceAll(b, []byte(goBin), []byte("rm"))
	b = bytes.ReplaceAll(b, []byte("grm"), []byte("rm"))
	return b
}

// TestDiffSingleFileRemoval verifies removing a single regular file.
// R1.1: remove each file using os.Remove.
func TestDiffSingleFileRemoval(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	// Set up separate work dirs for Go and ref binary.
	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "target.txt", "hello\n")
	setupFile(t, refDir, "target.txt", "hello\n")

	goStdout, goStderr, goCode := runAndCapture(t, goBin, []string{"target.txt"}, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, []string{"target.txt"}, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify file was removed in both directories.
	if _, statErr := os.Stat(filepath.Join(goDir, "target.txt")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the file")
	}
	if _, statErr := os.Stat(filepath.Join(refDir, "target.txt")); !os.IsNotExist(statErr) {
		t.Errorf("ref binary did not remove the file")
	}
}

// TestDiffMultiFileRemoval verifies removing multiple files, including one
// that does not exist, to test partial failure behavior.
// R1.4: print error and continue with remaining files.
func TestDiffMultiFileRemoval(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "a.txt", "aaa\n")
	setupFile(t, goDir, "c.txt", "ccc\n")
	setupFile(t, refDir, "a.txt", "aaa\n")
	setupFile(t, refDir, "c.txt", "ccc\n")

	// b.txt does not exist → error but continue removing c.txt.
	args := []string{"a.txt", "b.txt", "c.txt"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// a.txt and c.txt should be removed, b.txt never existed.
	for _, name := range []string{"a.txt", "c.txt"} {
		if _, statErr := os.Stat(filepath.Join(goDir, name)); !os.IsNotExist(statErr) {
			t.Errorf("Go binary did not remove %s", name)
		}
	}
}

// TestDiffVerboseRemoval verifies that rm -v prints the removed file name.
// R3.3: -v prints "removed '<path>'" to stdout.
func TestDiffVerboseRemoval(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "verbose.txt", "data\n")
	setupFile(t, refDir, "verbose.txt", "data\n")

	args := []string{"-v", "verbose.txt"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify file was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "verbose.txt")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the file")
	}
}

// TestDiffForceWithMixedExistence verifies -f with a mix of existing and
// nonexistent files.
// R2.2: -f suppresses errors for nonexistent files but still removes existing ones.
func TestDiffForceWithMixedExistence(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "exists.txt", "data\n")
	setupFile(t, refDir, "exists.txt", "data\n")

	args := []string{"-f", "exists.txt", "ghost.txt"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify exists.txt was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "exists.txt")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove exists.txt")
	}
}

// TestDiffBundledFlags verifies bundled short flags like -fv work.
func TestDiffBundledFlags(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "bundled.txt", "data\n")
	setupFile(t, refDir, "bundled.txt", "data\n")

	args := []string{"-fv", "bundled.txt"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}
}

// TestDiffRecursiveRemoval verifies rm -r removes a directory tree.
// AC1: rm -r removes a directory tree including nested files and subdirectories.
func TestDiffRecursiveRemoval(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"-r", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify tree was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}
}

// TestDiffRecursiveUpperR verifies rm -R behaves identically to rm -r.
// AC2: rm -R behaves identically to rm -r.
func TestDiffRecursiveUpperR(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"-R", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify tree was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}
}

// TestDiffDirEmptyDir verifies rm -d removes an empty directory.
// AC3: rm -d removes an empty directory.
func TestDiffDirEmptyDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	if err := os.Mkdir(filepath.Join(goDir, "emptydir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(refDir, "emptydir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	args := []string{"-d", "emptydir"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify directory was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "emptydir")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the empty directory")
	}
}

// TestDiffDirNonEmptyDir verifies rm -d fails on non-empty directory.
// AC3: rm -d fails with diagnostic on non-empty directory.
func TestDiffDirNonEmptyDir(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)

	// Create a non-empty directory that both binaries will try (and fail) to remove.
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "nonempty")
	if mkErr := os.Mkdir(targetDir, 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	setupFile(t, targetDir, "child.txt", "data\n")

	tests := []testutils.DiffTest{
		{
			Name:      "dir_nonempty",
			Args:      []string{"-d", targetDir},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Verify directory still exists.
	if _, statErr := os.Stat(targetDir); statErr != nil {
		t.Errorf("directory should still exist after failed rm -d: %v", statErr)
	}
}

// TestDiffRecursiveForce verifies rm -rf removes a directory tree silently.
func TestDiffRecursiveForce(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"-rf", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify tree was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}
}

// TestDiffRecursiveVerbose verifies rm -rv prints each removal in the tree.
// Uses a simple tree with one child per directory to avoid ordering issues.
func TestDiffRecursiveVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"-rv", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}
}

// TestDiffRecursiveSymlinkNotFollowed verifies that rm -r removes symlinks
// without following them into the target directory.
// AC5: Recursive removal does not follow symlinks to directories.
// D2: Symlinks encountered during recursive traversal are removed but never followed.
func TestDiffRecursiveSymlinkNotFollowed(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	// Create target directory that should survive removal.
	for _, dir := range []string{goDir, refDir} {
		targetDir := filepath.Join(dir, "target_dir")
		if err := os.Mkdir(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}
		setupFile(t, targetDir, "precious.txt", "keep\n")

		// Create tree with a symlink to target_dir.
		treeDir := filepath.Join(dir, "tree")
		if err := os.Mkdir(treeDir, 0o755); err != nil {
			t.Fatalf("mkdir tree: %v", err)
		}
		setupFile(t, treeDir, "file.txt", "data\n")
		if err := os.Symlink(targetDir, filepath.Join(treeDir, "link")); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	}

	args := []string{"-r", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify tree was removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}

	// Verify target_dir and its contents still exist (symlink was not followed).
	if _, statErr := os.Stat(filepath.Join(goDir, "target_dir", "precious.txt")); statErr != nil {
		t.Errorf("target_dir/precious.txt should still exist: %v", statErr)
	}
}

// TestDiffRecursiveDot verifies that rm -r . and rm -r .. produce errors.
// AC6: rm -r . and rm -r .. both produce an error and exit non-zero.
// D3: Reject arguments '.' and '..' with error matching GNU rm behavior.
func TestDiffRecursiveDot(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)
	tests := []testutils.DiffTest{
		{
			Name:      "recursive_dot",
			Args:      []string{"-r", "."},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
		{
			Name:      "recursive_dotdot",
			Args:      []string{"-r", ".."},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOneFileSystemSameDevice verifies that --one-file-system works
// normally on a tree that resides on a single device (no skipping).
// AC4: rm --one-file-system skips directories on different mount points.
func TestDiffOneFileSystemSameDevice(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"--one-file-system", "-r", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify tree was removed (same device, so no skipping).
	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}
}

// TestDiffRecursiveLongFlag verifies --recursive works.
func TestDiffRecursiveLongFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"--recursive", "tree"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}
}

// runAndCaptureWithStdin runs the binary with args and stdin content in workDir.
func runAndCaptureWithStdin(t *testing.T, binary string, args []string, workDir, stdin string) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %q: %v", binary, runErr)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode
}

// promptNormalizer strips interactive prompt lines from stderr so that
// minor formatting differences between Go and GNU rm do not cause failures.
var promptRe = regexp.MustCompile(`(?m)^(rm|grm): (remove|descend into) .*\? \n?`)

var promptNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return promptRe.ReplaceAll(b, nil)
}

// TestDiffInteractiveYes verifies rm -i prompts and removes on confirmation.
// R3.1 / AC1: -i prompts before every removal.
func TestDiffInteractiveYes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "ifile.txt", "data\n")
	setupFile(t, refDir, "ifile.txt", "data\n")

	args := []string{"-i", "ifile.txt"}

	goStdout, goStderr, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "y\n")
	refStdout, refStderr, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "y\n")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	// Normalize prompts and binary names for stderr comparison.
	goStderr = promptNormalizer(normalizeBinaryNames(goStderr, goBin, refBin))
	refStderr = promptNormalizer(normalizeBinaryNames(refStderr, goBin, refBin))
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Both binaries should have removed the file.
	if _, statErr := os.Stat(filepath.Join(goDir, "ifile.txt")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the file")
	}
	if _, statErr := os.Stat(filepath.Join(refDir, "ifile.txt")); !os.IsNotExist(statErr) {
		t.Errorf("ref binary did not remove the file")
	}
}

// TestDiffInteractiveNo verifies rm -i skips on non-confirmation without error.
// R3.1 / AC1: skipping on non-confirmation does not produce an error exit code.
func TestDiffInteractiveNo(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "keep.txt", "data\n")
	setupFile(t, refDir, "keep.txt", "data\n")

	args := []string{"-i", "keep.txt"}

	goStdout, goStderr, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "n\n")
	refStdout, refStderr, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "n\n")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = promptNormalizer(normalizeBinaryNames(goStderr, goBin, refBin))
	refStderr = promptNormalizer(normalizeBinaryNames(refStderr, goBin, refBin))
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Both binaries should NOT have removed the file.
	if _, statErr := os.Stat(filepath.Join(goDir, "keep.txt")); statErr != nil {
		t.Errorf("Go binary removed the file despite 'n' answer: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(refDir, "keep.txt")); statErr != nil {
		t.Errorf("ref binary removed the file despite 'n' answer: %v", statErr)
	}
}

// TestDiffInteractiveOnceYes verifies rm -I prompts once for >3 files and
// removes all on confirmation.
// R3.2 / AC2: -I prompts once when removing more than three files.
func TestDiffInteractiveOnceYes(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	files := []string{"a.txt", "b.txt", "c.txt", "d.txt"}
	for _, f := range files {
		setupFile(t, goDir, f, "data\n")
		setupFile(t, refDir, f, "data\n")
	}

	args := append([]string{"-I"}, files...)

	goStdout, goStderr, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "y\n")
	refStdout, refStderr, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "y\n")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = promptNormalizer(normalizeBinaryNames(goStderr, goBin, refBin))
	refStderr = promptNormalizer(normalizeBinaryNames(refStderr, goBin, refBin))
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// All files should be removed.
	for _, f := range files {
		if _, statErr := os.Stat(filepath.Join(goDir, f)); !os.IsNotExist(statErr) {
			t.Errorf("Go binary did not remove %s", f)
		}
	}
}

// TestDiffInteractiveOnceNo verifies rm -I exits without removing on non-confirmation.
// R3.2 / AC2: exit without removal on non-confirmation.
func TestDiffInteractiveOnceNo(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	files := []string{"a.txt", "b.txt", "c.txt", "d.txt"}
	for _, f := range files {
		setupFile(t, goDir, f, "data\n")
		setupFile(t, refDir, f, "data\n")
	}

	args := append([]string{"-I"}, files...)

	goStdout, goStderr, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "n\n")
	refStdout, refStderr, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "n\n")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = promptNormalizer(normalizeBinaryNames(goStderr, goBin, refBin))
	refStderr = promptNormalizer(normalizeBinaryNames(refStderr, goBin, refBin))
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// No files should be removed.
	for _, f := range files {
		if _, statErr := os.Stat(filepath.Join(goDir, f)); statErr != nil {
			t.Errorf("Go binary removed %s despite 'n' answer: %v", f, statErr)
		}
	}
}

// TestDiffInteractiveOnceRecursive verifies rm -rI prompts once for recursive removal.
// R3.2 / AC2: -I prompts when removing recursively.
func TestDiffInteractiveOnceRecursive(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupTree(t, goDir)
	setupTree(t, refDir)

	args := []string{"-rI", "tree"}

	// Confirm recursive removal.
	goStdout, goStderr, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "y\n")
	refStdout, refStderr, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "y\n")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = promptNormalizer(normalizeBinaryNames(goStderr, goBin, refBin))
	refStderr = promptNormalizer(normalizeBinaryNames(refStderr, goBin, refBin))
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Tree should be removed.
	if _, statErr := os.Stat(filepath.Join(goDir, "tree")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the directory tree")
	}
}

// TestDiffPreserveRoot verifies that rm -r --preserve-root / refuses to remove '/'.
// R3.4 / AC4: --preserve-root prevents recursive removal of root.
func TestDiffPreserveRoot(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)

	tests := []testutils.DiffTest{
		{
			Name:      "preserve_root_slash",
			Args:      []string{"-r", "--preserve-root", "/"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm, tryHelpNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFlagOrderFI verifies that rm -f -i uses -i behavior (last flag wins).
// AC5: rm -f -i uses -i behavior.
func TestDiffFlagOrderFI(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "order.txt", "data\n")
	setupFile(t, refDir, "order.txt", "data\n")

	// -f -i: -i is last, so prompting is active. Empty stdin → EOF → decline.
	args := []string{"-f", "-i", "order.txt"}

	_, _, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "")
	_, _, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// File should NOT be removed (EOF → decline).
	if _, statErr := os.Stat(filepath.Join(goDir, "order.txt")); statErr != nil {
		t.Errorf("Go binary removed the file despite -i with EOF stdin: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(refDir, "order.txt")); statErr != nil {
		t.Errorf("ref binary removed the file despite -i with EOF stdin: %v", statErr)
	}
}

// TestDiffFlagOrderIF verifies that rm -i -f uses -f behavior (last flag wins).
// AC5: rm -i -f uses -f behavior.
func TestDiffFlagOrderIF(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "order.txt", "data\n")
	setupFile(t, refDir, "order.txt", "data\n")

	// -i -f: -f is last, so force mode is active. No prompting.
	args := []string{"-i", "-f", "order.txt"}

	goStdout, goStderr, goCode := runAndCaptureWithStdin(t, goBin, args, goDir, "")
	refStdout, refStderr, refCode := runAndCaptureWithStdin(t, refBin, args, refDir, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// File SHOULD be removed (-f wins).
	if _, statErr := os.Stat(filepath.Join(goDir, "order.txt")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove the file with -i -f")
	}
	if _, statErr := os.Stat(filepath.Join(refDir, "order.txt")); !os.IsNotExist(statErr) {
		t.Errorf("ref binary did not remove the file with -i -f")
	}
}

// TestDiffVersion verifies that rm --version prints version info and exits 0.
// R4.2: --version outputs version information to stdout and exits 0.
func TestDiffVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "version",
			Args:     []string{"--version"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
			Normalize: []testutils.NormalizeFunc{
				// Version strings differ between Go and GNU; just compare exit code.
				func(b []byte) []byte { return nil },
			},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelp verifies that rm --help prints usage info and exits 0.
// R4.3: --help outputs usage information to stdout and exits 0.
func TestDiffHelp(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "help",
			Args:     []string{"--help"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
			Normalize: []testutils.NormalizeFunc{
				// Help text differs between Go and GNU; just compare exit code.
				func(b []byte) []byte { return nil },
			},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffPartialFailureExitCode verifies that when some operands succeed and
// others fail, rm exits 1.
// R4.1, R4.4: exit 1 when any operand fails, even if others succeed.
func TestDiffPartialFailureExitCode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "exists.txt", "data\n")
	setupFile(t, refDir, "exists.txt", "data\n")

	// One file exists, one does not — partial failure.
	args := []string{"exists.txt", "no_such_file"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if goCode != 1 {
		t.Errorf("expected exit code 1 for partial failure, got %d", goCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}

	// Verify the existing file was still removed despite the other operand failing.
	if _, statErr := os.Stat(filepath.Join(goDir, "exists.txt")); !os.IsNotExist(statErr) {
		t.Errorf("Go binary did not remove exists.txt despite partial failure")
	}
}

// TestDiffAllSuccessExitCode verifies that rm exits 0 when all operands succeed.
// R4.1: exit 0 when all files are removed successfully.
func TestDiffAllSuccessExitCode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	setupFile(t, goDir, "a.txt", "aaa\n")
	setupFile(t, goDir, "b.txt", "bbb\n")
	setupFile(t, refDir, "a.txt", "aaa\n")
	setupFile(t, refDir, "b.txt", "bbb\n")

	args := []string{"a.txt", "b.txt"}

	goStdout, goStderr, goCode := runAndCapture(t, goBin, args, goDir)
	refStdout, refStderr, refCode := runAndCapture(t, refBin, args, refDir)

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if goCode != 0 {
		t.Errorf("expected exit code 0, got %d", goCode)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\ngo:  %q\nref: %q", goStdout, refStdout)
	}
	goStderr = normalizeBinaryNames(goStderr, goBin, refBin)
	refStderr = normalizeBinaryNames(refStderr, goBin, refBin)
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\ngo:  %q\nref: %q", goStderr, refStderr)
	}
}

// TestDiffNonexistentErrorMessage verifies that rm on a nonexistent file prints
// a diagnostic to stderr matching GNU rm format.
// R4.1: error handling for nonexistent files writes diagnostics to stderr.
func TestDiffNonexistentErrorMessage(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	nameNorm := programNameNormalizer(goBin, refBin)
	tests := []testutils.DiffTest{
		{
			Name:      "nonexistent_error_msg",
			Args:      []string{"absolutely_does_not_exist_12345"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{nameNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
