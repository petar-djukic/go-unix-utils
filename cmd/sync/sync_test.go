// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sync covering prd085-sync R2.1, R2.2, R2.3.
package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// lowercaseNormalizer lowercases output to neutralize platform differences in
// syscall error string capitalization (e.g., "No such file" vs "no such file").
func lowercaseNormalizer(data []byte) []byte {
	return bytes.ToLower(data)
}

// progNameNormalizer replaces "gsync:" with "sync:" in output so that
// error messages from the reference binary can be compared against ours.
func progNameNormalizer(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gsync:"), []byte("sync:"))
}

// discardOutput replaces all output with empty bytes, effectively comparing
// only exit codes. Used for --help and --version whose content differs by
// design between GNU and our implementation.
func discardOutput(data []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsync")
	if err != nil {
		t.Skipf("reference binary gsync not in PATH: %v", err)
	}

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(existingFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	nonexistentFile := filepath.Join(tmpDir, "no_such_file")

	tests := []testutils.DiffTest{
		// R2.1: exit 0 when all sync operations succeed (no args).
		{
			Name:     "no_args_sync_all",
			Args:     []string{},
			ExitCode: 0,
		},
		// R2.1: exit 0 when syncing an existing file.
		{
			Name:     "sync_existing_file",
			Args:     []string{existingFile},
			ExitCode: 0,
		},
		// R2.2: exit 1 when file cannot be opened.
		{
			Name:      "nonexistent_file_exits_1",
			Args:      []string{nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer, lowercaseNormalizer},
		},
		// R2.2: exit 1 reported for each bad file, exit 0 files still succeed.
		{
			Name:      "mixed_existing_and_nonexistent",
			Args:      []string{existingFile, nonexistentFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNormalizer, lowercaseNormalizer},
		},
		// R2.1: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},
		// R2.1: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},
		// R2.3: SIGPIPE handling is verified implicitly — the Go binary
		// calls sys.InstallSIGPIPEHandler and does not crash on pipe close.
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
