// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/readlink.
// Tests cover srd050-readlink R4.1, R4.2, R4.3.
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
// R4.1: replaces binary paths with "PROG" so error message structure can
// be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// stderrClearNormalizer discards all stderr content. Used when the Go
// and GNU binaries differ on whether they emit stderr for a given case
// but agree on stdout and exit code.
func stderrClearNormalizer(b []byte) []byte {
	return nil
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

// createFixtures sets up a directory with files, subdirectories, and symlinks
// for differential tests. Returns the resolved fixture root.
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

	// Symlink to a directory.
	dirLink := filepath.Join(dir, "link_to_subdir")
	if err := os.Symlink(subDir, dirLink); err != nil {
		t.Fatalf("create fixture dir symlink: %v", err)
	}

	// Chained symlink: link_chain -> link_to_file -> real_file.txt.
	chainLink := filepath.Join(dir, "link_chain")
	if err := os.Symlink(symLink, chainLink); err != nil {
		t.Fatalf("create fixture chain symlink: %v", err)
	}

	return dir
}

// TestDiff exercises default, canonicalization, and flag behavior.
// R4.1: differential tests comparing stdout and exit codes.
// R4.2: covers symlink target, non-symlink, -f, -e, -m, -n, multiple operands.
// R4.3: LC_ALL=C is set by RunDiffTests.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skipf("reference binary greadlink not in PATH: %v", err)
	}

	dir := createFixtures(t)
	realFile := filepath.Join(dir, "real_file.txt")
	subDir := filepath.Join(dir, "subdir")
	symLink := filepath.Join(dir, "link_to_file")
	dirLink := filepath.Join(dir, "link_to_subdir")
	chainLink := filepath.Join(dir, "link_chain")
	nonexistent := filepath.Join(dir, "nonexistent")

	tests := []testutils.DiffTest{
		// R1.1: symlink operand prints the immediate target.
		{
			Name: "default_symlink",
			Args: []string{symLink},
		},
		// R1.1: chained symlink prints the immediate target (next link).
		{
			Name: "default_chain_symlink",
			Args: []string{chainLink},
		},
		// R1.1: directory symlink prints the immediate target.
		{
			Name: "default_dir_symlink",
			Args: []string{dirLink},
		},
		// R1.2: non-symlink operand prints nothing, exits 1.
		{
			Name:     "default_non_symlink_file",
			Args:     []string{realFile},
			ExitCode: 1,
		},
		// R1.2: directory (not a symlink) prints nothing, exits 1.
		{
			Name:     "default_non_symlink_dir",
			Args:     []string{subDir},
			ExitCode: 1,
		},
		// R1.2: nonexistent path prints nothing, exits 1.
		{
			Name:     "default_nonexistent",
			Args:     []string{nonexistent},
			ExitCode: 1,
		},

		// R1.3: -f resolves canonical path for existing symlink.
		{
			Name: "canon_f_symlink",
			Args: []string{"-f", symLink},
		},
		// R1.3: -f resolves canonical path for regular file.
		{
			Name: "canon_f_regular_file",
			Args: []string{"-f", realFile},
		},
		// R1.3: -f with chain symlink fully resolves.
		{
			Name: "canon_f_chain",
			Args: []string{"-f", chainLink},
		},
		// R1.3: -f with nonexistent last component (parent exists).
		{
			Name: "canon_f_partial",
			Args: []string{"-f", filepath.Join(dir, "nonexistent_file")},
		},
		// R1.3: -f with nonexistent parent directory exits 1.
		// Stderr content differs; clear to compare stdout and exit code only.
		{
			Name:      "canon_f_missing_parent",
			Args:      []string{"-f", filepath.Join(dir, "no", "such", "file")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClearNormalizer},
		},
		// R1.3: --canonicalize long flag.
		{
			Name: "canon_long_flag",
			Args: []string{"--canonicalize", symLink},
		},

		// R1.4: -e resolves existing symlink.
		{
			Name: "existing_e_symlink",
			Args: []string{"-e", symLink},
		},
		// R1.4: -e resolves existing regular file.
		{
			Name: "existing_e_regular_file",
			Args: []string{"-e", realFile},
		},
		// R1.4: -e with nonexistent path exits 1.
		// Stderr content differs; clear to compare stdout and exit code only.
		{
			Name:      "existing_e_nonexistent",
			Args:      []string{"-e", nonexistent},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClearNormalizer},
		},
		// R1.4: --canonicalize-existing long flag.
		{
			Name: "existing_long_flag",
			Args: []string{"--canonicalize-existing", realFile},
		},

		// R1.5: -m resolves even when no component exists.
		{
			Name: "missing_m_nonexistent",
			Args: []string{"-m", filepath.Join(dir, "no", "such", "path")},
		},
		// R1.5: -m resolves existing path normally.
		{
			Name: "missing_m_existing",
			Args: []string{"-m", realFile},
		},
		// R1.5: -m resolves symlink in existing prefix.
		{
			Name: "missing_m_symlink_prefix",
			Args: []string{"-m", filepath.Join(symLink, "child")},
		},
		// R1.5: --canonicalize-missing long flag.
		{
			Name: "missing_long_flag",
			Args: []string{"--canonicalize-missing", filepath.Join(dir, "x")},
		},

		// R1.6: -n suppresses trailing newline for single operand.
		{
			Name: "no_newline_single",
			Args: []string{"-n", symLink},
		},
		// R1.6: -n with -f.
		{
			Name: "no_newline_with_f",
			Args: []string{"-n", "-f", symLink},
		},
		// R1.6: --no-newline long flag.
		{
			Name: "no_newline_long_flag",
			Args: []string{"--no-newline", symLink},
		},

		// R2.1: multiple operands print each on a separate line.
		{
			Name: "multiple_operands",
			Args: []string{symLink, dirLink},
		},
		// R2.1: multiple operands with -f.
		{
			Name: "multiple_operands_f",
			Args: []string{"-f", symLink, realFile},
		},
		// R2.2: -n is ignored with multiple operands (newlines always printed).
		// GNU greadlink emits a warning; Go binary does not. Clear stderr.
		{
			Name:      "no_newline_ignored_multiple",
			Args:      []string{"-n", symLink, dirLink},
			Normalize: []testutils.NormalizeFunc{stderrClearNormalizer},
		},

		// Mixed success/failure: some operands fail in default mode, exit 1.
		{
			Name:     "multiple_mixed_default",
			Args:     []string{symLink, realFile, dirLink},
			ExitCode: 1,
		},
		// Mixed success/failure with -e.
		// Stderr content differs; clear to compare stdout and exit code only.
		{
			Name:      "multiple_mixed_e",
			Args:      []string{"-e", realFile, nonexistent, symLink},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrClearNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrors exercises error handling paths.
// R4.2: covers no operand, unknown flags, and edge cases.
// R4.3: LC_ALL=C is set by RunDiffTests.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("greadlink")
	if err != nil {
		t.Skipf("reference binary greadlink not in PATH: %v", err)
	}

	dir := createFixtures(t)
	symLink := filepath.Join(dir, "link_to_file")

	tests := []testutils.DiffTest{
		// R3.1: no operand prints usage error, exit 1.
		{
			Name:      "no_operand",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.2: unknown long flag produces error, exit 1.
		{
			Name:      "unknown_long_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// R3.2: unknown short flag produces error, exit 1.
		{
			Name:      "unknown_short_flag",
			Args:      []string{"-Z"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
		// Double-dash ends flag processing.
		{
			Name: "double_dash",
			Args: []string{"-f", "--", symLink},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
