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
