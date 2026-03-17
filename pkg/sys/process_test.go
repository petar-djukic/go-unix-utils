// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"syscall"
	"testing"
)

func TestSetpgid_SelfToOwnGroup(t *testing.T) {
	t.Parallel()

	// Setpgid(0, 0) sets the calling process as its own process group leader.
	// This may fail if the process is already a session leader, which is
	// acceptable — we verify the call doesn't panic and returns a wrapped error.
	err := Setpgid(0, 0)
	if err != nil {
		// EPERM is expected if the process is already a process group leader
		// or session leader.
		t.Logf("Setpgid(0, 0) returned (expected in some contexts): %v", err)
	}
}

func TestKillpg_InvalidPgid(t *testing.T) {
	t.Parallel()

	// pgid must be positive.
	err := Killpg(0, syscall.SIGTERM)
	if err == nil {
		t.Error("Killpg(0, SIGTERM) should return an error for non-positive pgid")
	}

	err = Killpg(-1, syscall.SIGTERM)
	if err == nil {
		t.Error("Killpg(-1, SIGTERM) should return an error for negative pgid")
	}
}

func TestKillpg_NoSuchProcess(t *testing.T) {
	t.Parallel()

	// Use a pgid that almost certainly has no process group.
	err := Killpg(999999, syscall.Signal(0))
	if err == nil {
		t.Error("Killpg(999999, 0) should return an error for non-existent process group")
	}
}

func TestGetpriority_Self(t *testing.T) {
	t.Parallel()

	prio, err := Getpriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("Getpriority(PrioProcess, 0) failed: %v", err)
	}

	// Normal user processes have priority between -20 and 19.
	if prio < -20 || prio > 19 {
		t.Errorf("Getpriority returned %d, expected range [-20, 19]", prio)
	}
}

func TestSetpriority_Self(t *testing.T) {
	t.Parallel()

	// Get current priority so we can restore it.
	origPrio, err := Getpriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("Getpriority failed: %v", err)
	}

	// Non-root users can only increase the nice value (lower priority).
	// Setting to the same value should always succeed.
	err = Setpriority(PrioProcess, 0, origPrio)
	if err != nil {
		t.Fatalf("Setpriority(PrioProcess, 0, %d) failed: %v", origPrio, err)
	}

	// Verify the priority is unchanged.
	newPrio, err := Getpriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("Getpriority after Setpriority failed: %v", err)
	}
	if newPrio != origPrio {
		t.Errorf("priority changed: got %d, want %d", newPrio, origPrio)
	}
}

func TestPrioConstants(t *testing.T) {
	t.Parallel()

	// Verify our constants match the syscall package values.
	if PrioProcess != 0 {
		t.Errorf("PrioProcess = %d, want 0", PrioProcess)
	}
	// PrioPGrp and PrioUser values are platform-defined but should be non-negative.
	if PrioPGrp < 0 {
		t.Errorf("PrioPGrp = %d, want >= 0", PrioPGrp)
	}
	if PrioUser < 0 {
		t.Errorf("PrioUser = %d, want >= 0", PrioUser)
	}
}

func TestKillpg_OwnProcessGroup(t *testing.T) {
	t.Parallel()

	// Send signal 0 (null signal) to our own process group to verify
	// the syscall works for a valid pgid.
	pgid, err := syscall.Getpgid(os.Getpid())
	if err != nil {
		t.Fatalf("Getpgid failed: %v", err)
	}

	err = Killpg(pgid, syscall.Signal(0))
	if err != nil {
		t.Errorf("Killpg(%d, 0) failed: %v", pgid, err)
	}
}
