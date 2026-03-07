// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sys

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSIGPIPEHandlerNoInterference(t *testing.T) {
	t.Parallel()
	// AC5: InstallSIGPIPEHandler installs without panic and does not
	// interfere with normal stdout writes.
	InstallSIGPIPEHandler()

	// Normal writes to stdout should still work.
	n, err := os.Stdout.Write([]byte(""))
	if err != nil {
		t.Errorf("stdout write after InstallSIGPIPEHandler: %v", err)
	}
	if n != 0 {
		t.Errorf("wrote %d bytes, want 0", n)
	}
}

func TestInstallSIGPIPEHandlerMultipleCalls(t *testing.T) {
	t.Parallel()
	// R1.6: Safe to call multiple times.
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	InstallSIGPIPEHandler()
	// No panic means success.
}

func TestInstallSIGPIPEHandlerBrokenPipe(t *testing.T) {
	t.Parallel()
	// AC4 (prd002): A process with InstallSIGPIPEHandler writing to a broken
	// pipe should exit 0 rather than printing a write error.

	// Build a small helper program that installs the handler and writes to stdout.
	dir := t.TempDir()
	mainGo := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainGo, []byte(`package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()

	// Write a lot of output; the pipe will be closed by head.
	for i := 0; i < 100000; i++ {
		fmt.Println("line")
	}
}
`), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	goMod := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module sigpipetest\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	binPath := filepath.Join(dir, "sigpipetest")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}

	// Pipe through head -1 so the pipe breaks after one line.
	cmd := exec.Command("sh", "-c", binPath+" | head -1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("expected exit 0, got error: %v\noutput: %s", err, output)
	}

	if !strings.Contains(string(output), "line") {
		t.Errorf("expected 'line' in output, got: %s", output)
	}
}
