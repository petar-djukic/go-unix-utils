// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Process group management functions wrapping POSIX setpgid, killpg, getpgid,
// and setsid syscalls. Used by cmd/xargs for -P parallel process groups and
// by cmd/timeout for process group signal delivery.
// Implements prd002-sys R2.1-R2.4 (process group management).
package sys

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Setpgid sets the process group ID of the process specified by pid to pgid.
// If pid is 0, the calling process's PID is used. If pgid is 0, the process
// specified by pid becomes the process group leader. (prd002-sys R2.1)
func Setpgid(pid, pgid int) error {
	if err := unix.Setpgid(pid, pgid); err != nil {
		return fmt.Errorf("setpgid(%d, %d): %w", pid, pgid, err)
	}
	return nil
}

// Killpg sends signal sig to all processes in the process group pgid.
// Wraps kill(-pgid, sig) per POSIX convention. (prd002-sys R2.2)
func Killpg(pgid int, sig unix.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("killpg: invalid process group ID %d", pgid)
	}
	if err := unix.Kill(-pgid, sig); err != nil {
		return fmt.Errorf("killpg(%d, %v): %w", pgid, sig, err)
	}
	return nil
}

// Getpgid returns the process group ID of the process specified by pid.
// If pid is 0, the calling process's process group ID is returned.
// (prd002-sys R2.3)
func Getpgid(pid int) (int, error) {
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return 0, fmt.Errorf("getpgid(%d): %w", pid, err)
	}
	return pgid, nil
}

// Setsid creates a new session with the calling process as the session leader
// and process group leader. Returns the new session ID. The calling process
// must not already be a process group leader. (prd002-sys R2.4)
func Setsid() (int, error) {
	sid, err := unix.Setsid()
	if err != nil {
		return 0, fmt.Errorf("setsid: %w", err)
	}
	return sid, nil
}
