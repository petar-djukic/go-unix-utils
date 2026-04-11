// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/dir via differential testing against gdir.
// Tests srd107-dir R1.1-R1.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// createFixture creates a test directory with files for column layout testing.
func createFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
	}
	for _, f := range files {
		err := os.WriteFile(filepath.Join(dir, f), []byte(f), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// normalizeProgramName replaces "gdir:" with "dir:" in output so
// error messages from the reference binary match the Go binary.
func normalizeProgramName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gdir:"), []byte("dir:"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdir")
	if err != nil {
		t.Skipf("reference binary gdir not in PATH: %v", err)
	}
	dir := createFixture(t)

	tests := []testutils.DiffTest{
		// R1.1: multi-column output by default.
		{
			Name:    "default_multicolumn",
			Args:    []string{dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.4: default to current directory when no args.
		{
			Name:    "default_no_args",
			Args:    []string{},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.3: sort in C locale order.
		{
			Name:    "sorted_output",
			Args:    []string{dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: accepts -l flag.
		{
			Name: "long_format",
			Args: []string{"-l", dir},
			Env:  []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{
				testutils.TimestampNormalizer,
			},
		},
		// R1.5: accepts -a flag (show all entries including . and ..).
		{
			Name:    "show_all",
			Args:    []string{"-a", dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: accepts -1 flag (one per line).
		{
			Name:    "one_per_line",
			Args:    []string{"-1", dir},
			Env:     []string{"LC_ALL=C"},
			WorkDir: dir,
		},
		// R1.5: accepts -r flag (reverse sort).
		{
			Name:    "reverse_sort",
			Args:    []string{"-r", dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R1.5: accepts --color=never flag.
		{
			Name:    "color_never",
			Args:    []string{"--color=never", dir},
			Env:     []string{"LC_ALL=C", "COLUMNS=80"},
			WorkDir: dir,
		},
		// R2.2/R2.3: exit code for non-existent path.
		{
			Name:      "nonexistent_path",
			Args:      []string{"/no/such/path/xyzzy"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
