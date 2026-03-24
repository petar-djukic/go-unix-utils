// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/readlink against GNU greadlink.
// Covers prd050-readlink R4.1-R4.3 (differential testing).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU and Go binaries.
// Strips binary paths, "Try ... --help" lines, and the GNU "ignoring
// --no-newline" warning so cosmetic differences do not cause failures.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?readlink|greadlink`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	ignoreNL := regexp.MustCompile(`(?m)^.*ignoring --no-newline.*\n?`)
	// Normalize different error message wording.
	flagErr := regexp.MustCompile(`(?m)^readlink: (unrecognized option|flag provided but not defined:) '?--?([^'\n]+)'?\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("readlink"))
		b = tryHelp.ReplaceAll(b, nil)
		b = ignoreNL.ReplaceAll(b, nil)
		b = flagErr.ReplaceAll(b, []byte("readlink: bad option --$2\n"))
		return b
	}
}

// clearStderrNormalizer returns a normalizer that clears stderr content.
// Used for tests where stderr verbosity differs between GNU and Go
// (GNU readlink without -v does not print errors for canon modes,
// while the Go implementation always reports errors for canon modes).
func clearStderrNormalizer() testutils.NormalizeFunc {
	return func(b []byte) []byte {
		return nil
	}
}

// setupFixtures creates symlinks and files in tmpDir for testing.
func setupFixtures(t *testing.T, tmpDir string) {
	t.Helper()

	// Regular file.
	if err := os.WriteFile(filepath.Join(tmpDir, "regular.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("create regular file: %v", err)
	}

	// Simple symlink pointing to the regular file (absolute target).
	if err := os.Symlink(filepath.Join(tmpDir, "regular.txt"), filepath.Join(tmpDir, "link_to_regular")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	// Relative symlink.
	if err := os.Symlink("regular.txt", filepath.Join(tmpDir, "rel_link")); err != nil {
		t.Fatalf("create relative symlink: %v", err)
	}

	// Chained symlink: chain_link -> link_to_regular -> regular.txt.
	if err := os.Symlink(filepath.Join(tmpDir, "link_to_regular"), filepath.Join(tmpDir, "chain_link")); err != nil {
		t.Fatalf("create chain symlink: %v", err)
	}

	// Subdirectory with a file inside.
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested\n"), 0o644); err != nil {
		t.Fatalf("create nested file: %v", err)
	}

	// Symlink to subdirectory.
	if err := os.Symlink(subDir, filepath.Join(tmpDir, "link_to_subdir")); err != nil {
		t.Fatalf("create dir symlink: %v", err)
	}

	// Dangling symlink (target does not exist).
	if err := os.Symlink(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dangling")); err != nil {
		t.Fatalf("create dangling symlink: %v", err)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skipf("reference binary greadlink not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	setupFixtures(t, tmpDir)

	errNorm := stderrNormalizer()
	clearStderr := clearStderrNormalizer()

	// Build absolute paths for fixtures.
	regularFile := filepath.Join(tmpDir, "regular.txt")
	symlink := filepath.Join(tmpDir, "link_to_regular")
	relSymlink := filepath.Join(tmpDir, "rel_link")
	chainLink := filepath.Join(tmpDir, "chain_link")
	dirLink := filepath.Join(tmpDir, "link_to_subdir")
	danglingLink := filepath.Join(tmpDir, "dangling")
	nonexistent := filepath.Join(tmpDir, "no_such_file")
	missingDeep := filepath.Join(tmpDir, "no_such_dir", "no_such_file")

	tests := []testutils.DiffTest{
		// --- R4.1/R4.2: Default mode tests ---

		// Default mode — symlink prints immediate target.
		{
			Name: "default_symlink",
			Args: []string{symlink},
		},
		// Default mode — relative symlink prints relative target.
		{
			Name: "default_relative_symlink",
			Args: []string{relSymlink},
		},
		// Default mode — non-symlink exits 1, no stdout.
		{
			Name:      "default_non_symlink",
			Args:      []string{regularFile},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// Default mode — nonexistent path exits 1.
		{
			Name:      "default_nonexistent",
			Args:      []string{nonexistent},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// Default mode — directory (not a symlink) exits 1.
		{
			Name:      "default_directory",
			Args:      []string{tmpDir},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// Default mode — dangling symlink still prints target.
		{
			Name: "default_dangling_symlink",
			Args: []string{danglingLink},
		},

		// --- R4.2: -f (canonicalize) tests ---

		// -f — existing symlink resolves to canonical path.
		{
			Name: "canon_f_symlink",
			Args: []string{"-f", symlink},
		},
		// -f — chain of symlinks resolves fully.
		{
			Name: "canon_f_chain",
			Args: []string{"-f", chainLink},
		},
		// -f — regular file returns canonical path.
		{
			Name: "canon_f_regular",
			Args: []string{"-f", regularFile},
		},
		// -f — path through symlinked directory.
		{
			Name: "canon_f_through_symlink_dir",
			Args: []string{"-f", filepath.Join(dirLink, "nested.txt")},
		},
		// -f — last component missing, parent dir exists.
		{
			Name: "canon_f_partial_path",
			Args: []string{"-f", filepath.Join(tmpDir, "no_such_file")},
		},
		// -f — directory in path missing, exits 1.
		{
			Name:      "canon_f_missing_dir",
			Args:      []string{"-f", missingDeep},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},

		// --- R4.2: -e (canonicalize-existing) tests ---

		// -e — all components exist via symlink.
		{
			Name: "canon_e_existing",
			Args: []string{"-e", symlink},
		},
		// -e — regular file.
		{
			Name: "canon_e_regular",
			Args: []string{"-e", regularFile},
		},
		// -e — missing file exits 1.
		{
			Name:      "canon_e_missing",
			Args:      []string{"-e", nonexistent},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},
		// -e — dangling symlink exits 1.
		{
			Name:      "canon_e_dangling",
			Args:      []string{"-e", danglingLink},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},

		// --- R4.2: -m (canonicalize-missing) tests ---

		// -m — existing symlink.
		{
			Name: "canon_m_existing",
			Args: []string{"-m", symlink},
		},
		// -m — completely missing path still prints.
		{
			Name: "canon_m_missing",
			Args: []string{"-m", missingDeep},
		},
		// -m — regular file.
		{
			Name: "canon_m_regular",
			Args: []string{"-m", regularFile},
		},

		// --- R4.2: -n (no-newline) tests ---

		// -n — suppress trailing newline for single operand.
		{
			Name: "no_newline_single",
			Args: []string{"-n", symlink},
		},
		// -n with -f.
		{
			Name: "no_newline_canon_f",
			Args: []string{"-n", "-f", regularFile},
		},

		// --- R4.2: -z (zero/NUL delimiter) tests ---

		// -z — NUL-terminated output.
		{
			Name: "zero_single",
			Args: []string{"-z", symlink},
		},
		// -z with -f.
		{
			Name: "zero_canon_f",
			Args: []string{"-z", "-f", regularFile},
		},

		// --- R4.2/R4.3: Multiple argument tests ---

		// Multiple args — each on separate line.
		{
			Name: "multi_args",
			Args: []string{"-f", symlink, regularFile},
		},
		// Multiple args with -n — -n ignored per R2.2.
		{
			Name:      "multi_args_n_ignored",
			Args:      []string{"-n", "-f", symlink, regularFile},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Multiple args with -z — NUL between results.
		{
			Name: "multi_args_zero",
			Args: []string{"-z", "-f", symlink, regularFile},
		},
		// Multiple args — mixed success/failure, exit 1.
		{
			Name:      "multi_mixed_exit",
			Args:      []string{"-e", symlink, nonexistent},
			Normalize: []testutils.NormalizeFunc{clearStderr},
		},

		// --- R4.2: Error case tests ---

		// No arguments — error, exit 1.
		{
			Name:      "no_args_error",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Invalid option — error, exit 1.
		{
			Name:      "invalid_option",
			Args:      []string{"--invalid-option"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},

		// --- R4.2: Long flag form tests ---

		{
			Name: "long_canonicalize",
			Args: []string{"--canonicalize", regularFile},
		},
		{
			Name: "long_canonicalize_existing",
			Args: []string{"--canonicalize-existing", regularFile},
		},
		{
			Name: "long_canonicalize_missing",
			Args: []string{"--canonicalize-missing", nonexistent},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
