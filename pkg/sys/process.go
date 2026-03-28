// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R3.1–R3.3: process group management (Setpgid, Killpg)
// and process priority adjustment (Setpriority).

package sys

import (
	"fmt"
	"syscall"
)

// Setpgid sets the process group ID of the process specified by pid.
// If pid is 0, the calling process's PID is used. If pgid is 0, the
// process specified by pid becomes a process group leader.
//
// R3.1: wraps syscall.Setpgid for creating process groups.
// Used by timeout and xargs -P to manage child process groups.
func Setpgid(pid, pgid int) error {
	err := syscall.Setpgid(pid, pgid)
	if err != nil {
		return fmt.Errorf("setpgid(%d, %d): %w", pid, pgid, err)
	}
	return nil
}

// Killpg sends a signal to all processes in the specified process group.
//
// R3.2: wraps syscall.Kill with negative pgid to target the entire
// process group. Used by timeout to signal all children on expiry
// and by xargs -P to clean up parallel workers.
func Killpg(pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("killpg: invalid pgid %d: must be positive", pgid)
	}
	err := syscall.Kill(-pgid, sig)
	if err != nil {
		return fmt.Errorf("killpg(%d, %v): %w", pgid, sig, err)
	}
	return nil
}

// Setpriority sets the scheduling priority of a process, process group,
// or user. The which parameter selects the target type:
// syscall.PRIO_PROCESS, syscall.PRIO_PGRP, or syscall.PRIO_USER.
// The who parameter identifies the target (0 means the calling process).
// The prio parameter is the new priority value (-20 to 19).
//
// R3.3: wraps syscall.Setpriority for adjusting process nice values.
// Used by nice and renice utilities.
func Setpriority(which, who, prio int) error {
	err := syscall.Setpriority(which, who, prio)
	if err != nil {
		return fmt.Errorf("setpriority(%d, %d, %d): %w", which, who, prio, err)
	}
	return nil
}
