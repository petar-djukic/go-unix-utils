// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var progRe = regexp.MustCompile(`(?m)^(\S*/)?g?sync:`)
var tryRe = regexp.MustCompile(`'(\S*/)?g?sync --help'`)

func normStderr(b []byte) []byte {
	b = progRe.ReplaceAll(b, []byte("PROG:"))
	b = tryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsync")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("test data\n"), 0644); err != nil {
		t.Fatal(err)
	}

	norm := []testutils.NormalizeFunc{normStderr}

	tests := []testutils.DiffTest{
		{
			Name: "no-args",
		},
		{
			Name: "file",
			Args: []string{tmpFile},
		},
		{
			Name: "data-file",
			Args: []string{"-d", tmpFile},
		},
		{
			Name: "data-long-file",
			Args: []string{"--data", tmpFile},
		},
		{
			Name: "file-system-file",
			Args: []string{"-f", tmpFile},
		},
		{
			Name: "file-system-long-file",
			Args: []string{"--file-system", tmpFile},
		},
		{
			Name: "file-system-no-file",
			Args: []string{"-f"},
		},
		{
			Name:      "data-no-file",
			Args:      []string{"-d"},
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "data-long-no-file",
			Args:      []string{"--data"},
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "both-flags",
			Args:      []string{"-d", "-f", tmpFile},
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "both-flags-combined",
			Args:      []string{"-df", tmpFile},
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "nonexistent-file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent")},
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name:      "invalid-flag",
			Args:      []string{"-x"},
			ExitCode:  1,
			Normalize: norm,
		},
		{
			Name: "file-system-nonexistent",
			Args: []string{"-f", filepath.Join(tmpDir, "nonexistent")},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
