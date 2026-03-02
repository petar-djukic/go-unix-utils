// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for stat.go: Stat, Lstat, and FileMetadata extraction.
// Implements: prd002-sys (R2)
package sys

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("populates FileMetadata fields from temp file", func(t *testing.T) {
		meta, err := Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q): %v", path, err)
		}

		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat(%q): %v", path, err)
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("fi.Sys() type = %T, want *syscall.Stat_t", fi.Sys())
		}

		// ModTime must match os.Stat (prd002-sys R2.1)
		if !meta.ModTime.Equal(fi.ModTime()) {
			t.Errorf("ModTime = %v, want %v", meta.ModTime, fi.ModTime())
		}

		// Uid and Gid must match syscall.Stat_t (prd002-sys R2.2)
		if meta.Uid != st.Uid {
			t.Errorf("Uid = %d, want %d", meta.Uid, st.Uid)
		}
		if meta.Gid != st.Gid {
			t.Errorf("Gid = %d, want %d", meta.Gid, st.Gid)
		}

		// Blocks must match syscall.Stat_t (prd002-sys R2.2)
		if meta.Blocks != st.Blocks {
			t.Errorf("Blocks = %d, want %d", meta.Blocks, st.Blocks)
		}

		// Ino must be non-zero (prd002-sys R2.2)
		if meta.Ino == 0 {
			t.Error("Ino = 0, want non-zero")
		}

		// Dev must be non-zero (prd002-sys R2.2)
		if meta.Dev == 0 {
			t.Error("Dev = 0, want non-zero")
		}

		// Nlink for a newly created file should be 1 (prd002-sys R2.2)
		if meta.Nlink != 1 {
			t.Errorf("Nlink = %d, want 1", meta.Nlink)
		}

		// Blksize must be positive (prd002-sys R2.2)
		if meta.Blksize <= 0 {
			t.Errorf("Blksize = %d, want > 0", meta.Blksize)
		}
	})

	t.Run("returns error for nonexistent path", func(t *testing.T) {
		_, err := Stat(filepath.Join(dir, "nonexistent"))
		if err == nil {
			t.Fatal("Stat(nonexistent) returned nil error, want non-nil")
		}
	})
}

func TestLstat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("lstat test content")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Run("populates FileMetadata fields from temp file", func(t *testing.T) {
		meta, err := Lstat(path)
		if err != nil {
			t.Fatalf("Lstat(%q): %v", path, err)
		}

		fi, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("os.Lstat(%q): %v", path, err)
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("fi.Sys() type = %T, want *syscall.Stat_t", fi.Sys())
		}

		// ModTime must match os.Lstat (prd002-sys R2.1)
		if !meta.ModTime.Equal(fi.ModTime()) {
			t.Errorf("ModTime = %v, want %v", meta.ModTime, fi.ModTime())
		}

		// Uid and Gid must match syscall.Stat_t (prd002-sys R2.2)
		if meta.Uid != st.Uid {
			t.Errorf("Uid = %d, want %d", meta.Uid, st.Uid)
		}
		if meta.Gid != st.Gid {
			t.Errorf("Gid = %d, want %d", meta.Gid, st.Gid)
		}

		// Blocks must match syscall.Stat_t (prd002-sys R2.2)
		if meta.Blocks != st.Blocks {
			t.Errorf("Blocks = %d, want %d", meta.Blocks, st.Blocks)
		}

		// Ino must be non-zero (prd002-sys R2.2)
		if meta.Ino == 0 {
			t.Error("Ino = 0, want non-zero")
		}

		// Dev must be non-zero (prd002-sys R2.2)
		if meta.Dev == 0 {
			t.Error("Dev = 0, want non-zero")
		}

		// Nlink for a regular file should be >= 1 (prd002-sys R2.2)
		if meta.Nlink < 1 {
			t.Errorf("Nlink = %d, want >= 1", meta.Nlink)
		}

		// Blksize must be positive (prd002-sys R2.2)
		if meta.Blksize <= 0 {
			t.Errorf("Blksize = %d, want > 0", meta.Blksize)
		}
	})

	t.Run("does not follow symlinks", func(t *testing.T) {
		target := filepath.Join(dir, "symlink_target")
		if err := os.WriteFile(target, []byte("target content"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		link := filepath.Join(dir, "symlink")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("Symlink: %v", err)
		}

		// Stat follows the symlink — returns the target's inode.
		targetMeta, err := Stat(link)
		if err != nil {
			t.Fatalf("Stat(%q): %v", link, err)
		}

		// Lstat does NOT follow the symlink — returns the symlink's own inode.
		linkMeta, err := Lstat(link)
		if err != nil {
			t.Fatalf("Lstat(%q): %v", link, err)
		}

		// The symlink's inode must differ from the target's inode (prd002-sys R2.1).
		if linkMeta.Ino == targetMeta.Ino {
			t.Errorf("Lstat returned target inode %d; want symlink's own inode (different from target)", targetMeta.Ino)
		}

		// Cross-check: os.Lstat should also report the symlink inode.
		osLstat, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("os.Lstat(%q): %v", link, err)
		}
		if osLstat.Mode()&os.ModeSymlink == 0 {
			t.Fatal("os.Lstat did not report symlink mode bit")
		}

		// Verify Lstat inode matches os.Lstat's underlying inode.
		st, ok := osLstat.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("os.Lstat Sys() type = %T, want *syscall.Stat_t", osLstat.Sys())
		}
		if linkMeta.Ino == 0 {
			t.Error("Lstat symlink Ino = 0, want non-zero")
		}
		if linkMeta.Uid != st.Uid {
			t.Errorf("Lstat symlink Uid = %d, want %d", linkMeta.Uid, st.Uid)
		}
	})

	t.Run("returns error for nonexistent path", func(t *testing.T) {
		_, err := Lstat(filepath.Join(dir, "nonexistent"))
		if err == nil {
			t.Fatal("Lstat(nonexistent) returned nil error, want non-nil")
		}
	})
}
