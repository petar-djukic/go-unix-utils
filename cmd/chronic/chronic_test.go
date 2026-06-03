// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd112-chronic R2.1, R2.2, R2.3.
package main

import (
	"context"
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("chronic")
	if err != nil {
		t.Skip("reference binary chronic not found")
	}
	tests := []testutils.DiffTest{
		{Name: "success_suppressed", Args: []string{"echo", "hello"}, Env: []string{"LC_ALL=C"}, ExitCode: 0},
		{Name: "failure_exit_one", Args: []string{"false"}, Env: []string{"LC_ALL=C"}, ExitCode: 1},
		{Name: "exit_code_passthrough", Args: []string{"sh", "-c", "exit 42"}, Env: []string{"LC_ALL=C"}, ExitCode: 42},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestCommandNotFound(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "nonexistent_cmd_xyz_5632")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 100 {
		t.Fatalf("expected exit 100 for command not found, got: %v", err)
	}
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "sh", "-c", "head -c 1000000 /dev/zero; exit 1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	_ = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("chronic timed out; SIGPIPE handler may not be installed")
	}
}
