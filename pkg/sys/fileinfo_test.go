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
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// AC2: verify Mode, Size, ModTime are populated
	if fi.Mode.IsDir() {
		t.Errorf("expected regular file, got directory mode")
	}
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	if fi.Nlink == 0 {
		t.Error("Nlink is 0, expected at least 1")
	}
	if fi.Ino == 0 {
		t.Error("Ino is 0")
	}
	if fi.Info == nil {
		t.Error("Info is nil")
	}
}

func TestLstatRegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	if fi.Mode.IsDir() {
		t.Errorf("expected regular file, got directory mode")
	}
	if fi.Size != 4 {
		t.Errorf("Size = %d, want 4", fi.Size)
	}
}

func TestLstatSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target content"), 0o644); err != nil {
		t.Fatalf("writing target file: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// AC3: Lstat on a symlink returns symlink metadata
	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("expected ModeSymlink in mode %v", fi.Mode)
	}

	// Stat should follow the symlink and return regular file metadata
	fiStat, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}
	if fiStat.Mode&os.ModeSymlink != 0 {
		t.Errorf("Stat should follow symlink, but got ModeSymlink in mode %v", fiStat.Mode)
	}
}

func TestStatNonexistent(t *testing.T) {
	t.Parallel()
	// AC4: both Stat and Lstat return non-nil error for nonexistent paths
	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Stat on nonexistent path returned nil error")
	}
}

func TestLstatNonexistent(t *testing.T) {
	t.Parallel()
	_, err := Lstat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Lstat on nonexistent path returned nil error")
	}
}

// TestStatDuFields verifies that Stat populates Dev, Ino, and Blocks fields
// required by du for hard-link deduplication and physical block-count
// accumulation. (prd002-sys R2.5)
func TestStatDuFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "dufile")
	if err := os.WriteFile(path, []byte("block test data"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// R2.5: Dev must be non-zero (every file lives on a device).
	if fi.Dev == 0 {
		t.Error("Dev is 0, expected non-zero device ID")
	}
	// R2.5: Ino must be non-zero.
	if fi.Ino == 0 {
		t.Error("Ino is 0, expected non-zero inode number")
	}
	// R2.5: Blocks must be non-negative for a file with content.
	if fi.Blocks < 0 {
		t.Errorf("Blocks = %d, expected non-negative", fi.Blocks)
	}
	// A file with 15 bytes of content should have at least 1 block allocated.
	if fi.Blocks == 0 {
		t.Error("Blocks is 0 for a non-empty file, expected at least 1 block")
	}
}

// TestStatFindFields verifies that Stat populates Mode, Size, ModTime, Uid,
// Gid, and Nlink fields required by find predicates (-type, -size, -newer,
// -perm, -uid, -gid). (prd002-sys R2.6)
func TestStatFindFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "findfile")
	content := []byte("find test data")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// R2.6: Mode must reflect a regular file.
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode = %v, expected regular file", fi.Mode)
	}
	// R2.6: Size must match content length.
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	// R2.6: ModTime must be populated.
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	// R2.6: Uid must match the current user (file was just created).
	if fi.Uid != uint32(os.Getuid()) {
		t.Errorf("Uid = %d, want %d", fi.Uid, os.Getuid())
	}
	// R2.6: Gid must match the current user's group.
	if fi.Gid != uint32(os.Getgid()) {
		t.Errorf("Gid = %d, want %d", fi.Gid, os.Getgid())
	}
	// R2.6: Nlink must be at least 1.
	if fi.Nlink == 0 {
		t.Error("Nlink is 0, expected at least 1")
	}
}
