// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func TestStat_RegularFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.Size != 5 {
		t.Errorf("Size = %d, want 5", fi.Size)
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
		t.Error("Ino = 0, want non-zero")
	}
	if fi.Dev == 0 {
		t.Error("Dev = 0, want non-zero")
	}
	if fi.Info == nil {
		t.Error("Info is nil")
	}
	if fi.Mode.IsDir() {
		t.Error("Mode.IsDir() = true for regular file")
	}
}

func TestStat_Directory(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	fi, err := sys.Stat(tmp)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if !fi.Mode.IsDir() {
		t.Error("Mode.IsDir() = false for directory")
	}
	if fi.Ino == 0 {
		t.Error("Ino = 0, want non-zero")
	}
}

func TestStat_NonExistent(t *testing.T) {
	t.Parallel()
	_, err := sys.Stat("/nonexistent-path-that-does-not-exist")
	if err == nil {
		t.Error("Stat on nonexistent path returned nil error")
	}
}

func TestLstat_Symlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating target: %v", err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := sys.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}

	if fi.Mode&os.ModeSymlink == 0 {
		t.Error("Lstat on symlink: ModeSymlink bit not set")
	}
}

func TestStat_FollowsSymlink(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating target: %v", err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := sys.Stat(link)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.Mode&os.ModeSymlink != 0 {
		t.Error("Stat on symlink: ModeSymlink bit set (should follow symlink)")
	}
	if fi.Size != 4 {
		t.Errorf("Size = %d, want 4 (target content)", fi.Size)
	}
}

func TestStat_ModTime(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	// ModTime should be close to os.Stat's ModTime
	osInfo, _ := os.Stat(path)
	diff := fi.ModTime.Sub(osInfo.ModTime())
	if diff < -time.Second || diff > time.Second {
		t.Errorf("ModTime %v differs from os.Stat ModTime %v by %v", fi.ModTime, osInfo.ModTime(), diff)
	}
}

func TestStat_AccessTime(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.AccessTime.IsZero() {
		t.Error("AccessTime is zero")
	}
}

func TestStat_ChangeTime(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.ChangeTime.IsZero() {
		t.Error("ChangeTime is zero")
	}
}

func TestStat_HardLinkCount(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "original")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}
	link := filepath.Join(tmp, "hardlink")
	if err := os.Link(path, link); err != nil {
		t.Fatalf("creating hard link: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if fi.Nlink != 2 {
		t.Errorf("Nlink = %d, want 2", fi.Nlink)
	}
}

func TestLstat_NonExistent(t *testing.T) {
	t.Parallel()
	_, err := sys.Lstat("/nonexistent-path-that-does-not-exist")
	if err == nil {
		t.Error("Lstat on nonexistent path returned nil error")
	}
}

// TestStat_PlatformFields verifies that FileInfo fields match values obtained
// directly from unix.Stat_t, confirming correct field mapping. (prd002-sys R2.2, R2.3)
func TestStat_PlatformFields(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		t.Fatalf("unix.Stat: %v", err)
	}

	if fi.Blocks != st.Blocks {
		t.Errorf("Blocks = %d, want %d", fi.Blocks, st.Blocks)
	}
	if fi.Blksize != int64(st.Blksize) {
		t.Errorf("Blksize = %d, want %d", fi.Blksize, int64(st.Blksize))
	}
	if fi.Ino != st.Ino {
		t.Errorf("Ino = %d, want %d", fi.Ino, st.Ino)
	}
	if fi.Dev != uint64(uint32(st.Dev)) {
		t.Errorf("Dev = %d, want %d", fi.Dev, uint64(uint32(st.Dev)))
	}
	if fi.Rdev != uint64(uint32(st.Rdev)) {
		t.Errorf("Rdev = %d, want %d", fi.Rdev, uint64(uint32(st.Rdev)))
	}
	if fi.Uid != st.Uid {
		t.Errorf("Uid = %d, want %d", fi.Uid, st.Uid)
	}
	if fi.Gid != st.Gid {
		t.Errorf("Gid = %d, want %d", fi.Gid, st.Gid)
	}
	if fi.Nlink != uint64(st.Nlink) {
		t.Errorf("Nlink = %d, want %d", fi.Nlink, uint64(st.Nlink))
	}
	if fi.Size != st.Size {
		t.Errorf("Size = %d, want %d", fi.Size, st.Size)
	}
}
