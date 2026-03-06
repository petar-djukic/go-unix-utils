// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsha1sum")
	if err != nil {
		t.Skipf("reference binary gsha1sum not in PATH: %v", err)
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "hello.txt"), "hello\n")
	writeFile(t, filepath.Join(dir, "checksums.sha1"),
		"f572d396fae9206628714fb2ce00f72e94f2258f  hello.txt\n")
	writeFile(t, filepath.Join(dir, "bad.sha1"),
		"0000000000000000000000000000000000000000  hello.txt\n")

	tests := []testutils.DiffTest{
		{Name: "compute_stdin", Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		{Name: "compute_file", Args: []string{"hello.txt"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
		{Name: "binary_mode", Args: []string{"-b", "hello.txt"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
		{Name: "tag_format", Args: []string{"--tag", "hello.txt"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
		{Name: "check_valid", Args: []string{"--check", "checksums.sha1"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
		{Name: "check_failure", Args: []string{"--check", "bad.sha1"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
		{Name: "missing_file", Args: []string{"nonexistent.txt"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
		{Name: "empty_stdin", Stdin: []byte{}, Env: []string{"LC_ALL=C"}},
		{Name: "multiple_files", Args: []string{"hello.txt", "hello.txt"}, WorkDir: dir, Env: []string{"LC_ALL=C"}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}
