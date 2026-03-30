// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/dir differential tests for prd107-dir R1.1-R1.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeDirName normalizes the binary name in error messages so
// gdir and dir outputs can be compared.
func normalizeDirName(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = normalizeDirLine(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

// normalizeDirLine normalizes a single line of output.
func normalizeDirLine(line []byte) []byte {
	if colonIdx := bytes.Index(line, []byte(": ")); colonIdx >= 0 {
		prog := filepath.Base(string(line[:colonIdx]))
		if prog == "dir" || prog == "gdir" {
			return append([]byte("dir"), line[colonIdx:]...)
		}
	}
	if bytes.HasPrefix(line, []byte("Try '")) {
		if spIdx := bytes.Index(line[5:], []byte(" ")); spIdx >= 0 {
			prog := filepath.Base(string(line[5 : 5+spIdx]))
			if prog == "dir" || prog == "gdir" {
				return append([]byte("Try 'dir"), line[5+spIdx:]...)
			}
		}
	}
	return line
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdir")
	if err != nil {
		t.Skipf("reference binary gdir not in PATH: %v", err)
	}

	basicDir := setupBasicDir(t)
	escapeDir := setupEscapeDir(t)
	norm := testutils.ComposeNormalizers(normalizeDirName)

	tests := []testutils.DiffTest{
		{
			// R1.4: no arguments defaults to current directory.
			// R1.1: multi-column output.
			// R1.3: C locale sort, dotfiles hidden.
			Name:      "R1.1_R1.3_R1.4_default_listing",
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.1: multi-column output with explicit directory argument.
			Name:      "R1.1_explicit_dir_arg",
			Args:      []string{basicDir},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.3: dotfiles shown with -a flag.
			Name:      "R1.3_show_all_with_a",
			Args:      []string{"-a"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.3: dotfiles shown with -A flag (without . and ..).
			Name:      "R1.3_almost_all_with_A",
			Args:      []string{"-A"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.2: C-style escaping of non-printable characters.
			Name:      "R1.2_escape_special_chars",
			WorkDir:   escapeDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.1: single-column override with -1.
			Name:      "R1.1_single_column_override",
			Args:      []string{"-1"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// setupBasicDir creates a directory with regular files and dotfiles.
func setupBasicDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	names := []string{
		"alpha", "beta", "gamma", "delta", "epsilon",
		"zeta", "eta", "theta", "iota", "kappa",
		".hidden", ".secret",
	}
	for _, name := range names {
		err := os.WriteFile(filepath.Join(dir, name), nil, 0o644)
		if err != nil {
			t.Fatalf("create fixture %s: %v", name, err)
		}
	}
	return dir
}

// setupEscapeDir creates files with names that require C-style escaping.
func setupEscapeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	names := []string{
		"normal",
		"with\ttab",
		"back\\slash",
	}
	for _, name := range names {
		err := os.WriteFile(filepath.Join(dir, name), nil, 0o644)
		if err != nil {
			t.Fatalf("create escape fixture %q: %v", name, err)
		}
	}
	return dir
}
