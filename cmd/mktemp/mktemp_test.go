// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential and structural tests for cmd/mktemp.
// Implements prd036-mktemp R4.1-R4.4: differential testing against gmktemp.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeNonEmpty replaces any non-empty output with a fixed string.
// R4.4: mktemp output is non-deterministic (random names), so stdout
// cannot be compared byte-for-byte between Go and reference binaries.
// This normalizer allows RunDiffTests to compare exit codes while
// accepting that both stdout and stderr content will differ.
func normalizeNonEmpty(b []byte) []byte {
	if len(bytes.TrimSpace(b)) == 0 {
		return b
	}
	return []byte("NORMALIZED\n")
}

// TestDiff runs differential tests comparing the Go binary against gmktemp.
// R4.1: compares exit codes and structural properties.
// R4.4: does not compare exact filenames.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not in PATH")
	}

	normalize := []testutils.NormalizeFunc{normalizeNonEmpty}

	tests := []testutils.DiffTest{
		{
			Name:      "default_file",
			Args:      []string{},
			Normalize: normalize,
		},
		{
			Name:      "directory_mode",
			Args:      []string{"-d"},
			Normalize: normalize,
		},
		{
			Name:      "custom_template",
			Args:      []string{"myfile.XXXXXX"},
			Normalize: normalize,
		},
		{
			Name:      "too_few_xs",
			Args:      []string{"ab"},
			Normalize: normalize,
		},
		{
			Name:      "no_trailing_xs",
			Args:      []string{"noXatend"},
			Normalize: normalize,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestStructural verifies structural properties of mktemp output.
// R4.2: output is a valid path, file/dir exists, name matches template,
// permission bits match specification.
func TestStructural(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("default_creates_file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := runMktemp(t, goBin, tmpDir)

		// R1.1: file exists in TMPDIR.
		info := statOrFail(t, path)
		if info.IsDir() {
			t.Error("expected file, got directory")
		}
		// R1.4: permission 0600.
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("expected mode 0600, got %04o", perm)
		}
		// R1.2: default template prefix is "tmp.".
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "tmp.") {
			t.Errorf("expected prefix 'tmp.', got %s", base)
		}
		// R1.2: 10 random chars after prefix.
		suffix := strings.TrimPrefix(base, "tmp.")
		if len(suffix) != 10 {
			t.Errorf("expected 10 random chars, got %d", len(suffix))
		}
	})

	t.Run("directory_creates_dir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := runMktemp(t, goBin, tmpDir, "-d")

		// R2.1: created entry is a directory.
		info := statOrFail(t, path)
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}
		// R2.2: permission 0700.
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("expected mode 0700, got %04o", perm)
		}
	})

	t.Run("custom_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := runMktemp(t, goBin, tmpDir, "myapp.XXXXXX")

		// R1.3: name matches template pattern.
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "myapp.") {
			t.Errorf("expected prefix 'myapp.', got %s", base)
		}
		suffix := strings.TrimPrefix(base, "myapp.")
		if len(suffix) != 6 {
			t.Errorf("expected 6 random chars, got %d", len(suffix))
		}
		statOrFail(t, path)
	})

	t.Run("template_with_three_xs", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := runMktemp(t, goBin, tmpDir, "minXXX")

		// R1.4: minimum 3 Xs is accepted.
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "min") {
			t.Errorf("expected prefix 'min', got %s", base)
		}
		statOrFail(t, path)
	})

	t.Run("too_few_xs_fails", func(t *testing.T) {
		t.Parallel()
		// R1.5: exit 1 with diagnostic to stderr.
		cmd := exec.Command(goBin, "abcXX")
		cmd.Env = buildEnv(t.TempDir())
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected failure for template with too few Xs")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error type: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit 1, got %d", exitErr.ExitCode())
		}
		// R1.5: diagnostic to stderr.
		if stderr.Len() == 0 {
			t.Error("expected stderr diagnostic, got empty")
		}
	})

	t.Run("absolute_path_in_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := runMktemp(t, goBin, tmpDir)

		// R1.1: path is in the TMPDIR directory.
		if dir := filepath.Dir(path); dir != tmpDir {
			t.Errorf("expected dir %s, got %s", tmpDir, dir)
		}
	})
}

// runMktemp executes the Go mktemp binary with TMPDIR set and returns
// the trimmed stdout output.
func runMktemp(t *testing.T, bin, tmpDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = buildEnv(tmpDir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("mktemp %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// buildEnv constructs an environment with TMPDIR and LC_ALL=C set.
func buildEnv(tmpDir string) []string {
	env := os.Environ()
	env = append(env, "TMPDIR="+tmpDir, "LC_ALL=C")
	return env
}

// statOrFail calls os.Stat and fails the test if the path does not exist.
func statOrFail(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected path to exist: %v", err)
	}
	return info
}
