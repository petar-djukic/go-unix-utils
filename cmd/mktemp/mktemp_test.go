// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R4.1–R4.4: differential and structural tests for
// mktemp R1.1–R1.5 (core behavior, template validation), R2.1–R2.3
// (directory mode), R3.1–R3.6 (suffix mode, quiet flag, tmpdir mode,
// -t legacy mode, -u dry-run).
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestStructural verifies structural properties of mktemp output without
// requiring the reference binary. Covers R1.1–R1.5, R2.1–R2.3.
func TestStructural(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("default_creates_file_in_tmpdir", func(t *testing.T) {
		t.Parallel()
		path := runAndCapture(t, goBin)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyDefaultPattern(t, filepath.Base(path))
		verifyPermissions(t, path, 0o600)
	})

	t.Run("custom_template_in_cwd", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := "myapp.XXXXX"
		path := runAndCaptureInDir(t, goBin, tmpDir, template)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, tmpDir)
		verifyPatternWithSuffix(t, filepath.Base(path), "myapp.", 5, "")
		verifyPermissions(t, path, 0o600)
	})

	t.Run("custom_template_10_xs", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := "test.XXXXXXXXXX"
		path := runAndCaptureInDir(t, goBin, tmpDir, template)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPatternWithSuffix(t, filepath.Base(path), "test.", 10, "")
	})

	t.Run("error_too_few_xs", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, os.TempDir(), "foo.XX")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "too few X's") {
			t.Fatalf("expected 'too few X's' in stderr, got: %s", stderr)
		}
	})

	t.Run("error_nonexistent_parent", func(t *testing.T) {
		t.Parallel()
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		template := filepath.Join(badDir, "tmp.XXXXXXXXXX")
		code, _, stderr := runBinary(t, goBin, "", template)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if stderr == "" {
			t.Fatal("expected error on stderr, got empty")
		}
	})

	t.Run("no_trailing_xs_rejected", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, os.TempDir(), "noXatend")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "too few X's") {
			t.Fatalf("expected 'too few X's' in stderr, got: %s", stderr)
		}
	})
}

// TestStructuralDirMode verifies structural properties of directory mode
// output. Covers R2.1–R2.3.
func TestStructuralDirMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("d_flag_creates_directory", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "-d")
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyDefaultPattern(t, filepath.Base(path))
		verifyPermissions(t, path, 0o700)
	})

	t.Run("directory_long_flag_creates_directory", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "--directory")
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPermissions(t, path, 0o700)
	})

	t.Run("d_flag_custom_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "mydir.XXXXXX")
		path := runAndCaptureWithFlags(t, goBin, "-d", template)
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPathPrefix(t, path, tmpDir)
		verifyPatternWithSuffix(t, filepath.Base(path), "mydir.", 6, "")
		verifyPermissions(t, path, 0o700)
	})

	t.Run("d_flag_error_nonexistent_parent", func(t *testing.T) {
		t.Parallel()
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		template := filepath.Join(badDir, "tmp.XXXXXXXXXX")
		code, _, stderr := runBinary(t, goBin, "", "-d", template)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if stderr == "" {
			t.Fatal("expected error on stderr, got empty")
		}
	})

	t.Run("d_flag_error_too_few_xs", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, os.TempDir(), "-d", "foo.XX")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "too few X's") {
			t.Fatalf("expected 'too few X's' in stderr, got: %s", stderr)
		}
	})

	t.Run("d_flag_suffix_in_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "dir.XXXXXX.tmp")
		path := runAndCaptureWithFlags(t, goBin, "-d", template)
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPatternWithSuffix(t, filepath.Base(path), "dir.", 6, ".tmp")
		verifyPermissions(t, path, 0o700)
	})
}

// TestStructuralSuffix verifies suffix mode and --suffix flag behavior.
// Covers R3.3.
func TestStructuralSuffix(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("suffix_in_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "tmp.XXXXXX.txt")
		path := runAndCaptureWithFlags(t, goBin, template)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPatternWithSuffix(t, filepath.Base(path), "tmp.", 6, ".txt")
		verifyPermissions(t, path, 0o600)
	})

	t.Run("suffix_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "tmp.XXXXXX")
		path := runAndCaptureWithFlags(t, goBin, "--suffix=.log", template)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPatternWithSuffix(t, filepath.Base(path), "tmp.", 6, ".log")
	})

	t.Run("suffix_flag_overrides_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "tmp.XXXXXX.txt")
		path := runAndCaptureWithFlags(t, goBin, "--suffix=.log", template)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPatternWithSuffix(t, filepath.Base(path), "tmp.", 6, ".log")
	})

	t.Run("suffix_flag_default_template", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "--suffix=.dat")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPatternWithSuffix(t, filepath.Base(path), "tmp.", 10, ".dat")
	})
}

// TestStructuralQuiet verifies -q/--quiet flag behavior.
// Covers R3.6.
func TestStructuralQuiet(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("quiet_suppresses_stderr_on_bad_template", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, os.TempDir(), "-q", "foo.XX")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr with -q, got: %s", stderr)
		}
	})

	t.Run("quiet_long_flag_suppresses_stderr", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, os.TempDir(),
			"--quiet", "foo.XX")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr with --quiet, got: %s", stderr)
		}
	})

	t.Run("quiet_suppresses_stderr_on_creation_failure", func(t *testing.T) {
		t.Parallel()
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		template := filepath.Join(badDir, "tmp.XXXXXXXXXX")
		code, _, stderr := runBinary(t, goBin, "", "-q", template)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr with -q, got: %s", stderr)
		}
	})
}

// TestStructuralTmpdir verifies --tmpdir and -p flag behavior.
// Covers R3.5 (--tmpdir without value) and R3.6 (--tmpdir=DIR).
func TestStructuralTmpdir(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("tmpdir_no_value_uses_tmpdir_env", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "--tmpdir")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyDefaultPattern(t, filepath.Base(path))
	})

	t.Run("tmpdir_with_value_uses_dir", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin, "--tmpdir="+customDir)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyDefaultPattern(t, filepath.Base(path))
	})

	t.Run("p_flag_uses_dir", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin, "-p", customDir)
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyDefaultPattern(t, filepath.Base(path))
	})

	t.Run("tmpdir_with_custom_template", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin,
			"--tmpdir="+customDir, "myapp.XXXXXX")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyPatternWithSuffix(t, filepath.Base(path), "myapp.", 6, "")
	})

	t.Run("p_flag_with_custom_template", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin,
			"-p", customDir, "myapp.XXXXXX")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyPatternWithSuffix(t, filepath.Base(path), "myapp.", 6, "")
	})

	t.Run("tmpdir_with_d_flag", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin,
			"-d", "--tmpdir="+customDir)
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyPermissions(t, path, 0o700)
	})

	t.Run("p_flag_with_d_flag", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin,
			"-d", "-p", customDir)
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyPermissions(t, path, 0o700)
	})

	t.Run("tmpdir_with_suffix_flag", func(t *testing.T) {
		t.Parallel()
		customDir := t.TempDir()
		path := runAndCaptureWithFlags(t, goBin,
			"--tmpdir="+customDir, "--suffix=.log")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, customDir)
		verifyPatternWithSuffix(t, filepath.Base(path), "tmp.", 10, ".log")
	})

	t.Run("tmpdir_nonexistent_dir_fails", func(t *testing.T) {
		t.Parallel()
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		code, _, stderr := runBinary(t, goBin, "",
			"--tmpdir="+badDir)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if stderr == "" {
			t.Fatal("expected error on stderr, got empty")
		}
	})
}

// TestStructuralTFlag verifies -t legacy mode behavior.
// R3.4: -t treats template as a filename prefix in TMPDIR. R4.3 coverage.
func TestStructuralTFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("t_flag_default_template", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "-t")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyDefaultPattern(t, filepath.Base(path))
		verifyPermissions(t, path, 0o600)
	})

	t.Run("t_flag_custom_template", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "-t", "myapp.XXXXXX")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyPatternWithSuffix(t, filepath.Base(path), "myapp.", 6, "")
	})

	t.Run("t_flag_with_d_flag", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin, "-t", "-d")
		defer os.RemoveAll(path)
		verifyDirExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyPermissions(t, path, 0o700)
	})

	t.Run("t_flag_with_suffix", func(t *testing.T) {
		t.Parallel()
		path := runAndCaptureWithFlags(t, goBin,
			"-t", "--suffix=.log", "app.XXXXXX")
		defer os.Remove(path)
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyPatternWithSuffix(t, filepath.Base(path), "app.", 6, ".log")
	})
}

// TestStructuralDryRun verifies -u/--dry-run mode behavior.
// R3.5: prints name without creating, with warning. R4.3 coverage.
func TestStructuralDryRun(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("u_flag_prints_path_no_file", func(t *testing.T) {
		t.Parallel()
		code, stdout, _ := runBinary(t, goBin, "", "-u")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		path := strings.TrimSpace(stdout)
		if path == "" {
			t.Fatal("expected path on stdout")
		}
		verifyNotExists(t, path)
		verifyPathPrefix(t, path, os.TempDir())
		verifyDefaultPattern(t, filepath.Base(path))
	})

	t.Run("dry_run_long_flag", func(t *testing.T) {
		t.Parallel()
		code, stdout, _ := runBinary(t, goBin, "", "--dry-run")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		path := strings.TrimSpace(stdout)
		verifyNotExists(t, path)
	})

	t.Run("u_flag_with_d_flag_no_dir", func(t *testing.T) {
		t.Parallel()
		code, stdout, _ := runBinary(t, goBin, "", "-u", "-d")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		path := strings.TrimSpace(stdout)
		verifyNotExists(t, path)
	})

	t.Run("u_flag_custom_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "app.XXXXXX")
		code, stdout, _ := runBinary(t, goBin, "", "-u", template)
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		path := strings.TrimSpace(stdout)
		verifyNotExists(t, path)
		verifyPathPrefix(t, path, tmpDir)
		verifyPatternWithSuffix(t, filepath.Base(path), "app.", 6, "")
	})

	t.Run("u_flag_prints_warning", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, "", "-u")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d", code)
		}
		if !strings.Contains(stderr, "dry-run") {
			t.Fatalf("expected dry-run warning on stderr, got: %s", stderr)
		}
	})

	t.Run("u_flag_error_too_few_xs", func(t *testing.T) {
		t.Parallel()
		code, _, stderr := runBinary(t, goBin, os.TempDir(), "-u", "foo.XX")
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		if !strings.Contains(stderr, "too few X's") {
			t.Fatalf("expected 'too few X's' in stderr, got: %s", stderr)
		}
	})
}

// TestDiff runs differential tests against gmktemp for error cases where
// exit codes should match. Success cases use structural comparison (R4.4).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	t.Run("default_both_succeed", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceedFile(t, goBin, refBin, nil)
	})

	t.Run("custom_template_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "app.XXXXXXX")
		verifyBothSucceedFile(t, goBin, refBin, []string{template})
	})

	t.Run("error_too_few_xs_exit_code", func(t *testing.T) {
		t.Parallel()
		verifyBothFail(t, goBin, refBin, []string{"foo.XX"})
	})

	t.Run("d_flag_both_succeed", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceedDir(t, goBin, refBin, []string{"-d"})
	})

	t.Run("d_flag_custom_template_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "dtest.XXXXXXX")
		verifyBothSucceedDir(t, goBin, refBin, []string{"-d", template})
	})

	t.Run("d_flag_error_too_few_xs", func(t *testing.T) {
		t.Parallel()
		verifyBothFail(t, goBin, refBin, []string{"-d", "foo.XX"})
	})

	t.Run("suffix_in_template_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "app.XXXXXX.txt")
		verifyBothSucceedFile(t, goBin, refBin, []string{template})
	})

	t.Run("suffix_flag_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "app.XXXXXX")
		verifyBothSucceedFile(t, goBin, refBin,
			[]string{"--suffix=.log", template})
	})

	t.Run("quiet_error_both_fail", func(t *testing.T) {
		t.Parallel()
		verifyBothFail(t, goBin, refBin, []string{"-q", "foo.XX"})
	})

	// R4.1: differential tests for --tmpdir mode.
	t.Run("tmpdir_no_value_both_succeed", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceedFile(t, goBin, refBin, []string{"--tmpdir"})
	})

	t.Run("tmpdir_with_dir_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		verifyBothSucceedFile(t, goBin, refBin,
			[]string{"--tmpdir=" + tmpDir})
	})

	t.Run("p_flag_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		verifyBothSucceedFile(t, goBin, refBin,
			[]string{"-p", tmpDir})
	})

	t.Run("tmpdir_with_template_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		verifyBothSucceedFile(t, goBin, refBin,
			[]string{"--tmpdir=" + tmpDir, "app.XXXXXX"})
	})

	// R4.2: differential tests for flag combinations with --tmpdir.
	t.Run("tmpdir_with_d_flag_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		verifyBothSucceedDir(t, goBin, refBin,
			[]string{"-d", "--tmpdir=" + tmpDir})
	})

	t.Run("p_flag_with_d_flag_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		verifyBothSucceedDir(t, goBin, refBin,
			[]string{"-d", "-p", tmpDir})
	})

	t.Run("tmpdir_with_suffix_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		verifyBothSucceedFile(t, goBin, refBin,
			[]string{"--tmpdir=" + tmpDir, "--suffix=.log"})
	})

	t.Run("tmpdir_with_quiet_error_both_fail", func(t *testing.T) {
		t.Parallel()
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		verifyBothFail(t, goBin, refBin,
			[]string{"-q", "--tmpdir=" + badDir})
	})

	// R4.3: differential tests for -t legacy mode.
	t.Run("t_flag_both_succeed", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceedFile(t, goBin, refBin, []string{"-t", "app.XXXXXX"})
	})

	t.Run("t_flag_with_d_both_succeed", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceedDir(t, goBin, refBin,
			[]string{"-t", "-d", "app.XXXXXX"})
	})

	// R4.3: differential tests for -u dry-run mode.
	t.Run("u_flag_both_succeed_no_file", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceedDryRun(t, goBin, refBin, []string{"-u"})
	})

	t.Run("u_flag_with_template_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "dry.XXXXXX")
		verifyBothSucceedDryRun(t, goBin, refBin, []string{"-u", template})
	})

	t.Run("u_flag_error_too_few_xs", func(t *testing.T) {
		t.Parallel()
		verifyBothFail(t, goBin, refBin, []string{"-u", "foo.XX"})
	})
}

// runAndCapture runs the binary with no args and returns the trimmed stdout.
func runAndCapture(t *testing.T, bin string) string {
	t.Helper()
	code, stdout, stderr := runBinary(t, bin, "")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	return strings.TrimSpace(stdout)
}

// runAndCaptureInDir runs the binary with a template in a specific directory.
func runAndCaptureInDir(t *testing.T, bin, dir, template string) string {
	t.Helper()
	fullTemplate := filepath.Join(dir, template)
	code, stdout, stderr := runBinary(t, bin, "", fullTemplate)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	return strings.TrimSpace(stdout)
}

// runAndCaptureWithFlags runs the binary with arbitrary flags and returns
// the trimmed stdout path.
func runAndCaptureWithFlags(t *testing.T, bin string, args ...string) string {
	t.Helper()
	code, stdout, stderr := runBinary(t, bin, "", args...)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	return strings.TrimSpace(stdout)
}

// runBinary executes the binary with optional args and returns exit code,
// stdout, and stderr.
func runBinary(t *testing.T, bin, workDir string, args ...string) (int, string, string) {
	t.Helper()
	var filtered []string
	for _, a := range args {
		if a != "" {
			filtered = append(filtered, a)
		}
	}
	cmd := exec.Command(bin, filtered...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute %s: %v", bin, err)
		}
	}
	return exitCode, stdout.String(), stderr.String()
}

// verifyFileExists checks that the path exists and is a regular file.
func verifyFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file to exist at %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected regular file at %s, got mode %v", path, info.Mode())
	}
}

// verifyDirExists checks that the path exists and is a directory.
func verifyDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory to exist at %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory at %s, got mode %v", path, info.Mode())
	}
}

// verifyNotExists checks that the path does not exist (for dry-run tests).
func verifyNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("expected path %s to not exist, but it does", path)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking path %s: %v", path, err)
	}
}

// verifyPathPrefix checks that the path starts with the expected directory.
func verifyPathPrefix(t *testing.T, path, expectedDir string) {
	t.Helper()
	dir := filepath.Dir(path)
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedExpected, _ := filepath.EvalSymlinks(expectedDir)
	if resolvedDir != resolvedExpected {
		t.Fatalf("expected path in %s, got %s", resolvedExpected, resolvedDir)
	}
}

// verifyDefaultPattern checks that the filename matches tmp.[a-zA-Z0-9]{10}.
func verifyDefaultPattern(t *testing.T, basename string) {
	t.Helper()
	verifyPatternWithSuffix(t, basename, "tmp.", 10, "")
}

// verifyPatternWithSuffix checks that the filename has the expected prefix
// followed by exactly n alphanumeric characters and then the suffix.
func verifyPatternWithSuffix(t *testing.T, basename, prefix string, n int, suffix string) {
	t.Helper()
	pattern := "^" + regexp.QuoteMeta(prefix) +
		fmt.Sprintf("[a-zA-Z0-9]{%d}", n) +
		regexp.QuoteMeta(suffix) + "$"
	matched, err := regexp.MatchString(pattern, basename)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Fatalf("filename %q does not match pattern %s", basename, pattern)
	}
}

// verifyPermissions checks that the file/dir has the expected permission bits.
func verifyPermissions(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	got := info.Mode().Perm()
	if got != expected {
		t.Fatalf("expected permissions %o, got %o for %s", expected, got, path)
	}
}

// verifyBothSucceedFile runs both binaries and checks they both exit 0 and
// produce valid file paths. Does not compare exact filenames (R4.4).
func verifyBothSucceedFile(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goCode, goStdout, goStderr := runBinary(t, goBin, "", args...)
	refCode, refStdout, _ := runBinary(t, refBin, "", args...)
	if refCode != 0 {
		t.Skipf("reference binary failed with exit %d", refCode)
	}
	if goCode != 0 {
		t.Fatalf("go binary failed with exit %d; stderr: %s", goCode, goStderr)
	}
	goPath := strings.TrimSpace(goStdout)
	refPath := strings.TrimSpace(refStdout)
	defer os.Remove(goPath)
	defer os.Remove(refPath)
	verifyFileExists(t, goPath)
	verifyFileExists(t, refPath)
}

// verifyBothSucceedDir runs both binaries and checks they both exit 0 and
// produce valid directory paths. Does not compare exact names (R4.4).
func verifyBothSucceedDir(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goCode, goStdout, goStderr := runBinary(t, goBin, "", args...)
	refCode, refStdout, _ := runBinary(t, refBin, "", args...)
	if refCode != 0 {
		t.Skipf("reference binary failed with exit %d", refCode)
	}
	if goCode != 0 {
		t.Fatalf("go binary failed with exit %d; stderr: %s", goCode, goStderr)
	}
	goPath := strings.TrimSpace(goStdout)
	refPath := strings.TrimSpace(refStdout)
	defer os.RemoveAll(goPath)
	defer os.RemoveAll(refPath)
	verifyDirExists(t, goPath)
	verifyDirExists(t, refPath)
}

// verifyBothSucceedDryRun runs both binaries with -u and checks both exit 0
// and produce stdout output without creating files. R4.3, R4.4.
func verifyBothSucceedDryRun(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goCode, goStdout, goStderr := runBinary(t, goBin, "", args...)
	refCode, refStdout, _ := runBinary(t, refBin, "", args...)
	if refCode != 0 {
		t.Skipf("reference binary failed with exit %d", refCode)
	}
	if goCode != 0 {
		t.Fatalf("go binary failed with exit %d; stderr: %s", goCode, goStderr)
	}
	goPath := strings.TrimSpace(goStdout)
	refPath := strings.TrimSpace(refStdout)
	if goPath == "" {
		t.Fatal("go binary produced no stdout for dry-run")
	}
	if refPath == "" {
		t.Fatal("ref binary produced no stdout for dry-run")
	}
	verifyNotExists(t, goPath)
	verifyNotExists(t, refPath)
}

// verifyBothFail runs both binaries and checks they both exit with non-zero.
func verifyBothFail(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goCode, _, _ := runBinary(t, goBin, "", args...)
	refCode, _, _ := runBinary(t, refBin, "", args...)
	if refCode == 0 {
		t.Skipf("reference binary unexpectedly succeeded")
	}
	if goCode != refCode {
		t.Fatalf("exit code mismatch: go=%d, ref=%d", goCode, refCode)
	}
}
