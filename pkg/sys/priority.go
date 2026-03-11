// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys R3.3 (process priority management for nice and renice)
package sys

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// SetPriority sets the scheduling priority of the process, process group, or user
// identified by which and who to prio. which must be one of syscall.PRIO_PROCESS,
// syscall.PRIO_PGRP, or syscall.PRIO_USER. Lower numeric values mean higher
// scheduling priority (range is typically -20 to 19).
//
// Intended for nice and renice, which adjust the nice value of running processes.
//
// R3.3: Process priority management via setpriority(2).
func SetPriority(which, who, prio int) error {
	if err := unix.Setpriority(which, who, prio); err != nil {
		return fmt.Errorf("setpriority(%d, %d, %d): %w", which, who, prio, err)
	}
	return nil
}
