// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mktemp against gmktemp (GNU coreutils).
// Implements prd036-mktemp R1.1-R1.5, R2.1-R2.3, R3.1-R3.6 test coverage.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests for error cases and informational flags
// using RunDiffTests directly. These tests do not depend on random output.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.3: --version exits 0.
		{
			Name:      "version_exit_0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R1.3: --help exits 0.
		{
			Name:      "help_exit_0",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R1.4: template with fewer than 3 trailing Xs exits 1.
		{
			Name:      "too_few_Xs",
			Args:      []string{"foo.XX"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
		// R1.4: template with no Xs exits 1.
		{
			Name:      "no_Xs",
			Args:      []string{"noexes"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
		// R1.4: non-existent directory exits 1.
		{
			Name:      "bad_tmpdir",
			Args:      []string{},
			Env:       []string{"TMPDIR=/nonexistent/path/for/mktemp/test"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
		// R3.3: suffix containing a slash exits 1.
		{
			Name:      "suffix_with_slash",
			Args:      []string{"--suffix=/bad", "tmp.XXX"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
		// R3.1: --tmpdir with non-existent directory exits 1.
		{
			Name:      "tmpdir_nonexistent",
			Args:      []string{"--tmpdir=/nonexistent/dir/for/mktemp"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
		// R3.4: -t with template containing slash exits 1.
		{
			Name:      "t_flag_template_with_slash",
			Args:      []string{"-t", "sub/dir.XXX"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCreation runs structural differential tests for temp file creation.
// Since temp file names are random, we verify structural properties rather
// than exact output: exit code, file existence, path structure, and permissions.
// R4.1, R4.2, R4.4.
func TestDiffCreation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	tests := []struct {
		name        string
		args        []string
		useTMPDIR   bool           // if true, set TMPDIR to workDir; output is absolute
		pattern     *regexp.Regexp // pattern the output path must match
		permissions os.FileMode    // expected file permissions
		isDir       bool           // true if expecting a directory
	}{
		// R1.1, R1.2: default template creates file in TMPDIR.
		{
			name:        "default_template",
			args:        nil,
			useTMPDIR:   true,
			pattern:     regexp.MustCompile(`/tmp\.[A-Za-z0-9]{10}$`),
			permissions: 0o600,
		},
		// R1.3: custom template with trailing Xs creates in cwd.
		{
			name:        "custom_template",
			args:        []string{"myapp.XXXXXX"},
			pattern:     regexp.MustCompile(`myapp\.[A-Za-z0-9]{6}$`),
			permissions: 0o600,
		},
		// R1.3: template with exactly 3 Xs (minimum).
		{
			name:        "min_Xs_template",
			args:        []string{"test.XXX"},
			pattern:     regexp.MustCompile(`test\.[A-Za-z0-9]{3}$`),
			permissions: 0o600,
		},
		// R2.1-R2.3: -d creates a directory in TMPDIR with mode 0700.
		{
			name:        "directory_flag",
			args:        []string{"-d"},
			useTMPDIR:   true,
			pattern:     regexp.MustCompile(`/tmp\.[A-Za-z0-9]{10}$`),
			permissions: 0o700,
			isDir:       true,
		},
		// R2.1: -d with custom template.
		{
			name:        "directory_custom_template",
			args:        []string{"-d", "mydir.XXXXXX"},
			pattern:     regexp.MustCompile(`mydir\.[A-Za-z0-9]{6}$`),
			permissions: 0o700,
			isDir:       true,
		},
		// R3.3: --suffix appends after random characters.
		{
			name:        "suffix_flag",
			args:        []string{"--suffix=.txt", "tmp.XXX"},
			pattern:     regexp.MustCompile(`tmp\.[A-Za-z0-9]{3}\.txt$`),
			permissions: 0o600,
		},
		// R3.3: --suffix with -d.
		{
			name:        "suffix_with_directory",
			args:        []string{"-d", "--suffix=.d", "tmpd.XXX"},
			pattern:     regexp.MustCompile(`tmpd\.[A-Za-z0-9]{3}\.d$`),
			permissions: 0o700,
			isDir:       true,
		},
		// R3.4: -t treats template as filename prefix in TMPDIR.
		{
			name:        "t_flag_in_tmpdir",
			args:        []string{"-t", "myprefix.XXXXXX"},
			useTMPDIR:   true,
			pattern:     regexp.MustCompile(`myprefix\.[A-Za-z0-9]{6}$`),
			permissions: 0o600,
		},
		// R3.4: -t with -d creates directory in TMPDIR.
		{
			name:        "t_flag_with_directory",
			args:        []string{"-t", "-d", "mydir.XXXXXX"},
			useTMPDIR:   true,
			pattern:     regexp.MustCompile(`mydir\.[A-Za-z0-9]{6}$`),
			permissions: 0o700,
			isDir:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workDir := t.TempDir()
			var env []string
			if tc.useTMPDIR {
				env = buildTestEnvWithTMPDIR(workDir)
			} else {
				env = buildTestEnv()
			}

			// Run both binaries with the same workDir.
			refOut, refExit := runMktempInDir(t, refBin, tc.args, env, workDir)
			goOut, goExit := runMktempInDir(t, goBin, tc.args, env, workDir)

			// Exit codes must match.
			if refExit != goExit {
				t.Errorf("exit code mismatch: ref=%d go=%d", refExit, goExit)
			}
			if goExit != 0 {
				t.Fatalf("expected exit 0, got %d; output: %s", goExit, goOut)
			}

			// Both outputs must match the expected pattern.
			refPath := strings.TrimSpace(string(refOut))
			goPath := strings.TrimSpace(string(goOut))

			if !tc.pattern.MatchString(refPath) {
				t.Errorf("ref output %q does not match pattern %s", refPath, tc.pattern)
			}
			if !tc.pattern.MatchString(goPath) {
				t.Errorf("go output %q does not match pattern %s", goPath, tc.pattern)
			}

			// Resolve relative paths for file existence checks.
			refResolved := resolvePath(workDir, refPath)
			goResolved := resolvePath(workDir, goPath)

			// R4.2: file/directory must exist after creation.
			if _, err := os.Stat(goResolved); err != nil {
				t.Errorf("go created path does not exist: %v", err)
			}
			if _, err := os.Stat(refResolved); err != nil {
				t.Errorf("ref created path does not exist: %v", err)
			}

			// R4.2: verify directory vs file type matches expectation.
			goInfo, err := os.Stat(goResolved)
			if err == nil {
				if tc.isDir && !goInfo.IsDir() {
					t.Errorf("go output is not a directory")
				}
				if !tc.isDir && goInfo.IsDir() {
					t.Errorf("go output is a directory, expected file")
				}
				// R4.2: permissions must match specification.
				gotPerm := goInfo.Mode().Perm()
				if gotPerm != tc.permissions {
					t.Errorf("go permissions: got %04o, want %04o", gotPerm, tc.permissions)
				}
			}

			// Clean up created files/directories.
			os.Remove(refResolved) // best-effort cleanup
			os.Remove(goResolved)  // best-effort cleanup
		})
	}
}

// TestTmpdirFlag verifies R3.1-R3.2: --tmpdir and -p control the parent directory.
func TestTmpdirFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("tmpdir_explicit_dir", func(t *testing.T) {
		t.Parallel()
		parentDir := t.TempDir()

		env := buildTestEnv()
		out, exitCode := runMktempInDir(t, goBin, []string{"--tmpdir=" + parentDir, "app.XXXXXX"}, env, t.TempDir())
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
		}

		path := strings.TrimSpace(string(out))

		// R3.1: must be in the specified parent directory.
		if filepath.Dir(path) != parentDir {
			t.Errorf("expected file in %s, got dir %s", parentDir, filepath.Dir(path))
		}

		// File must exist.
		if _, err := os.Stat(path); err != nil {
			t.Errorf("created file does not exist: %v", err)
		}

		os.Remove(path) // best-effort cleanup
	})

	t.Run("p_short_flag", func(t *testing.T) {
		t.Parallel()
		parentDir := t.TempDir()

		env := buildTestEnv()
		out, exitCode := runMktempInDir(t, goBin, []string{"-p", parentDir, "short.XXXXXX"}, env, t.TempDir())
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
		}

		path := strings.TrimSpace(string(out))
		if filepath.Dir(path) != parentDir {
			t.Errorf("expected file in %s, got dir %s", parentDir, filepath.Dir(path))
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("created file does not exist: %v", err)
		}

		os.Remove(path) // best-effort cleanup
	})

	t.Run("tmpdir_no_value_uses_TMPDIR", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		env := buildTestEnvWithTMPDIR(tmpDir)
		out, exitCode := runMktempInDir(t, goBin, []string{"--tmpdir", "val.XXXXXX"}, env, t.TempDir())
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
		}

		path := strings.TrimSpace(string(out))
		// R3.2: --tmpdir without value uses TMPDIR.
		if filepath.Dir(path) != tmpDir {
			t.Errorf("expected file in %s, got dir %s", tmpDir, filepath.Dir(path))
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("created file does not exist: %v", err)
		}

		os.Remove(path) // best-effort cleanup
	})

	t.Run("tmpdir_with_directory_flag", func(t *testing.T) {
		t.Parallel()
		parentDir := t.TempDir()

		env := buildTestEnv()
		out, exitCode := runMktempInDir(t, goBin, []string{"-d", "--tmpdir=" + parentDir, "dir.XXXXXX"}, env, t.TempDir())
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
		}

		path := strings.TrimSpace(string(out))
		if filepath.Dir(path) != parentDir {
			t.Errorf("expected dir in %s, got dir %s", parentDir, filepath.Dir(path))
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("created directory does not exist: %v", err)
		}
		if !info.IsDir() {
			t.Errorf("expected directory, got file")
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("expected permissions 0700, got %04o", got)
		}

		os.Remove(path) // best-effort cleanup
	})
}

// TestLegacyTFlag verifies R3.4: -t treats template as a filename prefix
// in the TMPDIR directory.
func TestLegacyTFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("t_uses_TMPDIR", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		env := buildTestEnvWithTMPDIR(tmpDir)
		out, exitCode := runMktempInDir(t, goBin, []string{"-t", "app.XXXXXX"}, env, t.TempDir())
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
		}

		path := strings.TrimSpace(string(out))
		// R3.4: must be in TMPDIR.
		if filepath.Dir(path) != tmpDir {
			t.Errorf("expected file in %s, got dir %s", tmpDir, filepath.Dir(path))
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("created file does not exist: %v", err)
		}

		os.Remove(path) // best-effort cleanup
	})

	t.Run("t_falls_back_to_tmp", func(t *testing.T) {
		t.Parallel()

		// No TMPDIR set, no -p: should use /tmp.
		env := buildTestEnv()
		// Remove TMPDIR from env.
		var filtered []string
		for _, e := range env {
			if !strings.HasPrefix(e, "TMPDIR=") {
				filtered = append(filtered, e)
			}
		}
		out, exitCode := runMktempInDir(t, goBin, []string{"-t", "fall.XXXXXX"}, filtered, t.TempDir())
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
		}

		path := strings.TrimSpace(string(out))
		if filepath.Dir(path) != "/tmp" {
			t.Errorf("expected file in /tmp, got dir %s", filepath.Dir(path))
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("created file does not exist: %v", err)
		}

		os.Remove(path) // best-effort cleanup
	})

	t.Run("t_rejects_template_with_slash", func(t *testing.T) {
		t.Parallel()

		env := buildTestEnv()
		_, exitCode := runMktempInDir(t, goBin, []string{"-t", "sub/dir.XXX"}, env, t.TempDir())
		if exitCode != 1 {
			t.Errorf("expected exit 1 for template with slash under -t, got %d", exitCode)
		}
	})
}

// TestDefaultCreation verifies R1.1: mktemp with no arguments creates a file
// and prints the path.
func TestDefaultCreation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()

	env := buildTestEnvWithTMPDIR(tmpDir)
	out, exitCode := runMktemp(t, goBin, nil, env)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d; output: %s", exitCode, out)
	}

	path := strings.TrimSpace(string(out))

	// Must be in the specified TMPDIR.
	if filepath.Dir(path) != tmpDir {
		t.Errorf("expected file in %s, got %s", tmpDir, filepath.Dir(path))
	}

	// Must match default template pattern.
	base := filepath.Base(path)
	matched, _ := regexp.MatchString(`^tmp\.[A-Za-z0-9]{10}$`, base)
	if !matched {
		t.Errorf("filename %q does not match default template tmp.XXXXXXXXXX", base)
	}

	// File must exist.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("created file does not exist: %v", err)
	}

	// R1.4: permissions must be 0600.
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("expected permissions 0600, got %04o", got)
	}

	os.Remove(path) // best-effort cleanup
}

// TestInvalidTemplate verifies R1.4: templates with fewer than 3 trailing Xs
// cause exit 1 with a diagnostic on stderr.
func TestInvalidTemplate(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name     string
		template string
	}{
		{"two_Xs", "foo.XX"},
		{"one_X", "foo.X"},
		{"no_Xs", "noexes"},
		{"Xs_not_trailing", "XXXfoo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.template)
			cmd.Env = buildTestEnv()
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Fatalf("expected non-zero exit for template %q", tc.template)
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("unexpected error type: %v", err)
			}
			if exitErr.ExitCode() != 1 {
				t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
			}

			// Must have diagnostic on stderr.
			if stderr.Len() == 0 {
				t.Errorf("expected diagnostic on stderr for template %q", tc.template)
			}
		})
	}
}

// TestDryRun verifies R3.5: -u/--dry-run prints the name without creating
// the file or directory, and emits a warning on stderr.
func TestDryRun(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("dry_run_no_file_created", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		env := buildTestEnvWithTMPDIR(tmpDir)
		cmd := exec.Command(goBin, "-u")
		cmd.Env = env
		cmd.Dir = tmpDir

		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				t.Fatalf("expected exit 0, got %d; stderr: %s", exitErr.ExitCode(), errBuf.String())
			}
			t.Fatalf("failed to execute: %v", runErr)
		}

		path := strings.TrimSpace(outBuf.String())
		if path == "" {
			t.Fatal("expected non-empty path output")
		}

		// R3.5: file must NOT exist (dry-run).
		if _, err := os.Stat(path); err == nil {
			t.Errorf("dry-run should not create file, but %s exists", path)
			os.Remove(path) // best-effort cleanup
		}

		// R3.5: must print a warning on stderr about dry-run being discouraged.
		stderrStr := errBuf.String()
		if !strings.Contains(stderrStr, "discouraged") {
			t.Errorf("expected dry-run warning on stderr containing 'discouraged', got: %s", stderrStr)
		}
	})

	t.Run("dry_run_long_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		env := buildTestEnvWithTMPDIR(tmpDir)
		cmd := exec.Command(goBin, "--dry-run")
		cmd.Env = env
		cmd.Dir = tmpDir

		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				t.Fatalf("expected exit 0, got %d; stderr: %s", exitErr.ExitCode(), errBuf.String())
			}
			t.Fatalf("failed to execute: %v", runErr)
		}

		path := strings.TrimSpace(outBuf.String())
		if path == "" {
			t.Fatal("expected non-empty path output")
		}

		// File must NOT exist.
		if _, err := os.Stat(path); err == nil {
			t.Errorf("dry-run should not create file, but %s exists", path)
			os.Remove(path) // best-effort cleanup
		}
	})

	t.Run("dry_run_with_directory_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		env := buildTestEnvWithTMPDIR(tmpDir)
		cmd := exec.Command(goBin, "-u", "-d")
		cmd.Env = env
		cmd.Dir = tmpDir

		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				t.Fatalf("expected exit 0, got %d; stderr: %s", exitErr.ExitCode(), errBuf.String())
			}
			t.Fatalf("failed to execute: %v", runErr)
		}

		path := strings.TrimSpace(outBuf.String())
		if path == "" {
			t.Fatal("expected non-empty path output")
		}

		// Directory must NOT exist.
		if _, err := os.Stat(path); err == nil {
			t.Errorf("dry-run should not create directory, but %s exists", path)
			os.Remove(path) // best-effort cleanup
		}
	})

	t.Run("dry_run_with_custom_template", func(t *testing.T) {
		t.Parallel()
		workDir := t.TempDir()

		env := buildTestEnv()
		cmd := exec.Command(goBin, "-u", "myapp.XXXXXX")
		cmd.Env = env
		cmd.Dir = workDir

		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &bytes.Buffer{}

		runErr := cmd.Run()
		if runErr != nil {
			t.Fatalf("failed to execute: %v", runErr)
		}

		path := strings.TrimSpace(outBuf.String())
		matched, _ := regexp.MatchString(`myapp\.[A-Za-z0-9]{6}$`, path)
		if !matched {
			t.Errorf("dry-run output %q does not match expected template pattern", path)
		}

		// File must NOT exist.
		resolved := resolvePath(workDir, path)
		if _, err := os.Stat(resolved); err == nil {
			t.Errorf("dry-run should not create file, but %s exists", resolved)
			os.Remove(resolved) // best-effort cleanup
		}
	})
}

// TestDryRunDiff runs differential tests for -u/--dry-run against gmktemp.
// R3.5, R4.1: both binaries must exit 0 and produce structurally matching output.
func TestDryRunDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	env := buildTestEnvWithTMPDIR(tmpDir)

	// Run both binaries with -u.
	refOut, refExit := runMktempInDir(t, refBin, []string{"-u"}, env, tmpDir)
	goOut, goExit := runMktempInDir(t, goBin, []string{"-u"}, env, tmpDir)

	// Exit codes must match (both 0).
	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d", refExit, goExit)
	}
	if goExit != 0 {
		t.Fatalf("expected exit 0, got %d; output: %s", goExit, goOut)
	}

	// Both outputs must be in TMPDIR with the default pattern.
	refPath := strings.TrimSpace(string(refOut))
	goPath := strings.TrimSpace(string(goOut))

	pattern := regexp.MustCompile(`/tmp\.[A-Za-z0-9]{10}$`)
	if !pattern.MatchString(refPath) {
		t.Errorf("ref dry-run output %q does not match expected pattern", refPath)
	}
	if !pattern.MatchString(goPath) {
		t.Errorf("go dry-run output %q does not match expected pattern", goPath)
	}

	// Neither file should exist (dry-run).
	if _, err := os.Stat(refPath); err == nil {
		t.Errorf("ref dry-run should not create file, but %s exists", refPath)
	}
	if _, err := os.Stat(goPath); err == nil {
		t.Errorf("go dry-run should not create file, but %s exists", goPath)
	}
}

// TestQuiet verifies R3.6: -q/--quiet suppresses error diagnostics on stderr.
func TestQuiet(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("quiet_suppresses_stderr_on_error", func(t *testing.T) {
		t.Parallel()

		// Use a non-existent TMPDIR to trigger an error.
		env := buildTestEnvWithTMPDIR("/nonexistent/path/for/mktemp/quiet/test")
		cmd := exec.Command(goBin, "-q")
		cmd.Env = env

		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		if runErr == nil {
			t.Fatal("expected non-zero exit with non-existent TMPDIR")
		}

		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("unexpected error type: %v", runErr)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}

		// R3.6: stderr must be empty when -q is used.
		if errBuf.Len() != 0 {
			t.Errorf("expected empty stderr with -q, got: %s", errBuf.String())
		}
	})

	t.Run("quiet_long_flag", func(t *testing.T) {
		t.Parallel()

		env := buildTestEnvWithTMPDIR("/nonexistent/path/for/mktemp/quiet/test2")
		cmd := exec.Command(goBin, "--quiet")
		cmd.Env = env

		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		if runErr == nil {
			t.Fatal("expected non-zero exit with non-existent TMPDIR")
		}

		// R3.6: stderr must be empty.
		if errBuf.Len() != 0 {
			t.Errorf("expected empty stderr with --quiet, got: %s", errBuf.String())
		}
	})

	t.Run("quiet_success_still_prints_path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// R3.6: -q suppresses creation failure, but successful run still prints.
		env := buildTestEnvWithTMPDIR(tmpDir)
		cmd := exec.Command(goBin, "-q")
		cmd.Env = env
		cmd.Dir = tmpDir

		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		if runErr != nil {
			t.Fatalf("expected exit 0, got error: %v", runErr)
		}

		path := strings.TrimSpace(outBuf.String())
		if path == "" {
			t.Fatal("expected non-empty path output with -q on success")
		}

		// File must exist.
		if _, err := os.Stat(path); err != nil {
			t.Errorf("created file does not exist: %v", err)
		}

		os.Remove(path) // best-effort cleanup
	})
}

// TestQuietDiff runs differential tests for -q/--quiet against gmktemp.
// R3.6, R4.1: both binaries must exit 1 with empty stderr when -q is used.
func TestQuietDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R3.6: -q with non-existent TMPDIR suppresses creation failure stderr.
		{
			Name:     "quiet_nonexistent_tmpdir",
			Args:     []string{"-q"},
			Env:      []string{"TMPDIR=/nonexistent/path/for/mktemp/qdiff"},
			ExitCode: 1,
		},
		// R3.6: --quiet with bad template still prints template validation error
		// (quiet only suppresses creation failures, matching GNU behavior).
		{
			Name:      "quiet_bad_template_still_prints",
			Args:      []string{"--quiet", "foo.XX"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName, normalizeStderrContent},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runMktemp executes the binary and returns stdout and exit code.
func runMktemp(t *testing.T, binary string, args []string, env []string) (stdout []byte, exitCode int) {
	t.Helper()
	return runMktempInDir(t, binary, args, env, "")
}

// runMktempInDir executes the binary in the specified directory and returns
// stdout and exit code.
func runMktempInDir(t *testing.T, binary string, args []string, env []string, dir string) (stdout []byte, exitCode int) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Env = env
	if dir != "" {
		cmd.Dir = dir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return outBuf.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("failed to execute %s: %v", binary, runErr)
	}
	return outBuf.Bytes(), 0
}

// resolvePath resolves a potentially relative path against a base directory.
func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

// buildTestEnv constructs the environment with LC_ALL=C set.
func buildTestEnv() []string {
	var result []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "LC_ALL=") {
			result = append(result, e)
		}
	}
	return append(result, "LC_ALL=C")
}

// buildTestEnvWithTMPDIR constructs the environment with LC_ALL=C and TMPDIR set.
func buildTestEnvWithTMPDIR(tmpDir string) []string {
	var result []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "LC_ALL=") || strings.HasPrefix(e, "TMPDIR=") {
			continue
		}
		result = append(result, e)
	}
	return append(result, "LC_ALL=C", "TMPDIR="+tmpDir)
}

// normalizeProgramName replaces gmktemp/mktemp binary names and paths with
// "mktemp" and lowercases so OS-level error string differences do not cause
// false divergence.
func normalizeProgramName(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		line = normalizePathPrefix(line)
		lines[i] = bytes.ReplaceAll(line, []byte("gmktemp"), []byte("mktemp"))
	}
	return bytes.ToLower(bytes.Join(lines, []byte("\n")))
}

// normalizePathPrefix replaces a leading "/path/to/gmktemp: " or
// "/path/to/mktemp: " with "mktemp: ".
func normalizePathPrefix(line []byte) []byte {
	colonIdx := bytes.Index(line, []byte(": "))
	if colonIdx == -1 {
		return line
	}
	prog := line[:colonIdx]
	if slashIdx := bytes.LastIndexByte(prog, '/'); slashIdx >= 0 {
		base := prog[slashIdx+1:]
		if bytes.Equal(base, []byte("gmktemp")) || bytes.Equal(base, []byte("mktemp")) {
			return append([]byte("mktemp"), line[colonIdx:]...)
		}
	}
	return line
}

// normalizeStderrContent normalizes error message content to handle wording
// differences between GNU mktemp and our implementation.
func normalizeStderrContent(b []byte) []byte {
	// Both implementations produce diagnostics; normalize to just check exit code.
	// Clear stderr content since error message wording may differ.
	if len(b) > 0 {
		return []byte("error\n")
	}
	return b
}

// clearOutput returns nil, used for tests where output content differs
// by design (e.g., --version, --help) but exit code must match.
func clearOutput(b []byte) []byte {
	return nil
}
