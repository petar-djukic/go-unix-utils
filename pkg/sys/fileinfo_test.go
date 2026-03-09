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
		t.Fatalf("write test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// AC1: FileInfo struct populated correctly.
	if fi.Size != int64(len(content)) {
		t.Errorf("Size: got %d, want %d", fi.Size, len(content))
	}
	if fi.Nlink < 1 {
		t.Errorf("Nlink: got %d, want >= 1", fi.Nlink)
	}
	if fi.Uid == 0 && os.Getuid() != 0 {
		t.Errorf("Uid: got 0 but current user is not root")
	}
	if fi.Ino == 0 {
		t.Errorf("Ino: got 0, want non-zero")
	}
	if fi.Dev == 0 {
		t.Errorf("Dev: got 0, want non-zero")
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime: is zero")
	}
	if fi.AccessTime.IsZero() {
		t.Error("AccessTime: is zero")
	}
	if fi.ChangeTime.IsZero() {
		t.Error("ChangeTime: is zero")
	}
	if fi.Info == nil {
		t.Error("Info: is nil")
	}
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode: not a regular file: %v", fi.Mode)
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
		t.Errorf("Mode: not a directory: %v", fi.Mode)
	}
	if fi.Nlink < 2 {
		t.Errorf("Nlink: got %d, want >= 2 for directory", fi.Nlink)
	}
}

func TestStatNonExistent(t *testing.T) {
	t.Parallel()

	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Stat on nonexistent path: expected error, got nil")
	}
}

func TestLstatSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// AC3: Lstat on a symlink returns symlink's own mode.
	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Lstat on symlink: Mode does not have ModeSymlink set: %v", fi.Mode)
	}

	// Stat should follow the symlink and return a regular file.
	fi2, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat on symlink: %v", err)
	}
	if !fi2.Mode.IsRegular() {
		t.Errorf("Stat on symlink: expected regular file mode, got %v", fi2.Mode)
	}
}

func TestLstatNonExistent(t *testing.T) {
	t.Parallel()

	_, err := Lstat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Lstat on nonexistent path: expected error, got nil")
	}
}

func TestStatHardLinkCount(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "original")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	linkPath := filepath.Join(dir, "hardlink")
	if err := os.Link(path, linkPath); err != nil {
		t.Skipf("os.Link not supported: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Nlink != 2 {
		t.Errorf("Nlink: got %d, want 2 after creating hard link", fi.Nlink)
	}
}
