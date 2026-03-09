// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStat_RegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	if fi.Info == nil {
		t.Fatal("FileInfo.Info is nil")
	}
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}
	if fi.Nlink < 1 {
		t.Errorf("Nlink = %d, want >= 1", fi.Nlink)
	}
	if fi.Uid != uint32(os.Getuid()) {
		t.Errorf("Uid = %d, want %d", fi.Uid, os.Getuid())
	}
	if fi.Gid != uint32(os.Getgid()) {
		t.Errorf("Gid = %d, want %d", fi.Gid, os.Getgid())
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want nonzero")
	}
	if fi.Dev == 0 {
		t.Error("Dev = 0, want nonzero")
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
	if fi.Mode.IsDir() {
		t.Error("Mode reports directory for a regular file")
	}
}

func TestStat_Directory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	fi, err := Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}

	if !fi.Mode.IsDir() {
		t.Errorf("Mode.IsDir() = false, want true")
	}
	if fi.Nlink < 2 {
		t.Errorf("Nlink = %d, want >= 2 for directory", fi.Nlink)
	}
}

func TestStat_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("Stat on nonexistent path returned nil error")
	}
}

func TestLstat_Symlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	// AC3: Lstat on a symlink returns the symlink's own mode (type bits show symlink).
	lfi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}
	if lfi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Lstat mode %v does not have ModeSymlink set", lfi.Mode)
	}

	// Stat on the same path follows the symlink and returns the target's mode.
	sfi, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}
	if sfi.Mode&os.ModeSymlink != 0 {
		t.Errorf("Stat mode %v has ModeSymlink set (should follow symlink)", sfi.Mode)
	}

	// The target is a regular file, so Stat should report it as such.
	if !sfi.Mode.IsRegular() {
		t.Errorf("Stat mode %v is not regular file", sfi.Mode)
	}
}

func TestLstat_RegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "regular")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("creating file: %v", err)
	}

	fi, err := Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}

	if fi.Mode&os.ModeSymlink != 0 {
		t.Errorf("Lstat on regular file has ModeSymlink set")
	}
	if !fi.Mode.IsRegular() {
		t.Errorf("Lstat on regular file: Mode.IsRegular() = false")
	}
}

func TestStat_Blksize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "blktest")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("creating file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d, want positive", fi.Blksize)
	}
}
