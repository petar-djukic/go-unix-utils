// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func TerminalWidth() (int, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, fmt.Errorf("sys: terminal width: %w", err)
	}
	return int(ws.Col), nil
}

func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}
