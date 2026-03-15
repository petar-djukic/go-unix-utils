// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements process group management (setpgid, killpg) and process priority
// (getpriority, setpriority) syscall wrappers for pkg/sys.
// Referenced by ARCHITECTURE.yaml pkg/sys capabilities for timeout and xargs -P
// (process groups) and nice/renice (process priority).

package sys

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// Setpgid sets the process group ID of the process specified by pid.
// If pid is 0, the calling process's PID is used. If pgid is 0, the process
// specified by pid becomes a process group leader (pgid set to its own pid).
//
// Used by timeout and xargs -P to create process groups for signal delivery.
func Setpgid(pid, pgid int) error {
	if err := unix.Setpgid(pid, pgid); err != nil {
		return fmt.Errorf("sys.Setpgid(%d, %d): %w", pid, pgid, err)
	}
	return nil
}

// Killpg sends signal sig to all processes in the process group pgid.
// pgid must be positive. This is equivalent to kill(-pgid, sig).
//
// Used by timeout to signal the entire child process group on expiry.
func Killpg(pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("sys.Killpg: pgid must be positive, got %d", pgid)
	}
	if err := unix.Kill(-pgid, sig); err != nil {
		return fmt.Errorf("sys.Killpg(%d, %v): %w", pgid, sig, err)
	}
	return nil
}

// PrioProcess, PrioPGrp, and PrioUser identify the target type for
// Getpriority and Setpriority.
const (
	PrioProcess = unix.PRIO_PROCESS
	PrioPGrp    = unix.PRIO_PGRP
	PrioUser    = unix.PRIO_USER
)

// Getpriority returns the scheduling priority of the process, process group,
// or user specified by which and who. which must be one of PrioProcess,
// PrioPGrp, or PrioUser. If who is 0, it refers to the calling process,
// process group, or user respectively.
//
// Used by nice and renice to query current process priority.
func Getpriority(which, who int) (int, error) {
	prio, err := unix.Getpriority(which, who)
	if err != nil {
		return 0, fmt.Errorf("sys.Getpriority(%d, %d): %w", which, who, err)
	}
	return prio, nil
}

// Setpriority sets the scheduling priority of the process, process group,
// or user specified by which and who. which must be one of PrioProcess,
// PrioPGrp, or PrioUser. If who is 0, it refers to the calling process,
// process group, or user respectively. prio ranges from -20 (highest) to
// 19 (lowest).
//
// Used by nice and renice to adjust process priority.
func Setpriority(which, who, prio int) error {
	if err := unix.Setpriority(which, who, prio); err != nil {
		return fmt.Errorf("sys.Setpriority(%d, %d, %d): %w", which, who, prio, err)
	}
	return nil
}
