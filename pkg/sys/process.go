// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys process group management (setpgid, killpg) and
// process priority (getpriority, setpriority) for xargs -P and nice/renice.

package sys

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// Process priority "which" constants for GetPriority and SetPriority.
const (
	// PrioProcess targets a single process by PID.
	PrioProcess = unix.PRIO_PROCESS
	// PrioPGRP targets a process group by PGID.
	PrioPGRP = unix.PRIO_PGRP
	// PrioUser targets all processes owned by a UID.
	PrioUser = unix.PRIO_USER
)

// SetProcessGroup sets the process group ID of the process specified by pid.
// If pid is 0, the calling process's PGID is set. If pgid is 0, the process's
// own PID is used as the new PGID (creating a new process group).
func SetProcessGroup(pid, pgid int) error {
	if err := unix.Setpgid(pid, pgid); err != nil {
		return fmt.Errorf("setpgid(%d, %d): %w", pid, pgid, err)
	}
	return nil
}

// KillProcessGroup sends a signal to every process in the process group
// identified by pgid. Equivalent to killpg(pgid, sig), implemented as
// kill(-pgid, sig).
func KillProcessGroup(pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("killpg: pgid must be positive, got %d", pgid)
	}
	if err := unix.Kill(-pgid, sig); err != nil {
		return fmt.Errorf("killpg(%d, %v): %w", pgid, sig, err)
	}
	return nil
}

// GetPriority returns the scheduling priority (nice value) for the target
// specified by which and who. which must be one of PrioProcess, PrioPGRP,
// or PrioUser. who is the PID, PGID, or UID respectively; 0 means the
// calling process/group/user.
func GetPriority(which, who int) (int, error) {
	prio, err := unix.Getpriority(which, who)
	if err != nil {
		return 0, fmt.Errorf("getpriority(%d, %d): %w", which, who, err)
	}
	return prio, nil
}

// SetPriority sets the scheduling priority (nice value) for the target
// specified by which and who. which must be one of PrioProcess, PrioPGRP,
// or PrioUser. who is the PID, PGID, or UID respectively; 0 means the
// calling process/group/user. prio ranges from -20 (highest) to 19 (lowest).
func SetPriority(which, who, prio int) error {
	if err := unix.Setpriority(which, who, prio); err != nil {
		return fmt.Errorf("setpriority(%d, %d, %d): %w", which, who, prio, err)
	}
	return nil
}
