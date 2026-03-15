// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestSetpgid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pid     int
		pgid    int
		wantErr bool
	}{
		{
			name:    "invalid pid returns error",
			pid:     -1,
			pgid:    0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Setpgid(tc.pid, tc.pgid)
			if tc.wantErr && err == nil {
				t.Errorf("Setpgid(%d, %d) = nil, want error", tc.pid, tc.pgid)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Setpgid(%d, %d) = %v, want nil", tc.pid, tc.pgid, err)
			}
		})
	}
}

func TestSetpgidOnChildProcess(t *testing.T) {
	t.Parallel()

	// Start a child process with its own process group.
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill() // best-effort cleanup
		_ = cmd.Wait()
	}()

	pid := cmd.Process.Pid

	// The child is already in its own process group (via SysProcAttr).
	// Verify we can send signal 0 to the child's process group.
	err := Killpg(pid, 0)
	if err != nil {
		t.Errorf("Killpg(%d, 0) = %v, want nil", pid, err)
	}
}

func TestKillpg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pgid    int
		sig     syscall.Signal
		wantErr bool
	}{
		{
			name:    "zero pgid returns error",
			pgid:    0,
			sig:     0,
			wantErr: true,
		},
		{
			name:    "negative pgid returns error",
			pgid:    -1,
			sig:     0,
			wantErr: true,
		},
		{
			name:    "nonexistent process group returns error",
			pgid:    99999999,
			sig:     0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := Killpg(tc.pgid, tc.sig)
			if tc.wantErr && err == nil {
				t.Errorf("Killpg(%d, %v) = nil, want error", tc.pgid, tc.sig)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Killpg(%d, %v) = %v, want nil", tc.pgid, tc.sig, err)
			}
		})
	}
}

func TestKillpgSignalZeroOnOwnGroup(t *testing.T) {
	t.Parallel()

	// Signal 0 to our own process group should succeed.
	pgid := os.Getpid()
	// Use the process's own pgid (which may differ from pid).
	rawPgid, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatalf("Getpgid(0) = %v", err)
	}
	pgid = rawPgid

	if err := Killpg(pgid, 0); err != nil {
		t.Errorf("Killpg(%d, 0) = %v, want nil", pgid, err)
	}
}

func TestSetpgidErrorMessage(t *testing.T) {
	t.Parallel()

	err := Setpgid(-1, 0)
	if err == nil {
		t.Fatal("Setpgid(-1, 0) = nil, want error")
	}
	want := "setpgid(-1, 0):"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error message = %q, want prefix %q", got, want)
	}
}

func TestKillpgErrorMessage(t *testing.T) {
	t.Parallel()

	err := Killpg(0, 0)
	if err == nil {
		t.Fatal("Killpg(0, 0) = nil, want error")
	}
	want := "killpg(0, "
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error message = %q, want prefix %q", got, want)
	}
}
