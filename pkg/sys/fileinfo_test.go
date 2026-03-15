// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatRegularFile verifies that Stat returns correct metadata for a
// regular file.
// AC1, AC2, AC4.
func TestStatRegularFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	if fi.Mode.IsDir() {
		t.Errorf("expected regular file, got directory")
	}
	if fi.Size != 5 {
		t.Errorf("Size = %d, want 5", fi.Size)
	}
	if fi.Nlink < 1 {
		t.Errorf("Nlink = %d, want >= 1", fi.Nlink)
	}
	if fi.Uid == 0 && os.Getuid() != 0 {
		t.Errorf("Uid = 0 but process is not root")
	}
	if fi.Ino == 0 {
		t.Errorf("Ino = 0, expected non-zero inode")
	}
	if fi.Dev == 0 {
		t.Errorf("Dev = 0, expected non-zero device")
	}
	if fi.Info == nil {
		t.Errorf("Info is nil, expected os.FileInfo")
	}

	// AC4: time fields must be non-zero for existing files.
	if fi.ModTime.IsZero() {
		t.Errorf("ModTime is zero")
	}
	if fi.AccessTime.IsZero() {
		t.Errorf("AccessTime is zero")
	}
	if fi.ChangeTime.IsZero() {
		t.Errorf("ChangeTime is zero")
	}

	// Times should be recent (within last minute).
	now := time.Now()
	if now.Sub(fi.ModTime) > time.Minute {
		t.Errorf("ModTime %v is not recent", fi.ModTime)
	}
}

// TestStatDirectory verifies that Stat returns correct metadata for a
// directory.
// AC2.
func TestStatDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	fi, err := Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}

	if !fi.Mode.IsDir() {
		t.Errorf("expected directory, got mode %v", fi.Mode)
	}
	if fi.Nlink < 2 {
		t.Errorf("Nlink = %d, want >= 2 for a directory", fi.Nlink)
	}
}

// TestStatFollowsSymlink verifies that Stat follows symbolic links and
// returns the target's metadata.
// AC2.
func TestStatFollowsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("creating target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}

	// Stat follows the symlink, so mode should be a regular file.
	if !fi.Mode.IsRegular() {
		t.Errorf("Stat on symlink: expected regular file mode, got %v", fi.Mode)
	}
	if fi.Size != 7 {
		t.Errorf("Size = %d, want 7", fi.Size)
	}
}

// TestLstatSymlink verifies that Lstat returns the symlink's own metadata
// without following it.
// AC3.
func TestLstatSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("content"), 0o644); err != nil {
		t.Fatalf("creating target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}

	// AC3: Lstat must report symlink type, not target type.
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Lstat on symlink: expected ModeSymlink bit set, got %v", fi.Mode)
	}
}

// TestLstatRegularFile verifies that Lstat works on a regular file (no
// symlink involved).
func TestLstatRegularFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	if !fi.Mode.IsRegular() {
		t.Errorf("expected regular file, got mode %v", fi.Mode)
	}
	if fi.Size != 4 {
		t.Errorf("Size = %d, want 4", fi.Size)
	}
}

// TestStatNonExistent verifies that Stat returns an error for a non-existent
// path.
// AC5.
func TestStatNonExistent(t *testing.T) {
	t.Parallel()

	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatalf("Stat on non-existent path: expected error, got nil")
	}
}

// TestLstatNonExistent verifies that Lstat returns an error for a
// non-existent path.
// AC5.
func TestLstatNonExistent(t *testing.T) {
	t.Parallel()

	_, err := Lstat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatalf("Lstat on non-existent path: expected error, got nil")
	}
}

// TestStatBlocksNonNegative verifies that Blocks is non-negative for a file
// with content.
func TestStatBlocksNonNegative(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "blocks")
	if err := os.WriteFile(path, []byte("some content for blocks"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	if fi.Blocks < 0 {
		t.Errorf("Blocks = %d, expected non-negative", fi.Blocks)
	}
	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d, expected positive", fi.Blksize)
	}
}
