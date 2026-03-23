// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R2.6: find predicates (-type, -size, -newer, -perm,
// -uid, -gid) require Mode, Size, ModTime, Uid, Gid, Nlink from the stat
// struct. These fields are provided by the FileInfo struct in fileinfo.go
// via Stat and Lstat.

package sys

import (
	"fmt"
	"syscall"
)

// Getpriority returns the scheduling priority of a process, process group,
// or user. The which parameter selects the target type:
// syscall.PRIO_PROCESS, syscall.PRIO_PGRP, or syscall.PRIO_USER.
// The who parameter identifies the target (0 means the calling process).
//
// R3.3: complements Setpriority for reading the current nice value.
// Used by nice to read the baseline priority before adjustment.
func Getpriority(which, who int) (int, error) {
	prio, err := syscall.Getpriority(which, who)
	if err != nil {
		return 0, fmt.Errorf("getpriority(%d, %d): %w", which, who, err)
	}
	return prio, nil
}
