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

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?touch\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("touch"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtouch")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name: "create new file",
			Args: []string{"newfile"},
			ExpectedFiles: map[string][]byte{
				"newfile": {},
			},
		},
		{
			Name: "update existing file",
			Args: []string{"existing"},
		},
		{
			Name: "no-create short flag",
			Args: []string{"-c", "nonexistent"},
		},
		{
			Name: "no-create long flag",
			Args: []string{"--no-create", "nonexistent"},
		},
		{
			Name: "multiple files",
			Args: []string{"fileA", "fileB", "fileC"},
			ExpectedFiles: map[string][]byte{
				"fileA": {},
				"fileB": {},
				"fileC": {},
			},
		},
		{
			Name:     "missing operand",
			Args:     []string{},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
			},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestCreateNewFile(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile")

	cmd := exec.Command(bin, path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestNoCreateSkipsCreation(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "absent")

	cmd := exec.Command(bin, "-c", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -c failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to not exist, got: %v", err)
	}
}

func TestMultipleFiles(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()

	paths := []string{
		filepath.Join(dir, "a"),
		filepath.Join(dir, "b"),
		filepath.Join(dir, "c"),
	}

	cmd := exec.Command(bin, paths...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch failed: %v", err)
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("file %s not created: %v", p, err)
		}
	}
}
