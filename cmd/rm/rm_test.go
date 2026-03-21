// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd058-rm R1.1–R1.4: basic file removal,
// directory refusal, dot/dotdot refusal, and error continuation.
// Tests for prd058-rm R2.1–R2.4: recursive removal, force mode, -d flag.
// Tests for prd058-rm R3.3: -v/--verbose flag.
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
// messages with "rm" so that "grm:" and "/path/to/rm:" both become "rm:".
func binaryNameNormalizer(b []byte) []byte {
	re := regexp.MustCompile(`(?m)^(?:\S+/)?g?rm:`)
	b = re.ReplaceAll(b, []byte("rm:"))
	reTry := regexp.MustCompile(`Try '[^']*' for more information\.`)
	b = reTry.ReplaceAll(b, []byte("Try 'rm --help' for more information."))
	return b
}

// errorCaseNormalizer normalizes error message casing differences
// between GNU (capitalized) and Go os package (lowercase).
func errorCaseNormalizer(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("No such file or directory"),
		[]byte("no such file or directory"))
	b = bytes.ReplaceAll(b, []byte("Not a directory"),
		[]byte("not a directory"))
	b = bytes.ReplaceAll(b, []byte("Is a directory"),
		[]byte("is a directory"))
	b = bytes.ReplaceAll(b, []byte("Directory not empty"),
		[]byte("directory not empty"))
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not in PATH")
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
		forceNoArgs(normalizers),
		forceNonexistent(t, normalizers),
		removeNonexistent(t, normalizers),
		removeDirWithoutR(t, normalizers),
		removeDotDir(t, normalizers),
		removeDotDotDir(t, normalizers),
		removeDirWithoutROrD(t, normalizers),
		removeDNonEmptyDir(t, normalizers),
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// forceNoArgs tests that -f with no files exits 0 silently.
func forceNoArgs(normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	return testutils.DiffTest{
		Name:      "force_no_args",
		Args:      []string{"-f"},
		ExitCode:  0,
		Normalize: normalizers,
	}
}

// forceNonexistent tests -f on a nonexistent file exits 0.
func forceNonexistent(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name:      "force_nonexistent",
		Args:      []string{"-f", filepath.Join(dir, "no_such_file.txt")},
		ExitCode:  0,
		Normalize: normalizers,
	}
}

// removeNonexistent tests removing a nonexistent file without -f.
func removeNonexistent(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name:      "remove_nonexistent",
		Args:      []string{filepath.Join(dir, "no_such_file.txt")},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// removeDirWithoutR tests removing a directory without -r flag. R1.2.
func removeDirWithoutR(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	subdir := filepath.Join(dir, "mydir")
	mkdirAll(t, subdir)
	return testutils.DiffTest{
		Name:      "remove_dir_without_r",
		Args:      []string{subdir},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// removeDotDir tests refusing to remove ".". R1.3.
func removeDotDir(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name:      "remove_dot",
		Args:      []string{filepath.Join(dir, ".")},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// removeDotDotDir tests refusing to remove "..". R1.3.
func removeDotDotDir(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	return testutils.DiffTest{
		Name:      "remove_dotdot",
		Args:      []string{filepath.Join(dir, "..")},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// removeDirWithoutROrD tests that a directory fails without -r or -d. R2.4.
func removeDirWithoutROrD(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	subdir := filepath.Join(dir, "emptydir")
	mkdirAll(t, subdir)
	return testutils.DiffTest{
		Name:      "dir_without_r_or_d",
		Args:      []string{subdir},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// removeDNonEmptyDir tests -d on a non-empty directory (should fail). R2.4.
func removeDNonEmptyDir(t *testing.T, normalizers []testutils.NormalizeFunc) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	subdir := filepath.Join(dir, "notempty")
	mkdirAll(t, subdir)
	writeTestFile(t, filepath.Join(subdir, "file.txt"), "data\n")
	return testutils.DiffTest{
		Name:      "d_nonempty_dir",
		Args:      []string{"-d", subdir},
		ExitCode:  1,
		Normalize: normalizers,
	}
}

// TestRemoveOps tests actual removal operations using only the Go binary,
// since rm is destructive and the differential test framework runs
// both binaries sequentially in the same working directory.
func TestRemoveOps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f := filepath.Join(dir, "file.txt")
		writeTestFile(t, f, "content\n")
		runExpectSuccess(t, goBin, f)
		assertNotExists(t, f)
	})

	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		writeTestFile(t, a, "aaa\n")
		writeTestFile(t, b, "bbb\n")
		runExpectSuccess(t, goBin, a, b)
		assertNotExists(t, a)
		assertNotExists(t, b)
	})

	t.Run("force_nonexistent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		code, _ := runBinaryCmd(t, goBin, "-f", filepath.Join(dir, "nope"))
		requireExit(t, 0, code)
	})

	t.Run("error_continues", func(t *testing.T) {
		t.Parallel()
		// R1.4: error on first file, still removes second
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.txt")
		exists := filepath.Join(dir, "exists.txt")
		writeTestFile(t, exists, "content\n")
		code, _ := runBinaryCmd(t, goBin, missing, exists)
		requireExit(t, 1, code)
		assertNotExists(t, exists) // second file still removed
	})

	t.Run("dir_refused_without_r", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "mydir")
		mkdirAll(t, subdir)
		code, _ := runBinaryCmd(t, goBin, subdir)
		requireExit(t, 1, code)
		assertFileExists(t, subdir) // directory still exists
	})
}

// TestRecursiveOps tests -r recursive removal using only the Go binary. R2.1.
func TestRecursiveOps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("recursive_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "top", "mid", "bot")
		mkdirAll(t, sub)
		writeTestFile(t, filepath.Join(sub, "file.txt"), "data\n")
		writeTestFile(t, filepath.Join(dir, "top", "root.txt"), "root\n")
		top := filepath.Join(dir, "top")
		runExpectSuccess(t, goBin, "-r", top)
		assertNotExists(t, top)
	})

	t.Run("recursive_uppercase_R", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "d1")
		mkdirAll(t, sub)
		writeTestFile(t, filepath.Join(sub, "f.txt"), "x\n")
		runExpectSuccess(t, goBin, "-R", sub)
		assertNotExists(t, sub)
	})

	t.Run("recursive_long_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "d1")
		mkdirAll(t, sub)
		writeTestFile(t, filepath.Join(sub, "f.txt"), "x\n")
		runExpectSuccess(t, goBin, "--recursive", sub)
		assertNotExists(t, sub)
	})

	t.Run("recursive_force_nonexistent", func(t *testing.T) {
		t.Parallel()
		// R2.3: -rf on nonexistent exits 0
		dir := t.TempDir()
		code, _ := runBinaryCmd(t, goBin, "-rf",
			filepath.Join(dir, "nonexistent"))
		requireExit(t, 0, code)
	})

	t.Run("recursive_force_tree", func(t *testing.T) {
		t.Parallel()
		// R2.3: -rf removes tree silently
		dir := t.TempDir()
		sub := filepath.Join(dir, "tree")
		mkdirAll(t, filepath.Join(sub, "a", "b"))
		writeTestFile(t, filepath.Join(sub, "a", "b", "c.txt"), "c\n")
		writeTestFile(t, filepath.Join(sub, "a", "x.txt"), "x\n")
		code, out := runBinaryCmd(t, goBin, "-rf", sub)
		requireExit(t, 0, code)
		assertNotExists(t, sub)
		if len(out) > 0 {
			t.Fatalf("expected no output with -rf, got: %s", out)
		}
	})
}

// TestDirFlag tests -d/--dir empty directory removal. R2.4.
func TestDirFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("empty_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "empty")
		mkdirAll(t, sub)
		runExpectSuccess(t, goBin, "-d", sub)
		assertNotExists(t, sub)
	})

	t.Run("nonempty_dir_fails", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "notempty")
		mkdirAll(t, sub)
		writeTestFile(t, filepath.Join(sub, "f.txt"), "data\n")
		code, _ := runBinaryCmd(t, goBin, "-d", sub)
		requireExit(t, 1, code)
		assertFileExists(t, sub)
	})

	t.Run("long_flag_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "empty2")
		mkdirAll(t, sub)
		runExpectSuccess(t, goBin, "--dir", sub)
		assertNotExists(t, sub)
	})
}

// TestVerboseRecursive tests -rv verbose output during recursive removal.
func TestVerboseRecursive(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("recursive_verbose", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		sub := filepath.Join(dir, "d")
		mkdirAll(t, sub)
		f := filepath.Join(sub, "f.txt")
		writeTestFile(t, f, "data\n")
		code, stdout, stderr := runBinarySplit(t, goBin, "-rv", sub)
		requireExit(t, 0, code)
		assertNotExists(t, sub)
		// Should mention both file and directory
		wantFile := fmt.Sprintf("removed '%s'\n", f)
		wantDir := fmt.Sprintf("removed directory '%s'\n", sub)
		if string(stdout) != wantFile+wantDir {
			t.Fatalf("verbose stdout: got %q, want %q",
				string(stdout), wantFile+wantDir)
		}
		requireStrEqual(t, "", string(stderr), "verbose stderr")
	})
}

// TestVerbose tests -v/--verbose output. R3.3.
func TestVerbose(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f := filepath.Join(dir, "file.txt")
		writeTestFile(t, f, "content\n")
		code, stdout, stderr := runBinarySplit(t, goBin, "-v", f)
		requireExit(t, 0, code)
		assertNotExists(t, f)
		want := fmt.Sprintf("removed '%s'\n", f)
		requireStrEqual(t, want, string(stdout), "verbose stdout")
		requireStrEqual(t, "", string(stderr), "verbose stderr")
	})

	t.Run("multiple_files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		writeTestFile(t, a, "aaa\n")
		writeTestFile(t, b, "bbb\n")
		code, stdout, _ := runBinarySplit(t, goBin, "-v", a, b)
		requireExit(t, 0, code)
		wantA := fmt.Sprintf("removed '%s'\n", a)
		wantB := fmt.Sprintf("removed '%s'\n", b)
		requireStrEqual(t, wantA+wantB, string(stdout), "verbose stdout")
	})

	t.Run("long_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f := filepath.Join(dir, "file.txt")
		writeTestFile(t, f, "content\n")
		code, stdout, _ := runBinarySplit(t, goBin, "--verbose", f)
		requireExit(t, 0, code)
		want := fmt.Sprintf("removed '%s'\n", f)
		requireStrEqual(t, want, string(stdout), "verbose stdout")
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
	if !bytes.Contains(out, []byte("rm")) {
		t.Fatalf("--version output missing 'rm': %s", out)
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

// runBinaryCmd runs the binary returning exit code and combined output.
func runBinaryCmd(t *testing.T, bin string, args ...string) (int, []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
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
func runBinarySplit(t *testing.T, bin string, args ...string) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
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
