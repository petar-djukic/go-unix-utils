// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler registers a SIGPIPE handler that exits the process
// with code 0, matching GNU coreutils behavior. All cmd/ utilities call this
// at the start of main() so that piping to head or grep -q does not produce
// a broken-pipe error.
// R1.5: uses signal.Notify with a buffered channel; exits via os.Exit(0).
// R1.6: safe to call multiple times; each call starts one goroutine.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
