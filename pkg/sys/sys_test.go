// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTerminalWidth_NotTerminal verifies that TerminalWidth returns a non-nil
// error when stdout is not a terminal (it is a pipe in the test runner).
//
// Per prd002-sys AC1.
func TestTerminalWidth_NotTerminal(t *testing.T) {
	// In the test runner, stdout is typically a pipe, not a terminal.
	// TerminalWidth should return an error.
	_, err := TerminalWidth()
	if err == nil {
		// If running in a real terminal (interactive test), this might pass.
		// Skip rather than fail in that case.
		t.Skip("stdout appears to be a terminal; cannot test non-terminal error path")
	}
}

// TestIsTerminal_Pipe verifies that IsTerminal returns false for a pipe fd.
//
// Per prd002-sys R1.3.
func TestIsTerminal_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if IsTerminal(r.Fd()) {
		t.Errorf("IsTerminal returned true for pipe read end")
	}
	if IsTerminal(w.Fd()) {
		t.Errorf("IsTerminal returned true for pipe write end")
	}
}

// TestStat_RegularFile verifies that Stat returns correct FileInfo fields for a
// known regular file.
//
// Per prd002-sys AC2.
func TestStat_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")

	content := []byte("hello world\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	fi, err := Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// Verify Mode indicates a regular file.
	if !fi.Mode.IsRegular() {
		t.Errorf("Mode.IsRegular() = false, want true; Mode = %v", fi.Mode)
	}

	// Verify Size matches written content.
	if fi.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", fi.Size, len(content))
	}

	// Verify Nlink is at least 1.
	if fi.Nlink < 1 {
		t.Errorf("Nlink = %d, want >= 1", fi.Nlink)
	}

	// Verify Uid matches the current user.
	if fi.Uid != uint32(os.Getuid()) {
		t.Errorf("Uid = %d, want %d", fi.Uid, os.Getuid())
	}

	// Verify Gid matches the current group.
	if fi.Gid != uint32(os.Getgid()) {
		t.Errorf("Gid = %d, want %d", fi.Gid, os.Getgid())
	}

	// Verify Ino is non-zero.
	if fi.Ino == 0 {
		t.Errorf("Ino = 0, want non-zero")
	}

	// Verify Dev is non-zero.
	if fi.Dev == 0 {
		t.Errorf("Dev = 0, want non-zero")
	}

	// Verify Blocks is non-negative.
	if fi.Blocks < 0 {
		t.Errorf("Blocks = %d, want >= 0", fi.Blocks)
	}

	// Verify Blksize is positive.
	if fi.Blksize <= 0 {
		t.Errorf("Blksize = %d, want > 0", fi.Blksize)
	}

	// Verify ModTime is recent (within the last minute).
	if time.Since(fi.ModTime) > time.Minute {
		t.Errorf("ModTime = %v, want within the last minute", fi.ModTime)
	}

	// Verify Info is non-nil and consistent.
	if fi.Info == nil {
		t.Fatalf("Info is nil")
	}
	if fi.Info.Size() != fi.Size {
		t.Errorf("Info.Size() = %d, want %d", fi.Info.Size(), fi.Size)
	}
}

// TestLstat_Symlink verifies that Lstat on a symlink returns FileInfo with Mode
// type bits showing symlink, not the target's mode.
//
// Per prd002-sys AC3.
func TestLstat_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("target content\n"), 0644); err != nil {
		t.Fatalf("creating target file: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}

	// Mode type bits must indicate a symlink.
	if fi.Mode.Type()&os.ModeSymlink == 0 {
		t.Errorf("Lstat on symlink: Mode = %v, want ModeSymlink bit set", fi.Mode)
	}

	// Stat (following link) should show a regular file.
	fiFolowed, err := Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}
	if !fiFolowed.Mode.IsRegular() {
		t.Errorf("Stat on symlink target: Mode = %v, want regular file", fiFolowed.Mode)
	}
}

// TestStat_Directory verifies that Stat returns correct Mode for a directory.
func TestStat_Directory(t *testing.T) {
	dir := t.TempDir()

	fi, err := Stat(dir)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dir, err)
	}

	if !fi.Mode.IsDir() {
		t.Errorf("Mode.IsDir() = false for directory; Mode = %v", fi.Mode)
	}
}

// TestStat_NonExistent verifies that Stat returns an error for a non-existent
// path.
func TestStat_NonExistent(t *testing.T) {
	_, err := Stat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Errorf("Stat on non-existent path returned nil error")
	}
}

// TestLstat_NonExistent verifies that Lstat returns an error for a non-existent
// path.
func TestLstat_NonExistent(t *testing.T) {
	_, err := Lstat("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Errorf("Lstat on non-existent path returned nil error")
	}
}

// TestInstallSIGPIPEHandler verifies that InstallSIGPIPEHandler can be called
// without panicking and registers the handler.
//
// Per prd002-sys AC4. Full behavior (os.Exit(0) on SIGPIPE) is verified in
// integration tests; here we verify the function runs without error.
func TestInstallSIGPIPEHandler(t *testing.T) {
	// Should not panic.
	InstallSIGPIPEHandler()
}

// TestOnTerminalResize verifies that OnTerminalResize registers callbacks
// without panicking. Full SIGWINCH behavior is not testable without a real
// terminal, but we verify registration works and multiple calls are accepted.
//
// Per prd002-sys AC5, R4.1-R4.2.
func TestOnTerminalResize(t *testing.T) {
	called := false
	OnTerminalResize(func(width int) {
		called = true
		_ = called
	})

	// Register a second callback to verify multiple registrations.
	OnTerminalResize(func(width int) {
		_ = width
	})

	// Verify callbacks were registered (internal state check).
	resizeState.mu.Lock()
	count := len(resizeState.callbacks)
	resizeState.mu.Unlock()

	if count < 2 {
		t.Errorf("registered callback count = %d, want >= 2", count)
	}
}

// TestStat_HardLink verifies that two hard links to the same file report the
// same Dev and Ino values and Nlink >= 2.
//
// Per prd002-sys R2.5 (du needs st_dev and st_ino for hard-link deduplication).
func TestStat_HardLink(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	linked := filepath.Join(dir, "linked")

	if err := os.WriteFile(original, []byte("shared content\n"), 0644); err != nil {
		t.Fatalf("creating original file: %v", err)
	}
	if err := os.Link(original, linked); err != nil {
		t.Fatalf("creating hard link: %v", err)
	}

	fiOrig, err := Stat(original)
	if err != nil {
		t.Fatalf("Stat(%q): %v", original, err)
	}
	fiLink, err := Stat(linked)
	if err != nil {
		t.Fatalf("Stat(%q): %v", linked, err)
	}

	if fiOrig.Dev != fiLink.Dev {
		t.Errorf("Dev mismatch: original=%d, linked=%d", fiOrig.Dev, fiLink.Dev)
	}
	if fiOrig.Ino != fiLink.Ino {
		t.Errorf("Ino mismatch: original=%d, linked=%d", fiOrig.Ino, fiLink.Ino)
	}
	if fiOrig.Nlink < 2 {
		t.Errorf("Nlink = %d after hard link, want >= 2", fiOrig.Nlink)
	}
}
