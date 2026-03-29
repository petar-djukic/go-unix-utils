// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import "syscall"

// SetPriority sets the scheduling priority for the specified process.
// which is one of syscall.PRIO_PROCESS, PRIO_PGRP, or PRIO_USER.
// who identifies the target (pid, pgid, or uid; 0 means the caller).
// prio is the new priority value (typically -20 to 19).
//
// R2.6 (prd002): wraps syscall.Setpriority for use by nice and renice commands.
func SetPriority(which, who, prio int) error {
	return syscall.Setpriority(which, who, prio)
}

// GetPriority returns the scheduling priority for the specified process.
// which and who have the same meaning as in SetPriority.
//
// R3.1 (prd002): wraps syscall.Getpriority for reading current process priority.
func GetPriority(which, who int) (int, error) {
	return syscall.Getpriority(which, who)
}

// Setpgid sets the process group ID of the process specified by pid.
// If pid is 0, the caller's process group is set. If pgid is 0, the
// process ID of the specified process is used as the process group ID.
//
// R3.2 (prd002): wraps syscall.Setpgid for use by timeout and xargs -P.
func Setpgid(pid, pgid int) error {
	return syscall.Setpgid(pid, pgid)
}

// Killpg sends a signal to all processes in the specified process group.
// It calls syscall.Kill with a negated pgid per POSIX convention.
//
// R3.3 (prd002): wraps syscall.Kill(-pgid, sig) to signal an entire process group.
func Killpg(pgid int, sig syscall.Signal) error {
	return syscall.Kill(-pgid, sig)
}
