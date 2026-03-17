// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatRegularFile verifies Stat returns correct metadata for a regular file.
func TestStatRegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// R2.2: verify basic fields.
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	if fi.Nlink < 1 {
		t.Errorf("Nlink = %d, want >= 1", fi.Nlink)
	}
	if fi.Uid == 0 && os.Getuid() != 0 {
		t.Errorf("Uid = 0 but running as non-root user %d", os.Getuid())
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want non-zero")
	}
	if fi.Dev == 0 {
		t.Error("Dev = 0, want non-zero")
	}
	if fi.Blocks < 0 {
		t.Errorf("Blocks = %d, want >= 0", fi.Blocks)
	}
	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d, want > 0", fi.Blksize)
	}
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode = %v, want regular file", fi.Mode)
	}
	if fi.Info == nil {
		t.Error("Info is nil, want non-nil os.FileInfo")
	}

	// R2.3: verify timestamps are reasonable (within the last minute).
	now := time.Now()
	if fi.ModTime.Before(now.Add(-time.Minute)) || fi.ModTime.After(now.Add(time.Minute)) {
		t.Errorf("ModTime = %v, want close to now (%v)", fi.ModTime, now)
	}
	if fi.AccessTime.Before(now.Add(-time.Minute)) || fi.AccessTime.After(now.Add(time.Minute)) {
		t.Errorf("AccessTime = %v, want close to now (%v)", fi.AccessTime, now)
	}
	if fi.ChangeTime.Before(now.Add(-time.Minute)) || fi.ChangeTime.After(now.Add(time.Minute)) {
		t.Errorf("ChangeTime = %v, want close to now (%v)", fi.ChangeTime, now)
	}
}

// TestStatDirectory verifies Stat returns correct metadata for a directory.
func TestStatDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	fi, err := Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}

	if !fi.Mode.IsDir() {
		t.Errorf("Mode = %v, want directory", fi.Mode)
	}
	if fi.Nlink < 2 {
		t.Errorf("Nlink = %d, want >= 2 for directory", fi.Nlink)
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want non-zero")
	}
}

// TestStatFollowsSymlinks verifies Stat follows symlinks to the target.
func TestStatFollowsSymlinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	content := []byte("target content\n")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}

	// Stat follows symlinks: should see regular file, not symlink.
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode = %v, want regular file (Stat should follow symlink)", fi.Mode)
	}
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d (target content size)", fi.Size, len(content))
	}
}

// TestLstatPreservesSymlink verifies Lstat does not follow symlinks.
// AC3: Lstat on a symlink returns the symlink's own mode bits.
func TestLstatPreservesSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}

	// Lstat should report symlink type, not regular file.
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Mode = %v, want ModeSymlink set (Lstat should not follow symlink)", fi.Mode)
	}
}

// TestLstatRegularFile verifies Lstat works on regular files (non-symlinks).
func TestLstatRegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "regular")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	fi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	if !fi.Mode.IsRegular() {
		t.Errorf("Mode = %v, want regular file", fi.Mode)
	}
}

// TestStatNonexistent verifies Stat returns an error for a nonexistent path.
func TestStatNonexistent(t *testing.T) {
	t.Parallel()

	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Stat on nonexistent path: got nil error, want error")
	}
}

// TestLstatNonexistent verifies Lstat returns an error for a nonexistent path.
func TestLstatNonexistent(t *testing.T) {
	t.Parallel()

	_, err := Lstat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Lstat on nonexistent path: got nil error, want error")
	}
}

// TestStatHardLinkNlink verifies Nlink increments when a hard link is created.
func TestStatHardLinkNlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "original")
	link := filepath.Join(dir, "hardlink")

	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	fi1, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	if fi1.Nlink != 1 {
		t.Errorf("Nlink before link = %d, want 1", fi1.Nlink)
	}

	if err := os.Link(path, link); err != nil {
		t.Fatalf("creating hard link: %v", err)
	}

	fi2, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) after link: %v", path, err)
	}
	if fi2.Nlink != 2 {
		t.Errorf("Nlink after link = %d, want 2", fi2.Nlink)
	}

	// Both paths should share the same inode.
	fiLink, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}
	if fi2.Ino != fiLink.Ino {
		t.Errorf("Ino mismatch: original=%d, link=%d", fi2.Ino, fiLink.Ino)
	}
	if fi2.Dev != fiLink.Dev {
		t.Errorf("Dev mismatch: original=%d, link=%d", fi2.Dev, fiLink.Dev)
	}
}
