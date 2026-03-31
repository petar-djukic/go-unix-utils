// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// InstallSIGPIPEHandler installs a SIGPIPE handler that exits 0 when
// stdout or stderr is closed by a downstream consumer.
//
// R2.1 (prd002): uses signal.Notify with a buffered channel; exits via os.Exit(0).
// Safe to call multiple times; each call adds a listener on the same signal.
func InstallSIGPIPEHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGPIPE)
	go func() {
		<-ch
		os.Exit(0)
	}()
}
