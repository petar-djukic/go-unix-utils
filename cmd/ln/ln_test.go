// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd037-ln R1.1–R1.4, R2.1–R2.4, R4.1–R4.3 differential tests
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// programNameNormalizer replaces the binary name (gln or the full Go binary
// path) with the canonical name "ln" so stderr messages are comparable.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("ln"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("ln"))
		b = bytes.ReplaceAll(b, []byte("gln"), []byte("ln"))
		return b
	}
}

// outputClearNormalizer replaces all output with empty bytes. Used for
// --version and --help differential tests where content differs but exit
// code must match.
var outputClearNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return nil
}

// TestDiffErrors uses RunDiffTests for error cases where both binaries
// fail identically without modifying filesystem state.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
	}

	// R1.3: hard link to directory must fail.
	t.Run("hard_link_directory_error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "subdir")
		if mkErr := os.Mkdir(subdir, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "hard_link_to_directory",
				Args:      []string{subdir, filepath.Join(dir, "link")},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.4: destination exists without -f (hard link).
	t.Run("hard_link_exists_error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("src\n"), 0o644) //nolint:errcheck
		os.WriteFile(dst, []byte("dst\n"), 0o644) //nolint:errcheck
		tests := []testutils.DiffTest{
			{
				Name:      "hard_link_file_exists",
				Args:      []string{src, dst},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R2.1: destination exists without -f (symlink).
	t.Run("symbolic_link_exists_error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("src\n"), 0o644) //nolint:errcheck
		os.WriteFile(dst, []byte("dst\n"), 0o644) //nolint:errcheck
		tests := []testutils.DiffTest{
			{
				Name:      "symlink_file_exists",
				Args:      []string{"-s", src, dst},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// R1.1: missing operand.
	t.Run("missing_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "no_args",
				Args:      []string{},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// --help exits 0.
	t.Run("help_flag", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "help_exits_0",
				Args:      []string{"--help"},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  0,
				Normalize: []testutils.NormalizeFunc{outputClearNormalizer},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	// --version exits 0.
	t.Run("version_flag", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "version_exits_0",
				Args:      []string{"--version"},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  0,
				Normalize: []testutils.NormalizeFunc{outputClearNormalizer},
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// runLinkBinary runs a binary with given args in the specified working
// directory and returns combined output and exit code.
func runLinkBinary(t *testing.T, bin string, args []string, workDir string) ([]byte, []byte, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
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
			t.Fatalf("failed to run %q: %v", bin, err)
		}
	}
	return stdout.Bytes(), stderr.Bytes(), exitCode
}

// TestHardLinkBasic verifies R1.1: ln TARGET LINK_NAME creates a hard link.
// Tested independently because ln is destructive.
func TestHardLinkBasic(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	// Create source files.
	os.WriteFile(filepath.Join(goDir, "src.txt"), []byte("hello\n"), 0o644)  //nolint:errcheck
	os.WriteFile(filepath.Join(refDir, "src.txt"), []byte("hello\n"), 0o644) //nolint:errcheck

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{filepath.Join(goDir, "src.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{filepath.Join(refDir, "src.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// R4.3: verify hard link (same inode).
	goSrc, _ := os.Stat(filepath.Join(goDir, "src.txt"))
	goLink, err := os.Stat(filepath.Join(goDir, "link.txt"))
	if err != nil {
		t.Fatalf("hard link not created: %v", err)
	}
	if !os.SameFile(goSrc, goLink) {
		t.Error("hard link does not share inode with source")
	}
}

// TestHardLinkMultiTarget verifies R1.2: ln TARGET... DIRECTORY.
func TestHardLinkMultiTarget(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "a.txt"), []byte("a\n"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(d, "b.txt"), []byte("b\n"), 0o644) //nolint:errcheck
		os.Mkdir(filepath.Join(d, "dest"), 0o755)                     //nolint:errcheck
	}

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{filepath.Join(goDir, "a.txt"), filepath.Join(goDir, "b.txt"), filepath.Join(goDir, "dest")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{filepath.Join(refDir, "a.txt"), filepath.Join(refDir, "b.txt"), filepath.Join(refDir, "dest")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// R4.3: verify both links exist.
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, statErr := os.Stat(filepath.Join(goDir, "dest", name)); statErr != nil {
			t.Errorf("link %q not created in dest: %v", name, statErr)
		}
	}
}

// TestSymlinkBasic verifies R2.1: -s creates a symbolic link.
func TestSymlinkBasic(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "target.txt"), []byte("target\n"), 0o644) //nolint:errcheck
	}

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-s", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "symlink.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-s", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "symlink.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// R4.3: verify it's a symlink pointing to the right target.
	got, err := os.Readlink(filepath.Join(goDir, "symlink.txt"))
	if err != nil {
		t.Fatalf("expected symlink: %v", err)
	}
	if got != filepath.Join(goDir, "target.txt") {
		t.Errorf("symlink target mismatch: got %q, want %q", got, filepath.Join(goDir, "target.txt"))
	}
}

// TestSymlinkLongFlag verifies R2.1: --symbolic creates a symbolic link.
func TestSymlinkLongFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "target.txt"), []byte("long\n"), 0o644) //nolint:errcheck

	_, _, code := runLinkBinary(t, goBin,
		[]string{"--symbolic", filepath.Join(dir, "target.txt"), filepath.Join(dir, "link.txt")}, "")
	if code != 0 {
		t.Fatalf("ln --symbolic failed with exit code %d", code)
	}

	got, err := os.Readlink(filepath.Join(dir, "link.txt"))
	if err != nil {
		t.Fatalf("expected symlink: %v", err)
	}
	if got != filepath.Join(dir, "target.txt") {
		t.Errorf("symlink target mismatch: got %q, want %q", got, filepath.Join(dir, "target.txt"))
	}
}

// TestSymlinkToDirectory verifies R2.2: -s allows symlinks to directories.
func TestSymlinkToDirectory(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.Mkdir(filepath.Join(d, "mydir"), 0o755) //nolint:errcheck
	}

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-s", filepath.Join(goDir, "mydir"), filepath.Join(goDir, "dirlink")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-s", filepath.Join(refDir, "mydir"), filepath.Join(refDir, "dirlink")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// R4.3: verify symlink points to a directory.
	got, err := os.Readlink(filepath.Join(goDir, "dirlink"))
	if err != nil {
		t.Fatalf("expected symlink: %v", err)
	}
	if got != filepath.Join(goDir, "mydir") {
		t.Errorf("symlink target mismatch: got %q, want %q", got, filepath.Join(goDir, "mydir"))
	}
	info, err := os.Stat(filepath.Join(goDir, "dirlink"))
	if err != nil {
		t.Fatalf("stat on symlink failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected symlink to resolve to a directory")
	}
}

// TestSymlinkTargetStoredAsIs verifies R2.3: target string is stored as-is.
func TestSymlinkTargetStoredAsIs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data\n"), 0o644) //nolint:errcheck

	// Use a relative target — it should be stored literally.
	_, _, code := runLinkBinary(t, goBin,
		[]string{"-s", "file.txt", filepath.Join(dir, "rel_link.txt")}, dir)
	if code != 0 {
		t.Fatalf("ln -s failed with exit code %d", code)
	}

	got, err := os.Readlink(filepath.Join(dir, "rel_link.txt"))
	if err != nil {
		t.Fatalf("expected symlink: %v", err)
	}
	if got != "file.txt" {
		t.Errorf("symlink target should be stored as-is: got %q, want %q", got, "file.txt")
	}
}

// TestRelativeSymlink verifies R2.4: -r creates a relative symbolic link.
func TestRelativeSymlink(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// R2.4: -sr bundled flags.
	t.Run("bundled_sr", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.Mkdir(filepath.Join(d, "sub"), 0o755)                                           //nolint:errcheck
			os.WriteFile(filepath.Join(d, "sub", "target.txt"), []byte("rel\n"), 0o644)        //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sr", filepath.Join(goDir, "sub", "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sr", filepath.Join(refDir, "sub", "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// R4.3: verify the symlink is relative.
		goGot, err := os.Readlink(filepath.Join(goDir, "link.txt"))
		if err != nil {
			t.Fatalf("expected symlink: %v", err)
		}
		refGot, refErr := os.Readlink(filepath.Join(refDir, "link.txt"))
		if refErr != nil {
			t.Fatalf("expected ref symlink: %v", refErr)
		}
		if goGot != refGot {
			t.Errorf("relative symlink mismatch: go=%q ref=%q", goGot, refGot)
		}
		if goGot != filepath.Join("sub", "target.txt") {
			t.Errorf("expected relative symlink %q, got %q", filepath.Join("sub", "target.txt"), goGot)
		}
	})

	// R2.4: --relative long flag.
	t.Run("long_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.Mkdir(filepath.Join(d, "deep"), 0o755)                                          //nolint:errcheck
			os.WriteFile(filepath.Join(d, "deep", "target.txt"), []byte("deep\n"), 0o644)      //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-s", "--relative", filepath.Join(goDir, "deep", "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-s", "--relative", filepath.Join(refDir, "deep", "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		goGot, err := os.Readlink(filepath.Join(goDir, "link.txt"))
		if err != nil {
			t.Fatalf("expected symlink: %v", err)
		}
		refGot, refErr := os.Readlink(filepath.Join(refDir, "link.txt"))
		if refErr != nil {
			t.Fatalf("expected ref symlink: %v", refErr)
		}
		if goGot != refGot {
			t.Errorf("relative symlink mismatch: go=%q ref=%q", goGot, refGot)
		}
	})

	// R2.4: relative symlink with upward traversal (../file).
	t.Run("upward_traversal", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "root.txt"), []byte("root\n"), 0o644) //nolint:errcheck
			os.Mkdir(filepath.Join(d, "child"), 0o755)                          //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-s", "-r", filepath.Join(goDir, "root.txt"), filepath.Join(goDir, "child", "up_link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-s", "-r", filepath.Join(refDir, "root.txt"), filepath.Join(refDir, "child", "up_link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		goGot, err := os.Readlink(filepath.Join(goDir, "child", "up_link.txt"))
		if err != nil {
			t.Fatalf("expected symlink: %v", err)
		}
		refGot, refErr := os.Readlink(filepath.Join(refDir, "child", "up_link.txt"))
		if refErr != nil {
			t.Fatalf("expected ref symlink: %v", refErr)
		}
		if goGot != refGot {
			t.Errorf("relative symlink mismatch: go=%q ref=%q", goGot, refGot)
		}
		if goGot != filepath.Join("..", "root.txt") {
			t.Errorf("expected %q, got %q", filepath.Join("..", "root.txt"), goGot)
		}
	})
}
