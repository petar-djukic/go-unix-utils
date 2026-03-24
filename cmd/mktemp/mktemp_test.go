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

// normalizeAlways replaces any output (including empty) with a fixed string.
// Used for tests where stderr content is expected to differ between
// Go and reference binaries (e.g., dry-run warning messages).
func normalizeAlways(b []byte) []byte {
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
	normalizeAll := []testutils.NormalizeFunc{normalizeAlways}

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
		{
			// R3.1: -p with explicit directory.
			Name:      "p_flag_explicit_dir",
			Args:      []string{"-p", t.TempDir(), "test.XXXXXX"},
			Normalize: normalize,
		},
		{
			// R3.1: --tmpdir= with explicit directory.
			Name:      "tmpdir_eq_explicit_dir",
			Args:      []string{"--tmpdir=" + t.TempDir(), "test.XXXXXX"},
			Normalize: normalize,
		},
		{
			// R3.2: --tmpdir without value uses TMPDIR.
			Name:      "tmpdir_no_value",
			Args:      []string{"--tmpdir", "test.XXXXXX"},
			Normalize: normalize,
		},
		{
			// R3.3: --suffix appends after random chars.
			Name:      "suffix_flag",
			Args:      []string{"--suffix=.txt"},
			Normalize: normalize,
		},
		{
			// R3.4: -t treats template as name in TMPDIR.
			Name:      "t_flag_legacy",
			Args:      []string{"-t", "myprefix.XXXXXX"},
			Normalize: normalize,
		},
		{
			// R3.3: suffix with directory separator must fail.
			Name:      "suffix_with_slash",
			Args:      []string{"--suffix=/bad"},
			Normalize: normalize,
		},
		{
			// R3.5: -u dry-run prints name without creating.
			Name:      "dry_run",
			Args:      []string{"-u"},
			Normalize: normalizeAll,
		},
		{
			// R3.5: -u with -d dry-run directory mode.
			Name:      "dry_run_directory",
			Args:      []string{"-u", "-d"},
			Normalize: normalizeAll,
		},
		{
			// R3.6: -q suppresses creation error messages.
			Name:      "quiet_creation_failure",
			Args:      []string{"-q", "-p", "/nonexistent/dir", "test.XXXXXX"},
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

	t.Run("directory_prints_path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R2.3: must print path of created directory to stdout.
		path := runMktemp(t, goBin, tmpDir, "-d")
		if dir := filepath.Dir(path); dir != tmpDir {
			t.Errorf("expected dir %s, got %s", tmpDir, dir)
		}
		info := statOrFail(t, path)
		if !info.IsDir() {
			t.Error("expected directory, got file")
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
		assertFailure(t, goBin, t.TempDir(), "abcXX")
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

	t.Run("p_flag_overrides_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		pDir := t.TempDir()
		// R3.1: -p DIR overrides TMPDIR.
		path := runMktempWithEnvAndArgs(t, goBin, tmpDir,
			[]string{"-p", pDir, "test.XXXXXX"})
		if dir := filepath.Dir(path); dir != pDir {
			t.Errorf("expected dir %s, got %s", pDir, dir)
		}
		statOrFail(t, path)
	})

	t.Run("tmpdir_eq_overrides_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		pDir := t.TempDir()
		// R3.1: --tmpdir=DIR overrides TMPDIR.
		path := runMktempWithEnvAndArgs(t, goBin, tmpDir,
			[]string{"--tmpdir=" + pDir, "test.XXXXXX"})
		if dir := filepath.Dir(path); dir != pDir {
			t.Errorf("expected dir %s, got %s", pDir, dir)
		}
		statOrFail(t, path)
	})

	t.Run("tmpdir_no_value_uses_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.2: --tmpdir without value uses TMPDIR.
		path := runMktempWithEnvAndArgs(t, goBin, tmpDir,
			[]string{"--tmpdir", "test.XXXXXX"})
		if dir := filepath.Dir(path); dir != tmpDir {
			t.Errorf("expected dir %s, got %s", tmpDir, dir)
		}
		statOrFail(t, path)
	})

	t.Run("suffix_appended", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.3: --suffix=SUFF appends after random chars.
		path := runMktemp(t, goBin, tmpDir, "--suffix=.txt")
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".txt") {
			t.Errorf("expected suffix '.txt', got %s", base)
		}
		statOrFail(t, path)
	})

	t.Run("suffix_with_custom_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.3: suffix with custom template.
		path := runMktemp(t, goBin, tmpDir, "--suffix=.log", "app.XXXXXX")
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "app.") {
			t.Errorf("expected prefix 'app.', got %s", base)
		}
		if !strings.HasSuffix(base, ".log") {
			t.Errorf("expected suffix '.log', got %s", base)
		}
		statOrFail(t, path)
	})

	t.Run("suffix_with_slash_fails", func(t *testing.T) {
		t.Parallel()
		// R3.3: suffix must not contain directory separator.
		assertFailure(t, goBin, t.TempDir(), "--suffix=/bad")
	})

	t.Run("t_flag_uses_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.4: -t forces template into TMPDIR.
		path := runMktemp(t, goBin, tmpDir, "-t", "myprefix.XXXXXX")
		if dir := filepath.Dir(path); dir != tmpDir {
			t.Errorf("expected dir %s, got %s", tmpDir, dir)
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "myprefix.") {
			t.Errorf("expected prefix 'myprefix.', got %s", base)
		}
		statOrFail(t, path)
	})

	t.Run("t_flag_with_p_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		pDir := t.TempDir()
		// R3.4 + R3.1: -p overrides -t's directory choice.
		path := runMktempWithEnvAndArgs(t, goBin, tmpDir,
			[]string{"-t", "-p", pDir, "foo.XXXXXX"})
		if dir := filepath.Dir(path); dir != pDir {
			t.Errorf("expected dir %s, got %s", pDir, dir)
		}
		statOrFail(t, path)
	})

	t.Run("d_with_suffix", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R2.1 + R3.3: directory mode with suffix.
		path := runMktemp(t, goBin, tmpDir, "-d", "--suffix=.dir")
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".dir") {
			t.Errorf("expected suffix '.dir', got %s", base)
		}
		info := statOrFail(t, path)
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}
		// R2.2: directory mode 0700.
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("expected mode 0700, got %04o", perm)
		}
	})

	t.Run("dry_run_no_file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.5: -u prints name without creating.
		path := runMktemp(t, goBin, tmpDir, "-u")
		if _, err := os.Stat(path); err == nil {
			t.Error("expected file not to exist in dry-run mode")
		}
	})

	t.Run("dry_run_prints_path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.5: must print the name to stdout.
		path := runMktemp(t, goBin, tmpDir, "-u")
		if dir := filepath.Dir(path); dir != tmpDir {
			t.Errorf("expected dir %s, got %s", tmpDir, dir)
		}
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "tmp.") {
			t.Errorf("expected prefix 'tmp.', got %s", base)
		}
	})

	t.Run("dry_run_prints_warning", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.5: must print a warning to stderr.
		cmd := exec.Command(goBin, "-u")
		cmd.Env = buildEnv(tmpDir)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("dry-run failed: %v", err)
		}
		if strings.TrimSpace(string(out)) == "" {
			t.Error("expected path on stdout")
		}
		if stderr.Len() == 0 {
			t.Error("expected warning on stderr")
		}
	})

	t.Run("dry_run_directory_no_dir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// R3.5 + R2.1: -u -d prints name without creating directory.
		path := runMktemp(t, goBin, tmpDir, "-u", "-d")
		if _, err := os.Stat(path); err == nil {
			t.Error("expected directory not to exist in dry-run mode")
		}
	})

	t.Run("quiet_suppresses_creation_error", func(t *testing.T) {
		t.Parallel()
		// R3.6: -q suppresses creation error messages.
		// Use a non-existent parent dir to trigger a creation error.
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		cmd := exec.Command(goBin, "-q", "-p", badDir, "test.XXXXXX")
		cmd.Env = buildEnv(t.TempDir())
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected failure")
		}
		if stderr.Len() != 0 {
			t.Errorf("expected no stderr with -q, got: %s",
				stderr.String())
		}
	})

	t.Run("quiet_exits_1", func(t *testing.T) {
		t.Parallel()
		// R3.6: -q still exits 1 on creation failure.
		badDir := filepath.Join(t.TempDir(), "nonexistent")
		cmd := exec.Command(goBin, "-q", "-p", badDir, "test.XXXXXX")
		cmd.Env = buildEnv(t.TempDir())
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected failure")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit 1, got %d", exitErr.ExitCode())
		}
	})
}

// runMktemp executes the Go mktemp binary with TMPDIR set and returns
// the trimmed stdout output.
func runMktemp(t *testing.T, bin, tmpDir string, args ...string) string {
	t.Helper()
	return runMktempWithEnvAndArgs(t, bin, tmpDir, args)
}

// runMktempWithEnvAndArgs runs mktemp with the given TMPDIR and args.
func runMktempWithEnvAndArgs(t *testing.T, bin, tmpDir string, args []string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = buildEnv(tmpDir)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = ee.Stderr
		}
		t.Fatalf("mktemp %v failed: %v\nstderr: %s", args, err, stderr)
	}
	return strings.TrimSpace(string(out))
}

// assertFailure runs mktemp and verifies it exits with status 1.
func assertFailure(t *testing.T, bin, tmpDir string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = buildEnv(tmpDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected failure, got success")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit 1, got %d", exitErr.ExitCode())
	}
	if stderr.Len() == 0 {
		t.Error("expected stderr diagnostic, got empty")
	}
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
