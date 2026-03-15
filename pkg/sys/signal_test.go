// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"testing"
)

func TestInstallSIGPIPEHandler_MultipleCalls(t *testing.T) {
	t.Parallel()

	// R1.6: InstallSIGPIPEHandler must be safe to call multiple times.
	// Verify it does not panic when called repeatedly.
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
}
