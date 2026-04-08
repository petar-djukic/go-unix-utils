// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/realpath.
// Tests cover srd049-realpath R3.1, R3.2, R3.3.
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
