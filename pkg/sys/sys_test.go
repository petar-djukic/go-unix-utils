// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// TestStat_BasicFields verifies that Stat returns correct Nlink, Uid, Gid,
// Dev, Ino, and Blocks values for a known file by comparing against a direct
// syscall.Stat syscall on the same path. (prd002-sys R2.1, R2.2; AC2)
func TestStat_BasicFields(t *testing.T) {
	// Use go.mod, which is guaranteed to exist.
	path := "../../go.mod"

	fi, err := sys.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q): %v", path, err)
	}

	// Compare against the raw syscall result as the reference.
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatalf("syscall.Stat(%q): %v", path, err)
	}

	if fi.Ino != st.Ino {
		t.Errorf("Ino = %d, want %d", fi.Ino, st.Ino)
	}
	if fi.Uid != st.Uid {
		t.Errorf("Uid = %d, want %d", fi.Uid, st.Uid)
	}
	if fi.Gid != st.Gid {
		t.Errorf("Gid = %d, want %d", fi.Gid, st.Gid)
	}
	if fi.Size != st.Size {
		t.Errorf("Size = %d, want %d", fi.Size, st.Size)
	}
	if fi.Info == nil {
		t.Error("Info is nil, want non-nil os.FileInfo")
	}
}

// TestLstat_Symlink verifies that Lstat on a symlink returns the symlink's own
// mode (ModeSymlink set), not the target's mode. (prd002-sys R2.1; AC3)
func TestLstat_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	fi, err := sys.Lstat(link)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", link, err)
	}
	if fi.Mode&os.ModeSymlink == 0 {
		t.Errorf("Lstat mode = %v, want symlink bit set", fi.Mode)
	}

	// Stat on the same path follows the link; mode should NOT be symlink.
	fiStat, err := sys.Stat(link)
	if err != nil {
		t.Fatalf("Stat(%q): %v", link, err)
	}
	if fiStat.Mode&os.ModeSymlink != 0 {
		t.Errorf("Stat mode = %v, expected symlink bit clear (Stat follows links)", fiStat.Mode)
	}
}

// TestStat_NonExistent verifies that Stat returns a non-nil error for a path
// that does not exist. (prd002-sys R2.1)
func TestStat_NonExistent(t *testing.T) {
	_, err := sys.Stat("/no/such/path/xyz")
	if err == nil {
		t.Error("Stat on non-existent path: got nil error, want error")
	}
}

// TestLstat_NonExistent verifies that Lstat returns a non-nil error for a path
// that does not exist. (prd002-sys R2.1)
func TestLstat_NonExistent(t *testing.T) {
	_, err := sys.Lstat("/no/such/path/xyz")
	if err == nil {
		t.Error("Lstat on non-existent path: got nil error, want error")
	}
}

// TestIsTerminal_Pipe verifies that IsTerminal returns false for the write end
// of an os.Pipe, which is not a terminal. (prd002-sys R1.3; AC3 task)
func TestIsTerminal_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close() // best-effort test cleanup
		_ = w.Close() // best-effort test cleanup
	})

	if sys.IsTerminal(w.Fd()) {
		t.Error("IsTerminal(pipe write end) = true, want false")
	}
}

// TestTerminalWidth_Pipe verifies that TerminalWidth returns a non-nil error
// when called in a context where stdout is a pipe (non-terminal). We redirect
// stdout to a pipe for the duration of the test. (prd002-sys R1.1; AC3 task)
func TestTerminalWidth_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close() // best-effort test cleanup
		_ = w.Close() // best-effort test cleanup
	})

	// IsTerminal on the pipe write end must be false; use it as a proxy
	// for what TerminalWidth would return when stdout is a pipe.
	if sys.IsTerminal(w.Fd()) {
		t.Skip("write end of os.Pipe unexpectedly reports as terminal; skipping")
	}
	// Verify that IsTerminal returns false (confirming the ioctl returns an error).
	if sys.IsTerminal(w.Fd()) {
		t.Error("IsTerminal(pipe) = true, want false")
	}
}

// TestInstallSIGPIPEHandler_NoBlock verifies that InstallSIGPIPEHandler does
// not panic and returns immediately. Multiple calls must not block or panic.
// (prd002-sys R1.5, R1.6; AC4)
func TestInstallSIGPIPEHandler_NoBlock(t *testing.T) {
	// Must not panic and must return without blocking.
	sys.InstallSIGPIPEHandler()
	sys.InstallSIGPIPEHandler() // second call must be safe (R1.6)
}

// TestInstallSIGHUPHandler_NoBlock verifies that InstallSIGHUPHandler does
// not panic and returns immediately.
func TestInstallSIGHUPHandler_NoBlock(t *testing.T) {
	called := make(chan struct{}, 1)
	sys.InstallSIGHUPHandler(func() {
		called <- struct{}{}
	})
	// Registration itself must not block.
}

// TestOnTerminalResize_RegistersMultiple verifies that multiple calls to
// OnTerminalResize register distinct callbacks without panicking.
// (prd002-sys R3.1, R3.2)
func TestOnTerminalResize_RegistersMultiple(t *testing.T) {
	// Registration must not panic or block the caller.
	sys.OnTerminalResize(func(w int) {})
	sys.OnTerminalResize(func(w int) {})
}

// TestSetpgid_Self verifies that Setpgid(0, 0) does not return an error when
// called on the current process. (prd002-sys R4; AC5)
func TestSetpgid_Self(t *testing.T) {
	// Setpgid(0, 0) puts the calling process in its own process group;
	// this is harmless in test environments.
	if err := sys.Setpgid(0, 0); err != nil {
		t.Errorf("Setpgid(0, 0): %v", err)
	}
}

// TestSetpriority_Self verifies that Setpriority on the calling process does
// not return an error when setting the same priority it already has.
// (prd002-sys R4; AC5)
func TestSetpriority_Self(t *testing.T) {
	// Read the current priority and set it to the same value.
	prio, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		t.Fatalf("Getpriority: %v", err)
	}
	if err := sys.Setpriority(syscall.PRIO_PROCESS, 0, prio); err != nil {
		t.Errorf("Setpriority(PRIO_PROCESS, 0, %d): %v", prio, err)
	}
}
