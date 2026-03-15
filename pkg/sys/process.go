// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements process group management functions for pkg/sys.
// Setpgid wraps the setpgid syscall and Killpg sends signals to process groups.

package sys

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// Setpgid sets the process group ID of the process specified by pid to pgid.
// If pid is 0, the calling process's PID is used. If pgid is 0, the process
// specified by pid becomes a process group leader (its pgid is set to its pid).
//
// This wraps the setpgid(2) syscall via golang.org/x/sys/unix, which abstracts
// platform differences between Darwin and Linux.
func Setpgid(pid, pgid int) error {
	if err := unix.Setpgid(pid, pgid); err != nil {
		return fmt.Errorf("setpgid(%d, %d): %w", pid, pgid, err)
	}
	return nil
}

// Killpg sends the signal sig to all processes in the process group pgid.
// pgid must be greater than 0. Signal 0 can be used to check whether the
// process group exists without sending an actual signal.
//
// This wraps kill(2) with a negated pgid via golang.org/x/sys/unix, which is
// the POSIX-standard way to send a signal to a process group.
func Killpg(pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("killpg(%d, %v): pgid must be positive", pgid, sig)
	}
	if err := unix.Kill(-pgid, sig); err != nil {
		return fmt.Errorf("killpg(%d, %v): %w", pgid, sig, err)
	}
	return nil
}
