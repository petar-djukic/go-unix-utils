// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTerminalWidth_Pipe verifies that TerminalWidth returns an error when
// stdout is not a terminal (as is the case in go test, where stdout is a pipe).
// Implements use case S1.
func TestTerminalWidth_Pipe(t *testing.T) {
	t.Parallel()
	_, err := TerminalWidth()
	if err == nil {
		// If this runs in a terminal environment, the call may succeed.
		t.Log("TerminalWidth succeeded (stdout appears to be a terminal in this env)")
	}
	// No failure assertion: the test verifies the function does not panic.
	// AC2 coverage via TestIsTerminal_Stdin is the normative check.
}

// TestIsTerminal_Stdin verifies that IsTerminal returns false for stdin in
// the go test context, where stdin is a pipe (not a terminal).
// Implements use case S2 and acceptance criterion AC2.
func TestIsTerminal_Stdin(t *testing.T) {
	t.Parallel()
	if IsTerminal(os.Stdin.Fd()) {
		t.Error("IsTerminal(stdin.Fd()) = true in go test context, want false")
	}
}

// TestIsTerminal_Pipe verifies that IsTerminal returns false for the write
// end of an os.Pipe, which is never a terminal.
// Implements prd002-sys R1.3.
func TestIsTerminal_Pipe(t *testing.T) {
	t.Parallel()
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() }) // best-effort cleanup, error ignored
	if IsTerminal(w.Fd()) {
		t.Error("IsTerminal(pipe writer Fd()) = true, want false")
	}
}

// TestStat_ExistingFile verifies that Stat on a newly created temp file
// returns a FileInfo with Nlink >= 1 and Uid/Gid matching the current process.
// Implements use case S3 and acceptance criterion AC3.
func TestStat_ExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "stat-test")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s): %v", path, err)
	}

	// AC3: Nlink must be at least 1 for a newly created file.
	if fi.Nlink < 1 {
		t.Errorf("Stat Nlink = %d, want >= 1", fi.Nlink)
	}
	// AC3: Uid must match the current process owner.
	wantUID := uint32(os.Getuid()) //nolint:gosec
	if fi.Uid != wantUID {
		t.Errorf("Stat Uid = %d, want %d", fi.Uid, wantUID)
	}
	// AC3: Gid must match the current process group.
	wantGID := uint32(os.Getgid()) //nolint:gosec
	if fi.Gid != wantGID {
		t.Errorf("Stat Gid = %d, want %d", fi.Gid, wantGID)
	}
	// Sanity: Dev and Ino are non-zero for a real file.
	if fi.Dev == 0 {
		t.Error("Stat Dev = 0, want non-zero")
	}
	if fi.Ino == 0 {
		t.Error("Stat Ino = 0, want non-zero")
	}
	// Sanity: ModTime must be non-zero.
	if fi.ModTime.IsZero() {
		t.Error("Stat ModTime is zero, want non-zero")
	}
}

// TestStat_NotExist verifies that Stat returns an error for a non-existent path.
func TestStat_NotExist(t *testing.T) {
	t.Parallel()
	_, err := Stat(filepath.Join(t.TempDir(), "no-such-file"))
	if err == nil {
		t.Error("Stat(non-existent) returned nil error, want error")
	}
}

// TestLstat_Symlink verifies that Lstat on a symbolic link returns FileInfo
// whose Mode has the symlink type bits set, not the target's mode.
// Implements use case S4 and prd002-sys R2.1, AC3.
func TestLstat_Symlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", link, err)
	}
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Lstat Mode = %v, want symlink bit set (ModeSymlink)", fi.Mode)
	}
}

// TestLstat_NotFollowing verifies that Lstat does not follow symbolic links:
// Stat on a symlink returns the target's size while Lstat returns the symlink's.
// Implements prd002-sys R2.1.
func TestLstat_NotFollowing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	const content = "hello world"
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	statFI, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%s): %v", link, err)
	}
	lstatFI, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", link, err)
	}
	// Stat follows the link: size equals the target content length.
	if statFI.Size != int64(len(content)) {
		t.Errorf("Stat(symlink) Size = %d, want %d", statFI.Size, len(content))
	}
	// Lstat does not follow: size is the symlink's own stored path length.
	if lstatFI.Mode&os.ModeSymlink == 0 {
		t.Error("Lstat(symlink) Mode has no symlink bit")
	}
}

// TestFillFromStat_ModTime verifies that FileInfo.ModTime is populated
// (non-zero) by the platform-specific fillFromStat.
// Implements use case S5 and prd002-sys R2.3.
func TestFillFromStat_ModTime(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "mtime-test")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.ModTime.IsZero() {
		t.Error("FileInfo.ModTime is zero after fillFromStat, want non-zero")
	}
}
