// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd008-ls R1.1-R1.8, R2.3-R2.6.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
