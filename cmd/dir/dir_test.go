// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/dir differential tests for prd107-dir R1.1-R1.5, R2.1-R2.4.
package main

import (
	"bytes"
	"fmt"
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
		// R1.5: ls-compatible flag tests
		{
			// R1.5: long format listing with -l.
			Name:      "R1.5_long_format",
			Args:      []string{"-l"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.5: classify indicator with -F.
			Name:      "R1.5_classify_indicator",
			Args:      []string{"-1F"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.5: reverse sort with -r.
			Name:      "R1.5_reverse_sort",
			Args:      []string{"-1r"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.5: across column layout with -x.
			Name:      "R1.5_across_columns",
			Args:      []string{"-x"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R2.1: successful listing exits 0 (implicit in all passing tests).
		// R2.2: nonexistent path exits with error.
		{
			// R2.2/R2.3: nonexistent path produces error and nonzero exit.
			Name:      "R2.2_nonexistent_path",
			Args:      []string{"/nonexistent_path_for_dir_test"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R2.3: invalid option exits 2.
		{
			// R2.3: invalid option produces exit code 2.
			Name:      "R2.3_invalid_option",
			Args:      []string{"--invalid-flag-xyz"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R2.4: SIGPIPE handling — piping to head exits cleanly.
		{
			Name:      "R2.4_sigpipe_large_listing",
			Args:      []string{"-1", basicDir},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPE verifies that dir handles SIGPIPE gracefully when piped
// to a truncating consumer (R2.4).
func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	headBin, err := exec.LookPath("head")
	if err != nil {
		t.Skip("head not in PATH")
	}

	dir := t.TempDir()
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("file_%03d", i)
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatalf("create fixture: %v", err)
		}
	}

	// Pipe dir -1 output through head -1 to trigger SIGPIPE.
	dirCmd := exec.Command(goBin, "-1", dir)
	dirCmd.Env = append(os.Environ(), "LC_ALL=C")
	headCmd := exec.Command(headBin, "-1")
	headCmd.Stdin, _ = dirCmd.StdoutPipe()
	headCmd.Stdout = os.Stdout

	if err := headCmd.Start(); err != nil {
		t.Fatalf("start head: %v", err)
	}
	if err := dirCmd.Run(); err != nil {
		// SIGPIPE should cause exit 0, not an error.
		t.Errorf("dir exited with error after SIGPIPE: %v", err)
	}
	_ = headCmd.Wait() // best-effort: head may already have exited
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
