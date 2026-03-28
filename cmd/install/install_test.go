// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/install against GNU ginstall.
// Covers prd101-install R3.1 (core copy modes), R3.2 (option flags),
// R3.3 (error cases and exit code verification).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// installNormalizer normalizes messages between ginstall and install.
// Replaces binary name prefix and strips GNU "Try --help" lines.
func installNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`ginstall:`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	permDenied := regexp.MustCompile(`(?i)permission denied`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("install:"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(
			b, []byte("No such file or directory"))
		b = permDenied.ReplaceAll(
			b, []byte("Permission denied"))
		return b
	}
}

// createFixture creates a file in dir and returns its absolute path.
func createFixture(
	t *testing.T, dir, name, content string,
) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("create fixture %s: %v", p, err)
	}
	return p
}

// createFixtureWithMode creates a file and sets exact permissions.
func createFixtureWithMode(
	t *testing.T, dir, name, content string, perm os.FileMode,
) string {
	t.Helper()
	p := createFixture(t, dir, name, content)
	if err := os.Chmod(p, perm); err != nil {
		t.Fatalf("chmod %s: %v", p, err)
	}
	return p
}

// setupBackupWorkDir creates a dir with src.txt and dest.txt.
func setupBackupWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	createFixture(t, dir, "src.txt", "new content\n")
	createFixture(t, dir, "dest.txt", "old content\n")
	return dir
}

// setupCompareWorkDir creates a dir where dest matches src content
// and has 0755 permissions so -C detects them as identical.
func setupCompareWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	createFixture(t, dir, "src.txt", "same\n")
	createFixtureWithMode(t, dir, "dest.txt", "same\n", 0o755)
	return dir
}

// TestDiff runs differential tests against ginstall.
// R3.1: core copy modes, R3.2: option flags, R3.3: error cases.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("ginstall")
	if err != nil {
		t.Skipf("reference binary ginstall not in PATH: %v", err)
	}

	norm := installNormalizer()
	dropAll := func(b []byte) []byte { return nil }

	fix := t.TempDir()
	src1 := createFixture(t, fix, "src1.txt", "hello\n")
	src2 := createFixture(t, fix, "src2.txt", "world\n")

	// Pre-create destination directory for multi-source tests.
	multiDest := filepath.Join(fix, "multi_out")
	if err := os.Mkdir(multiDest, 0o755); err != nil {
		t.Fatal(err)
	}

	// Target directory for -t flag tests.
	tDest := filepath.Join(fix, "t_out")
	if err := os.Mkdir(tDest, 0o755); err != nil {
		t.Fatal(err)
	}

	// WorkDirs for stateful tests.
	bkDir := setupBackupWorkDir(t)
	bkSufDir := setupBackupWorkDir(t)
	cmpDir := setupCompareWorkDir(t)

	// Unreadable source for permission denied test.
	unreadable := createFixtureWithMode(
		t, fix, "noperm.txt", "secret\n", 0o000)

	tests := []testutils.DiffTest{
		// --- R3.1: Core copy modes ---
		{
			Name: "basic_copy",
			Args: []string{
				src1, filepath.Join(fix, "out_basic.txt"),
			},
		},
		{
			Name: "dir_creation_d",
			Args: []string{
				"-d", filepath.Join(fix, "newdir", "sub"),
			},
		},
		{
			Name: "dir_creation_d_multiple",
			Args: []string{
				"-d",
				filepath.Join(fix, "dA"),
				filepath.Join(fix, "dB", "dC"),
			},
		},
		{
			Name:    "copy_to_existing_dir",
			Args:    []string{src1, multiDest},
			WorkDir: fix,
		},
		{
			Name:    "multiple_sources_to_dir",
			Args:    []string{src1, src2, multiDest},
			WorkDir: fix,
		},
		{
			Name: "create_leading_D",
			Args: []string{
				"-D", src1,
				filepath.Join(fix, "deep", "nest", "out.txt"),
			},
		},

		// --- R3.2: Option flags ---
		// -m (mode)
		{
			Name: "mode_octal_644",
			Args: []string{
				"-m", "0644", src1,
				filepath.Join(fix, "out_m644.txt"),
			},
		},
		// -b (backup with default ~ suffix)
		{
			Name:    "backup_default_suffix_b",
			Args:    []string{"-b", "src.txt", "dest.txt"},
			WorkDir: bkDir,
		},
		// -S (custom backup suffix)
		{
			Name: "backup_custom_suffix_S",
			Args: []string{
				"-b", "-S", ".bak", "src.txt", "dest.txt",
			},
			WorkDir: bkSufDir,
		},
		// -C (compare, skip if identical)
		{
			Name:    "compare_identical_C",
			Args:    []string{"-C", "src.txt", "dest.txt"},
			WorkDir: cmpDir,
		},
		// -v (verbose copy)
		{
			Name: "verbose_copy_v",
			Args: []string{
				"-v", src1,
				filepath.Join(fix, "out_v.txt"),
			},
		},
		// -T (no-target-directory)
		{
			Name: "no_target_dir_T",
			Args: []string{
				"-T", src1,
				filepath.Join(fix, "out_T.txt"),
			},
		},
		// -t (target-directory)
		{
			Name: "target_dir_t",
			Args: []string{"-t", tDest, src1},
		},

		// --- R3.3: Error cases ---
		{
			Name:      "err_no_args",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{dropAll},
		},
		{
			Name:      "err_missing_dest_operand",
			Args:      []string{src1},
			Normalize: []testutils.NormalizeFunc{dropAll},
		},
		{
			Name: "err_missing_source",
			Args: []string{
				filepath.Join(fix, "nonexistent"),
				filepath.Join(fix, "out_miss.txt"),
			},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			Name: "err_permission_denied",
			Args: []string{
				unreadable,
				filepath.Join(fix, "out_perm.txt"),
			},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			Name: "err_conflict_t_and_T",
			Args: []string{
				"-t", tDest, "-T", src1,
				filepath.Join(fix, "out_ct.txt"),
			},
			Normalize: []testutils.NormalizeFunc{dropAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestInstallDefaultPermissions verifies AC3: default 0755 permissions.
func TestInstallDefaultPermissions(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	writeTestFile(t, srcFile, "hello\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")

	cmd := exec.Command(goBin, srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("cannot stat dest: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o755 {
		t.Errorf("permissions = %o, want 0755", got)
	}

	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("cannot read dest: %v", err)
	}
	if string(content) != "hello\n" {
		t.Errorf("content = %q, want %q", string(content), "hello\n")
	}
}

// TestInstallCustomMode verifies -m flag sets permissions.
func TestInstallCustomMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	writeTestFile(t, srcFile, "data\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")

	cmd := exec.Command(goBin, "-m", "0644", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -m failed: %v\noutput: %s", err, out)
	}

	info, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("cannot stat dest: %v", err)
	}
	got := info.Mode().Perm()
	if got != 0o644 {
		t.Errorf("permissions = %o, want 0644", got)
	}
}

// TestInstallDirMode verifies -d creates directories.
func TestInstallDirMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "a", "b", "c")
	dir2 := filepath.Join(tmpDir, "x", "y")

	cmd := exec.Command(goBin, "-d", dir1, dir2)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -d failed: %v\noutput: %s", err, out)
	}

	assertDirExists(t, dir1)
	assertDirExists(t, dir2)
}

// TestInstallCopyToDirectory verifies copy to existing directory.
func TestInstallCopyToDirectory(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	writeTestFile(t, srcFile, "content\n", 0o644)
	destDir := filepath.Join(tmpDir, "destdir")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, srcFile, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install to dir failed: %v\noutput: %s", err, out)
	}

	expected := filepath.Join(destDir, "source.txt")
	assertFileContent(t, expected, "content\n")

	info, err := os.Stat(expected)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
	}
}

// TestInstallMultipleSources verifies multiple source files to directory.
func TestInstallMultipleSources(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	src1 := filepath.Join(tmpDir, "a.txt")
	src2 := filepath.Join(tmpDir, "b.txt")
	writeTestFile(t, src1, "aaa\n", 0o644)
	writeTestFile(t, src2, "bbb\n", 0o644)
	destDir := filepath.Join(tmpDir, "out")
	if err := os.Mkdir(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, src1, src2, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install multiple failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, filepath.Join(destDir, "a.txt"), "aaa\n")
	assertFileContent(t, filepath.Join(destDir, "b.txt"), "bbb\n")
}

// TestInstallExitCodeOnError verifies exit 1 on failure.
func TestInstallExitCodeOnError(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "/nonexistent/source", "/tmp/dest")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code for missing source")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}

// TestInstallCreateLeadingDirs verifies -D creates parent directories.
func TestInstallCreateLeadingDirs(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	writeTestFile(t, srcFile, "data\n", 0o644)
	destFile := filepath.Join(tmpDir, "deep", "nested", "dest.txt")

	cmd := exec.Command(goBin, "-D", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -D failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, destFile, "data\n")
}

// TestInstallBackupCreatesFile verifies -b creates a backup file.
func TestInstallBackupCreatesFile(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	writeTestFile(t, srcFile, "new\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")
	writeTestFile(t, destFile, "old\n", 0o644)

	cmd := exec.Command(goBin, "-b", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -b failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, destFile, "new\n")
	assertFileContent(t, destFile+"~", "old\n")
}

// TestInstallBackupCustomSuffix verifies -S sets backup suffix.
func TestInstallBackupCustomSuffix(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	writeTestFile(t, srcFile, "new\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")
	writeTestFile(t, destFile, "old\n", 0o644)

	cmd := exec.Command(goBin, "-b", "-S", ".bak", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -b -S failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, destFile, "new\n")
	assertFileContent(t, destFile+".bak", "old\n")
}

// TestInstallCompareSkips verifies -C skips identical files.
func TestInstallCompareSkips(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	writeTestFile(t, srcFile, "same\n", 0o644)
	destFile := filepath.Join(tmpDir, "dest.txt")
	writeTestFile(t, destFile, "same\n", 0o755)
	if err := os.Chmod(destFile, 0o755); err != nil {
		t.Fatal(err)
	}

	// Record original modification time.
	origInfo, err := os.Stat(destFile)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "-C", srcFile, destFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -C failed: %v\noutput: %s", err, out)
	}

	// Dest should not have been rewritten; mtime unchanged.
	newInfo, err := os.Stat(destFile)
	if err != nil {
		t.Fatal(err)
	}
	if !newInfo.ModTime().Equal(origInfo.ModTime()) {
		t.Error("-C should have skipped identical file")
	}
}

// TestInstallTargetDir verifies -t flag copies into target directory.
func TestInstallTargetDir(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "src.txt")
	writeTestFile(t, srcFile, "data\n", 0o644)
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(goBin, "-t", targetDir, srcFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install -t failed: %v\noutput: %s", err, out)
	}

	assertFileContent(t, filepath.Join(targetDir, "src.txt"), "data\n")
}

// writeTestFile creates a file with the given content and permissions.
func writeTestFile(
	t *testing.T, path, content string, perm os.FileMode,
) {
	t.Helper()
	if err := os.WriteFile(
		path, []byte(content), perm,
	); err != nil {
		t.Fatalf("cannot write test file %s: %v", path, err)
	}
}

// assertDirExists checks that a directory exists.
func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("directory %s does not exist: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", path)
	}
}

// assertFileContent checks that a file has the expected content.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("cannot read %s: %v", path, err)
		return
	}
	if string(got) != want {
		t.Errorf("content of %s = %q, want %q",
			path, string(got), want)
	}
}
