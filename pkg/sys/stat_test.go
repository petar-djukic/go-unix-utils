// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for Stat, Lstat, and FileInfo field population (prd002-sys R2.1, R2.2, R2.3).
package sys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStat_RegularFile verifies that Stat returns a fully populated FileInfo
// for a regular file with known properties (prd002-sys R2.1, R2.2).
func TestStat_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello, stat\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// Mode: must be a regular file with 0644 permissions.
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode.IsRegular() = false; expected true")
	}
	if perm := fi.Mode.Perm(); perm != 0644 {
		t.Errorf("Mode.Perm() = %04o; expected 0644", perm)
	}

	// Size: must equal the written content length.
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d; expected %d", fi.Size, len(content))
	}

	// Nlink: a freshly created regular file has exactly 1 hard link.
	if fi.Nlink != 1 {
		t.Errorf("Nlink = %d; expected 1", fi.Nlink)
	}

	// Uid/Gid: must match the current process (we created the file).
	if fi.Uid != uint32(os.Getuid()) {
		t.Errorf("Uid = %d; expected %d", fi.Uid, os.Getuid())
	}
	if fi.Gid != uint32(os.Getgid()) {
		t.Errorf("Gid = %d; expected %d", fi.Gid, os.Getgid())
	}

	// Dev: must be nonzero (every filesystem has a device number).
	if fi.Dev == 0 {
		t.Error("Dev = 0; expected nonzero device ID")
	}

	// Ino: must be nonzero.
	if fi.Ino == 0 {
		t.Error("Ino = 0; expected nonzero inode number")
	}

	// Blocks: must be nonnegative (a 12-byte file still occupies at least one block).
	if fi.Blocks < 0 {
		t.Errorf("Blocks = %d; expected nonnegative", fi.Blocks)
	}

	// Blksize: must be positive.
	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d; expected positive", fi.Blksize)
	}

	// ModTime: must be recent (within the last 60 seconds).
	if time.Since(fi.ModTime) > 60*time.Second {
		t.Errorf("ModTime = %v; expected within the last 60 seconds", fi.ModTime)
	}

	// Info: the underlying os.FileInfo must be populated.
	if fi.Info == nil {
		t.Error("Info is nil; expected non-nil os.FileInfo")
	}
	if fi.Info.Name() != "testfile" {
		t.Errorf("Info.Name() = %q; expected %q", fi.Info.Name(), "testfile")
	}
}

// TestStat_HardLink verifies that Nlink increments when a hard link is created
// (prd002-sys R2.2).
func TestStat_HardLink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "original")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}
	link := filepath.Join(dir, "hardlink")
	if err := os.Link(path, link); err != nil {
		t.Fatalf("creating hard link: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	if fi.Nlink != 2 {
		t.Errorf("Nlink = %d; expected 2 after creating one hard link", fi.Nlink)
	}

	// Both paths should share the same inode.
	fiLink, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}
	if fi.Ino != fiLink.Ino {
		t.Errorf("Ino mismatch: original=%d, hardlink=%d; expected same", fi.Ino, fiLink.Ino)
	}
	if fi.Dev != fiLink.Dev {
		t.Errorf("Dev mismatch: original=%d, hardlink=%d; expected same", fi.Dev, fiLink.Dev)
	}
}

// TestStat_NonexistentPath verifies that Stat returns a non-nil error for a
// path that does not exist (prd002-sys R2.1).
func TestStat_NonexistentPath(t *testing.T) {
	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Stat on nonexistent path returned nil error; expected non-nil")
	}
}

// TestLstat_Symlink verifies that Lstat returns metadata for the symlink itself,
// not the target (prd002-sys R2.1, AC3).
func TestLstat_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target content"), 0644); err != nil {
		t.Fatalf("creating target file: %v", err)
	}
	link := filepath.Join(dir, "symlink")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}

	// The mode type bits must indicate a symlink.
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Mode = %v; expected ModeSymlink bit set for symlink", fi.Mode)
	}
}

// TestLstat_RegularFile verifies that Lstat on a regular file (not a symlink)
// returns the same metadata as Stat (prd002-sys R2.1).
func TestLstat_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "regular")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	statFi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}
	lstatFi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	// For a regular file, Stat and Lstat should agree on all fields.
	if statFi.Ino != lstatFi.Ino {
		t.Errorf("Ino mismatch: Stat=%d, Lstat=%d", statFi.Ino, lstatFi.Ino)
	}
	if statFi.Mode != lstatFi.Mode {
		t.Errorf("Mode mismatch: Stat=%v, Lstat=%v", statFi.Mode, lstatFi.Mode)
	}
	if statFi.Size != lstatFi.Size {
		t.Errorf("Size mismatch: Stat=%d, Lstat=%d", statFi.Size, lstatFi.Size)
	}
}

// TestStat_FollowsSymlink verifies that Stat follows a symlink and returns
// metadata for the target file, not the symlink itself (prd002-sys R2.1).
func TestStat_FollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	content := []byte("target data for stat")
	if err := os.WriteFile(target, content, 0644); err != nil {
		t.Fatalf("creating target file: %v", err)
	}
	link := filepath.Join(dir, "symlink")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q) via symlink: %v", link, err)
	}

	// Stat follows the symlink; mode should be regular file, not symlink.
	if fi.Mode&os.ModeSymlink != 0 {
		t.Error("Stat on symlink returned ModeSymlink; expected regular file mode (should follow symlink)")
	}
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode.IsRegular() = false; expected true (Stat should follow symlink to target)")
	}
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d; expected %d (target file size)", fi.Size, len(content))
	}
}

// TestLstat_NonexistentPath verifies that Lstat returns a non-nil error for a
// path that does not exist (prd002-sys R2.1).
func TestLstat_NonexistentPath(t *testing.T) {
	_, err := Lstat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Lstat on nonexistent path returned nil error; expected non-nil")
	}
}

// TestStat_Directory verifies that Stat correctly identifies a directory
// (prd002-sys R2.2).
func TestStat_Directory(t *testing.T) {
	dir := t.TempDir()

	fi, err := Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}
	if !fi.Mode.IsDir() {
		t.Errorf("Mode.IsDir() = false for directory; expected true")
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0 for directory; expected nonzero")
	}
}
