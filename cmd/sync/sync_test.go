// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sync against GNU gsync.
// Covers prd085-sync R2.1 (--version), R2.2 (--help),
// R2.3 (error handling for nonexistent files).
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

// normalizeVersionString replaces version output with a sentinel so
// GNU and go-unix-utils version strings compare equal.
func normalizeVersionString(b []byte) []byte {
	if len(b) > 0 {
		return []byte("version output present\n")
	}
	return b
}

// normalizeHelpOutput replaces help output with a sentinel so
// GNU and go-unix-utils help text compare equal.
func normalizeHelpOutput(b []byte) []byte {
	if len(b) > 0 {
		return []byte("help output present\n")
	}
	return b
}

// stderrNormalizer normalizes error messages between GNU gsync and Go sync.
// Handles binary name differences, file paths, and capitalization.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?sync|gsync`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	absPath := regexp.MustCompile(`'/[^']*'`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("sync"))
		b = tryHelp.ReplaceAll(b, nil)
		b = absPath.ReplaceAll(b, []byte("'PATH'"))
		b = bytes.ToLower(b)
		return b
	}
}

// TestDiff runs differential tests comparing the Go sync binary against gsync.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsync")
	if err != nil {
		t.Skipf("reference binary gsync not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()
	versionNorm := []testutils.NormalizeFunc{normalizeVersionString}
	helpNorm := []testutils.NormalizeFunc{normalizeHelpOutput}

	// Create a temp file for file-based sync tests.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("sync test data\n"), 0o644); err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	tests := []testutils.DiffTest{
		// R2.1: --version prints version info and exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: versionNorm,
		},
		// R2.2: --help prints usage info and exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: helpNorm,
		},
		// R1.1: no arguments syncs all filesystems.
		{
			Name: "no_args",
		},
		// R1.2: sync a specific file.
		{
			Name: "file_sync",
			Args: []string{tmpFile},
		},
		// R1.3: --data flag syncs file data only.
		{
			Name: "data_flag",
			Args: []string{"--data", tmpFile},
		},
		// R1.3: -d short flag.
		{
			Name: "data_flag_short",
			Args: []string{"-d", tmpFile},
		},
		// R1.4: --file-system flag syncs the filesystem.
		{
			Name: "file_system_flag",
			Args: []string{"--file-system", tmpFile},
		},
		// R1.4: -f short flag.
		{
			Name: "file_system_flag_short",
			Args: []string{"-f", tmpFile},
		},
		// R2.3: nonexistent file produces error and exits 1.
		{
			Name:      "nonexistent_file",
			Args:      []string{"no_such_file_xyz"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.3: --data without FILE exits 1.
		{
			Name:      "data_no_file",
			Args:      []string{"--data"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.4: --file-system without FILE exits 1.
		{
			Name:      "file_system_no_file",
			Args:      []string{"--file-system"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
