// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Process priority management for pkg/sys.
// Implements srd002-sys architecture capability: process priority (setpriority)
// for nice and renice utility support.
package sys

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Priority "which" constants matching POSIX PRIO_PROCESS, PRIO_PGRP, PRIO_USER.
const (
	PrioProcess = unix.PRIO_PROCESS
	PrioPgrp    = unix.PRIO_PGRP
	PrioUser    = unix.PRIO_USER
)

// Getpriority returns the scheduling priority for the target identified by
// which (PrioProcess, PrioPgrp, or PrioUser) and who (PID, PGID, or UID;
// 0 means the calling process/group/user). Wraps unix.Getpriority.
// Required by cmd/nice to read the current priority before adjustment.
func Getpriority(which, who int) (int, error) {
	prio, err := unix.Getpriority(which, who)
	if err != nil {
		return 0, fmt.Errorf("getpriority(%d, %d): %w", which, who, err)
	}
	// unix.Getpriority returns 20 - actual_priority on Linux to avoid
	// ambiguity with -1 error return. The unix package handles this
	// internally and returns the actual priority value.
	return prio, nil
}

// Setpriority sets the scheduling priority for the target identified by
// which (PrioProcess, PrioPgrp, or PrioUser) and who (PID, PGID, or UID;
// 0 means the calling process/group/user). prio is the new priority value
// (typically -20 to 19). Wraps unix.Setpriority.
// Required by cmd/nice and cmd/renice to adjust process priority.
func Setpriority(which, who, prio int) error {
	if err := unix.Setpriority(which, who, prio); err != nil {
		return fmt.Errorf("setpriority(%d, %d, %d): %w", which, who, prio, err)
	}
	return nil
}

// TODO: Process group management (Setpgid, Killpg, Getpgid) requested by task R1
// but listed in srd002-sys non_goals: "pkg/sys does not provide process-group or
// terminal control (TIOCGPGRP, tcsetpgrp); those are deferred to the xargs SRD."
// Per constitution E6, skipping implementation. See srd002-sys non_goals.
