// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/vdir differential tests for prd108-vdir R1.1-R1.4.
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

// normalizeVdirName normalizes the binary name in error messages so
// gvdir and vdir outputs can be compared.
func normalizeVdirName(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = normalizeVdirLine(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

// normalizeVdirLine normalizes a single line of output.
func normalizeVdirLine(line []byte) []byte {
	if colonIdx := bytes.Index(line, []byte(": ")); colonIdx >= 0 {
		prog := filepath.Base(string(line[:colonIdx]))
		if prog == "vdir" || prog == "gvdir" {
			return append([]byte("vdir"), line[colonIdx:]...)
		}
	}
	if bytes.HasPrefix(line, []byte("Try '")) {
		if spIdx := bytes.Index(line[5:], []byte(" ")); spIdx >= 0 {
			prog := filepath.Base(string(line[5 : 5+spIdx]))
			if prog == "vdir" || prog == "gvdir" {
				return append([]byte("Try 'vdir"), line[5+spIdx:]...)
			}
		}
	}
	return line
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gvdir")
	if err != nil {
		t.Skipf("reference binary gvdir not in PATH: %v", err)
	}

	basicDir := setupBasicDir(t)
	escapeDir := setupEscapeDir(t)
	norm := testutils.ComposeNormalizers(normalizeVdirName)

	tests := []testutils.DiffTest{
		{
			// R1.1: long format output by default.
			// R1.4: C locale sort, dotfiles hidden.
			// R1.5: defaults to current directory.
			Name:      "R1.1_R1.4_default_long_listing",
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.1: long format output with explicit directory argument.
			Name:      "R1.1_explicit_dir_arg",
			Args:      []string{basicDir},
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.4: dotfiles shown with -a flag.
			Name:      "R1.4_show_all_with_a",
			Args:      []string{"-a"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.4: dotfiles shown with -A flag (without . and ..).
			Name:      "R1.4_almost_all_with_A",
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
			// R1.6: single-column override with -1.
			Name:      "R1.6_single_column_override",
			Args:      []string{"-1"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.6: multi-column override with -C.
			Name:      "R1.6_multi_column_override",
			Args:      []string{"-C"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.6: classify indicator with -F.
			Name:      "R1.6_classify_indicator",
			Args:      []string{"-F"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.6: reverse sort with -r.
			Name:      "R1.6_reverse_sort",
			Args:      []string{"-r"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		// R2.1: successful listing exits 0 (implicit in all passing tests).
		// R2.2: nonexistent path exits with error.
		{
			// R2.2/R2.3: nonexistent path produces error and nonzero exit.
			Name:      "R2.2_nonexistent_path",
			Args:      []string{"/nonexistent_path_for_vdir_test"},
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPE verifies that vdir handles SIGPIPE gracefully when piped
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

	// Pipe vdir output through head -1 to trigger SIGPIPE.
	vdirCmd := exec.Command(goBin, dir)
	vdirCmd.Env = append(os.Environ(), "LC_ALL=C")
	headCmd := exec.Command(headBin, "-1")
	headCmd.Stdin, _ = vdirCmd.StdoutPipe()
	headCmd.Stdout = os.Stdout

	if err := headCmd.Start(); err != nil {
		t.Fatalf("start head: %v", err)
	}
	if err := vdirCmd.Run(); err != nil {
		// SIGPIPE should cause exit 0, not an error.
		t.Errorf("vdir exited with error after SIGPIPE: %v", err)
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
