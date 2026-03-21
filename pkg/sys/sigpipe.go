// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd002-sys R1.5–R1.7: InstallSIGPIPEHandler for GNU coreutils
// compatible broken-pipe behavior.
package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that causes the process
// to exit 0 when stdout or stderr is closed by a downstream consumer,
// matching GNU coreutils behavior. Uses signal.Notify (not signal.Ignore)
// so that deferred cleanup functions do not run on SIGPIPE.
// Safe to call multiple times; each invocation starts one goroutine.
// Implements prd002-sys R1.5–R1.6.
func InstallSIGPIPEHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGPIPE)
	go func() {
		<-ch
		os.Exit(0)
	}()
}
