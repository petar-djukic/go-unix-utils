// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd036-mktemp R4.1–R4.2: differential and structural tests for
// core mktemp behavior (R1.1–R1.4).
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
// requiring the reference binary. Covers R1.1, R1.2, R1.3, R1.4.
func TestStructural(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("default_creates_file_in_tmpdir", func(t *testing.T) {
		t.Parallel()
		path := runAndCapture(t, goBin)
		defer os.Remove(path) // AC4: cleanup
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
		defer os.Remove(path) // AC4: cleanup
		verifyFileExists(t, path)
		verifyPathPrefix(t, path, tmpDir)
		verifyPattern(t, filepath.Base(path), "myapp.", 5)
		verifyPermissions(t, path, 0o600)
	})

	t.Run("custom_template_10_xs", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := "test.XXXXXXXXXX"
		path := runAndCaptureInDir(t, goBin, tmpDir, template)
		defer os.Remove(path) // AC4: cleanup
		verifyFileExists(t, path)
		verifyPattern(t, filepath.Base(path), "test.", 10)
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
		code, _, _ := runBinary(t, goBin, "", template)
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
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

// TestDiff runs differential tests against gmktemp for error cases where
// exit codes should match. Success cases use structural comparison (D3).
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	t.Run("default_both_succeed", func(t *testing.T) {
		t.Parallel()
		verifyBothSucceed(t, goBin, refBin, nil)
	})

	t.Run("custom_template_both_succeed", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "app.XXXXXXX")
		verifyBothSucceed(t, goBin, refBin, []string{template})
	})

	t.Run("error_too_few_xs_exit_code", func(t *testing.T) {
		t.Parallel()
		verifyBothFail(t, goBin, refBin, []string{"foo.XX"})
	})
}

// runAndCapture runs the binary with no args and returns the trimmed stdout.
func runAndCapture(t *testing.T, bin string) string {
	t.Helper()
	code, stdout, stderr := runBinary(t, bin, "", "")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	return strings.TrimSpace(stdout)
}

// runAndCaptureInDir runs the binary in a specific directory with the given
// template argument and returns the trimmed stdout path.
func runAndCaptureInDir(t *testing.T, bin, dir, template string) string {
	t.Helper()
	fullTemplate := filepath.Join(dir, template)
	code, stdout, stderr := runBinary(t, bin, "", fullTemplate)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	return strings.TrimSpace(stdout)
}

// runBinary executes the binary with optional args and returns exit code,
// stdout, and stderr. If workDir is empty, a temp dir is used.
func runBinary(t *testing.T, bin, workDir string, args ...string) (int, string, string) {
	t.Helper()
	// Filter out empty args.
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

// verifyPathPrefix checks that the path starts with the expected directory.
func verifyPathPrefix(t *testing.T, path, expectedDir string) {
	t.Helper()
	dir := filepath.Dir(path)
	// Resolve symlinks for comparison (e.g., /tmp → /private/tmp on macOS).
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedExpected, _ := filepath.EvalSymlinks(expectedDir)
	if resolvedDir != resolvedExpected {
		t.Fatalf("expected path in %s, got %s", resolvedExpected, resolvedDir)
	}
}

// verifyDefaultPattern checks that the filename matches tmp.[a-zA-Z0-9]{10}.
func verifyDefaultPattern(t *testing.T, basename string) {
	t.Helper()
	verifyPattern(t, basename, "tmp.", 10)
}

// verifyPattern checks that the filename has the expected prefix followed
// by exactly n alphanumeric characters.
func verifyPattern(t *testing.T, basename, prefix string, n int) {
	t.Helper()
	pattern := "^" + regexp.QuoteMeta(prefix) + "[a-zA-Z0-9]{" +
		strings.Repeat("", 0) + fmt.Sprintf("%d", n) + "}$"
	matched, err := regexp.MatchString(pattern, basename)
	if err != nil {
		t.Fatalf("regex error: %v", err)
	}
	if !matched {
		t.Fatalf("filename %q does not match pattern %s", basename, pattern)
	}
}

// verifyPermissions checks that the file has the expected permission bits.
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

// verifyBothSucceed runs both binaries and checks they both exit 0 and
// produce valid paths. Does not compare exact filenames (D3, R4.4).
func verifyBothSucceed(t *testing.T, goBin, refBin string, args []string) {
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
	defer os.Remove(goPath)  // AC4: cleanup
	defer os.Remove(refPath) // AC4: cleanup
	verifyFileExists(t, goPath)
	verifyFileExists(t, refPath)
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

