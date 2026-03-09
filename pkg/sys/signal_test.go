// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/exec"
	"testing"
)

// TestInstallSIGPIPEHandler verifies that a process with the SIGPIPE handler
// exits 0 when writing to a broken pipe, rather than crashing or returning
// a write error exit code.
// AC4: InstallSIGPIPEHandler causes exit 0 on SIGPIPE.
func TestInstallSIGPIPEHandler(t *testing.T) {
	t.Parallel()

	if os.Getenv("TEST_SIGPIPE_SUBPROCESS") == "1" {
		// Subprocess: install the handler and write to stdout (which is a broken pipe).
		InstallSIGPIPEHandler()
		// Write enough to trigger SIGPIPE. The pipe reader has already closed.
		for range 1000 {
			_, _ = os.Stdout.WriteString("output line\n") // best-effort write to trigger SIGPIPE
		}
		// If we reach here, SIGPIPE wasn't delivered; exit with non-zero.
		os.Exit(1)
	}

	// Parent: run ourselves as a subprocess with the pipe closed immediately.
	cmd := exec.Command(os.Args[0], "-test.run=^TestInstallSIGPIPEHandler$")
	cmd.Env = append(os.Environ(), "TEST_SIGPIPE_SUBPROCESS=1")

	// Create a pipe, then close the read end before the subprocess writes.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	cmd.Stdout = w

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting subprocess: %v", err)
	}

	// Close both ends in the parent: close read end to trigger SIGPIPE in child,
	// close write end since the parent doesn't use it.
	r.Close()
	w.Close()

	err = cmd.Wait()
	if err != nil {
		t.Errorf("subprocess exited with error: %v; want exit 0", err)
	}
}
