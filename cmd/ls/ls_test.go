// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd008-ls R1.1-R1.8, R2.3-R2.6, R3.4-R3.15, R4.1-R4.9 (R4.5 via code review).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?ls\b`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("ls"))
	})

	basicDir := setupDir(t, "apple", "banana", "cherry")
	dotDir := setupDir(t, ".hidden", ".secret", "shown", "visible")
	sortDir := setupDir(t, "Banana", "apple", "Cherry", "date")
	onlyDotDir := setupDir(t, ".hidden", ".secret")
	timeSortDir := setupTimeSortDir(t)
	sizeSortDir := setupSizeSortDir(t)
	humanSizeDir := setupHumanSizeDir(t)
	classifyDir := setupClassifyDir(t)
	recursiveDir := setupRecursiveDir(t)
	recursiveSymlinkDir := setupRecursiveSymlinkDir(t)
	recursiveTimeSortDir := setupRecursiveTimeSortDir(t)

	tests := []testutils.DiffTest{
		{
			Name: "empty-directory",
		},
		{
			Name: "basic-listing",
			Args: []string{basicDir},
		},
		{
			Name: "dot-files-hidden",
			Args: []string{dotDir},
		},
		{
			Name: "c-locale-sort",
			Args: []string{sortDir},
		},
		{
			Name: "only-dot-files",
			Args: []string{onlyDotDir},
		},
		{
			Name:      "unknown-long-option",
			Args:      []string{"--badopt"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name: "single-column-flag",
			Args: []string{"-1", basicDir},
		},
		{
			Name: "long-format",
			Args: []string{"-l", basicDir},
		},
		{
			Name: "long-format-file",
			Args: []string{"-l", filepath.Join(basicDir, "apple")},
		},
		{
			Name: "long-format-empty",
			Args: []string{"-l"},
		},
		{
			Name: "combined-one-long",
			Args: []string{"-1l", basicDir},
		},
		{
			Name: "combined-long-one",
			Args: []string{"-l1", basicDir},
		},
		{
			Name:      "unknown-short-option",
			Args:      []string{"-z"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
		},
		{
			Name: "dir-flag",
			Args: []string{"-d", basicDir},
		},
		{
			Name: "dir-flag-long",
			Args: []string{"-ld", basicDir},
		},
		{
			Name: "all-almost-all-precedence",
			Args: []string{"-1", "-aA", dotDir},
		},
		{
			Name: "almost-all-all-precedence",
			Args: []string{"-1", "-Aa", dotDir},
		},
		{
			Name: "time-sort",
			Args: []string{"-1t", timeSortDir},
		},
		{
			Name: "size-sort",
			Args: []string{"-1S", sizeSortDir},
		},
		{
			Name: "color-never",
			Args: []string{"--color=never", basicDir},
		},
		{
			Name: "long-human-readable",
			Args: []string{"-lh", humanSizeDir},
		},
		{
			Name: "long-human-readable-blocks",
			Args: []string{"-lhs", humanSizeDir},
		},
		{
			Name: "blocks-human-readable",
			Args: []string{"-1sh", humanSizeDir},
		},
		{
			Name: "human-readable-no-long",
			Args: []string{"-1h", basicDir},
		},
		{
			Name: "classify-flag",
			Args: []string{"-1F", classifyDir},
		},
		{
			Name: "classify-long",
			Args: []string{"-lF", classifyDir},
		},
		{
			Name: "classify-columns",
			Args: []string{"-CF", classifyDir},
		},
		{
			Name: "recursive-flag",
			Args: []string{"-1R", recursiveDir},
		},
		{
			Name: "recursive-long",
			Args: []string{"-lR", recursiveDir},
		},
		{
			Name: "recursive-all",
			Args: []string{"-1Ra", recursiveDir},
		},
		{
			Name: "recursive-columns",
			Args: []string{"-CR", recursiveDir},
		},
		{
			Name: "recursive-no-follow-symlink",
			Args: []string{"-1R", recursiveSymlinkDir},
		},
		{
			Name: "recursive-time-sort",
			Args: []string{"-1Rt", recursiveTimeSortDir},
		},
		{
			Name: "recursive-almost-all",
			Args: []string{"-1RA", recursiveDir},
		},
		{
			Name: "numeric-ids",
			Args: []string{"-n", basicDir},
		},
		{
			Name: "numeric-ids-with-1",
			Args: []string{"-n1", basicDir},
		},
		{
			Name: "format-lC",
			Args: []string{"-lC", basicDir},
		},
		{
			Name: "format-Cl",
			Args: []string{"-Cl", basicDir},
		},
		{
			Name: "format-l1",
			Args: []string{"-l1", basicDir},
		},
		{
			Name: "format-1l",
			Args: []string{"-1l", basicDir},
		},
		{
			Name: "format-x1",
			Args: []string{"-x1", basicDir},
		},
		{
			Name: "format-1x",
			Args: []string{"-1x", basicDir},
		},
		{
			Name: "format-Cx",
			Args: []string{"-Cx", basicDir},
		},
		{
			Name: "format-xC",
			Args: []string{"-xC", basicDir},
		},
		{
			Name: "format-xl",
			Args: []string{"-xl", basicDir},
		},
		{
			Name: "format-lx",
			Args: []string{"-lx", basicDir},
		},
		{
			Name: "format-nC",
			Args: []string{"-nC", basicDir},
		},
		{
			Name: "format-Cn",
			Args: []string{"-Cn", basicDir},
		},
		{
			Name: "recursive-numeric",
			Args: []string{"-nR", recursiveDir},
		},
		{
			Name: "recursive-inode",
			Args: []string{"-1iR", recursiveDir},
		},
		{
			Name: "recursive-blocks",
			Args: []string{"-1sR", recursiveDir},
		},
		{
			Name: "recursive-classify",
			Args: []string{"-1FR", recursiveDir},
		},
		{
			Name: "recursive-inode-blocks-classify",
			Args: []string{"-1isFR", recursiveDir},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func setupDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func setupTimeSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	for i, name := range []string{"oldest", "middle", "newest"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, nil, 0644); err != nil {
			t.Fatal(err)
		}
		mt := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func setupHumanSizeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []struct {
		name string
		size int
	}{
		{"big", 10240},
		{"medium", 2048},
		{"tiny", 10},
	} {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, make([]byte, f.size), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func setupSizeSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []struct {
		name string
		size int
	}{
		{"large", 300}, {"medium", 200}, {"small", 100},
	} {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, make([]byte, f.size), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func setupClassifyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "regular"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "executable"), nil, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("regular", filepath.Join(dir, "symlink")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func setupRecursiveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file2"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".hidden"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(sub, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file3"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func setupRecursiveSymlinkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "realdir")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "file2"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("realdir", filepath.Join(dir, "linkdir")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func setupRecursiveTimeSortDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	for i, name := range []string{"alpha", "beta"} {
		sub := filepath.Join(dir, name)
		if err := os.Mkdir(sub, 0755); err != nil {
			t.Fatal(err)
		}
		mt := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(sub, mt, mt); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "child"), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file1"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	ft := base.Add(2 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "file1"), ft, ft); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	t.Run("success-exits-0", func(t *testing.T) {
		dir := setupDir(t, "a", "b")
		cmd := exec.Command(goBin, dir)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit 0, got error: %v", err)
		}
	})

	t.Run("nonexistent-path-exits-1", func(t *testing.T) {
		cmd := exec.Command(goBin, "/no/such/path")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		out, err := cmd.CombinedOutput()
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected exit error, got: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit 1, got %d", exitErr.ExitCode())
		}
		if !strings.Contains(string(out), "cannot access") {
			t.Errorf("expected diagnostic on stderr, got: %s", out)
		}
	})

	t.Run("mixed-valid-invalid-exits-1", func(t *testing.T) {
		dir := setupDir(t, "a")
		cmd := exec.Command(goBin, "-1", dir, "/no/such/path")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected exit error, got: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit 1, got %d", exitErr.ExitCode())
		}
		if !strings.Contains(stdout.String(), "a") {
			t.Errorf("expected accessible entries in stdout, got: %s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "cannot access") {
			t.Errorf("expected diagnostic on stderr, got: %s", stderr.String())
		}
	})

	t.Run("invalid-option-exits-2", func(t *testing.T) {
		cmd := exec.Command(goBin, "--badopt")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected exit error, got: %v", err)
		}
		if exitErr.ExitCode() != 2 {
			t.Errorf("expected exit 2, got %d", exitErr.ExitCode())
		}
	})
}
