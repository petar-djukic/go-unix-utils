// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd058-rm R1.1-R1.4, R2.2, R3.3 differential tests
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
