// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mktemp against gmktemp (GNU coreutils).
// Implements prd036-mktemp R1.1-R1.4 test coverage.
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

			// R4.2: file must exist after creation.
			if _, err := os.Stat(goResolved); err != nil {
				t.Errorf("go created file does not exist: %v", err)
			}
			if _, err := os.Stat(refResolved); err != nil {
				t.Errorf("ref created file does not exist: %v", err)
			}

			// R4.2: file permissions must match specification.
			goInfo, err := os.Stat(goResolved)
			if err == nil {
				gotPerm := goInfo.Mode().Perm()
				if gotPerm != tc.permissions {
					t.Errorf("go file permissions: got %04o, want %04o", gotPerm, tc.permissions)
				}
			}

			// Clean up created files.
			os.Remove(refResolved) // best-effort cleanup
			os.Remove(goResolved)  // best-effort cleanup
		})
	}
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
