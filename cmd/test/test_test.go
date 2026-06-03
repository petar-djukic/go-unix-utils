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

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtest")
	if err != nil {
		t.Skip("reference binary not found")
	}

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?test`)
	normName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("test"))
	})

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "exists")
	if err := os.WriteFile(tmpFile, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpEmpty := filepath.Join(tmpDir, "empty")
	if err := os.WriteFile(tmpEmpty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tmpExec := filepath.Join(tmpDir, "runme")
	if err := os.WriteFile(tmpExec, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tmpLink := filepath.Join(tmpDir, "link")
	if err := os.Symlink(tmpFile, tmpLink); err != nil {
		t.Fatal(err)
	}
	tmpSub := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(tmpSub, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		// R1.1: file type tests
		{Name: "f-regular", Args: []string{"-f", tmpFile}},
		{Name: "f-dir", Args: []string{"-f", tmpSub}, ExitCode: 1},
		{Name: "d-dir", Args: []string{"-d", tmpSub}},
		{Name: "d-file", Args: []string{"-d", tmpFile}, ExitCode: 1},
		{Name: "e-exists", Args: []string{"-e", tmpFile}},
		{Name: "e-missing", Args: []string{"-e", "/no/such/path"}, ExitCode: 1},
		{Name: "s-nonempty", Args: []string{"-s", tmpFile}},
		{Name: "s-empty", Args: []string{"-s", tmpEmpty}, ExitCode: 1},
		{Name: "r-readable", Args: []string{"-r", tmpFile}},
		{Name: "w-writable", Args: []string{"-w", tmpFile}},
		{Name: "x-executable", Args: []string{"-x", tmpExec}},
		{Name: "x-noexec", Args: []string{"-x", tmpFile}, ExitCode: 1},
		{Name: "L-symlink", Args: []string{"-L", tmpLink}},
		{Name: "h-symlink", Args: []string{"-h", tmpLink}},
		{Name: "L-regular", Args: []string{"-L", tmpFile}, ExitCode: 1},
		{Name: "c-devnull", Args: []string{"-c", "/dev/null"}},
		{Name: "c-regular", Args: []string{"-c", tmpFile}, ExitCode: 1},
		{Name: "p-regular", Args: []string{"-p", tmpFile}, ExitCode: 1},
		{Name: "S-regular", Args: []string{"-S", tmpFile}, ExitCode: 1},
		{Name: "G-owned", Args: []string{"-G", tmpFile}},
		{Name: "O-owned", Args: []string{"-O", tmpFile}},

		// R1.2: file comparisons
		{Name: "ef-same", Args: []string{tmpFile, "-ef", tmpFile}},
		{Name: "ef-diff", Args: []string{tmpFile, "-ef", tmpEmpty}, ExitCode: 1},
		{Name: "ef-link", Args: []string{tmpFile, "-ef", tmpLink}},
		{Name: "nt-missing-right", Args: []string{tmpFile, "-nt", "/no/such"}},
		{Name: "nt-missing-left", Args: []string{"/no/such", "-nt", tmpFile}, ExitCode: 1},
		{Name: "ot-missing-left", Args: []string{"/no/such", "-ot", tmpFile}},
		{Name: "ot-missing-right", Args: []string{tmpFile, "-ot", "/no/such"}, ExitCode: 1},

		// R2.1: string tests
		{Name: "z-empty", Args: []string{"-z", ""}},
		{Name: "z-nonempty", Args: []string{"-z", "hello"}, ExitCode: 1},
		{Name: "n-nonempty", Args: []string{"-n", "hello"}},
		{Name: "n-empty", Args: []string{"-n", ""}, ExitCode: 1},
		{Name: "str-equal", Args: []string{"abc", "=", "abc"}},
		{Name: "str-notequal", Args: []string{"abc", "=", "xyz"}, ExitCode: 1},
		{Name: "str-ne", Args: []string{"abc", "!=", "xyz"}},
		{Name: "str-ne-same", Args: []string{"abc", "!=", "abc"}, ExitCode: 1},
		{Name: "str-nonempty", Args: []string{"hello"}},
		{Name: "str-empty-false", Args: []string{""}, ExitCode: 1},

		// R2.2: integer comparisons
		{Name: "eq-true", Args: []string{"1", "-eq", "1"}},
		{Name: "eq-false", Args: []string{"1", "-eq", "2"}, ExitCode: 1},
		{Name: "ne-true", Args: []string{"1", "-ne", "2"}},
		{Name: "ne-false", Args: []string{"1", "-ne", "1"}, ExitCode: 1},
		{Name: "lt-true", Args: []string{"1", "-lt", "2"}},
		{Name: "lt-false", Args: []string{"2", "-lt", "1"}, ExitCode: 1},
		{Name: "le-true", Args: []string{"1", "-le", "1"}},
		{Name: "le-false", Args: []string{"2", "-le", "1"}, ExitCode: 1},
		{Name: "gt-true", Args: []string{"2", "-gt", "1"}},
		{Name: "gt-false", Args: []string{"1", "-gt", "2"}, ExitCode: 1},
		{Name: "ge-true", Args: []string{"2", "-ge", "2"}},
		{Name: "ge-false", Args: []string{"1", "-ge", "2"}, ExitCode: 1},

		// Edge cases
		{Name: "no-args", ExitCode: 1},
		{
			Name:      "invalid-int",
			Args:      []string{"abc", "-eq", "1"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normName},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
