// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys_test

import (
	"os"
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

// TestInstallSIGPIPEHandler_WriteClosedPipe verifies that after installing the
// SIGPIPE handler, writing to a pipe with a closed read end does not panic.
// The handler calls os.Exit(0) so in the test process we cannot observe the
// exit, but we verify no panic occurs from the write attempt. (prd002-sys R1.5, D4)
func TestInstallSIGPIPEHandler_WriteClosedPipe(t *testing.T) {
	t.Parallel()
	sys.InstallSIGPIPEHandler()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	r.Close()

	// Write to pipe with closed read end. This may trigger SIGPIPE (handled
	// by InstallSIGPIPEHandler) or return an error. Either way, no panic.
	_, _ = w.Write([]byte("data")) // best-effort write; error expected
	w.Close()
}
