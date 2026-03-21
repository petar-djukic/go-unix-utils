// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd057-mv R1.1–R1.4: basic move, rename,
// multi-file move, and error handling.
// Differential tests for prd057-mv R2.1–R2.4: overwrite control
// (interactive, force, no-clobber, permission errors).
// Tests for prd057-mv R3.1: verbose mode.
// Tests for prd057-mv R3.2: target directory (-t/--target-directory).
// Tests for prd057-mv R3.3: no-target-directory (-T/--no-target-directory).
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the binary name/path prefix in error
// messages with "mv" so that "gmv:" and "/path/to/mv:" both become "mv:".
func binaryNameNormalizer(b []byte) []byte {
	// Replace gmv with mv at line start
	re := regexp.MustCompile(`(?m)^(?:\S+/)?g?mv:`)
	b = re.ReplaceAll(b, []byte("mv:"))
	// Normalize "Try '...' for more information" lines
	reTry := regexp.MustCompile(`Try '[^']*' for more information\.`)
	b = reTry.ReplaceAll(b, []byte("Try 'mv --help' for more information."))
	return b
}

// errorCaseNormalizer normalizes error message casing differences
// between GNU (capitalized) and Go os package (lowercase).
func errorCaseNormalizer(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("No such file or directory"),
		[]byte("no such file or directory"))
	b = bytes.ReplaceAll(b, []byte("Not a directory"),
		[]byte("not a directory"))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skip("reference binary gmv not in PATH")
	}
	normalizers := []testutils.NormalizeFunc{
		binaryNameNormalizer,
		errorCaseNormalizer,
	}
	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: normalizers,
		},
		{
			Name:      "missing_dest",
			Args:      []string{"somefile"},
			ExitCode:  1,
			Normalize: normalizers,
		},
		missingSourceTest(t, normalizers),
		targetNotADirTest(t, normalizers),
		noClobberExistingTest(t, normalizers),
		interactiveDeclineTest(t, normalizers),
		verboseNoClobberTest(t, normalizers),
		verboseMissingSourceTest(t, normalizers),
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// missingSourceTest verifies error when source does not exist.
func missingSourceTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name: "missing_source",
		Args: []string{
			filepath.Join(dir, "nonexistent.txt"),
			filepath.Join(dir, "dst.txt"),
		},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// targetNotADirTest verifies error when multiple sources and target is not a dir.
func targetNotADirTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	// Create files that neither binary will successfully move,
	// since target (c.txt) is a regular file, not a directory.
	writeTestFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
	writeTestFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
	writeTestFile(t, filepath.Join(dir, "c.txt"), "ccc\n")
	return testutils.DiffTest{
		Name: "target_not_a_dir",
		Args: []string{
			filepath.Join(dir, "a.txt"),
			filepath.Join(dir, "b.txt"),
			filepath.Join(dir, "c.txt"),
		},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// noClobberExistingTest verifies -n does not overwrite existing dest. R2.3.
// Non-destructive: neither binary moves, state preserved for both runs.
func noClobberExistingTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "src.txt"), "source\n")
	writeTestFile(t, filepath.Join(dir, "dst.txt"), "dest\n")
	return testutils.DiffTest{
		Name: "no_clobber_existing_dest",
		Args: []string{
			"-n",
			filepath.Join(dir, "src.txt"),
			filepath.Join(dir, "dst.txt"),
		},
		ExitCode:  0,
		Normalize: normalizers,
	}
}

// interactiveDeclineTest verifies -i with "n" response skips overwrite. R2.1.
// Non-destructive: user declines, neither binary moves.
func interactiveDeclineTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "src.txt"), "source\n")
	writeTestFile(t, filepath.Join(dir, "dst.txt"), "dest\n")
	return testutils.DiffTest{
		Name: "interactive_decline",
		Args: []string{
			"-i",
			filepath.Join(dir, "src.txt"),
			filepath.Join(dir, "dst.txt"),
		},
		Stdin:     []byte("n\n"),
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// verboseNoClobberTest verifies -v -n with existing dest produces no output. R3.1.
// Non-destructive: no-clobber prevents any move.
func verboseNoClobberTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "src.txt"), "source\n")
	writeTestFile(t, filepath.Join(dir, "dst.txt"), "dest\n")
	return testutils.DiffTest{
		Name: "verbose_no_clobber",
		Args: []string{
			"-v", "-n",
			filepath.Join(dir, "src.txt"),
			filepath.Join(dir, "dst.txt"),
		},
		ExitCode:  0,
		Normalize: normalizers,
	}
}

// verboseMissingSourceTest verifies -v with missing source reports error. R3.1.
// Non-destructive: source doesn't exist, no move occurs.
func verboseMissingSourceTest(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name: "verbose_missing_source",
		Args: []string{
			"-v",
			filepath.Join(dir, "nonexistent.txt"),
			filepath.Join(dir, "dst.txt"),
		},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// TestMoveOps tests actual move operations using only the Go binary,
// since mv is destructive and the differential test framework runs
// both binaries sequentially in the same working directory.
func TestMoveOps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("single_file_rename", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "hello\n")
		runExpectSuccess(t, goBin, src, dst)
		assertFileContent(t, dst, "hello\n")
		assertNotExists(t, src)
	})

	t.Run("move_into_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "target")
		mkdirAll(t, subdir)
		src := filepath.Join(dir, "file.txt")
		writeTestFile(t, src, "content\n")
		runExpectSuccess(t, goBin, src, subdir)
		assertFileContent(t, filepath.Join(subdir, "file.txt"), "content\n")
		assertNotExists(t, src)
	})

	t.Run("multi_file_move_into_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "dest")
		mkdirAll(t, subdir)
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		writeTestFile(t, a, "aaa\n")
		writeTestFile(t, b, "bbb\n")
		runExpectSuccess(t, goBin, a, b, subdir)
		assertFileContent(t, filepath.Join(subdir, "a.txt"), "aaa\n")
		assertFileContent(t, filepath.Join(subdir, "b.txt"), "bbb\n")
		assertNotExists(t, a)
		assertNotExists(t, b)
	})

	t.Run("move_directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "srcdir")
		mkdirAll(t, srcDir)
		writeTestFile(t, filepath.Join(srcDir, "inner.txt"), "inner\n")
		dstDir := filepath.Join(dir, "dstdir")
		runExpectSuccess(t, goBin, srcDir, dstDir)
		assertFileContent(t, filepath.Join(dstDir, "inner.txt"), "inner\n")
		assertNotExists(t, srcDir)
	})

	t.Run("force_overwrite", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new content\n")
		writeTestFile(t, dst, "old content\n")
		runExpectSuccess(t, goBin, "-f", src, dst)
		assertFileContent(t, dst, "new content\n")
		assertNotExists(t, src)
	})

	t.Run("no_clobber_preserves", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new\n")
		writeTestFile(t, dst, "old\n")
		exitCode, _ := runBinaryCmd(t, goBin, nil, "-n", src, dst)
		requireExit(t, 0, exitCode)
		assertFileContent(t, dst, "old\n")
		assertFileExists(t, src)
	})

	t.Run("interactive_accept", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new\n")
		writeTestFile(t, dst, "old\n")
		exitCode, _ := runBinaryCmd(t, goBin, []byte("y\n"), "-i", src, dst)
		requireExit(t, 0, exitCode)
		assertFileContent(t, dst, "new\n")
		assertNotExists(t, src)
	})

	t.Run("interactive_decline", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new\n")
		writeTestFile(t, dst, "old\n")
		exitCode, _ := runBinaryCmd(t, goBin, []byte("n\n"), "-i", src, dst)
		requireExit(t, 1, exitCode)
		assertFileContent(t, dst, "old\n")
		assertFileExists(t, src)
	})

	t.Run("last_flag_fi_interactive_wins", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new\n")
		writeTestFile(t, dst, "old\n")
		// -f then -i: interactive wins, user declines
		exitCode, _ := runBinaryCmd(t, goBin, []byte("n\n"), "-f", "-i", src, dst)
		requireExit(t, 1, exitCode)
		assertFileContent(t, dst, "old\n")
		assertFileExists(t, src)
	})

	t.Run("last_flag_nf_force_wins", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new\n")
		writeTestFile(t, dst, "old\n")
		// -n then -f: force wins, overwrites
		exitCode, _ := runBinaryCmd(t, goBin, nil, "-n", "-f", src, dst)
		requireExit(t, 0, exitCode)
		assertFileContent(t, dst, "new\n")
		assertNotExists(t, src)
	})

	t.Run("last_flag_in_noclobber_wins", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "new\n")
		writeTestFile(t, dst, "old\n")
		// -i then -n: no-clobber wins, does not overwrite
		exitCode, _ := runBinaryCmd(t, goBin, nil, "-i", "-n", src, dst)
		requireExit(t, 0, exitCode)
		assertFileContent(t, dst, "old\n")
		assertFileExists(t, src)
	})
}

// TestVerbose tests -v/--verbose output. R3.1.
func TestVerbose(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("rename", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "hello\n")
		code, stdout, stderr := runBinarySplit(t, goBin, nil, "-v", src, dst)
		requireExit(t, 0, code)
		assertFileContent(t, dst, "hello\n")
		assertNotExists(t, src)
		want := fmt.Sprintf("renamed '%s' -> '%s'\n", src, dst)
		requireStrEqual(t, want, string(stderr), "verbose stderr")
		requireStrEqual(t, "", string(stdout), "verbose stdout")
	})

	t.Run("move_into_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "target")
		mkdirAll(t, subdir)
		src := filepath.Join(dir, "file.txt")
		writeTestFile(t, src, "content\n")
		code, _, stderr := runBinarySplit(t, goBin, nil, "--verbose", src, subdir)
		requireExit(t, 0, code)
		want := fmt.Sprintf("renamed '%s' -> '%s'\n",
			src, filepath.Join(subdir, "file.txt"))
		requireStrEqual(t, want, string(stderr), "verbose stderr")
	})
}

// TestTargetDirectory tests -t/--target-directory. R3.2.
func TestTargetDirectory(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("short_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "dest")
		mkdirAll(t, subdir)
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		writeTestFile(t, a, "aaa\n")
		writeTestFile(t, b, "bbb\n")
		runExpectSuccess(t, goBin, "-t", subdir, a, b)
		assertFileContent(t, filepath.Join(subdir, "a.txt"), "aaa\n")
		assertFileContent(t, filepath.Join(subdir, "b.txt"), "bbb\n")
		assertNotExists(t, a)
		assertNotExists(t, b)
	})

	t.Run("long_flag_equals", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "dest")
		mkdirAll(t, subdir)
		src := filepath.Join(dir, "src.txt")
		writeTestFile(t, src, "content\n")
		runExpectSuccess(t, goBin, "--target-directory="+subdir, src)
		assertFileContent(t, filepath.Join(subdir, "src.txt"), "content\n")
		assertNotExists(t, src)
	})

	t.Run("conflict_with_T", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		writeTestFile(t, src, "content\n")
		code, _ := runBinaryCmd(t, goBin, nil, "-t", dir, "-T", src)
		requireExit(t, 1, code)
	})
}

// TestNoTargetDirectory tests -T/--no-target-directory. R3.3.
func TestNoTargetDirectory(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("simple_rename", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		writeTestFile(t, src, "content\n")
		runExpectSuccess(t, goBin, "-T", src, dst)
		assertFileContent(t, dst, "content\n")
		assertNotExists(t, src)
	})

	t.Run("extra_operand", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		c := filepath.Join(dir, "c.txt")
		writeTestFile(t, a, "a\n")
		writeTestFile(t, b, "b\n")
		writeTestFile(t, c, "c\n")
		code, _ := runBinaryCmd(t, goBin, nil, "-T", a, b, c)
		requireExit(t, 1, code)
	})
}

// TestHelp verifies --help exits 0 with usage on stdout.
func TestHelp(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Fatalf("--help output missing Usage header: %s", out)
	}
}

// TestVersion verifies --version exits 0 with version on stdout.
func TestVersion(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !bytes.Contains(out, []byte("mv")) {
		t.Fatalf("--version output missing 'mv': %s", out)
	}
}

// runExpectSuccess runs the binary and fails the test if it exits non-zero.
func runExpectSuccess(t *testing.T, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success, got error: %v\noutput: %s", err, out)
	}
}

// runBinaryCmd runs the binary with optional stdin, returning exit code and output.
func runBinaryCmd(t *testing.T, bin string, stdinData []byte, args ...string) (int, []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if stdinData != nil {
		cmd.Stdin = bytes.NewReader(stdinData)
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, out
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), out
	}
	t.Fatalf("failed to run binary: %v", err)
	return 0, nil // unreachable
}

// runBinarySplit runs the binary capturing stdout and stderr separately.
func runBinarySplit(t *testing.T, bin string, stdinData []byte, args ...string) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if stdinData != nil {
		cmd.Stdin = bytes.NewReader(stdinData)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return 0, outBuf.Bytes(), errBuf.Bytes()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), outBuf.Bytes(), errBuf.Bytes()
	}
	t.Fatalf("failed to run binary: %v", err)
	return 0, nil, nil // unreachable
}

// requireExit fails the test if exit code doesn't match.
func requireExit(t *testing.T, want, got int) {
	t.Helper()
	if got != want {
		t.Fatalf("expected exit %d, got %d", want, got)
	}
}

// requireStrEqual fails the test if strings don't match.
func requireStrEqual(t *testing.T, want, got, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %q, want %q", label, got, want)
	}
}

// assertFileContent reads a file and checks it has the expected content.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("file %s: got %q, want %q", path, got, want)
	}
}

// assertNotExists checks that a file does not exist.
func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("expected %s to not exist, but it does", path)
	}
}

// assertFileExists checks that a file exists.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTestFile: %v", err)
	}
}

// mkdirAll creates a directory and all parents.
func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdirAll: %v", err)
	}
}
