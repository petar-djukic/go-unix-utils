// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd034-mkdir R1.1–R1.4, R2.1–R2.3, R3.1–R3.4 differential tests
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

// compareDirModes runs both binaries with the given args (each creating a dir
// in its own temp directory) and compares the resulting permission bits on a
// target relative path. This is necessary because mkdir is destructive: we
// cannot use RunDiffTests for creation scenarios.
func compareDirModes(t *testing.T, goBin, refBin string, flagArgs []string, targetRel string) {
	t.Helper()

	goDir := t.TempDir()
	refDir := t.TempDir()

	goTarget := filepath.Join(goDir, targetRel)
	refTarget := filepath.Join(refDir, targetRel)

	goArgs := append(flagArgs, goTarget)
	refArgs := append(flagArgs, refTarget)

	goCmd := exec.Command(goBin, goArgs...)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := goCmd.CombinedOutput(); err != nil {
		t.Fatalf("go binary %v failed: %v\noutput: %s", goArgs, err, out)
	}

	refCmd := exec.Command(refBin, refArgs...)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("ref binary %v failed: %v\noutput: %s", refArgs, err, out)
	}

	goInfo, err := os.Stat(goTarget)
	if err != nil {
		t.Fatalf("go dir %q not created: %v", goTarget, err)
	}
	refInfo, err := os.Stat(refTarget)
	if err != nil {
		t.Fatalf("ref dir %q not created: %v", refTarget, err)
	}

	if goInfo.Mode().Perm() != refInfo.Mode().Perm() {
		t.Errorf("permission mismatch for args %v target %q: go=%o ref=%o",
			flagArgs, targetRel, goInfo.Mode().Perm(), refInfo.Mode().Perm())
	}
}

// TestModeOctal verifies R3.1, R3.3: mkdir -m with octal mode sets the
// correct permission bits, matching gmkdir.
func TestModeOctal(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	tests := []struct {
		name string
		mode string
	}{
		{"0755", "0755"},
		{"0700", "0700"},
		{"0777", "0777"},
		{"0644", "0644"},
		{"755", "755"},
		{"0500", "0500"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareDirModes(t, goBin, refBin, []string{"-m", tc.mode}, "testdir")
		})
	}
}

// TestModeSymbolic verifies R3.1, R3.2: mkdir -m with symbolic mode sets the
// correct permission bits, matching gmkdir.
func TestModeSymbolic(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	tests := []struct {
		name string
		mode string
	}{
		{"u=rwx,go=rx", "u=rwx,go=rx"},
		{"a=rwx", "a=rwx"},
		{"u=rwx", "u=rwx"},
		{"u=rwx,g=rx,o=", "u=rwx,g=rx,o="},
		{"a=rx", "a=rx"},
		{"u=rwx,go=", "u=rwx,go="},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareDirModes(t, goBin, refBin, []string{"-m", tc.mode}, "testdir")
		})
	}
}

// TestModeLongOption verifies R3.1: --mode=MODE works the same as -m MODE.
func TestModeLongOption(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	compareDirModes(t, goBin, refBin, []string{"--mode=0750"}, "testdir")
}

// TestModeWithParents verifies R3.3: -p -m applies mode only to the final
// directory; intermediate directories get default permissions.
func TestModeWithParents(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	goTarget := filepath.Join(goDir, "a", "b", "c")
	refTarget := filepath.Join(refDir, "a", "b", "c")

	goCmd := exec.Command(goBin, "-p", "-m", "0700", goTarget)
	goCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := goCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mkdir -p -m 0700 failed: %v\noutput: %s", err, out)
	}

	refCmd := exec.Command(refBin, "-p", "-m", "0700", refTarget)
	refCmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := refCmd.CombinedOutput(); err != nil {
		t.Fatalf("gmkdir -p -m 0700 failed: %v\noutput: %s", err, out)
	}

	// R3.3: final directory should have mode 0700.
	for _, pair := range []struct {
		label string
		path  string
	}{
		{"go", goTarget},
		{"ref", refTarget},
	} {
		info, statErr := os.Stat(pair.path)
		if statErr != nil {
			t.Fatalf("%s: final dir not created: %v", pair.label, statErr)
		}
		if info.Mode().Perm() != 0o700 {
			t.Errorf("%s: final dir mode=%o, want 0700", pair.label, info.Mode().Perm())
		}
	}

	// R3.3: intermediate directories should have default permissions (matching between go and ref).
	for _, sub := range []string{"a", "a/b"} {
		goInfo, goErr := os.Stat(filepath.Join(goDir, sub))
		if goErr != nil {
			t.Fatalf("go: intermediate %q not created: %v", sub, goErr)
		}
		refInfo, refErr := os.Stat(filepath.Join(refDir, sub))
		if refErr != nil {
			t.Fatalf("ref: intermediate %q not created: %v", sub, refErr)
		}
		if goInfo.Mode().Perm() != refInfo.Mode().Perm() {
			t.Errorf("intermediate %q permission mismatch: go=%o ref=%o",
				sub, goInfo.Mode().Perm(), refInfo.Mode().Perm())
		}
	}
}

// TestModeBundledFlag verifies that -pm MODE works (bundled short flags).
func TestModeBundledFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	compareDirModes(t, goBin, refBin, []string{"-pm", "0755"}, "a/b")
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
