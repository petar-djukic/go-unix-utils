// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatRegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	if fi.Nlink < 1 {
		t.Errorf("Nlink = %d, want >= 1", fi.Nlink)
	}
	if fi.Uid != uint32(os.Getuid()) {
		t.Errorf("Uid = %d, want %d", fi.Uid, os.Getuid())
	}
	if fi.Gid != uint32(os.Getgid()) {
		t.Errorf("Gid = %d, want %d", fi.Gid, os.Getgid())
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want non-zero")
	}
	if fi.Dev == 0 {
		t.Error("Dev = 0, want non-zero")
	}
	if fi.Mode.IsDir() {
		t.Error("Mode.IsDir() = true for regular file")
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	if fi.Info == nil {
		t.Error("Info is nil")
	}
}

func TestStatDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	fi, err := Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if !fi.Mode.IsDir() {
		t.Error("Mode.IsDir() = false for directory")
	}
	if fi.Nlink < 2 {
		t.Errorf("Nlink = %d, want >= 2 for directory", fi.Nlink)
	}
}

func TestLstatSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// AC3: Lstat on symlink returns symlink's own mode bits.
	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fi.Mode&os.ModeSymlink == 0 {
		t.Error("Lstat on symlink: ModeSymlink not set")
	}

	// Stat follows symlink, so should return regular file mode.
	fi2, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi2.Mode&os.ModeSymlink != 0 {
		t.Error("Stat on symlink: ModeSymlink should not be set")
	}
}

func TestStatNonexistent(t *testing.T) {
	t.Parallel()
	_, err := Stat("/nonexistent-path-that-does-not-exist")
	if err == nil {
		t.Error("Stat on nonexistent path: expected error, got nil")
	}
}

func TestLstatNonexistent(t *testing.T) {
	t.Parallel()
	_, err := Lstat("/nonexistent-path-that-does-not-exist")
	if err == nil {
		t.Error("Lstat on nonexistent path: expected error, got nil")
	}
}

func TestStatAccessAndChangeTime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "timefile")
	if err := os.WriteFile(path, []byte("t"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.AccessTime.IsZero() {
		t.Error("AccessTime is zero")
	}
	if fi.ChangeTime.IsZero() {
		t.Error("ChangeTime is zero")
	}
}

func TestStatBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "blockfile")
	if err := os.WriteFile(path, []byte("some content for blocks"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.Blocks < 0 {
		t.Errorf("Blocks = %d, want >= 0", fi.Blocks)
	}
	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d, want > 0", fi.Blksize)
	}
}
