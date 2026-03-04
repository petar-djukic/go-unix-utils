// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build darwin || linux

// Tests for prd002-sys R2.1-R2.3: Stat, Lstat, and FileMetadata accessors.

package sys_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// TestStat_RegularFile verifies that Stat returns correct FileMetadata for a
// regular file: ModTime within the last 60 seconds and Blocks >= 0.
// R2.1: stat with symlink resolution on a regular file.
// R2.2: ModTime and Blocks accessors.
func TestStat_RegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	meta, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// AC4: ModTime within the last 60 seconds.
	if time.Since(meta.ModTime()) > 60*time.Second {
		t.Errorf("ModTime() = %v, want within last 60 seconds", meta.ModTime())
	}

	// AC5: Blocks >= 0 for a non-empty regular file.
	if meta.Blocks() < 0 {
		t.Errorf("Blocks() = %d, want >= 0", meta.Blocks())
	}
}

// TestStat_FollowsSymlink verifies that Stat on a symlink returns metadata of
// the target file, not the symlink itself. The target's mtime is set to 1 hour
// ago so it clearly differs from the symlink creation time.
// R2.1: stat follows symbolic links.
func TestStat_FollowsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set target mtime to 1 hour ago so it differs from symlink creation time.
	past := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(target, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	targetMeta, err := sys.Stat(target)
	if err != nil {
		t.Fatalf("Stat(target): %v", err)
	}

	// Stat on the symlink should follow it and return target metadata.
	linkMeta, err := sys.Stat(link)
	if err != nil {
		t.Fatalf("Stat(link): %v", err)
	}

	if !linkMeta.ModTime().Equal(targetMeta.ModTime()) {
		t.Errorf("Stat(link).ModTime() = %v, want %v (same as target)", linkMeta.ModTime(), targetMeta.ModTime())
	}

	if linkMeta.Blocks() != targetMeta.Blocks() {
		t.Errorf("Stat(link).Blocks() = %d, want %d (same as target)", linkMeta.Blocks(), targetMeta.Blocks())
	}
}

// TestLstat_Symlink verifies that Lstat returns metadata for the symlink itself
// without following it to the target. The target's mtime is set to 1 hour ago;
// the symlink's own mtime should be recent and differ from the target's.
// R2.1: lstat without symlink resolution.
func TestLstat_Symlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target content\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set target mtime to 1 hour ago so it clearly differs from symlink creation.
	past := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(target, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	targetMeta, err := sys.Stat(target)
	if err != nil {
		t.Fatalf("Stat(target): %v", err)
	}

	// Lstat should return metadata of the symlink, not the target.
	lstatMeta, err := sys.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(link): %v", err)
	}

	// The symlink was just created so its mtime should differ from the target's
	// mtime which was set to 1 hour ago.
	if lstatMeta.ModTime().Equal(targetMeta.ModTime()) {
		t.Errorf("Lstat(link).ModTime() = %v, same as target; want symlink's own mtime", lstatMeta.ModTime())
	}
}

// TestModTime_RecentFile verifies that ModTime returns a non-zero time within
// the last 60 seconds for a newly created file.
// R2.2: modification time accessor.
// R2.3: platform-specific stat field extraction.
func TestModTime_RecentFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "recent")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	meta, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if meta.ModTime().IsZero() {
		t.Error("ModTime() is zero, want non-zero")
	}

	elapsed := time.Since(meta.ModTime())
	if elapsed > 60*time.Second {
		t.Errorf("ModTime() is %v ago, want within 60 seconds", elapsed)
	}
}

// TestBlocks_NonEmptyFile verifies that Blocks returns a value >= 0 for a
// non-empty regular file.
// R2.2: disk block count accessor.
func TestBlocks_NonEmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "blocks")
	if err := os.WriteFile(path, []byte("some content for block allocation\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	meta, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if meta.Blocks() < 0 {
		t.Errorf("Blocks() = %d, want >= 0", meta.Blocks())
	}
}

// TestStat_NonExistent verifies that Stat returns a non-nil error wrapping the
// path for a file that does not exist.
// R2.1: error path for non-existent file.
func TestStat_NonExistent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := sys.Stat(path)
	if err == nil {
		t.Fatal("Stat on non-existent path returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not contain path %q", err.Error(), path)
	}
}

// TestLstat_NonExistent verifies that Lstat returns a non-nil error wrapping
// the path for a file that does not exist.
// R2.1: error path for non-existent file.
func TestLstat_NonExistent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := sys.Lstat(path)
	if err == nil {
		t.Fatal("Lstat on non-existent path returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not contain path %q", err.Error(), path)
	}
}
