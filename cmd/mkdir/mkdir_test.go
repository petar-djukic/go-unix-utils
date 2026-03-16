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
		// R2.1: --parents long form creates nested parent directories.
		{
			name: "parents_long_form",
			args: []string{"--parents", "x/y/z"},
		},
		// R2.1: -p creates deeply nested chain.
		{
			name: "p_deep_nested",
			args: []string{"-p", "d1/d2/d3/d4/d5"},
		},
		// R3.1: -m sets permission mode (octal).
		{
			name: "m_octal",
			args: []string{"-m", "0700", "restricted"},
		},
		// R2.1, R3.3: -p -m applies mode only to final directory.
		{
			name: "p_m_mode_nested",
			args: []string{"-p", "-m", "0700", "pm/child/target"},
		},
		// R3.4: -v prints verbose message for each created directory.
		{
			name: "v_single_dir",
			args: []string{"-v", "verbosedir"},
		},
		// R3.2, R3.4: -pv prints message for each directory in chain.
		{
			name: "pv_nested",
			args: []string{"-pv", "va/vb/vc"},
		},
		// R3.3: -Z silently accepted on non-SELinux (Darwin).
		{
			name: "Z_single_dir",
			args: []string{"-Z", "zdir"},
		},
		// R3.3: --context silently accepted on non-SELinux (Darwin).
		{
			name: "context_long_form",
			args: []string{"--context", "ctxdir"},
		},
		// R3.3: -Z combined with -p.
		{
			name: "Z_with_parents",
			args: []string{"-Zp", "za/zb/zc"},
		},
		// R3.4: -v combined with -Z.
		{
			name: "Zv_single_dir",
			args: []string{"-Zv", "zvdir"},
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

// TestDiffParentsExisting verifies R2.3: -p does not error when some
// intermediate directories already exist.
func TestDiffParentsExisting(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	t.Run("p_partial_existing", func(t *testing.T) {
		t.Parallel()

		refDir := t.TempDir()
		goDir := t.TempDir()

		// Pre-create the first intermediate directory in both work dirs.
		if err := os.Mkdir(filepath.Join(refDir, "existing"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.Mkdir(filepath.Join(goDir, "existing"), 0o755); err != nil {
			t.Fatalf("setup: %v", err)
		}

		args := []string{"-p", "existing/new/deep"}

		refOut, refErr, refExit := runCmd(t, refBin, args, refDir)
		goOut, goErr, goExit := runCmd(t, goBin, args, goDir)

		refOut = normalizeProgramName(refOut)
		refErr = normalizeProgramName(refErr)
		goOut = normalizeProgramName(goOut)
		goErr = normalizeProgramName(goErr)

		if refExit != goExit {
			t.Errorf("exit code mismatch: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refOut, goOut) {
			t.Errorf("stdout mismatch\nref: %q\ngo:  %q", refOut, goOut)
		}
		if !bytes.Equal(refErr, goErr) {
			t.Errorf("stderr mismatch\nref: %q\ngo:  %q", refErr, goErr)
		}

		// Verify the deep directory was actually created.
		target := filepath.Join(goDir, "existing", "new", "deep")
		if fi, err := os.Stat(target); err != nil {
			t.Errorf("target directory not created: %v", err)
		} else if !fi.IsDir() {
			t.Errorf("target is not a directory")
		}
	})
}

// TestParentsModePermissions verifies R3.3: -p -m applies the specified mode
// only to the final target directory, not to intermediate directories.
func TestParentsModePermissions(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	t.Run("p_m_final_only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "parent", "child", "final")
		cmd := exec.Command(goBin, "-p", "-m", "0700", target)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mkdir -p -m 0700 failed: %v\noutput: %s", err, out)
		}

		// R3.3: final directory must have mode 0700.
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("stat final: %v", err)
		}
		got := info.Mode().Perm()
		if got != 0o700 {
			t.Errorf("final directory: expected mode 0700, got %04o", got)
		}

		// R3.3: intermediate directories must have default permissions (umask applied to 0777).
		// We cannot predict the exact umask, but intermediates must NOT have mode 0700
		// unless the umask happens to produce it.
		parentDir := filepath.Join(dir, "parent")
		pInfo, err := os.Stat(parentDir)
		if err != nil {
			t.Fatalf("stat parent: %v", err)
		}
		parentMode := pInfo.Mode().Perm()
		// Default is 0777 & ~umask. Common umask 0022 yields 0755.
		// The intermediate must not have the restricted 0700 mode.
		if parentMode == 0o700 {
			t.Errorf("intermediate 'parent' has mode 0700; expected default permissions (umask applied)")
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
	// GNU binary may report its full argv[0] path in verbose and error messages.
	// Replace any path ending in /gmkdir or /mkdir with just "mkdir".
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		// Replace full path prefix before the binary name at line start.
		// E.g. "/opt/homebrew/bin/gmkdir: " → "mkdir: "
		// or "/opt/homebrew/bin/mkdir: " → "mkdir: "
		line = normalizePathPrefix(line)
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

// normalizePathPrefix replaces a leading "/path/to/gmkdir: " or
// "/path/to/mkdir: " with "mkdir: " so verbose and error output from
// the reference binary (which uses argv[0]) matches our output.
func normalizePathPrefix(line []byte) []byte {
	colonIdx := bytes.Index(line, []byte(": "))
	if colonIdx == -1 {
		return line
	}
	prog := line[:colonIdx]
	if slashIdx := bytes.LastIndexByte(prog, '/'); slashIdx >= 0 {
		base := prog[slashIdx+1:]
		if bytes.Equal(base, []byte("gmkdir")) || bytes.Equal(base, []byte("mkdir")) {
			return append([]byte("mkdir"), line[colonIdx:]...)
		}
	}
	return line
}

// clearOutput returns nil, used for tests where output content differs
// by design (e.g., --version) but exit code must match.
func clearOutput(b []byte) []byte {
	return nil
}
