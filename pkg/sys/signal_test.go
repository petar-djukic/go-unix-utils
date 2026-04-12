// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/exec"
	"testing"
)

// TestInstallSIGPIPEHandlerIdempotent verifies that calling
// InstallSIGPIPEHandler multiple times does not panic.
// R1.6: safe to call multiple times.
func TestInstallSIGPIPEHandlerIdempotent(t *testing.T) {
	t.Parallel()
	// Run in a subprocess so we don't affect the test process signal state.
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperSIGPIPEIdempotent")
	cmd.Env = append(os.Environ(), "GO_TEST_SIGPIPE_IDEMPOTENT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess failed: %v\n%s", err, out)
	}
}

// TestHelperSIGPIPEIdempotent is a helper test that runs in a subprocess.
// It calls InstallSIGPIPEHandler three times to verify no panic occurs.
func TestHelperSIGPIPEIdempotent(t *testing.T) {
	if os.Getenv("GO_TEST_SIGPIPE_IDEMPOTENT") != "1" {
		t.Skip("helper test only runs as subprocess")
	}
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
}

// TestInstallSIGPIPEHandlerExitsOnSIGPIPE verifies that a process with the
// SIGPIPE handler installed exits 0 when it writes to a broken pipe.
// R1.5: handler causes os.Exit(0) on SIGPIPE.
func TestInstallSIGPIPEHandlerExitsOnSIGPIPE(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperSIGPIPEExit")
	cmd.Env = append(os.Environ(), "GO_TEST_SIGPIPE_EXIT=1")
	// Connect stdout to a pipe, then close the read end before the
	// subprocess writes. This causes SIGPIPE when the subprocess writes.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	cmd.Stdout = w
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Close both ends in the parent. The subprocess holds the write end
	// via its stdout fd. Closing r triggers SIGPIPE on next write.
	r.Close()
	w.Close()
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

// TestHelperSIGPIPEExit is a helper test that installs the SIGPIPE handler
// and writes to stdout (which is a broken pipe). The handler should cause
// os.Exit(0).
func TestHelperSIGPIPEExit(t *testing.T) {
	if os.Getenv("GO_TEST_SIGPIPE_EXIT") != "1" {
		t.Skip("helper test only runs as subprocess")
	}
	InstallSIGPIPEHandler()
	// Write to stdout, which is a broken pipe. This triggers SIGPIPE.
	os.Stdout.Write([]byte("trigger SIGPIPE\n"))
	// If the handler worked, os.Exit(0) was called and we never reach here.
	// Write again to ensure the signal is delivered.
	os.Stdout.Write([]byte("second write\n"))
}
