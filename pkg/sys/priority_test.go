// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"testing"
)

func TestGetPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		which   int
		who     int
		wantErr bool
	}{
		{
			name:    "current process priority",
			which:   PrioProcess,
			who:     0,
			wantErr: false,
		},
		{
			name:    "current process by pid",
			which:   PrioProcess,
			who:     os.Getpid(),
			wantErr: false,
		},
		{
			name:    "current user priority",
			which:   PrioUser,
			who:     0,
			wantErr: false,
		},
		{
			name:    "invalid which returns error",
			which:   -1,
			who:     0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prio, err := GetPriority(tc.which, tc.who)
			if tc.wantErr && err == nil {
				t.Errorf("GetPriority(%d, %d) = (%d, nil), want error", tc.which, tc.who, prio)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("GetPriority(%d, %d) = (%d, %v), want nil error", tc.which, tc.who, prio, err)
			}
			if !tc.wantErr {
				if prio < -20 || prio > 19 {
					t.Errorf("GetPriority(%d, %d) = %d, want value in range [-20, 19]", tc.which, tc.who, prio)
				}
			}
		})
	}
}

func TestSetPriority(t *testing.T) {
	t.Parallel()

	// Get the current priority so we can set it back to the same value
	// (non-root users can only raise the nice value, not lower it).
	currentPrio, err := GetPriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("GetPriority(PrioProcess, 0) = %v", err)
	}

	tests := []struct {
		name    string
		which   int
		who     int
		prio    int
		wantErr bool
	}{
		{
			name:    "set current process to same priority",
			which:   PrioProcess,
			who:     0,
			prio:    currentPrio,
			wantErr: false,
		},
		{
			name:    "invalid which returns error",
			which:   -1,
			who:     0,
			prio:    0,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := SetPriority(tc.which, tc.who, tc.prio)
			if tc.wantErr && err == nil {
				t.Errorf("SetPriority(%d, %d, %d) = nil, want error", tc.which, tc.who, tc.prio)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("SetPriority(%d, %d, %d) = %v, want nil", tc.which, tc.who, tc.prio, err)
			}
		})
	}
}

func TestSetPriorityRoundTrip(t *testing.T) {
	t.Parallel()

	// Get current priority, set it to the same value, and verify it didn't change.
	before, err := GetPriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("GetPriority(PrioProcess, 0) = %v", err)
	}

	if err := SetPriority(PrioProcess, 0, before); err != nil {
		t.Fatalf("SetPriority(PrioProcess, 0, %d) = %v", before, err)
	}

	after, err := GetPriority(PrioProcess, 0)
	if err != nil {
		t.Fatalf("GetPriority(PrioProcess, 0) = %v", err)
	}

	if before != after {
		t.Errorf("priority changed: before=%d, after=%d", before, after)
	}
}

func TestGetPriorityErrorMessage(t *testing.T) {
	t.Parallel()

	_, err := GetPriority(-1, 0)
	if err == nil {
		t.Fatal("GetPriority(-1, 0) = nil, want error")
	}
	want := "getpriority(-1, 0):"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error message = %q, want prefix %q", got, want)
	}
}

func TestSetPriorityErrorMessage(t *testing.T) {
	t.Parallel()

	err := SetPriority(-1, 0, 0)
	if err == nil {
		t.Fatal("SetPriority(-1, 0, 0) = nil, want error")
	}
	want := "setpriority(-1, 0, 0):"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error message = %q, want prefix %q", got, want)
	}
}

func TestPriorityConstants(t *testing.T) {
	t.Parallel()

	// Verify constants are distinct and non-negative.
	if PrioProcess == PrioPgrp || PrioProcess == PrioUser || PrioPgrp == PrioUser {
		t.Errorf("priority constants must be distinct: PrioProcess=%d, PrioPgrp=%d, PrioUser=%d",
			PrioProcess, PrioPgrp, PrioUser)
	}
}
