// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys_test

import (
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func TestInstallSIGPIPEHandler_NoPanic(t *testing.T) {
	t.Parallel()
	// Verify InstallSIGPIPEHandler is callable without panic. The actual
	// broken-pipe exit behavior requires a subprocess test (the handler
	// calls os.Exit which would terminate the test process).
	sys.InstallSIGPIPEHandler()
}

func TestInstallSIGPIPEHandler_MultipleCalls(t *testing.T) {
	t.Parallel()
	// R1.6: safe to call multiple times.
	sys.InstallSIGPIPEHandler()
	sys.InstallSIGPIPEHandler()
	sys.InstallSIGPIPEHandler()
}
