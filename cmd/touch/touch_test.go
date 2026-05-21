// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
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
		{
			Name: "access only flag",
			Args: []string{"-a", "afile"},
			ExpectedFiles: map[string][]byte{
				"afile": {},
			},
		},
		{
			Name: "modification only flag",
			Args: []string{"-m", "mfile"},
			ExpectedFiles: map[string][]byte{
				"mfile": {},
			},
		},
		{
			Name: "explicit timestamp",
			Args: []string{"-t", "202401151030.00", "tfile"},
			ExpectedFiles: map[string][]byte{
				"tfile": {},
			},
		},
		{
			Name: "access only with timestamp",
			Args: []string{"-a", "-t", "202401151030.00", "atfile"},
			ExpectedFiles: map[string][]byte{
				"atfile": {},
			},
		},
		{
			Name: "modification only with timestamp",
			Args: []string{"-m", "-t", "202401151030.00", "mtfile"},
			ExpectedFiles: map[string][]byte{
				"mtfile": {},
			},
		},
		{
			Name: "both access and modification flags",
			Args: []string{"-a", "-m", "amfile"},
			ExpectedFiles: map[string][]byte{
				"amfile": {},
			},
		},
		{
			Name: "combined short flags",
			Args: []string{"-am", "amfile2"},
			ExpectedFiles: map[string][]byte{
				"amfile2": {},
			},
		},
		{
			Name:     "invalid timestamp",
			Args:     []string{"-t", "invalid", "file"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
			},
		},
		{
			Name: "timestamp without seconds",
			Args: []string{"-t", "202401151030", "tsfile"},
			ExpectedFiles: map[string][]byte{
				"tsfile": {},
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

func TestAccessOnlyPreservesModTime(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	os.WriteFile(path, []byte{}, 0644)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	os.Chtimes(path, past, past)

	cmd := exec.Command(bin, "-a", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -a failed: %v", err)
	}

	info, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if time.Since(info.AccessTime) > 5*time.Second {
		t.Errorf("access time not updated: %v", info.AccessTime)
	}
	if !info.ModTime.Equal(past) {
		t.Errorf("modification time changed: got %v, want %v", info.ModTime, past)
	}
}

func TestModOnlyPreservesAccessTime(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	os.WriteFile(path, []byte{}, 0644)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	os.Chtimes(path, past, past)

	cmd := exec.Command(bin, "-m", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -m failed: %v", err)
	}

	info, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.AccessTime.Equal(past) {
		t.Errorf("access time changed: got %v, want %v", info.AccessTime, past)
	}
	if time.Since(info.ModTime) > 5*time.Second {
		t.Errorf("modification time not updated: %v", info.ModTime)
	}
}

func TestExplicitTimestamp(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	cmd := exec.Command(bin, "-t", "202401151030.00", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -t failed: %v", err)
	}

	info, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)
	if !info.AccessTime.Equal(want) {
		t.Errorf("access time: got %v, want %v", info.AccessTime, want)
	}
	if !info.ModTime.Equal(want) {
		t.Errorf("modification time: got %v, want %v", info.ModTime, want)
	}
}

func TestExplicitTimestampAccessOnly(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	os.WriteFile(path, []byte{}, 0644)
	past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)
	os.Chtimes(path, past, past)

	cmd := exec.Command(bin, "-a", "-t", "202401151030.00", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -a -t failed: %v", err)
	}

	info, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	wantAccess := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)
	if !info.AccessTime.Equal(wantAccess) {
		t.Errorf("access time: got %v, want %v", info.AccessTime, wantAccess)
	}
	if !info.ModTime.Equal(past) {
		t.Errorf("modification time changed: got %v, want %v", info.ModTime, past)
	}
}

func TestExplicitTimestampModOnly(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	os.WriteFile(path, []byte{}, 0644)
	past := time.Date(2020, 6, 15, 12, 0, 0, 0, time.Local)
	os.Chtimes(path, past, past)

	cmd := exec.Command(bin, "-m", "-t", "202401151030.00", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -m -t failed: %v", err)
	}

	info, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	wantMod := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)
	if !info.AccessTime.Equal(past) {
		t.Errorf("access time changed: got %v, want %v", info.AccessTime, past)
	}
	if !info.ModTime.Equal(wantMod) {
		t.Errorf("modification time: got %v, want %v", info.ModTime, wantMod)
	}
}
