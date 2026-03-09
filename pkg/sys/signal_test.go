// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import "testing"

func TestInstallSIGPIPEHandler(t *testing.T) {
	t.Parallel()

	// AC5: InstallSIGPIPEHandler installs without panic.
	// We cannot fully test exit behavior in-process, but we verify it
	// does not panic and can be called multiple times (R1.6).
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler() // second call must not panic
}
