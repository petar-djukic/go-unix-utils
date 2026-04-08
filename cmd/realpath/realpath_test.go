// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/realpath.
// Tests cover srd049-realpath R3.1, R3.2, R3.3, R4.1, R4.2, R4.3.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// stderrNormalizer normalizes program name differences in error messages.
// R3.1/R3.2/R3.3: replaces binary paths with "PROG" so error message
// structure can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// stderrPresenceNormalizer replaces non-empty stderr with a constant.
// Used when error message format differs between Go and GNU binaries
// but both produce an error on the same input.
func stderrPresenceNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte("ERROR\n")
}

// resolvedTempDir creates a temp directory and resolves symlinks in the path.
// On macOS, t.TempDir() returns /var/folders/... but the physical path is
// /private/var/folders/.... Resolving ensures test paths match GNU output.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	return resolved
}

// createFixtures sets up a directory with files, subdirectories,
// and symlinks for differential tests. Returns the resolved fixture root.
func createFixtures(t *testing.T) string {
	t.Helper()
	dir := resolvedTempDir(t)

	realFile := filepath.Join(dir, "real_file.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("create fixture file: %v", err)
	}

	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create fixture subdir: %v", err)
	}

	symLink := filepath.Join(dir, "link_to_file")
	if err := os.Symlink(realFile, symLink); err != nil {
		t.Fatalf("create fixture symlink: %v", err)
	}

	return dir
}

// createNestedFixtures sets up a deeper directory tree for relative path tests.
// Returns the resolved fixture root. Tree structure:
//
//	root/
//	  a/
//	    b/
//	      c/
//	        deep.txt
//	    file_a.txt
//	  other/
//	    file_o.txt
//	  link_to_a -> a
func createNestedFixtures(t *testing.T) string {
	t.Helper()
	dir := resolvedTempDir(t)

	dirs := []string{
		filepath.Join(dir, "a", "b", "c"),
		filepath.Join(dir, "other"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("create nested dir %s: %v", d, err)
		}
	}

	files := map[string]string{
		filepath.Join(dir, "a", "b", "c", "deep.txt"): "deep",
		filepath.Join(dir, "a", "file_a.txt"):          "a",
		filepath.Join(dir, "other", "file_o.txt"):      "o",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("create nested file %s: %v", path, err)
		}
	}

	linkSrc := filepath.Join(dir, "a")
	linkDst := filepath.Join(dir, "link_to_a")
	if err := os.Symlink(linkSrc, linkDst); err != nil {
		t.Fatalf("create nested symlink: %v", err)
	}

	return dir
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skipf("reference binary grealpath not in PATH: %v", err)
	}

	fixtureDir := createFixtures(t)
	realFile := filepath.Join(fixtureDir, "real_file.txt")
	subDir := filepath.Join(fixtureDir, "subdir")
	symLink := filepath.Join(fixtureDir, "link_to_file")
	nonexistent := filepath.Join(fixtureDir, "nonexistent")

	tests := []testutils.DiffTest{
		// R3.1: no operand produces usage error, exit 1.
		{
			Name:      "no_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.2: unknown flag produces error, exit 1.
		{
			Name:      "unknown_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// Default: resolve existing file to canonical path.
		{
			Name: "default_existing_file",
			Args: []string{realFile},
		},

		// Default: resolve symlink to target's canonical path.
		{
			Name: "default_resolve_symlink",
			Args: []string{symLink},
		},

		// Default: resolve relative .. components.
		{
			Name: "default_dotdot",
			Args: []string{filepath.Join(subDir, "..", "real_file.txt")},
		},

		// -e: existing file resolves successfully.
		{
			Name: "canonicalize_existing_ok",
			Args: []string{"-e", realFile},
		},

		// -e: symlink with existing target resolves.
		{
			Name: "canonicalize_existing_symlink",
			Args: []string{"-e", symLink},
		},

		// -e: nonexistent path produces error, exit 1.
		// Stderr format differs between Go and GNU; check presence only.
		{
			Name:      "canonicalize_existing_missing",
			Args:      []string{"-e", nonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},

		// -m: nonexistent path still resolves without error.
		{
			Name: "canonicalize_missing_nonexistent",
			Args: []string{"-m", filepath.Join(fixtureDir, "no", "such", "path")},
		},

		// -m: existing path resolves normally.
		{
			Name: "canonicalize_missing_existing",
			Args: []string{"-m", realFile},
		},

		// -s: does not resolve symlink, prints the symlink path itself.
		{
			Name: "strip_symlink_not_resolved",
			Args: []string{"-s", symLink},
		},

		// -s: cleans .. components without following symlinks.
		{
			Name: "strip_dotdot",
			Args: []string{"-s", filepath.Join(subDir, "..", "real_file.txt")},
		},

		// -s: nonexistent path is cleaned without error.
		{
			Name: "strip_nonexistent",
			Args: []string{"-s", filepath.Join(fixtureDir, "no", "..", "real_file.txt")},
		},

		// R3.3: multiple paths with -e, some fail — errors for failures,
		// successful resolutions still printed, exit 1.
		{
			Name:      "mixed_success_failure_e",
			Args:      []string{"-e", realFile, nonexistent, symLink},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},

		// Multiple valid paths all succeed, exit 0.
		{
			Name: "multiple_valid_paths",
			Args: []string{realFile, symLink},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRelative exercises --relative-to and --relative-base flags.
// R4.1: differential tests for relative path output.
// R4.2: covers --relative-to, --relative-base, and combined flag interactions.
// R4.3: all tests inherit LC_ALL=C from RunDiffTests.
func TestDiffRelative(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skipf("reference binary grealpath not in PATH: %v", err)
	}

	root := createNestedFixtures(t)
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "a", "b")
	dirC := filepath.Join(root, "a", "b", "c")
	deepFile := filepath.Join(root, "a", "b", "c", "deep.txt")
	fileA := filepath.Join(root, "a", "file_a.txt")
	dirOther := filepath.Join(root, "other")
	fileO := filepath.Join(root, "other", "file_o.txt")
	linkA := filepath.Join(root, "link_to_a")

	tests := []testutils.DiffTest{
		// R2.1: --relative-to prints path relative to the given directory.
		{
			Name: "relative_to_child",
			Args: []string{"--relative-to=" + dirA, deepFile},
		},
		// R2.1: --relative-to with sibling directory requires ../
		{
			Name: "relative_to_sibling",
			Args: []string{"--relative-to=" + dirOther, fileA},
		},
		// R2.1: --relative-to with same directory returns file name.
		{
			Name: "relative_to_same_dir",
			Args: []string{"--relative-to=" + dirC, deepFile},
		},
		// R2.1: --relative-to with directory itself returns ".".
		{
			Name: "relative_to_dir_itself",
			Args: []string{"--relative-to=" + dirA, dirA},
		},
		// R2.1: --relative-to with multiple paths.
		{
			Name: "relative_to_multiple_paths",
			Args: []string{"--relative-to=" + root, fileA, deepFile, fileO},
		},

		// R2.2: --relative-base prints relative when path is under base.
		{
			Name: "relative_base_under",
			Args: []string{"--relative-base=" + dirA, deepFile},
		},
		// R2.2: --relative-base prints absolute when path is NOT under base.
		{
			Name: "relative_base_not_under",
			Args: []string{"--relative-base=" + dirA, fileO},
		},
		// R2.2: --relative-base with path equal to base returns ".".
		{
			Name: "relative_base_exact_match",
			Args: []string{"--relative-base=" + dirA, dirA},
		},
		// R2.2: --relative-base with multiple paths, some under and some not.
		{
			Name: "relative_base_mixed_paths",
			Args: []string{"--relative-base=" + dirA, fileA, fileO, deepFile},
		},

		// R2.3: both --relative-to and --relative-base set; path under base.
		{
			Name: "both_relative_under_base",
			Args: []string{
				"--relative-to=" + dirB,
				"--relative-base=" + dirA,
				deepFile,
			},
		},
		// R2.3: both set; path NOT under base prints absolute.
		{
			Name: "both_relative_not_under_base",
			Args: []string{
				"--relative-to=" + dirB,
				"--relative-base=" + dirA,
				fileO,
			},
		},
		// R2.3: both set; multiple paths with mixed base membership.
		{
			Name: "both_relative_mixed",
			Args: []string{
				"--relative-to=" + dirA,
				"--relative-base=" + dirA,
				fileA, fileO, deepFile,
			},
		},

		// D3: --relative-to combined with -s (no symlink resolution).
		{
			Name: "relative_to_with_strip",
			Args: []string{"-s", "--relative-to=" + root, linkA},
		},

		// D3: --relative-base combined with -e (canonicalize-existing).
		{
			Name: "relative_base_with_existing",
			Args: []string{"-e", "--relative-base=" + dirA, fileA},
		},

		// D3: --relative-base combined with -m (canonicalize-missing).
		{
			Name: "relative_base_with_missing",
			Args: []string{
				"-m",
				"--relative-base=" + root,
				filepath.Join(root, "no", "such", "path"),
			},
		},

		// D3: --relative-to combined with -m for nonexistent path.
		{
			Name: "relative_to_with_missing",
			Args: []string{
				"-m",
				"--relative-to=" + root,
				filepath.Join(root, "x", "y", "z"),
			},
		},

		// D3: --relative-base with -e on nonexistent path (error case).
		{
			Name:      "relative_base_existing_nonexistent",
			Args:      []string{"-e", "--relative-base=" + root, filepath.Join(root, "nope")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrorCases exercises additional error handling paths.
// R4.2/R4.3: error cases, nonexistent paths, multiple operands with failures.
func TestDiffErrorCases(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grealpath")
	if err != nil {
		t.Skipf("reference binary grealpath not in PATH: %v", err)
	}

	root := createFixtures(t)
	realFile := filepath.Join(root, "real_file.txt")
	symLink := filepath.Join(root, "link_to_file")
	nonexistent := filepath.Join(root, "nonexistent")
	deepNonexistent := filepath.Join(root, "a", "b", "c")

	tests := []testutils.DiffTest{
		// R3.3: -e with deeply nonexistent path.
		{
			Name:      "e_deep_nonexistent",
			Args:      []string{"-e", deepNonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},

		// R3.3: multiple paths all nonexistent with -e.
		{
			Name:      "e_all_nonexistent",
			Args:      []string{"-e", nonexistent, deepNonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},

		// R3.3: multiple args, first fails, rest succeed — exit 1.
		{
			Name:      "first_fails_rest_succeed",
			Args:      []string{"-e", nonexistent, realFile, symLink},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},

		// R3.3: multiple args, last fails — exit 1.
		{
			Name:      "last_fails",
			Args:      []string{"-e", realFile, symLink, nonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrPresenceNormalizer},
		},

		// -m resolves deeply nonexistent path without error.
		{
			Name: "m_deep_nonexistent",
			Args: []string{"-m", deepNonexistent},
		},

		// Double-dash ends flag processing; path-like arg treated as operand.
		{
			Name: "double_dash_existing_path",
			Args: []string{"--", realFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
