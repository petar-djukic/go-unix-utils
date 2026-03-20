// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd049-realpath R1.1–R1.5, R2.1–R2.3, R3.1–R3.3,
// R4.1–R4.3: default resolution, -e and -m existence modes, -s strip mode,
// --relative-to, --relative-base, error handling (missing operand, unknown
// flags, mixed success/failure), and differential test coverage against
// grealpath reference binary.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameNormalizer replaces the reference binary name and path in
// stderr so that "grealpath" and "/opt/.../grealpath" both become "realpath".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?realpath`)
	return re.ReplaceAll(data, []byte("realpath"))
}

// caseNormalizer lowercases output to handle platform differences in
// error messages (e.g., "No such file" vs "no such file").
var caseNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skipf("reference binary grealpath not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	canonTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	setupTestFixtures(t, canonTmpDir)

	symlink := filepath.Join(canonTmpDir, "link")
	errNorm := []testutils.NormalizeFunc{binaryNameNormalizer, caseNormalizer}

	tests := []testutils.DiffTest{
		// R1.1: resolve absolute path
		{
			Name: "absolute_existing",
			Args: []string{"/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: resolve relative path
		{
			Name:    "relative_dot",
			Args:    []string{"."},
			Env:     []string{"LC_ALL=C"},
			WorkDir: canonTmpDir,
		},
		// R1.1: resolve symlink
		{
			Name: "symlink_resolution",
			Args: []string{symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: resolve .. components
		{
			Name: "dot_dot_cleanup",
			Args: []string{filepath.Join(canonTmpDir, "sub", "..", "target")},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: default mode with nonexistent last component (CAN_ALL_BUT_LAST)
		{
			Name:      "nonexistent_last_default",
			Args:      []string{"/xyzzy_no_such_test_path"},
			Env:       []string{"LC_ALL=C"},
			Normalize: errNorm,
		},
		// R1.2: default mode with nonexistent parent
		{
			Name:      "nonexistent_parent_default",
			Args:      []string{"/xyzzy_no_such_parent/child"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.3: -e with nonexistent path
		{
			Name:      "canonicalize_existing_missing",
			Args:      []string{"-e", "/xyzzy_no_such_test_path"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R1.3: -e with existing path
		{
			Name: "canonicalize_existing_present",
			Args: []string{"-e", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -m with fully nonexistent path
		{
			Name: "canonicalize_missing",
			Args: []string{"-m", "/xyzzy_no_such_test_path/subdir"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -m with existing path
		{
			Name: "canonicalize_missing_existing",
			Args: []string{"-m", "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -m with partially existing path
		{
			Name: "canonicalize_missing_partial",
			Args: []string{"-m", filepath.Join(canonTmpDir, "nonexistent", "deep")},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -s does not resolve symlinks
		{
			Name: "strip_symlink_not_resolved",
			Args: []string{"-s", symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -s cleans .. components
		{
			Name: "strip_dot_dot",
			Args: []string{"-s", filepath.Join(canonTmpDir, "sub", "..", "target")},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.5: -s with relative path prepends working dir
		{
			Name:    "strip_relative",
			Args:    []string{"-s", "target"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: canonTmpDir,
		},
		// R1.5: --no-symlinks long form
		{
			Name: "no_symlinks_long",
			Args: []string{"--no-symlinks", symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: --relative-to
		{
			Name: "relative_to",
			Args: []string{"--relative-to=" + canonTmpDir, filepath.Join(canonTmpDir, "target")},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: --relative-to with deeper path
		{
			Name: "relative_to_deeper",
			Args: []string{"--relative-to=" + canonTmpDir, filepath.Join(canonTmpDir, "sub")},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: --relative-base with matching path
		{
			Name: "relative_base_match",
			Args: []string{"--relative-base=" + canonTmpDir, filepath.Join(canonTmpDir, "target")},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: --relative-base with non-matching path prints absolute
		{
			Name: "relative_base_no_match",
			Args: []string{"--relative-base=" + canonTmpDir, "/tmp"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: both --relative-to and --relative-base, path matches base
		{
			Name: "both_relative_match",
			Args: []string{
				"--relative-to=" + canonTmpDir,
				"--relative-base=" + canonTmpDir,
				filepath.Join(canonTmpDir, "target"),
			},
			Env: []string{"LC_ALL=C"},
		},
		// R2.3: both --relative-to and --relative-base, path does not match base
		{
			Name: "both_relative_no_match",
			Args: []string{
				"--relative-to=" + canonTmpDir,
				"--relative-base=" + canonTmpDir,
				"/tmp",
			},
			Env: []string{"LC_ALL=C"},
		},
		// R3.1: no operand
		{
			Name:      "no_operand",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2: unknown short flag
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-Z"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.2: unknown long flag
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--nonexistent-flag"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R3.3: multiple paths all succeed
		{
			Name: "multiple_paths_success",
			Args: []string{"/tmp", "/usr"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: multiple paths with mixed success
		{
			Name:      "multiple_paths_mixed",
			Args:      []string{"/tmp", "/xyzzy_no_such_parent/child", "/usr"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupTestFixtures creates symlinks and subdirectories for testing.
func setupTestFixtures(t *testing.T, dir string) {
	t.Helper()
	targetFile := filepath.Join(dir, "target")
	if err := os.WriteFile(targetFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetFile, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
}
