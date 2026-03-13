// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd037-ln R1.1–R1.4, R2.1–R2.4, R3.1–R3.6, R4.1–R4.3 differential tests
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

// TestBackupSimple verifies R3.1 and R3.5: --backup=simple creates a backup
// with the default ~ suffix before replacing the destination.
func TestBackupSimple(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
	}

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-s", "--backup=simple", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-s", "--backup=simple", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// R3.1: verify backup file exists with ~ suffix.
	goBackup, err := os.ReadFile(filepath.Join(goDir, "link.txt~"))
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	refBackup, err := os.ReadFile(filepath.Join(refDir, "link.txt~"))
	if err != nil {
		t.Fatalf("ref backup not created: %v", err)
	}
	if !bytes.Equal(goBackup, refBackup) {
		t.Errorf("backup content mismatch: go=%q ref=%q", goBackup, refBackup)
	}

	// Verify symlink was created.
	if _, err := os.Lstat(filepath.Join(goDir, "link.txt")); err != nil {
		t.Errorf("link not created: %v", err)
	}
}

// TestBackupNumbered verifies R3.3: --backup=numbered creates .~1~ style backups.
func TestBackupNumbered(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644)  //nolint:errcheck
		os.WriteFile(filepath.Join(d, "link.txt"), []byte("old1\n"), 0o644)   //nolint:errcheck
	}

	// First backup: should create .~1~
	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-s", "--backup=numbered", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-s", "--backup=numbered", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// R3.3: verify numbered backup .~1~ exists.
	goBackup, err := os.ReadFile(filepath.Join(goDir, "link.txt.~1~"))
	if err != nil {
		t.Fatalf("numbered backup .~1~ not created: %v", err)
	}
	refBackup, err := os.ReadFile(filepath.Join(refDir, "link.txt.~1~"))
	if err != nil {
		t.Fatalf("ref numbered backup .~1~ not created: %v", err)
	}
	if !bytes.Equal(goBackup, refBackup) {
		t.Errorf("numbered backup content mismatch: go=%q ref=%q", goBackup, refBackup)
	}

	// Second backup: create new dest and backup again, should create .~2~
	for _, d := range []string{goDir, refDir} {
		os.Remove(filepath.Join(d, "link.txt"))                              //nolint:errcheck
		os.WriteFile(filepath.Join(d, "link.txt"), []byte("old2\n"), 0o644)  //nolint:errcheck
	}

	_, _, goCode2 := runLinkBinary(t, goBin,
		[]string{"-s", "--backup=t", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode2 := runLinkBinary(t, refBin,
		[]string{"-s", "--backup=t", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode2 != refCode2 {
		t.Errorf("exit code mismatch (2nd): go=%d ref=%d", goCode2, refCode2)
	}

	// Verify .~2~ exists.
	goBackup2, err := os.ReadFile(filepath.Join(goDir, "link.txt.~2~"))
	if err != nil {
		t.Fatalf("numbered backup .~2~ not created: %v", err)
	}
	refBackup2, err := os.ReadFile(filepath.Join(refDir, "link.txt.~2~"))
	if err != nil {
		t.Fatalf("ref numbered backup .~2~ not created: %v", err)
	}
	if !bytes.Equal(goBackup2, refBackup2) {
		t.Errorf("numbered backup .~2~ content mismatch: go=%q ref=%q", goBackup2, refBackup2)
	}
}

// TestBackupExisting verifies R3.3: --backup=existing uses numbered if numbered
// backups already exist, otherwise simple.
func TestBackupExisting(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Sub-test: no prior numbered backups → simple backup.
	t.Run("no_numbered_uses_simple", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-s", "--backup=existing", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-s", "--backup=existing", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// Should create simple backup link.txt~.
		if _, err := os.Stat(filepath.Join(goDir, "link.txt~")); err != nil {
			t.Errorf("expected simple backup link.txt~: %v", err)
		}
		if _, err := os.Stat(filepath.Join(refDir, "link.txt~")); err != nil {
			t.Errorf("ref expected simple backup link.txt~: %v", err)
		}
	})

	// Sub-test: with prior numbered backup → numbered backup.
	t.Run("with_numbered_uses_numbered", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644)       //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)          //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt.~1~"), []byte("prev\n"), 0o644)    //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-s", "--backup=nil", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-s", "--backup=nil", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// Should create numbered backup link.txt.~2~.
		goBackup, err := os.ReadFile(filepath.Join(goDir, "link.txt.~2~"))
		if err != nil {
			t.Fatalf("numbered backup .~2~ not created: %v", err)
		}
		refBackup, err := os.ReadFile(filepath.Join(refDir, "link.txt.~2~"))
		if err != nil {
			t.Fatalf("ref numbered backup .~2~ not created: %v", err)
		}
		if !bytes.Equal(goBackup, refBackup) {
			t.Errorf("backup content mismatch: go=%q ref=%q", goBackup, refBackup)
		}
	})
}

// TestBackupSuffix verifies R3.6: -S/--suffix overrides the default ~ suffix.
func TestBackupSuffix(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	t.Run("suffix_long_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-s", "--backup=simple", "--suffix=.bak", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-s", "--backup=simple", "--suffix=.bak", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// Verify backup with custom suffix.
		if _, err := os.Stat(filepath.Join(goDir, "link.txt.bak")); err != nil {
			t.Errorf("backup with .bak suffix not created: %v", err)
		}
		if _, err := os.Stat(filepath.Join(refDir, "link.txt.bak")); err != nil {
			t.Errorf("ref backup with .bak suffix not created: %v", err)
		}
	})

	t.Run("suffix_short_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-s", "--backup=simple", "-S", ".orig", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-s", "--backup=simple", "-S", ".orig", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// Verify backup with -S suffix.
		if _, err := os.Stat(filepath.Join(goDir, "link.txt.orig")); err != nil {
			t.Errorf("backup with .orig suffix not created: %v", err)
		}
		if _, err := os.Stat(filepath.Join(refDir, "link.txt.orig")); err != nil {
			t.Errorf("ref backup with .orig suffix not created: %v", err)
		}
	})
}

// TestBackupVersionControl verifies R3.4: VERSION_CONTROL env var is used when
// --backup has no explicit argument.
func TestBackupVersionControl(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	t.Run("version_control_simple", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		goCmd := exec.Command(goBin, "-s", "--backup",
			filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt"))
		goCmd.Env = append(os.Environ(), "LC_ALL=C", "VERSION_CONTROL=simple")
		goCmd.Run() //nolint:errcheck

		refCmd := exec.Command(refBin, "-s", "--backup",
			filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt"))
		refCmd.Env = append(os.Environ(), "LC_ALL=C", "VERSION_CONTROL=simple")
		refCmd.Run() //nolint:errcheck

		// Should create simple backup link.txt~ (not numbered).
		if _, err := os.Stat(filepath.Join(goDir, "link.txt~")); err != nil {
			t.Errorf("VERSION_CONTROL=simple did not create simple backup: %v", err)
		}
		if _, err := os.Stat(filepath.Join(refDir, "link.txt~")); err != nil {
			t.Errorf("ref VERSION_CONTROL=simple did not create simple backup: %v", err)
		}
	})

	t.Run("version_control_numbered", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		goCmd := exec.Command(goBin, "-s", "--backup",
			filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt"))
		goCmd.Env = append(os.Environ(), "LC_ALL=C", "VERSION_CONTROL=numbered")
		goCmd.Run() //nolint:errcheck

		refCmd := exec.Command(refBin, "-s", "--backup",
			filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt"))
		refCmd.Env = append(os.Environ(), "LC_ALL=C", "VERSION_CONTROL=numbered")
		refCmd.Run() //nolint:errcheck

		// Should create numbered backup .~1~.
		if _, err := os.Stat(filepath.Join(goDir, "link.txt.~1~")); err != nil {
			t.Errorf("VERSION_CONTROL=numbered did not create numbered backup: %v", err)
		}
		if _, err := os.Stat(filepath.Join(refDir, "link.txt.~1~")); err != nil {
			t.Errorf("ref VERSION_CONTROL=numbered did not create numbered backup: %v", err)
		}
	})
}

// TestBackupShortFlag verifies R3.5: -b is shorthand for --backup (default method).
func TestBackupShortFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
	}

	// -sb bundles symbolic and backup.
	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-sb", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-sb", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// -b defaults to "existing" which without prior numbered backups creates simple backup.
	if _, err := os.Stat(filepath.Join(goDir, "link.txt~")); err != nil {
		t.Errorf("-b did not create backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(refDir, "link.txt~")); err != nil {
		t.Errorf("ref -b did not create backup: %v", err)
	}
}

// TestVerboseOutput verifies R3.4: -v prints the name of each link created.
// R4.2: differential test coverage for verbose output.
func TestVerboseOutput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
	}

	// R3.4: -v with symbolic link.
	t.Run("verbose_symlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("data\n"), 0o644) //nolint:errcheck
		}

		goStdout, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sv", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		refStdout, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sv", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// Normalize paths since tmp dirs differ.
		goOut := bytes.ReplaceAll(goStdout, []byte(goDir), []byte("/DIR"))
		refOut := bytes.ReplaceAll(refStdout, []byte(refDir), []byte("/DIR"))
		if !bytes.Equal(goOut, refOut) {
			t.Errorf("verbose output mismatch:\ngo:  %q\nref: %q", goOut, refOut)
		}
	})

	// R3.4: -v with hard link.
	t.Run("verbose_hardlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("data\n"), 0o644) //nolint:errcheck
		}

		goStdout, _, goCode := runLinkBinary(t, goBin,
			[]string{"-v", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "hardlink.txt")}, "")
		refStdout, _, refCode := runLinkBinary(t, refBin,
			[]string{"-v", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "hardlink.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		goOut := bytes.ReplaceAll(goStdout, []byte(goDir), []byte("/DIR"))
		refOut := bytes.ReplaceAll(refStdout, []byte(refDir), []byte("/DIR"))
		if !bytes.Equal(goOut, refOut) {
			t.Errorf("verbose output mismatch:\ngo:  %q\nref: %q", goOut, refOut)
		}
	})

	// R3.4: --verbose long flag.
	t.Run("verbose_long_flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "target.txt"), []byte("data\n"), 0o644) //nolint:errcheck

		stdout, _, code := runLinkBinary(t, goBin,
			[]string{"-s", "--verbose", filepath.Join(dir, "target.txt"), filepath.Join(dir, "link.txt")}, "")
		if code != 0 {
			t.Fatalf("ln -s --verbose failed with exit code %d", code)
		}
		if len(stdout) == 0 {
			t.Error("--verbose produced no output")
		}
	})

	// R3.4: -v with force replace shows output for the new link.
	t.Run("verbose_force_replace", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		goStdout, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sfv", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		refStdout, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sfv", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		goOut := bytes.ReplaceAll(goStdout, []byte(goDir), []byte("/DIR"))
		refOut := bytes.ReplaceAll(refStdout, []byte(refDir), []byte("/DIR"))
		if !bytes.Equal(goOut, refOut) {
			t.Errorf("verbose output mismatch:\ngo:  %q\nref: %q", goOut, refOut)
		}
	})

	_ = normalize // used in error subtests above
}

// TestNoDereference verifies R3.2: -n treats a destination symlink to a
// directory as a regular file rather than following it.
// R4.2: differential test coverage for no-dereference (-n).
func TestNoDereference(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// R3.2: -n with symlink to directory — should replace the symlink, not
	// create a link inside the directory.
	t.Run("no_deref_symlink_to_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("t\n"), 0o644) //nolint:errcheck
			os.Mkdir(filepath.Join(d, "realdir"), 0o755)                       //nolint:errcheck
			os.Symlink(filepath.Join(d, "realdir"), filepath.Join(d, "dirlink"))  //nolint:errcheck
		}

		// Without -n, ln -sf target dirlink would create dirlink/target.txt
		// With -n, ln -sfn target.txt dirlink replaces the symlink itself.
		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sfn", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "dirlink")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sfn", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "dirlink")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// R4.3: verify dirlink is now a symlink to target.txt, not a directory.
		goTarget, err := os.Readlink(filepath.Join(goDir, "dirlink"))
		if err != nil {
			t.Fatalf("dirlink is not a symlink after -n: %v", err)
		}
		refTarget, err := os.Readlink(filepath.Join(refDir, "dirlink"))
		if err != nil {
			t.Fatalf("ref dirlink is not a symlink after -n: %v", err)
		}
		// Compare basenames since temp dir paths differ between go and ref runs.
		if filepath.Base(goTarget) != filepath.Base(refTarget) {
			t.Errorf("symlink target mismatch: go=%q ref=%q", goTarget, refTarget)
		}
	})

	// R3.2: --no-dereference long flag.
	t.Run("no_deref_long_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("t\n"), 0o644) //nolint:errcheck
			os.Mkdir(filepath.Join(d, "realdir"), 0o755)                       //nolint:errcheck
			os.Symlink(filepath.Join(d, "realdir"), filepath.Join(d, "dirlink"))  //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sf", "--no-dereference", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "dirlink")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sf", "--no-dereference", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "dirlink")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		goTarget, err := os.Readlink(filepath.Join(goDir, "dirlink"))
		if err != nil {
			t.Fatalf("dirlink is not a symlink: %v", err)
		}
		refTarget, err := os.Readlink(filepath.Join(refDir, "dirlink"))
		if err != nil {
			t.Fatalf("ref dirlink is not a symlink: %v", err)
		}
		if filepath.Base(goTarget) != filepath.Base(refTarget) {
			t.Errorf("symlink target mismatch: go=%q ref=%q", goTarget, refTarget)
		}
	})
}

// TestBackupNone verifies R3.5: --backup=none does not create a backup and
// requires -f to replace existing destinations.
func TestBackupNone(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
	}

	// --backup=none with -f should replace without creating backup.
	t.Run("backup_none_with_force", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sf", "--backup=none", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sf", "--backup=none", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		// No backup file should exist.
		if _, err := os.Stat(filepath.Join(goDir, "link.txt~")); err == nil {
			t.Error("--backup=none should not create a backup file")
		}
		if _, err := os.Stat(filepath.Join(refDir, "link.txt~")); err == nil {
			t.Error("ref --backup=none should not create a backup file")
		}
	})

	// --backup=off is an alias for none.
	t.Run("backup_off_alias", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, d := range []string{goDir, refDir} {
			os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
		}

		_, _, goCode := runLinkBinary(t, goBin,
			[]string{"-sf", "--backup=off", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
		_, _, refCode := runLinkBinary(t, refBin,
			[]string{"-sf", "--backup=off", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

		if goCode != refCode {
			t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
		}

		if _, err := os.Stat(filepath.Join(goDir, "link.txt~")); err == nil {
			t.Error("--backup=off should not create a backup file")
		}
	})

	_ = normalize // available for error subtests
}

// TestForceReplace verifies R3.1: -f removes existing destination before link creation.
func TestForceReplace(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
	}

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-sf", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-sf", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}
	if goCode != 0 {
		t.Fatalf("-sf failed with exit code %d", goCode)
	}

	// Verify the symlink was created (no backup).
	if _, err := os.Lstat(filepath.Join(goDir, "link.txt")); err != nil {
		t.Errorf("link not created: %v", err)
	}
	// Verify no backup was created (no -b).
	if _, err := os.Stat(filepath.Join(goDir, "link.txt~")); err == nil {
		t.Error("-sf should not create backup without -b")
	}
}

// TestBackupForceCombo verifies -f with --backup creates backup then replaces.
func TestBackupForceCombo(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, d := range []string{goDir, refDir} {
		os.WriteFile(filepath.Join(d, "target.txt"), []byte("new\n"), 0o644) //nolint:errcheck
		os.WriteFile(filepath.Join(d, "link.txt"), []byte("old\n"), 0o644)   //nolint:errcheck
	}

	_, _, goCode := runLinkBinary(t, goBin,
		[]string{"-sf", "--backup=simple", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "link.txt")}, "")
	_, _, refCode := runLinkBinary(t, refBin,
		[]string{"-sf", "--backup=simple", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "link.txt")}, "")

	if goCode != refCode {
		t.Errorf("exit code mismatch: go=%d ref=%d", goCode, refCode)
	}

	// Verify backup exists.
	goBackup, err := os.ReadFile(filepath.Join(goDir, "link.txt~"))
	if err != nil {
		t.Fatalf("backup not created with -f --backup: %v", err)
	}
	refBackup, err := os.ReadFile(filepath.Join(refDir, "link.txt~"))
	if err != nil {
		t.Fatalf("ref backup not created with -f --backup: %v", err)
	}
	if !bytes.Equal(goBackup, refBackup) {
		t.Errorf("backup content mismatch: go=%q ref=%q", goBackup, refBackup)
	}
}
