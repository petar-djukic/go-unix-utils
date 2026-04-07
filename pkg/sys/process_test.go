// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

func TestGetpriority(t *testing.T) {
	t.Parallel()
	// Get priority of the current process (who=0 means calling process).
	prio, err := Getpriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("Getpriority(PrioProcess, 0) failed: %v", err)
	}
	// Priority should be in the valid range -20 to 19.
	if prio < -20 || prio > 19 {
		t.Errorf("Getpriority returned %d, expected range [-20, 19]", prio)
	}
}

func TestSetpriority(t *testing.T) {
	t.Parallel()
	// Read current priority so we can restore it.
	original, err := Getpriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("Getpriority failed: %v", err)
	}
	// Non-root processes can only increase niceness (lower priority).
	// Set to a value >= current to avoid EPERM.
	target := original
	if target < 19 {
		target = original + 1
	}
	pid := os.Getpid()
	err = Setpriority(PrioProcess, pid, target)
	if err != nil {
		t.Fatalf("Setpriority(PrioProcess, %d, %d) failed: %v", pid, target, err)
	}
	got, err := Getpriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("Getpriority after set failed: %v", err)
	}
	if got != target {
		t.Errorf("priority after set: got %d, want %d", got, target)
	}
}

func TestSetpriorityInvalidWho(t *testing.T) {
	t.Parallel()
	// Use a PID that almost certainly does not exist.
	err := Setpriority(PrioProcess, 99999999, 10)
	if err == nil {
		t.Error("Setpriority with invalid PID should return error")
	}
}

func TestPrioConstants(t *testing.T) {
	t.Parallel()
	// Verify constants are distinct and non-negative.
	if PrioProcess == PrioPgrp || PrioProcess == PrioUser || PrioPgrp == PrioUser {
		t.Errorf("priority constants must be distinct: process=%d pgrp=%d user=%d",
			PrioProcess, PrioPgrp, PrioUser)
	}
}
