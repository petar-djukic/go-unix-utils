// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/realpath against grealpath (GNU coreutils).
// Implements prd049-realpath R1.1-R1.5, R2.1-R2.3, R3.1-R3.3, R4.1-R4.3 test coverage.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	// D3: graceful skip if grealpath is not installed.
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skipf("reference binary grealpath not in PATH: %v", err)
	}

	// Create a temp directory with a symlink for symlink resolution tests.
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating target file: %v", err)
	}
	symlinkPath := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// Create a subdirectory for relative path tests.
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	// Create a nested directory structure for --relative-to and --relative-base tests.
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatalf("creating deep dir: %v", err)
	}
	siblingDir := filepath.Join(tmpDir, "a", "x")
	if err := os.Mkdir(siblingDir, 0o755); err != nil {
		t.Fatalf("creating sibling dir: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.1: --version prints version info to stdout, exit 0.
		{
			Name:      "R4.1_version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeVersion},
		},
		// R4.2: --help prints usage to stdout, exit 0.
		{
			Name:      "R4.2_help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelp},
		},
		// R1.1: resolve an existing absolute path.
		{
			Name:     "R1.1_absolute_path",
			Args:     []string{"/tmp"},
			ExitCode: 0,
		},
		// R1.1: resolve a relative path (dot).
		{
			Name:     "R1.1_relative_dot",
			Args:     []string{"."},
			WorkDir:  tmpDir,
			ExitCode: 0,
		},
		// R1.1: resolve a symlink to its target.
		{
			Name:     "R1.1_symlink_resolution",
			Args:     []string{symlinkPath},
			ExitCode: 0,
		},
		// R1.1: resolve path with .. component.
		{
			Name:     "R1.1_dotdot_component",
			Args:     []string{filepath.Join(subDir, "..")},
			ExitCode: 0,
		},
		// R1.2/R1.4: nonexistent path produces error, exit 1.
		{
			Name:      "R1.2_nonexistent_path",
			Args:      []string{"/nonexistent_path_xyz_abc_123"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.1: no operands produces usage error, exit 1.
		{
			Name:      "R3.1_no_operands",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.2: unknown short flag produces error, exit 1.
		{
			Name:      "R3.2_unknown_short_flag",
			Args:      []string{"-x", "/tmp"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.2: unknown long flag produces error, exit 1.
		{
			Name:      "R3.2_unknown_long_flag",
			Args:      []string{"--foobar", "/tmp"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.3: multiple paths, some failing — errors for failing, output for succeeding.
		{
			Name:      "R3.3_mixed_success_failure",
			Args:      []string{"/tmp", "/nonexistent_path_xyz_abc_123"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R1.1: multiple existing paths all resolve.
		{
			Name:     "R1.1_multiple_existing",
			Args:     []string{"/tmp", "/usr"},
			ExitCode: 0,
		},
		// R1.5: -s flag does not resolve symlinks.
		{
			Name:     "R1.5_strip_symlink",
			Args:     []string{"-s", symlinkPath},
			ExitCode: 0,
		},
		// R1.5: --no-symlinks long form.
		{
			Name:     "R1.5_no_symlinks_long",
			Args:     []string{"--no-symlinks", symlinkPath},
			ExitCode: 0,
		},
		// R1.5: -s cleans .. without resolving symlinks.
		{
			Name:     "R1.5_strip_dotdot",
			Args:     []string{"-s", filepath.Join(subDir, "..", "target.txt")},
			ExitCode: 0,
		},
		// R2.1: --relative-to makes output relative to the given directory.
		{
			Name:     "R2.1_relative_to_parent",
			Args:     []string{"--relative-to=" + tmpDir, targetFile},
			ExitCode: 0,
		},
		// R2.1: --relative-to with a sibling path.
		{
			Name:     "R2.1_relative_to_sibling",
			Args:     []string{"--relative-to=" + filepath.Join(tmpDir, "a"), deepDir},
			ExitCode: 0,
		},
		// R2.1: --relative-to with multiple paths (AC3: at least 3 inputs).
		{
			Name:     "R2.1_relative_to_multiple",
			Args:     []string{"--relative-to=" + tmpDir, targetFile, subDir, deepDir},
			ExitCode: 0,
		},
		// R2.2: --relative-base with path inside base prints relative.
		{
			Name:     "R2.2_relative_base_inside",
			Args:     []string{"--relative-base=" + filepath.Join(tmpDir, "a"), deepDir},
			ExitCode: 0,
		},
		// R2.2: --relative-base with path outside base prints absolute (AC4).
		{
			Name:     "R2.2_relative_base_outside",
			Args:     []string{"--relative-base=" + filepath.Join(tmpDir, "a"), "/tmp"},
			ExitCode: 0,
		},
		// R2.2: --relative-base with mixed inside and outside paths.
		{
			Name:     "R2.2_relative_base_mixed",
			Args:     []string{"--relative-base=" + filepath.Join(tmpDir, "a"), deepDir, "/tmp"},
			ExitCode: 0,
		},
		// R2.3: combined --relative-to and --relative-base (AC5).
		{
			Name:     "R2.3_combined_inside",
			Args:     []string{"--relative-to=" + filepath.Join(tmpDir, "a"), "--relative-base=" + filepath.Join(tmpDir, "a"), deepDir},
			ExitCode: 0,
		},
		// R2.3: combined flags with path outside base prints absolute.
		{
			Name:     "R2.3_combined_outside",
			Args:     []string{"--relative-to=" + filepath.Join(tmpDir, "a"), "--relative-base=" + filepath.Join(tmpDir, "a"), "/tmp"},
			ExitCode: 0,
		},
		// R2.3: combined flags with sibling directory inside base.
		{
			Name:     "R2.3_combined_sibling",
			Args:     []string{"--relative-to=" + filepath.Join(tmpDir, "a", "b"), "--relative-base=" + filepath.Join(tmpDir, "a"), siblingDir},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeStderr replaces all stderr output with empty bytes since the exact
// error message format may differ between implementations. Only exit codes
// and stdout are compared.
func normalizeStderr(b []byte) []byte {
	return nil
}

// normalizeVersion reduces version output to just the first line's prefix
// ("realpath") so different version strings (dev vs GNU) don't cause divergence.
// Both binaries must produce output that starts with the program name.
func normalizeVersion(b []byte) []byte {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[:i+1]
	}
	// Keep only the program name portion before the first space.
	if i := bytes.IndexByte(b, ' '); i >= 0 {
		return append(b[:i], '\n')
	}
	return b
}

// normalizeHelp reduces help output to empty since the exact help text differs
// between implementations. Both must exit 0 and produce some stdout output.
func normalizeHelp(b []byte) []byte {
	if len(b) > 0 {
		return []byte("help\n")
	}
	return b
}
