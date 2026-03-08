// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process to
// exit 0 when a downstream pipe consumer closes stdin, matching GNU coreutils
// behavior.
// R1.5: uses signal.Notify with a buffered channel; exits via os.Exit(0).
// R1.6: safe to call multiple times; each invocation starts one goroutine.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
