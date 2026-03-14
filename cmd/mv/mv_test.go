// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd057-mv R1.1-R1.4 differential tests
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

// programNameNormalizer replaces the binary name (gmv or the full Go binary
// path) with the canonical name "mv" so stderr messages are comparable.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("mv"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("mv"))
		b = bytes.ReplaceAll(b, []byte("gmv"), []byte("mv"))
		return b
	}
}

// tryHelpNormalizer removes the "Try 'mv --help'..." line that GNU mv appends
// to some error messages. The Go implementation does not emit this line.
var tryHelpRe = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

var tryHelpNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return tryHelpRe.ReplaceAll(b, nil)
}

// runMvAndCapture runs the binary with args in workDir and returns stdout, stderr, exit code.
func runMvAndCapture(t *testing.T, binary string, args []string, workDir string) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %q: %v", binary, err)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode
}

// setupFile creates a file with the given content.
func setupFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// setupDir creates a directory.
func setupDir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// TestDiffSingleFileRename verifies single-file rename produces identical state.
// R1.1: rename SOURCE to DEST on same filesystem.
func TestDiffSingleFileRename(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("rename_regular_file", func(t *testing.T) {
		t.Parallel()

		// Run reference binary in its own dir.
		refDir := t.TempDir()
		setupFile(t, filepath.Join(refDir, "src.txt"), "hello\n")
		refStdout, refStderr, refCode := runMvAndCapture(t, refBin,
			[]string{filepath.Join(refDir, "src.txt"), filepath.Join(refDir, "dst.txt")}, "")

		// Run Go binary in its own dir.
		goDir := t.TempDir()
		setupFile(t, filepath.Join(goDir, "src.txt"), "hello\n")
		goStdout, goStderr, goCode := runMvAndCapture(t, goBin,
			[]string{filepath.Join(goDir, "src.txt"), filepath.Join(goDir, "dst.txt")}, "")

		if refCode != goCode {
			t.Errorf("exit code: ref=%d go=%d", refCode, goCode)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}

		norm := programNameNormalizer(goBin, refBin)
		if !bytes.Equal(norm(refStderr), norm(goStderr)) {
			t.Errorf("stderr: ref=%q go=%q", refStderr, goStderr)
		}

		// Verify filesystem state.
		goContent, readErr := os.ReadFile(filepath.Join(goDir, "dst.txt"))
		if readErr != nil {
			t.Errorf("destination file not created: %v", readErr)
			return
		}
		refContent, _ := os.ReadFile(filepath.Join(refDir, "dst.txt"))
		if !bytes.Equal(refContent, goContent) {
			t.Errorf("file content: ref=%q go=%q", refContent, goContent)
		}
		if _, statErr := os.Stat(filepath.Join(goDir, "src.txt")); !os.IsNotExist(statErr) {
			t.Errorf("source file still exists after mv")
		}
	})
}

// TestDiffMoveIntoDirectory verifies moving files into a directory.
// R1.2, R1.4: move SOURCE into existing DEST directory.
func TestDiffMoveIntoDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("move_file_into_dir", func(t *testing.T) {
		t.Parallel()

		// Reference.
		refDir := t.TempDir()
		setupFile(t, filepath.Join(refDir, "file.txt"), "content\n")
		setupDir(t, filepath.Join(refDir, "subdir"))
		_, _, refCode := runMvAndCapture(t, refBin,
			[]string{filepath.Join(refDir, "file.txt"), filepath.Join(refDir, "subdir")}, "")

		// Go binary.
		goDir := t.TempDir()
		setupFile(t, filepath.Join(goDir, "file.txt"), "content\n")
		setupDir(t, filepath.Join(goDir, "subdir"))
		_, _, goCode := runMvAndCapture(t, goBin,
			[]string{filepath.Join(goDir, "file.txt"), filepath.Join(goDir, "subdir")}, "")

		if refCode != goCode {
			t.Errorf("exit code: ref=%d go=%d", refCode, goCode)
		}

		data, readErr := os.ReadFile(filepath.Join(goDir, "subdir", "file.txt"))
		if readErr != nil {
			t.Errorf("file not moved into directory: %v", readErr)
			return
		}
		if string(data) != "content\n" {
			t.Errorf("content mismatch: got %q", data)
		}
		if _, statErr := os.Stat(filepath.Join(goDir, "file.txt")); !os.IsNotExist(statErr) {
			t.Errorf("source file still exists after mv into dir")
		}
	})

	t.Run("move_multiple_files_into_dir", func(t *testing.T) {
		t.Parallel()

		goDir := t.TempDir()
		setupFile(t, filepath.Join(goDir, "a.txt"), "aaa\n")
		setupFile(t, filepath.Join(goDir, "b.txt"), "bbb\n")
		setupDir(t, filepath.Join(goDir, "dest"))

		_, _, goCode := runMvAndCapture(t, goBin,
			[]string{filepath.Join(goDir, "a.txt"), filepath.Join(goDir, "b.txt"),
				filepath.Join(goDir, "dest")}, "")

		if goCode != 0 {
			t.Errorf("exit code: got %d, want 0", goCode)
		}
		for _, name := range []string{"a.txt", "b.txt"} {
			if _, statErr := os.Stat(filepath.Join(goDir, "dest", name)); statErr != nil {
				t.Errorf("file %s not in dest dir: %v", name, statErr)
			}
		}
		for _, name := range []string{"a.txt", "b.txt"} {
			if _, statErr := os.Stat(filepath.Join(goDir, name)); !os.IsNotExist(statErr) {
				t.Errorf("source %s still exists", name)
			}
		}
	})
}

// TestDiffNoClobber verifies -n prevents overwriting.
// R2.3: -n no-clobber mode.
func TestDiffNoClobber(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("no_clobber_existing", func(t *testing.T) {
		t.Parallel()

		// Reference.
		refDir := t.TempDir()
		setupFile(t, filepath.Join(refDir, "src.txt"), "new\n")
		setupFile(t, filepath.Join(refDir, "dst.txt"), "old\n")
		_, _, refCode := runMvAndCapture(t, refBin,
			[]string{"-n", filepath.Join(refDir, "src.txt"), filepath.Join(refDir, "dst.txt")}, "")

		// Go binary.
		goDir := t.TempDir()
		setupFile(t, filepath.Join(goDir, "src.txt"), "new\n")
		setupFile(t, filepath.Join(goDir, "dst.txt"), "old\n")
		_, _, goCode := runMvAndCapture(t, goBin,
			[]string{"-n", filepath.Join(goDir, "src.txt"), filepath.Join(goDir, "dst.txt")}, "")

		if refCode != goCode {
			t.Errorf("exit code: ref=%d go=%d", refCode, goCode)
		}

		// Destination should keep original content.
		data, _ := os.ReadFile(filepath.Join(goDir, "dst.txt"))
		if string(data) != "old\n" {
			t.Errorf("no-clobber failed: dst content=%q, want %q", data, "old\n")
		}
		// Source should still exist.
		if _, statErr := os.Stat(filepath.Join(goDir, "src.txt")); statErr != nil {
			t.Errorf("source was removed despite -n: %v", statErr)
		}
	})
}

// TestDiffForceOverwrite verifies -f (default) overwrites without prompting.
// R2.2: -f force mode.
func TestDiffForceOverwrite(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	_, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("force_overwrite", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFile(t, filepath.Join(dir, "src.txt"), "new content\n")
		setupFile(t, filepath.Join(dir, "dst.txt"), "old content\n")

		_, _, code := runMvAndCapture(t, goBin,
			[]string{"-f", filepath.Join(dir, "src.txt"), filepath.Join(dir, "dst.txt")}, "")
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}

		data, readErr := os.ReadFile(filepath.Join(dir, "dst.txt"))
		if readErr != nil {
			t.Fatalf("destination not readable: %v", readErr)
		}
		if string(data) != "new content\n" {
			t.Errorf("force overwrite failed: got %q", data)
		}
	})
}

// TestDiffVerbose verifies -v prints move operation.
// R3.1: verbose output.
func TestDiffVerbose(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("verbose_rename", func(t *testing.T) {
		t.Parallel()

		norm := programNameNormalizer(goBin, refBin)
		pathNorm := func(dir string) testutils.NormalizeFunc {
			return func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(dir), []byte("/tmp/test"))
			}
		}

		// Reference.
		refDir := t.TempDir()
		setupFile(t, filepath.Join(refDir, "src.txt"), "data\n")
		refStdout, _, refCode := runMvAndCapture(t, refBin,
			[]string{"-v", filepath.Join(refDir, "src.txt"), filepath.Join(refDir, "dst.txt")}, "")

		// Go binary.
		goDir := t.TempDir()
		setupFile(t, filepath.Join(goDir, "src.txt"), "data\n")
		goStdout, _, goCode := runMvAndCapture(t, goBin,
			[]string{"-v", filepath.Join(goDir, "src.txt"), filepath.Join(goDir, "dst.txt")}, "")

		if refCode != goCode {
			t.Errorf("exit code: ref=%d go=%d", refCode, goCode)
		}

		// Normalize paths and binary names before comparison.
		normRef := norm(pathNorm(refDir)(refStdout))
		normGo := norm(pathNorm(goDir)(goStdout))
		if !bytes.Equal(normRef, normGo) {
			t.Errorf("verbose stdout:\nref: %q\ngo:  %q", normRef, normGo)
		}
	})
}

// TestDiffErrorCases verifies error handling matches GNU mv.
// R4.2: exit 1 on failure. These tests do not mutate filesystem state so
// RunDiffTests is safe.
func TestDiffErrorCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	norm := programNameNormalizer(goBin, refBin)
	tests := []testutils.DiffTest{
		{
			Name:      "missing_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{norm, tryHelpNormalizer},
		},
		{
			Name:      "missing_dest_operand",
			Args:      []string{"single_file"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{norm, tryHelpNormalizer},
		},
		{
			Name:      "nonexistent_source",
			Args:      []string{"/tmp/nonexistent_mv_test_file_xyz", "/tmp/target_mv_test_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{norm, tryHelpNormalizer},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffMoveDirectory verifies directory move without -r flag.
// R1.3: mv moves directories without requiring a recursive flag.
func TestDiffMoveDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	_, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("move_directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "srcdir")
		dstDir := filepath.Join(dir, "dstdir")
		setupDir(t, srcDir)
		setupFile(t, filepath.Join(srcDir, "inner.txt"), "inside\n")

		_, _, code := runMvAndCapture(t, goBin, []string{srcDir, dstDir}, "")
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}

		data, readErr := os.ReadFile(filepath.Join(dstDir, "inner.txt"))
		if readErr != nil {
			t.Errorf("inner file not found after move: %v", readErr)
			return
		}
		if string(data) != "inside\n" {
			t.Errorf("inner file content mismatch: got %q", data)
		}
		if _, statErr := os.Stat(srcDir); !os.IsNotExist(statErr) {
			t.Errorf("source directory still exists after mv")
		}
	})
}

// TestDiffTargetDirectory verifies -t flag.
// R3.2: -t DIRECTORY moves all sources into DIRECTORY.
func TestDiffTargetDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	_, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("target_directory_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFile(t, filepath.Join(dir, "file.txt"), "data\n")
		setupDir(t, filepath.Join(dir, "target"))

		_, _, code := runMvAndCapture(t, goBin,
			[]string{"-t", filepath.Join(dir, "target"), filepath.Join(dir, "file.txt")}, "")
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}

		data, readErr := os.ReadFile(filepath.Join(dir, "target", "file.txt"))
		if readErr != nil {
			t.Errorf("file not moved to target dir: %v", readErr)
			return
		}
		if string(data) != "data\n" {
			t.Errorf("content mismatch: got %q", data)
		}
	})
}

// TestDiffLastFlagWins verifies that conflicting -f/-i/-n flags resolve to
// the last one specified.
// R2.2: last flag wins.
func TestDiffLastFlagWins(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	_, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("n_then_f_overwrites", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFile(t, filepath.Join(dir, "src.txt"), "new\n")
		setupFile(t, filepath.Join(dir, "dst.txt"), "old\n")

		_, _, code := runMvAndCapture(t, goBin,
			[]string{"-n", "-f", filepath.Join(dir, "src.txt"), filepath.Join(dir, "dst.txt")}, "")
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "dst.txt"))
		if string(data) != "new\n" {
			t.Errorf("-n -f: expected overwrite, got %q", data)
		}
	})

	t.Run("f_then_n_no_overwrite", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		setupFile(t, filepath.Join(dir, "src.txt"), "new\n")
		setupFile(t, filepath.Join(dir, "dst.txt"), "old\n")

		_, _, code := runMvAndCapture(t, goBin,
			[]string{"-f", "-n", filepath.Join(dir, "src.txt"), filepath.Join(dir, "dst.txt")}, "")
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "dst.txt"))
		if string(data) != "old\n" {
			t.Errorf("-f -n: expected no overwrite, got %q", data)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "src.txt")); statErr != nil {
			t.Errorf("source removed despite -n: %v", statErr)
		}
	})
}

// TestDiffMultipleSourcePartialFailure verifies that partial failures continue
// and exit 1.
// R4.3: continue moving remaining files, exit 1 on any failure.
func TestDiffMultipleSourcePartialFailure(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	t.Run("partial_failure", func(t *testing.T) {
		t.Parallel()

		norm := programNameNormalizer(goBin, refBin)
		pathNorm := func(dir string) testutils.NormalizeFunc {
			return func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(dir), []byte("/tmp/test"))
			}
		}

		// Reference: separate dir so ref doesn't steal Go's files.
		refDir := t.TempDir()
		setupFile(t, filepath.Join(refDir, "good.txt"), "ok\n")
		setupDir(t, filepath.Join(refDir, "dest"))
		_, refStderr, refCode := runMvAndCapture(t, refBin,
			[]string{filepath.Join(refDir, "nonexistent.txt"), filepath.Join(refDir, "good.txt"),
				filepath.Join(refDir, "dest")}, "")

		// Go binary.
		goDir := t.TempDir()
		setupFile(t, filepath.Join(goDir, "good.txt"), "ok\n")
		setupDir(t, filepath.Join(goDir, "dest"))
		_, goStderr, goCode := runMvAndCapture(t, goBin,
			[]string{filepath.Join(goDir, "nonexistent.txt"), filepath.Join(goDir, "good.txt"),
				filepath.Join(goDir, "dest")}, "")

		if refCode != goCode {
			t.Errorf("exit code: ref=%d go=%d", refCode, goCode)
		}

		normRefStderr := tryHelpNormalizer(norm(pathNorm(refDir)(refStderr)))
		normGoStderr := tryHelpNormalizer(norm(pathNorm(goDir)(goStderr)))
		if !bytes.Equal(normRefStderr, normGoStderr) {
			t.Errorf("stderr:\nref: %q\ngo:  %q", normRefStderr, normGoStderr)
		}

		// The good file should have been moved despite the bad one failing.
		if _, statErr := os.Stat(filepath.Join(goDir, "dest", "good.txt")); statErr != nil {
			t.Errorf("good.txt was not moved: %v", statErr)
		}
	})
}
