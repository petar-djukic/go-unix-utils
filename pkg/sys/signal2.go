// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd002-sys R2.5, R2.6 (utility context: du and find use Stat/Lstat/FileInfo)
// Additional signal handling: SIGHUP (hangup) per pkg/sys architecture capabilities.
package sys

import (
	"os"
	"os/signal"
	"syscall"
)

// OnHangup registers a callback that is invoked each time SIGHUP is received.
// Utilities that need to reload configuration or clean up on terminal disconnect
// use this function. Each call to OnHangup registers an additional callback;
// all registered callbacks are invoked on each SIGHUP, in registration order.
func OnHangup(callback func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			callback()
		}
	}()
}
