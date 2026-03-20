// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd050-readlink R1.1–R1.4, R4.1–R4.3:
// default symlink target, -f canonicalize, -e canonicalize-existing,
// error handling, and differential test coverage against greadlink.
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
// stderr so that "greadlink" and "/opt/.../greadlink" both become "readlink".
var binaryNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	re := regexp.MustCompile(`[^\s']*g?readlink`)
	return re.ReplaceAll(data, []byte("readlink"))
}

// caseNormalizer lowercases output to handle platform differences in
// error messages (e.g., "No such file" vs "no such file").
var caseNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	return bytes.ToLower(data)
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skipf("reference binary greadlink not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	canonTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	setupTestFixtures(t, canonTmpDir)

	symlink := filepath.Join(canonTmpDir, "link")
	chainEnd := filepath.Join(canonTmpDir, "chain")
	usageNorm := []testutils.NormalizeFunc{binaryNameNormalizer, caseNormalizer}

	tests := []testutils.DiffTest{
		// R1.1: default — print symlink target
		{
			Name: "symlink_target",
			Args: []string{symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: symlink chain — prints immediate target, not final
		{
			Name: "symlink_chain_immediate",
			Args: []string{chainEnd},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: non-symlink operand exits 1 silently
		{
			Name:     "non_symlink",
			Args:     []string{filepath.Join(canonTmpDir, "target")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.2: nonexistent path exits 1 silently
		{
			Name:     "nonexistent_path",
			Args:     []string{filepath.Join(canonTmpDir, "no_such_file")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.3: -f canonicalize existing symlink
		{
			Name: "canonicalize_symlink",
			Args: []string{"-f", symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -f canonicalize regular file
		{
			Name: "canonicalize_regular",
			Args: []string{"-f", filepath.Join(canonTmpDir, "target")},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -f with nonexistent last component
		{
			Name: "canonicalize_missing_last",
			Args: []string{"-f", filepath.Join(canonTmpDir, "no_such_file")},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: -f with nonexistent parent exits 1 silently
		{
			Name:     "canonicalize_missing_parent",
			Args:     []string{"-f", filepath.Join(canonTmpDir, "no_such_dir", "child")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.3: --canonicalize long form
		{
			Name: "canonicalize_long",
			Args: []string{"--canonicalize", symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -e with existing path
		{
			Name: "canonicalize_existing_present",
			Args: []string{"-e", filepath.Join(canonTmpDir, "target")},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -e with symlink
		{
			Name: "canonicalize_existing_symlink",
			Args: []string{"-e", symlink},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: -e with nonexistent path exits 1 silently
		{
			Name:     "canonicalize_existing_missing",
			Args:     []string{"-e", filepath.Join(canonTmpDir, "no_such_file")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// R1.4: --canonicalize-existing long form
		{
			Name:     "canonicalize_existing_long",
			Args:     []string{"--canonicalize-existing", filepath.Join(canonTmpDir, "no_such_file")},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 1,
		},
		// Multiple operands
		{
			Name: "multiple_operands",
			Args: []string{symlink, chainEnd},
			Env:  []string{"LC_ALL=C"},
		},
		// No operand — usage error to stderr
		{
			Name:      "no_operand",
			Args:      []string{},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: usageNorm,
		},
		// -f with .. components
		{
			Name: "canonicalize_dot_dot",
			Args: []string{"-f", filepath.Join(canonTmpDir, "sub", "..", "target")},
			Env:  []string{"LC_ALL=C"},
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
	// Create a symlink chain: chain -> link -> target
	if err := os.Symlink(filepath.Join(dir, "link"), filepath.Join(dir, "chain")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
}
