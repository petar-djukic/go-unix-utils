// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

// InstallSIGPIPEHandler installs a SIGPIPE handler that exits 0 when
// stdout or stderr is closed by a downstream consumer.
//
// R1.5: uses signal.Notify with a buffered channel; exits via os.Exit(0).
// R1.6: safe to call multiple times.
func InstallSIGPIPEHandler() {
	panic("not implemented")
}
