// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/readlink against greadlink (GNU coreutils).
//
// Implements prd050-readlink R4.1, R4.2, R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces the reference binary path with the program name
// so stderr messages can be compared across binaries.
func stderrNormalizer() testutils.NormalizeFunc {
	re := regexp.MustCompile(`(?:greadlink|/[^\s:]+/greadlink)`)
	return func(b []byte) []byte {
		return re.ReplaceAll(b, []byte("readlink"))
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skipf("reference binary greadlink not in PATH: %v", err)
	}

	// Create test fixtures: a regular file and a symlink.
	tmpDir := t.TempDir()
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("creating regular file: %v", err)
	}
	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(regularFile, symlink); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}
	missingPath := filepath.Join(tmpDir, "no-such-file")
	partialPath := filepath.Join(tmpDir, "no-such-dir", "file.txt")

	// R4.3: All tests set LC_ALL=C.
	env := []string{"LC_ALL=C"}
	normalize := []testutils.NormalizeFunc{stderrNormalizer()}

	tests := []testutils.DiffTest{
		// R1.1: symlink target
		{
			Name: "symlink target",
			Args: []string{symlink},
			Env:  env,
		},
		// R1.2: non-symlink operand exits 1
		{
			Name:     "non-symlink operand",
			Args:     []string{regularFile},
			Env:      env,
			ExitCode: 1,
		},
		// R1.3: -f with existing path
		{
			Name: "canonicalize existing symlink",
			Args: []string{"-f", symlink},
			Env:  env,
		},
		// R1.3: -f with partial path (parent exists, last missing)
		{
			Name: "canonicalize partial path",
			Args: []string{"-f", missingPath},
			Env:  env,
		},
		// R1.4: -e with existing path
		{
			Name: "canonicalize-existing with symlink",
			Args: []string{"-e", symlink},
			Env:  env,
		},
		// R1.4: -e with missing path exits 1
		{
			Name:      "canonicalize-existing with missing path",
			Args:      []string{"-e", missingPath},
			Env:       env,
			ExitCode:  1,
			Normalize: normalize,
		},
		// R1.5: -m with missing path
		{
			Name: "canonicalize-missing with missing path",
			Args: []string{"-m", partialPath},
			Env:  env,
		},
		// R1.6: -n suppresses trailing newline
		{
			Name: "no-newline flag",
			Args: []string{"-n", symlink},
			Env:  env,
		},
		// R2.1: multiple operands
		{
			Name: "multiple operands",
			Args: []string{"-f", symlink, regularFile},
			Env:  env,
		},
		// R2.2: -n ignored with multiple operands
		{
			Name:      "no-newline ignored with multiple operands",
			Args:      []string{"-fn", symlink, regularFile},
			Env:       env,
			Normalize: normalize,
		},
		// R3.1: no operand
		{
			Name:      "missing operand",
			Args:      []string{},
			Env:       env,
			ExitCode:  1,
			Normalize: normalize,
		},
		// R3.2: unknown flag
		{
			Name:      "unknown flag",
			Args:      []string{"--bad-flag"},
			Env:       env,
			ExitCode:  1,
			Normalize: normalize,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
