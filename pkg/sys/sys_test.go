// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// --- R2.6 tests: FileInfo fields required by find predicates ---

func TestStatReturnsFileInfoFields(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "testfile")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	fi, err := Stat(f)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// R2.6: find predicates require Mode, Size, ModTime, Uid, Gid, Nlink.
	if fi.Mode == 0 {
		t.Error("Mode is zero")
	}
	if fi.Size != 5 {
		t.Errorf("Size = %d, want 5", fi.Size)
	}
	if fi.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	// Nlink should be at least 1 for a regular file.
	if fi.Nlink < 1 {
		t.Errorf("Nlink = %d, want >= 1", fi.Nlink)
	}
	// Uid and Gid: just verify they are populated (non-negative is always true for uint32).
	// Verify Ino and Dev are non-zero.
	if fi.Ino == 0 {
		t.Error("Ino is zero")
	}
	if fi.Dev == 0 {
		t.Error("Dev is zero")
	}
}

func TestLstatSymlinkReturnsSymlinkMode(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	fi, err := Lstat(link)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if fi.Mode&os.ModeSymlink == 0 {
		t.Error("Lstat on symlink did not return ModeSymlink in Mode")
	}
}

// --- R3.1–R3.3 tests: OnTerminalResize with multiple callbacks ---

func TestOnTerminalResizeMultipleCallbacks(t *testing.T) {
	// Reset package-level state for this test.
	resizeMu.Lock()
	resizeCallbacks = nil
	resizeOnce = sync.Once{}
	resizeMu.Unlock()

	var mu sync.Mutex
	var order []int

	OnTerminalResize(func(_ int) {
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
	})
	OnTerminalResize(func(_ int) {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
	})
	OnTerminalResize(func(_ int) {
		mu.Lock()
		order = append(order, 3)
		mu.Unlock()
	})

	// R3.2: all three callbacks should be registered.
	resizeMu.Lock()
	count := len(resizeCallbacks)
	resizeMu.Unlock()
	if count != 3 {
		t.Fatalf("registered %d callbacks, want 3", count)
	}

	// Simulate a SIGWINCH by directly invoking the callbacks with a test width.
	resizeMu.Lock()
	cbs := make([]func(int), len(resizeCallbacks))
	copy(cbs, resizeCallbacks)
	resizeMu.Unlock()

	for _, cb := range cbs {
		cb(80)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("got %d callback invocations, want 3", len(order))
	}
	for i, v := range order {
		if v != i+1 {
			t.Errorf("order[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestIsTerminalPipe(t *testing.T) {
	t.Parallel()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if IsTerminal(r.Fd()) {
		t.Error("IsTerminal returned true for a pipe read end")
	}
}

func TestStatNonexistent(t *testing.T) {
	t.Parallel()
	_, err := Stat("/nonexistent-path-that-should-not-exist")
	if err == nil {
		t.Error("Stat on nonexistent path returned nil error")
	}
}

func TestLstatNonexistent(t *testing.T) {
	t.Parallel()
	_, err := Lstat("/nonexistent-path-that-should-not-exist")
	if err == nil {
		t.Error("Lstat on nonexistent path returned nil error")
	}
}
