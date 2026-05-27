// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func programNameNormalizer(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gdir:"), []byte("dir:"))
}

func touchFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdir")
	if err != nil {
		t.Skip("reference binary not found")
	}

	errNorm := []testutils.NormalizeFunc{programNameNormalizer}

	basicDir := t.TempDir()
	for _, name := range []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
	} {
		touchFile(t, basicDir, name)
	}

	mixedDir := t.TempDir()
	for _, name := range []string{".hidden", "Bravo", "alpha", ".secret", "Charlie"} {
		touchFile(t, mixedDir, name)
	}

	sortDir := t.TempDir()
	for _, name := range []string{"A", "B", "a", "b", "Z", "z"} {
		touchFile(t, sortDir, name)
	}

	escapeDir := t.TempDir()
	for _, name := range []string{"normal", "has space", "back\\slash", "tab\there"} {
		touchFile(t, escapeDir, name)
	}

	defaultDir := t.TempDir()
	for _, name := range []string{"one", "two", "three"} {
		touchFile(t, defaultDir, name)
	}

	tests := []testutils.DiffTest{
		{Name: "basic-listing", Args: []string{basicDir}, Env: []string{"COLUMNS=80"}},
		{Name: "hidden-excluded", Args: []string{mixedDir}, Env: []string{"COLUMNS=80"}},
		{Name: "c-locale-sort", Args: []string{sortDir}, Env: []string{"COLUMNS=80"}},
		{Name: "escape-special", Args: []string{escapeDir}, Env: []string{"COLUMNS=80"}},
		{Name: "default-cwd", WorkDir: defaultDir, Env: []string{"COLUMNS=80"}},
		{
			Name:      "nonexistent",
			Args:      []string{"/nonexistent-path-12345"},
			ExitCode:  1,
			Normalize: errNorm,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
