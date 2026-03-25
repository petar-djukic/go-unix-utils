// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stat against GNU gstat.
// Covers prd082-stat R1.1-R1.4, R2.1-R2.3, R4.1-R4.3.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin runs a binary with args in the given working directory.
func runBin(t *testing.T, binary string, args []string, dir string) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, runErr)
		}
	}
	return cmdResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// stderrNormalizer normalizes error messages between GNU gstat and Go stat.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?stat|gstat`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	// Normalize case differences in system error messages.
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("stat"))
		b = tryHelp.ReplaceAll(b, nil)
		b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
		return b
	}
}

// createTestFile creates a regular file for testing.
func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("create test file: %v", err)
	}
	return p
}

// createTestDir creates a subdirectory for testing.
func createTestDir(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatalf("create test dir: %v", err)
	}
	return p
}

// createTestSymlink creates a symlink for testing.
func createTestSymlink(t *testing.T, dir, name, target string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.Symlink(target, p); err != nil {
		t.Fatalf("create test symlink: %v", err)
	}
	return p
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstat")
	if err != nil {
		t.Skipf("reference binary gstat not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()
	workDir := t.TempDir()

	// Create test fixtures in the workdir.
	helloFile := createTestFile(t, workDir, "hello.txt", "hello world\n")
	subDir := createTestDir(t, workDir, "subdir")
	linkPath := createTestSymlink(t, workDir, "mylink", "hello.txt")

	tests := []testutils.DiffTest{
		// ===== R1.1: Default output for regular file =====
		{
			Name:    "default_regular_file",
			Args:    []string{helloFile},
			WorkDir: workDir,
		},
		// ===== R1.1: Default output for directory =====
		{
			Name:    "default_directory",
			Args:    []string{subDir},
			WorkDir: workDir,
		},
		// ===== R1.1: Default output for symlink =====
		{
			Name:    "default_symlink",
			Args:    []string{linkPath},
			WorkDir: workDir,
		},
		// ===== R1.2: Multiple file arguments =====
		{
			Name:    "multiple_files",
			Args:    []string{helloFile, subDir},
			WorkDir: workDir,
		},
		// ===== R1.3: -L dereference symlink =====
		{
			Name:    "dereference_symlink",
			Args:    []string{"-L", linkPath},
			WorkDir: workDir,
		},
		// ===== R1.3: --dereference long form =====
		{
			Name:    "dereference_long",
			Args:    []string{"--dereference", linkPath},
			WorkDir: workDir,
		},
		// ===== R1.4: Nonexistent file exits 1 =====
		{
			Name:      "nonexistent_file",
			Args:      []string{filepath.Join(workDir, "nonexistent")},
			WorkDir:   workDir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// ===== R1.4: Mix of valid and invalid files =====
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{helloFile, filepath.Join(workDir, "nope"), subDir},
			WorkDir:   workDir,
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// ===== R2.1: Format directives =====

		// %s %n: size and name
		{
			Name:    "format_size_name",
			Args:    []string{"-c", "%s %n", helloFile},
			WorkDir: workDir,
		},
		// %a %A: octal and human-readable permissions
		{
			Name:    "format_permissions",
			Args:    []string{"-c", "%a %A", helloFile},
			WorkDir: workDir,
		},
		// %F: file type
		{
			Name:    "format_file_type_regular",
			Args:    []string{"-c", "%F", helloFile},
			WorkDir: workDir,
		},
		{
			Name:    "format_file_type_dir",
			Args:    []string{"-c", "%F", subDir},
			WorkDir: workDir,
		},
		{
			Name:    "format_file_type_symlink",
			Args:    []string{"-c", "%F", linkPath},
			WorkDir: workDir,
		},
		// %n %N: name and quoted name with link target
		{
			Name:    "format_name_quoted",
			Args:    []string{"-c", "%N", linkPath},
			WorkDir: workDir,
		},
		// %b %B: blocks and block size
		{
			Name:    "format_blocks",
			Args:    []string{"-c", "%b %B", helloFile},
			WorkDir: workDir,
		},
		// %d %D: device number decimal and hex
		{
			Name:    "format_device",
			Args:    []string{"-c", "%d %D", helloFile},
			WorkDir: workDir,
		},
		// %i: inode
		{
			Name:    "format_inode",
			Args:    []string{"-c", "%i", helloFile},
			WorkDir: workDir,
		},
		// %h: hard link count
		{
			Name:    "format_hard_links",
			Args:    []string{"-c", "%h", helloFile},
			WorkDir: workDir,
		},
		// %u %g %U %G: uid/gid numeric and names
		{
			Name:    "format_uid_gid",
			Args:    []string{"-c", "%u %g %U %G", helloFile},
			WorkDir: workDir,
		},
		// %f: raw mode hex
		{
			Name:    "format_raw_mode",
			Args:    []string{"-c", "%f", helloFile},
			WorkDir: workDir,
		},
		// %o: optimal I/O size
		{
			Name:    "format_io_size",
			Args:    []string{"-c", "%o", helloFile},
			WorkDir: workDir,
		},
		// %x %X: access time human and epoch
		{
			Name:    "format_access_time",
			Args:    []string{"-c", "%x %X", helloFile},
			WorkDir: workDir,
		},
		// %y %Y: modify time human and epoch
		{
			Name:    "format_modify_time",
			Args:    []string{"-c", "%y %Y", helloFile},
			WorkDir: workDir,
		},
		// %z %Z: change time human and epoch
		{
			Name:    "format_change_time",
			Args:    []string{"-c", "%z %Z", helloFile},
			WorkDir: workDir,
		},
		// %w %W: birth time (not available on most systems)
		{
			Name:    "format_birth_time",
			Args:    []string{"-c", "%w %W", helloFile},
			WorkDir: workDir,
		},
		// %t %T: major/minor device type
		{
			Name:    "format_major_minor",
			Args:    []string{"-c", "%t %T", helloFile},
			WorkDir: workDir,
		},
		// %%: literal percent
		{
			Name:    "format_literal_percent",
			Args:    []string{"-c", "100%%", helloFile},
			WorkDir: workDir,
		},
		// Combined directives
		{
			Name:    "format_combined",
			Args:    []string{"-c", "%a %A %F %n %s %b %i %h %u %g", helloFile},
			WorkDir: workDir,
		},
		// --format= long form
		{
			Name:    "format_long_form",
			Args:    []string{"--format=%s %n", helloFile},
			WorkDir: workDir,
		},

		// ===== R2.2: --printf with escape sequences =====
		{
			Name:    "printf_newline_tab",
			Args:    []string{"--printf", "%n\\t%s\\n", helloFile},
			WorkDir: workDir,
		},
		{
			Name:    "printf_no_trailing_newline",
			Args:    []string{"--printf", "%n", helloFile},
			WorkDir: workDir,
		},
		{
			Name:    "printf_equals_form",
			Args:    []string{"--printf=%s\\n", helloFile},
			WorkDir: workDir,
		},

		// ===== R2.3: Terse mode =====
		{
			Name:    "terse_short",
			Args:    []string{"-t", helloFile},
			WorkDir: workDir,
		},
		{
			Name:    "terse_long",
			Args:    []string{"--terse", helloFile},
			WorkDir: workDir,
		},
		{
			Name:    "terse_directory",
			Args:    []string{"-t", subDir},
			WorkDir: workDir,
		},

		// ===== R4.2: Missing operand exits 1 =====
		{
			Name:      "missing_operand",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersion verifies R4.1: --version outputs version to stdout, exits 0.
func TestVersion(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{"--version"}, t.TempDir())
	if res.exitCode != 0 {
		t.Errorf("--version exit code: want 0, got %d", res.exitCode)
	}
	out := string(res.stdout)
	if !strings.Contains(out, "stat") {
		t.Errorf("--version stdout should contain 'stat': %q", out)
	}
}

// TestHelp verifies R4.1: --help outputs usage to stdout, exits 0.
func TestHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{"--help"}, t.TempDir())
	if res.exitCode != 0 {
		t.Errorf("--help exit code: want 0, got %d", res.exitCode)
	}
	out := string(res.stdout)
	if !strings.Contains(out, "Usage:") {
		t.Errorf("--help stdout should contain 'Usage:': %q", out)
	}
}

// TestVersionDiff verifies --version exit code matches gstat --version.
func TestVersionDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstat")
	if err != nil {
		t.Skipf("reference binary gstat not in PATH: %v", err)
	}

	goRes := runBin(t, goBin, []string{"--version"}, t.TempDir())
	refRes := runBin(t, refBin, []string{"--version"}, t.TempDir())

	// R4.1: both must exit 0.
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("--version exit code mismatch: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}
	// Version text differs between implementations; verify structure only.
	if len(goRes.stdout) == 0 {
		t.Error("--version produced no stdout")
	}
	if len(goRes.stderr) != 0 {
		t.Errorf("--version wrote to stderr: %q", goRes.stderr)
	}
}

// TestHelpDiff verifies --help exit code matches gstat --help.
func TestHelpDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstat")
	if err != nil {
		t.Skipf("reference binary gstat not in PATH: %v", err)
	}

	goRes := runBin(t, goBin, []string{"--help"}, t.TempDir())
	refRes := runBin(t, refBin, []string{"--help"}, t.TempDir())

	// R4.1: both must exit 0.
	if refRes.exitCode != goRes.exitCode {
		t.Errorf("--help exit code mismatch: ref=%d go=%d",
			refRes.exitCode, goRes.exitCode)
	}
	// Help text differs between implementations; verify structure only.
	if len(goRes.stdout) == 0 {
		t.Error("--help produced no stdout")
	}
	if len(goRes.stderr) != 0 {
		t.Errorf("--help wrote to stderr: %q", goRes.stderr)
	}
}

// TestExitCodeSuccess verifies R4.1: exit 0 when all files processed.
func TestExitCodeSuccess(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	f := createTestFile(t, dir, "ok.txt", "data")
	res := runBin(t, goBin, []string{"-c", "%s", f}, dir)
	if res.exitCode != 0 {
		t.Errorf("exit code: want 0, got %d", res.exitCode)
	}
}

// TestExitCodeFailure verifies R4.2: exit 1 when a file cannot be accessed.
func TestExitCodeFailure(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	res := runBin(t, goBin,
		[]string{filepath.Join(dir, "nonexistent")}, dir)
	if res.exitCode != 1 {
		t.Errorf("exit code: want 1, got %d", res.exitCode)
	}
	if len(res.stderr) == 0 {
		t.Error("expected error message on stderr")
	}
}
