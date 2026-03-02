// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for Lstat and FileInfo extended file metadata.
//
// Implements: prd002-sys R2.1, R2.2, R2.3
package sys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Lstat regular file (prd002-sys R2.1, R2.2) ---

func TestLstat_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "regular.txt")
	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	if fi.Mode.IsDir() {
		t.Error("Mode.IsDir() = true, want false for regular file")
	}
	if fi.Mode&os.ModeSymlink != 0 {
		t.Error("Mode has ModeSymlink set, want clear for regular file")
	}
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	if fi.Nlink == 0 {
		t.Error("Nlink = 0, want > 0 for regular file")
	}
	// Uid and Gid should match the current process on Linux.
	if fi.Uid != uint32(os.Getuid()) {
		t.Errorf("Uid = %d, want %d", fi.Uid, os.Getuid())
	}
	if fi.Gid != uint32(os.Getgid()) {
		t.Errorf("Gid = %d, want %d", fi.Gid, os.Getgid())
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want nonzero inode number")
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero, want a valid timestamp")
	}
	if time.Since(fi.ModTime) > 10*time.Second {
		t.Errorf("ModTime = %v, expected within last 10 seconds", fi.ModTime)
	}
	if fi.Info == nil {
		t.Error("Info (os.FileInfo) = nil, want non-nil")
	}
}

// --- Lstat directory (prd002-sys R2.1, R2.2) ---

func TestLstat_Directory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}

	fi, err := Lstat(subdir)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", subdir, err)
	}

	if !fi.Mode.IsDir() {
		t.Error("Mode.IsDir() = false, want true for directory")
	}
	if fi.Nlink == 0 {
		t.Error("Nlink = 0, want > 0 for directory")
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want nonzero inode number")
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero, want a valid timestamp")
	}
}

// --- Lstat symlink (prd002-sys R2.1, R2.2) ---

func TestLstat_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("target content"), 0o644); err != nil {
		t.Fatalf("writing target file: %v", err)
	}

	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}

	if fi.Mode&os.ModeSymlink == 0 {
		t.Error("Mode does not have ModeSymlink set, want set for symlink")
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want nonzero inode number for symlink")
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero, want a valid timestamp")
	}
}

// --- Lstat nonexistent path (prd002-sys R2.1) ---

func TestLstat_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.txt")

	fi, err := Lstat(path)
	if err == nil {
		t.Errorf("Lstat(%q): expected error for nonexistent path, got fi=%+v", path, fi)
	}
}

// --- Blocks and Blksize populated (prd002-sys R2.2, R2.3) ---

func TestLstat_BlocksAndBlksize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocks.txt")
	// Write enough content to ensure at least one block is allocated.
	content := make([]byte, 4096)
	for i := range content {
		content[i] = 'x'
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	if fi.Blocks <= 0 {
		t.Errorf("Blocks = %d, want > 0 for a file with %d bytes", fi.Blocks, len(content))
	}
	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d, want > 0", fi.Blksize)
	}
}
