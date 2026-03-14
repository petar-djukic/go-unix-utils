// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys R3.1, R3.2 (process group management)
// pkg/sys architecture capabilities: setpgid/killpg for timeout and xargs -P.
package sys

import (
	"fmt"
	"syscall"
)

// SetPGID sets the process group ID of the process identified by pid to pgid.
// If pid is zero the calling process is used. If pgid is zero the process group
// ID is set to the same value as pid, creating a new process group.
//
// Intended for timeout and xargs -P, which need to place child processes into
// separate process groups for reliable signal delivery.
func SetPGID(pid, pgid int) error {
	if err := syscall.Setpgid(pid, pgid); err != nil {
		return fmt.Errorf("setpgid(%d, %d): %w", pid, pgid, err)
	}
	return nil
}

// KillPG sends signal sig to all processes in the process group identified by pgid.
// It calls kill(2) with a negative pid (-pgid), which delivers the signal to every
// process whose process group ID equals pgid.
//
// Intended for timeout and xargs -P, which send SIGTERM or SIGKILL to an entire
// child process group when a deadline is exceeded or the parent is interrupted.
func KillPG(pgid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pgid, sig); err != nil {
		return fmt.Errorf("killpg(%d, %v): %w", pgid, sig, err)
	}
	return nil
}

