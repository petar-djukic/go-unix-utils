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

var progRe = regexp.MustCompile(`(?m)^(\S*/)?g?stat:`)
var tryRe = regexp.MustCompile(`'(\S*/)?g?stat --help'`)

func normProg(b []byte) []byte {
	b = progRe.ReplaceAll(b, []byte("PROG:"))
	b = tryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

var fsNumRe = regexp.MustCompile(`(Total|Free|Available): \d+`)

func normBlocks(b []byte) []byte {
	return fsNumRe.ReplaceAll(b, []byte("$1: NNN"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstat")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(tmpFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(tmpDir, "empty")
	if err := os.WriteFile(emptyFile, nil, 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(tmpFile, link); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	normErr := []testutils.NormalizeFunc{normProg}
	normFS := []testutils.NormalizeFunc{normBlocks}

	tests := []testutils.DiffTest{
		{
			Name: "regular-file",
			Args: []string{tmpFile},
		},
		{
			Name: "empty-file",
			Args: []string{emptyFile},
		},
		{
			Name: "directory",
			Args: []string{subDir},
		},
		{
			Name: "symlink",
			Args: []string{link},
		},
		{
			Name: "symlink-deref",
			Args: []string{"-L", link},
		},
		{
			Name: "multiple-files",
			Args: []string{tmpFile, emptyFile},
		},
		{
			Name: "format-name-size",
			Args: []string{"-c", "%n %s", tmpFile},
		},
		{
			Name: "format-perms",
			Args: []string{"-c", "%a %A", tmpFile},
		},
		{
			Name: "format-ids",
			Args: []string{"-c", "%u %U %g %G", tmpFile},
		},
		{
			Name: "format-device-inode",
			Args: []string{"-c", "%d %D %i %h", tmpFile},
		},
		{
			Name: "format-blocks",
			Args: []string{"-c", "%b %B %o", tmpFile},
		},
		{
			Name: "format-type",
			Args: []string{"-c", "%F", tmpFile},
		},
		{
			Name: "format-type-dir",
			Args: []string{"-c", "%F", subDir},
		},
		{
			Name: "format-type-link",
			Args: []string{"-c", "%F", link},
		},
		{
			Name: "format-times",
			Args: []string{"-c", "%x %X %y %Y %z %Z", tmpFile},
		},
		{
			Name: "format-birth",
			Args: []string{"-c", "%w %W", tmpFile},
		},
		{
			Name: "format-raw-mode",
			Args: []string{"-c", "%f", tmpFile},
		},
		{
			Name: "format-quoted-name",
			Args: []string{"-c", "%N", tmpFile},
		},
		{
			Name: "format-quoted-link",
			Args: []string{"-c", "%N", link},
		},
		{
			Name: "format-mount",
			Args: []string{"-c", "%m", tmpFile},
		},
		{
			Name: "printf-basic",
			Args: []string{"--printf=%n\\t%s\\n", tmpFile},
		},
		{
			Name: "printf-no-newline",
			Args: []string{"--printf=%n", tmpFile},
		},
		{
			Name: "format-percent",
			Args: []string{"-c", "%%", tmpFile},
		},
		{
			Name: "terse",
			Args: []string{"-t", tmpFile},
		},
		{
			Name: "terse-link",
			Args: []string{"-t", link},
		},
		{
			Name: "fs-default",
			Args:      []string{"-f", tmpDir},
			Normalize: normFS,
		},
		{
			Name: "fs-format",
			Args:      []string{"-f", "-c", "%T %a %b %c %d %f", tmpDir},
			Normalize: normFS,
		},
		{
			Name: "fs-terse",
			Args:      []string{"-f", "-t", tmpDir},
			Normalize: normFS,
		},
		{
			Name:      "nonexistent",
			Args:      []string{filepath.Join(tmpDir, "nonexistent")},
			ExitCode:  1,
			Normalize: normErr,
		},
		{
			Name:      "mixed-error",
			Args:      []string{tmpFile, filepath.Join(tmpDir, "nonexistent"), emptyFile},
			ExitCode:  1,
			Normalize: normErr,
		},
		{
			Name:      "invalid-flag",
			Args:      []string{"-Z"},
			ExitCode:  1,
			Normalize: normErr,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
