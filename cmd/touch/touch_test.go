// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
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

func normalizeErrCase(b []byte) []byte {
	return bytes.ReplaceAll(b,
		[]byte("No such file or directory"),
		[]byte("no such file or directory"))
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
		{
			Name:     "missing reference file",
			Args:     []string{"-r", "nosuchref", "target"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
				normalizeErrCase,
			},
		},
		{
			Name: "date string ISO",
			Args: []string{"-d", "2024-01-15 10:30:00", "dfile"},
			ExpectedFiles: map[string][]byte{
				"dfile": {},
			},
		},
		{
			Name: "date string date only",
			Args: []string{"-d", "2024-01-15", "dfile2"},
			ExpectedFiles: map[string][]byte{
				"dfile2": {},
			},
		},
		{
			Name: "date string T separator",
			Args: []string{"-d", "2024-01-15T10:30:00", "dfile3"},
			ExpectedFiles: map[string][]byte{
				"dfile3": {},
			},
		},
		{
			Name:     "invalid date string",
			Args:     []string{"-d", "notadate", "file"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
			},
		},
		{
			Name:     "no-dereference short flag nonexistent",
			Args:     []string{"-h", "nosuchfile"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
				normalizeErrCase,
			},
		},
		{
			Name:     "no-dereference long flag nonexistent",
			Args:     []string{"--no-dereference", "nosuchfile"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
				normalizeErrCase,
			},
		},
		{
			Name:     "error continues processing remaining files",
			Args:     []string{"nodir/nofile", "goodfile"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
				normalizeErrCase,
			},
			ExpectedFiles: map[string][]byte{
				"goodfile": {},
			},
		},
	}

	refDir1 := t.TempDir()
	os.WriteFile(filepath.Join(refDir1, "existing"), []byte{}, 0644)
	refDir2 := t.TempDir()
	os.WriteFile(filepath.Join(refDir2, "existing"), []byte{}, 0644)

	tests = append(tests,
		testutils.DiffTest{
			Name:    "reference file short flag",
			Args:    []string{"-r", "existing", "rfile"},
			WorkDir: refDir1,
			ExpectedFiles: map[string][]byte{
				"rfile": {},
			},
		},
		testutils.DiffTest{
			Name:    "reference file long flag",
			Args:    []string{"--reference=existing", "rfile2"},
			WorkDir: refDir2,
			ExpectedFiles: map[string][]byte{
				"rfile2": {},
			},
		},
	)

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

func TestReferenceFile(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	ref := filepath.Join(dir, "ref")
	target := filepath.Join(dir, "target")

	os.WriteFile(ref, []byte{}, 0644)
	refTime := time.Date(2023, 6, 15, 14, 30, 0, 0, time.Local)
	os.Chtimes(ref, refTime, refTime)

	cmd := exec.Command(bin, "-r", ref, target)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -r failed: %v", err)
	}

	info, err := sys.Stat(target)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if !info.ModTime.Equal(refTime) {
		t.Errorf("mod time: got %v, want %v", info.ModTime, refTime)
	}
}

func TestMissingReferenceFile(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "target")

	cmd := exec.Command(bin, "-r", filepath.Join(dir, "nosuchref"), target)
	if err := cmd.Run(); err == nil {
		t.Fatal("expected error for missing reference file")
	}
}

func TestDateString(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	path := filepath.Join(dir, "dfile")

	cmd := exec.Command(bin, "-d", "2024-01-15 10:30:00", path)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -d failed: %v", err)
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

func TestNoDereferenceSymlink(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	os.WriteFile(target, []byte{}, 0644)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	os.Chtimes(target, past, past)
	os.Symlink(target, link)

	stamp := "2024-06-15 12:00:00"
	cmd := exec.Command(bin, "-h", "-d", stamp, link)
	if err := cmd.Run(); err != nil {
		t.Fatalf("touch -h -d failed: %v", err)
	}

	targetInfo, err := sys.Stat(target)
	if err != nil {
		t.Fatalf("stat target failed: %v", err)
	}
	if !targetInfo.ModTime.Equal(past) {
		t.Errorf("target mod time changed: got %v, want %v", targetInfo.ModTime, past)
	}

	linkInfo, err := sys.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link failed: %v", err)
	}
	want := time.Date(2024, 6, 15, 12, 0, 0, 0, time.Local)
	if !linkInfo.ModTime.Equal(want) {
		t.Errorf("link mod time: got %v, want %v", linkInfo.ModTime, want)
	}
}
