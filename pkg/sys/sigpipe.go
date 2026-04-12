// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// InstallSIGPIPEHandler installs a SIGPIPE handler causing the process to
// exit 0 when stdout or stderr is closed by a downstream consumer.
// Safe to call multiple times; only one goroutine is started. See srd002-sys R1.5, R1.6.
func InstallSIGPIPEHandler() {
	sigpipeOnce.Do(installSIGPIPEHandlerImpl)
}
