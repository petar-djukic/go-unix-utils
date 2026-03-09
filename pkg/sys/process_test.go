// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"syscall"
	"testing"
)

// TestSetProcessGroup_Self verifies that SetProcessGroup can set the calling
// process's own process group without error.
func TestSetProcessGroup_Self(t *testing.T) {
	t.Parallel()

	// Get current pgid to restore later.
	origPgid, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatalf("Getpgid(0): %v", err)
	}

	// Setting pgid to the current pgid is a no-op but must not error.
	if err := SetProcessGroup(0, origPgid); err != nil {
		t.Errorf("SetProcessGroup(0, %d) = %v, want nil", origPgid, err)
	}
}

// TestKillProcessGroup_InvalidPGID verifies that KillProcessGroup rejects
// non-positive pgid values.
func TestKillProcessGroup_InvalidPGID(t *testing.T) {
	t.Parallel()

	if err := KillProcessGroup(0, syscall.SIGTERM); err == nil {
		t.Error("KillProcessGroup(0, SIGTERM) = nil, want error for non-positive pgid")
	}
	if err := KillProcessGroup(-1, syscall.SIGTERM); err == nil {
		t.Error("KillProcessGroup(-1, SIGTERM) = nil, want error for negative pgid")
	}
}

// TestKillProcessGroup_OwnGroup verifies that sending signal 0 (null signal)
// to the current process group succeeds without actually killing anything.
func TestKillProcessGroup_OwnGroup(t *testing.T) {
	t.Parallel()

	pgid, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatalf("Getpgid(0): %v", err)
	}

	// Signal 0 checks permissions without sending a real signal.
	if err := KillProcessGroup(pgid, syscall.Signal(0)); err != nil {
		t.Errorf("KillProcessGroup(%d, 0) = %v, want nil", pgid, err)
	}
}

// TestGetPriority_Self verifies that GetPriority returns the calling
// process's nice value without error.
func TestGetPriority_Self(t *testing.T) {
	t.Parallel()

	prio, err := GetPriority(PrioProcess, os.Getpid())
	if err != nil {
		t.Fatalf("GetPriority(PrioProcess, self) = _, %v; want nil error", err)
	}

	// Nice values range from -20 to 19.
	if prio < -20 || prio > 19 {
		t.Errorf("GetPriority returned %d, want value in [-20, 19]", prio)
	}
}

// TestGetPriority_Zero verifies that who=0 targets the calling process.
func TestGetPriority_Zero(t *testing.T) {
	t.Parallel()

	_, err := GetPriority(PrioProcess, 0)
	if err != nil {
		t.Errorf("GetPriority(PrioProcess, 0) = _, %v; want nil error", err)
	}
}

// TestSetPriority_SameValue verifies that setting priority to the current
// value succeeds (idempotent operation).
func TestSetPriority_SameValue(t *testing.T) {
	t.Parallel()

	prio, err := GetPriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("GetPriority: %v", err)
	}

	// Setting to the same value should succeed.
	if err := SetPriority(PrioProcess, 0, prio); err != nil {
		t.Errorf("SetPriority(PrioProcess, 0, %d) = %v, want nil", prio, err)
	}
}

// TestPriorityConstants verifies that the exported priority constants
// have the expected POSIX values.
func TestPriorityConstants(t *testing.T) {
	t.Parallel()

	if PrioProcess != 0 {
		t.Errorf("PrioProcess = %d, want 0", PrioProcess)
	}
	if PrioPGRP != 1 {
		t.Errorf("PrioPGRP = %d, want 1", PrioPGRP)
	}
	if PrioUser != 2 {
		t.Errorf("PrioUser = %d, want 2", PrioUser)
	}
}
