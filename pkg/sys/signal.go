// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.5-R1.7: InstallSIGPIPEHandler function.

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process to
// exit 0 when stdout or stderr is closed by a downstream consumer, matching
// GNU coreutils behavior.
//
// R1.5: uses signal.Notify with a buffered channel of size 1; a dedicated
// goroutine calls os.Exit(0) when the channel receives. Does not use
// signal.Ignore so that deferred cleanup functions do not run on SIGPIPE.
// R1.6: safe to call multiple times; each invocation starts one goroutine.
func InstallSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}
