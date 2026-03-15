// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R2.5-R2.6: Process priority functions.
// GetPriority wraps the getpriority syscall and SetPriority wraps setpriority.

package sys

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Priority target constants matching POSIX prio_which values.
const (
	// PrioProcess targets a single process by PID.
	PrioProcess = unix.PRIO_PROCESS
	// PrioPgrp targets all processes in a process group by PGID.
	PrioPgrp = unix.PRIO_PGRP
	// PrioUser targets all processes owned by a user by UID.
	PrioUser = unix.PRIO_USER
)

// GetPriority returns the scheduling priority for the target specified by
// which and who. which must be one of PrioProcess, PrioPgrp, or PrioUser.
// If who is 0, the calling process, process group, or user is used.
//
// The returned priority is in the range -20 (highest) to 19 (lowest).
//
// R2.5: wraps getpriority(2) via golang.org/x/sys/unix.
func GetPriority(which int, who int) (int, error) {
	prio, err := unix.Getpriority(which, who)
	if err != nil {
		return 0, fmt.Errorf("getpriority(%d, %d): %w", which, who, err)
	}
	// On Linux, getpriority returns 20 - prio (so the range is 1..40).
	// The unix package handles this normalization and returns the actual
	// nice value in the range -20..19.
	return prio, nil
}

// SetPriority sets the scheduling priority for the target specified by
// which and who. which must be one of PrioProcess, PrioPgrp, or PrioUser.
// If who is 0, the calling process, process group, or user is used.
//
// prio must be in the range -20 (highest priority) to 19 (lowest priority).
// On most systems, only the superuser can lower the nice value (increase priority).
//
// R2.6: wraps setpriority(2) via golang.org/x/sys/unix, following the same
// error-handling conventions as Stat/Lstat.
func SetPriority(which int, who int, prio int) error {
	if err := unix.Setpriority(which, who, prio); err != nil {
		return fmt.Errorf("setpriority(%d, %d, %d): %w", which, who, prio, err)
	}
	return nil
}
