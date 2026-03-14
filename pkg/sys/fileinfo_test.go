// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func TestStat_MatchesOsStat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")
	content := []byte("hello, fileinfo\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}

	osInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("sys.Stat: %v", err)
	}

	// AC4: Mode, Size, ModTime match os.Stat.
	if fi.Mode != osInfo.Mode() {
		t.Errorf("Mode: got %v, want %v", fi.Mode, osInfo.Mode())
	}
	if fi.Size != osInfo.Size() {
		t.Errorf("Size: got %d, want %d", fi.Size, osInfo.Size())
	}
	if !fi.ModTime.Equal(osInfo.ModTime()) {
		t.Errorf("ModTime: got %v, want %v", fi.ModTime, osInfo.ModTime())
	}
	if fi.Info == nil {
		t.Error("Info field is nil")
	}
}

func TestLstat_DoesNotFollowSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("setup: Symlink: %v", err)
	}

	fi, err := sys.Lstat(link)
	if err != nil {
		t.Fatalf("sys.Lstat: %v", err)
	}

	// Lstat on a symlink must report it as a symlink, not the target.
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Lstat: expected symlink mode, got %v", fi.Mode)
	}
}

func TestStat_Follows_Symlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(target, []byte("target data"), 0o644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("setup: Symlink: %v", err)
	}

	fi, err := sys.Stat(link)
	if err != nil {
		t.Fatalf("sys.Stat: %v", err)
	}

	// Stat through a symlink must report the target (not a symlink).
	if fi.Mode&os.ModeSymlink != 0 {
		t.Errorf("Stat: expected regular file mode, got %v", fi.Mode)
	}
}

func TestStat_NonexistentPath(t *testing.T) {
	t.Parallel()

	_, err := sys.Stat("/does/not/exist/at/all")
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

func TestStat_FieldsPopulated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "fields.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("sys.Stat: %v", err)
	}

	if fi.Ino == 0 {
		t.Error("Ino is 0, expected non-zero inode number")
	}
	if fi.Dev == 0 {
		t.Error("Dev is 0, expected non-zero device number")
	}
	if fi.Nlink == 0 {
		t.Error("Nlink is 0, expected at least 1")
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	if fi.AccessTime.IsZero() {
		t.Error("AccessTime is zero")
	}
	if fi.ChangeTime.IsZero() {
		t.Error("ChangeTime is zero")
	}
}
