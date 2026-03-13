// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd034-mkdir R1.1–R1.4, R2.1–R2.3 differential tests
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

// programNameNormalizer replaces the binary name (gmkdir or the full go binary
// path) and its containing path with the canonical name "mkdir" so stderr
// messages are comparable between the Go and reference binaries.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("mkdir"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("mkdir"))
		b = bytes.ReplaceAll(b, []byte("gmkdir"), []byte("mkdir"))
		return b
	}
}

// errCaseNormalizer normalizes error message casing differences between Go's
// os package (lowercase) and GNU coreutils (title case). For example,
// "file exists" vs "File exists", "no such file or directory" vs
// "No such file or directory".
var errCaseNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?m): [A-Z][a-z]`)
	return re.ReplaceAllFunc(b, bytes.ToLower)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	// R1.3 (task R2): existing directory error — both binaries see an
	// already-existing directory and fail identically.
	t.Run("existing_dir_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		existing := filepath.Join(tmpDir, "already_exists")
		if err := os.Mkdir(existing, 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "error on existing directory",
				Args:      []string{existing},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.3 (task R2): existing file error.
	t.Run("existing_file_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		existingFile := filepath.Join(tmpDir, "afile")
		if err := os.WriteFile(existingFile, []byte("data"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "error on existing file",
				Args:      []string{existingFile},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.4 (task R3): missing intermediate directory error.
	t.Run("missing_intermediate_error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		nested := filepath.Join(tmpDir, "nonexistent", "child")
		tests := []testutils.DiffTest{
			{
				Name:      "error on missing intermediate",
				Args:      []string{nested},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// Missing operand error.
	t.Run("missing_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "no arguments",
				Args:      []string{},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestCreateSingleDir verifies R1.1: mkdir creates a single directory.
// This is tested independently (not diff) because mkdir is destructive —
// RunDiffTests would run the ref binary first, creating the dir, causing
// the Go binary to fail with "file exists".
func TestCreateSingleDir(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "newdir")

	cmd := exec.Command(goBin, target)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mkdir failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", target)
	}
}

// TestCreateMultipleDirs verifies R1.2: mkdir creates multiple directories.
func TestCreateMultipleDirs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	dirs := []string{
		filepath.Join(tmpDir, "dir_a"),
		filepath.Join(tmpDir, "dir_b"),
		filepath.Join(tmpDir, "dir_c"),
	}

	cmd := exec.Command(goBin, dirs...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mkdir failed: %v\noutput: %s", err, out)
	}

	for _, d := range dirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %q not created: %v", d, err)
		} else if !info.IsDir() {
			t.Errorf("%q is not a directory", d)
		}
	}
}

// TestHelp verifies R1.4: --help prints usage to stdout and exits 0.
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
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Errorf("--help output missing 'Usage:': %s", out)
	}
}

// TestVersion verifies R1.4: --version prints version to stdout and exits 0.
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
	if !bytes.Contains(out, []byte("mkdir")) {
		t.Errorf("--version output missing 'mkdir': %s", out)
	}
}

// TestParentsNestedCreation verifies R2.1: -p creates intermediate parent
// directories as needed. Tested independently because mkdir is destructive.
func TestParentsNestedCreation(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "a", "b", "c")

	cmd := exec.Command(goBin, "-p", target)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mkdir -p failed: %v\noutput: %s", err, out)
	}

	// Verify the full chain was created.
	for _, sub := range []string{"a", "a/b", "a/b/c"} {
		p := filepath.Join(tmpDir, sub)
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Errorf("directory %q not created: %v", sub, statErr)
		} else if !info.IsDir() {
			t.Errorf("%q is not a directory", sub)
		}
	}
}

// TestParentsLongOption verifies R2.1: --parents creates intermediate parents.
func TestParentsLongOption(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "x", "y")

	cmd := exec.Command(goBin, "--parents", target)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mkdir --parents failed: %v\noutput: %s", err, out)
	}

	info, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatalf("directory not created: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", target)
	}
}

// TestParentsExistingTarget verifies R2.2: -p does not report an error when
// the target directory already exists.
func TestParentsExistingTarget(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	tmpDir := t.TempDir()
	existing := filepath.Join(tmpDir, "already_here")
	if mkErr := os.Mkdir(existing, 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "parents existing target no error",
			Args:      []string{"-p", existing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  0,
			Normalize: normalize,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestParentsExistingIntermediate verifies R2.3: -p does not report an error
// when intermediate directories already exist.
func TestParentsExistingIntermediate(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		errCaseNormalizer,
	}

	// Create intermediate "a" but not "a/b".
	tmpGoDir := t.TempDir()
	tmpRefDir := t.TempDir()
	if mkErr := os.Mkdir(filepath.Join(tmpGoDir, "a"), 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}
	if mkErr := os.Mkdir(filepath.Join(tmpRefDir, "a"), 0o755); mkErr != nil {
		t.Fatalf("setup: %v", mkErr)
	}

	// Run Go binary in its own tmpdir.
	goTarget := filepath.Join(tmpGoDir, "a", "b")
	goCmd := exec.Command(goBin, "-p", goTarget)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	goOut, goErr := goCmd.CombinedOutput()
	if goErr != nil {
		t.Fatalf("go mkdir -p failed: %v\noutput: %s", goErr, goOut)
	}

	// Run ref binary in its own tmpdir.
	refTarget := filepath.Join(tmpRefDir, "a", "b")
	refCmd := exec.Command(refBin, "-p", refTarget)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	refOut, refErr := refCmd.CombinedOutput()
	if refErr != nil {
		t.Fatalf("gmkdir -p failed: %v\noutput: %s", refErr, refOut)
	}

	// Apply normalizers and compare.
	for _, fn := range normalize {
		goOut = fn(goOut)
		refOut = fn(refOut)
	}
	if !bytes.Equal(goOut, refOut) {
		t.Errorf("output mismatch:\n  go:  %q\n  ref: %q", goOut, refOut)
	}

	// Verify both created the directory.
	if _, statErr := os.Stat(goTarget); statErr != nil {
		t.Errorf("go binary did not create %q: %v", goTarget, statErr)
	}
	if _, statErr := os.Stat(refTarget); statErr != nil {
		t.Errorf("ref binary did not create %q: %v", refTarget, statErr)
	}
}
