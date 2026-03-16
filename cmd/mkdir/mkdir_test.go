// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkdir against gmkdir (GNU coreutils).
// Implements prd034-mkdir R4.1-R4.3 test coverage.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiff runs differential tests that do not create new directories
// (error cases and idempotent operations) using RunDiffTests directly.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	// Pre-create directories for error and existing-directory tests.
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	if err := os.Mkdir(existingDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.3: no arguments prints usage to stderr and exits non-zero.
		{
			Name:      "no_args",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.3: error when target already exists without -p.
		{
			Name:      "error_existing_no_p",
			Args:      []string{existingDir},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R2.2: -p on existing directory exits 0 without error.
		{
			Name: "p_existing_dir",
			Args: []string{"-p", existingDir},
		},
		// R1.4: error when parent does not exist without -p.
		{
			Name:      "error_missing_parent",
			Args:      []string{filepath.Join(tmpDir, "nonexistent", "deep", "dir")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R1.4: --version exits 0.
		{
			Name:      "version_exit_0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCreation runs differential tests for directory creation in isolated
// temp dirs (one per binary) since both binaries mutate the filesystem.
func TestDiffCreation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	tests := []struct {
		name string
		args []string // directory names are relative to workdir
	}{
		// R1.1: create a single directory.
		{
			name: "single_dir",
			args: []string{"newdir"},
		},
		// R1.2: create multiple directories.
		{
			name: "multiple_dirs",
			args: []string{"aaa", "bbb", "ccc"},
		},
		// R2.1: -p creates nested parent directories.
		{
			name: "p_nested",
			args: []string{"-p", "a/b/c"},
		},
		// R3.1: -m sets permission mode (octal).
		{
			name: "m_octal",
			args: []string{"-m", "0700", "restricted"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refDir := t.TempDir()
			goDir := t.TempDir()

			refOut, refErr, refExit := runCmd(t, refBin, tc.args, refDir)
			goOut, goErr, goExit := runCmd(t, goBin, tc.args, goDir)

			// Normalize program name.
			refOut = normalizeProgramName(refOut)
			refErr = normalizeProgramName(refErr)
			goOut = normalizeProgramName(goOut)
			goErr = normalizeProgramName(goErr)

			if refExit != goExit {
				t.Errorf("exit code mismatch: ref=%d go=%d\nargs: %v\nref stderr: %q\ngo stderr: %q",
					refExit, goExit, tc.args, refErr, goErr)
			}
			if !bytes.Equal(refOut, goOut) {
				t.Errorf("stdout mismatch\nargs: %v\nref: %q\ngo:  %q", tc.args, refOut, goOut)
			}
			if !bytes.Equal(refErr, goErr) {
				t.Errorf("stderr mismatch\nargs: %v\nref: %q\ngo:  %q", tc.args, refErr, goErr)
			}
		})
	}
}

// TestModePermissions verifies that -m correctly sets directory permissions.
// R3.1, R4.3.
func TestModePermissions(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("m_0700", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "restricted")
		cmd := exec.Command(goBin, "-m", "0700", target)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mkdir -m 0700 failed: %v\noutput: %s", err, out)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		got := info.Mode().Perm()
		if got != 0o700 {
			t.Errorf("expected mode 0700, got %04o", got)
		}
	})

	t.Run("m_0755", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "standard")
		cmd := exec.Command(goBin, "-m", "0755", target)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mkdir -m 0755 failed: %v\noutput: %s", err, out)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		got := info.Mode().Perm()
		if got != 0o755 {
			t.Errorf("expected mode 0755, got %04o", got)
		}
	})
}

// runCmd executes a binary and returns stdout, stderr, and exit code.
func runCmd(t *testing.T, binary string, args []string, workDir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Dir = workDir
	cmd.Env = buildTestEnv()

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return outBuf.Bytes(), errBuf.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("failed to execute %s: %v", binary, runErr)
	}
	return outBuf.Bytes(), errBuf.Bytes(), 0
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

// normalizeProgramName replaces the reference binary name and path with
// "mkdir", then lowercases the output so OS-level error string case
// differences (Go lowercase vs C strerror capitalized) do not cause
// false divergence.
func normalizeProgramName(b []byte) []byte {
	// GNU binary may report its full path in "Try '...' messages.
	// Replace any path ending in /mkdir or /gmkdir with just "mkdir".
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		// Replace the reference binary name.
		lines[i] = bytes.ReplaceAll(line, []byte("gmkdir"), []byte("mkdir"))
	}
	b = bytes.Join(lines, []byte("\n"))
	// Normalize the "Try '/path/to/mkdir --help'" to "Try 'mkdir --help'".
	// GNU uses the full argv[0] path; ours uses just the program name.
	for {
		idx := bytes.Index(b, []byte("Try '"))
		if idx == -1 {
			break
		}
		// Find the closing quote.
		rest := b[idx+5:]
		end := bytes.IndexByte(rest, '\'')
		if end == -1 {
			break
		}
		// Extract the content between quotes.
		inner := rest[:end]
		// Replace any path prefix before "mkdir" with just "mkdir".
		if mIdx := bytes.LastIndex(inner, []byte("mkdir")); mIdx > 0 {
			// Find the last path separator before "mkdir".
			prefix := inner[:mIdx]
			if slashIdx := bytes.LastIndexByte(prefix, '/'); slashIdx >= 0 {
				replacement := append([]byte("Try '"), inner[slashIdx+1:]...)
				replacement = append(replacement, '\'')
				old := b[idx : idx+5+end+1]
				b = bytes.Replace(b, old, replacement, 1)
				continue
			}
		}
		break
	}
	return bytes.ToLower(b)
}

// clearOutput returns nil, used for tests where output content differs
// by design (e.g., --version) but exit code must match.
func clearOutput(b []byte) []byte {
	return nil
}
