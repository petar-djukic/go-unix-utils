// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/vdir differential tests for prd108-vdir R1.1-R2.4.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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
	timeSortDir := setupTimeSortDir(t)
	sizeSortDir := setupSizeSortDir(t)
	extSortDir := setupExtSortDir(t)
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
		{
			// R1.5: -m comma format overrides default long listing.
			Name:      "R1.5_comma_format_override",
			Args:      []string{"-m"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R1.5: -x across format overrides default long listing.
			Name:      "R1.5_across_format_override",
			Args:      []string{"-x"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.1: --color=never disables color output.
			Name:      "R2.1_color_never",
			Args:      []string{"--color=never"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.1: --color=always enables color output.
			Name:      "R2.1_color_always",
			Args:      []string{"--color=always"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.2: -a shows . and .. entries.
			Name:      "R2.2_show_all_dotfiles",
			Args:      []string{"-a", "-1"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.2: -A shows dotfiles except . and ..
			Name:      "R2.2_almost_all_dotfiles",
			Args:      []string{"-A", "-1"},
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
		// R2.3: sorting flags with long listing output.
		{
			// R2.3: -t sorts by modification time in long listing.
			Name:      "R2.3_sort_by_time",
			Args:      []string{"-t"},
			WorkDir:   timeSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: -t -r reverses time sort order.
			Name:      "R2.3_sort_by_time_reversed",
			Args:      []string{"-t", "-r"},
			WorkDir:   timeSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: -S sorts by file size, largest first.
			Name:      "R2.3_sort_by_size",
			Args:      []string{"-S"},
			WorkDir:   sizeSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: -S -r reverses size sort order.
			Name:      "R2.3_sort_by_size_reversed",
			Args:      []string{"-S", "-r"},
			WorkDir:   sizeSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: -X sorts by extension.
			Name:      "R2.3_sort_by_extension",
			Args:      []string{"-X"},
			WorkDir:   extSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: --sort=time equivalent to -t.
			Name:      "R2.3_long_sort_time",
			Args:      []string{"--sort=time"},
			WorkDir:   timeSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: --sort=size equivalent to -S.
			Name:      "R2.3_long_sort_size",
			Args:      []string{"--sort=size"},
			WorkDir:   sizeSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: --sort=extension equivalent to -X.
			Name:      "R2.3_long_sort_extension",
			Args:      []string{"--sort=extension"},
			WorkDir:   extSortDir,
			Env:       []string{"LC_ALL=C"},
			Normalize: []testutils.NormalizeFunc{norm},
		},
		{
			// R2.3: --reverse equivalent to -r.
			Name:      "R2.3_long_reverse",
			Args:      []string{"--reverse"},
			WorkDir:   basicDir,
			Env:       []string{"LC_ALL=C"},
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
	for i := range 200 {
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

// setupTimeSortDir creates files with distinct modification times.
func setupTimeSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	files := []struct {
		name   string
		offset time.Duration
	}{
		{"oldest", 0},
		{"middle", 1 * time.Hour},
		{"newest", 2 * time.Hour},
	}
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("create time fixture %s: %v", f.name, err)
		}
		mtime := base.Add(f.offset)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("chtimes %s: %v", f.name, err)
		}
	}
	return dir
}

// setupSizeSortDir creates files with different sizes.
func setupSizeSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := []struct {
		name string
		size int
	}{
		{"small", 10},
		{"medium", 1000},
		{"large", 10000},
	}
	for _, f := range files {
		data := make([]byte, f.size)
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("create size fixture %s: %v", f.name, err)
		}
	}
	return dir
}

// setupExtSortDir creates files with different extensions.
func setupExtSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	names := []string{
		"readme",
		"main.go",
		"style.css",
		"data.txt",
		"app.go",
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("create ext fixture %s: %v", name, err)
		}
	}
	return dir
}
