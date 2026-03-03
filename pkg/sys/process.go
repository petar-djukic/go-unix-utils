// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements process group management and priority wrappers.
// Architecture: docs/ARCHITECTURE.yaml (pkg/sys/ capabilities: setpgid, killpg, setpriority)

package sys

import "syscall"

// Setpgid sets the process group ID of the process identified by pid to pgid.
// If pid is 0, the calling process is used. If pgid is 0, the process's own
// pid is used as the group ID. Required by xargs -P for process group
// management. (ARCHITECTURE pkg/sys/ capabilities)
func Setpgid(pid, pgid int) error {
	return syscall.Setpgid(pid, pgid)
}

// Killpg sends sig to all processes in process group pgid. Implemented via
// syscall.Kill with a negative pid, which is the POSIX equivalent of killpg.
// Required by xargs -P to terminate child process groups.
// (ARCHITECTURE pkg/sys/ capabilities)
func Killpg(pgid int, sig syscall.Signal) error {
	return syscall.Kill(-pgid, sig)
}

// Setpriority sets the scheduling priority of the process, process group, or
// user identified by which and who to prio. which must be one of
// syscall.PRIO_PROCESS, syscall.PRIO_PGRP, or syscall.PRIO_USER.
// Required by nice and renice. (ARCHITECTURE pkg/sys/ capabilities)
func Setpriority(which, who, prio int) error {
	return syscall.Setpriority(which, who, prio)
}
