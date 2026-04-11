// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements pee (moreutils), which tees stdin to multiple pipe
// commands. Implements srd113-pee R1.1-R1.3, R2.1-R2.2.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run reads all stdin, launches each command, writes the buffer to each,
// waits for all to complete, and returns the exit code.
// R1.1: read stdin, write to each command via sh -c.
// R1.2: wait for all commands.
// R2.1: exit 0 if all succeed, 1 if any fail.
func run(commands []string) int {
	if len(commands) == 0 {
		return 0
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pee: read stdin: %v\n", err)
		return 1
	}
	procs, ok := startAll(commands)
	if !ok {
		return 1
	}
	writeAll(procs, input)
	return waitAll(procs)
}

// proc holds a running child process and its stdin pipe.
type proc struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

// startAll launches all commands via sh -c. Returns the process list and
// true if all started successfully.
// R1.1: each command is executed via sh -c COMMAND.
// R1.3: stdout of each command goes to os.Stdout.
func startAll(commands []string) ([]proc, bool) {
	procs := make([]proc, 0, len(commands))
	for _, c := range commands {
		p, err := startOne(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pee: %v\n", err)
			cleanupStarted(procs)
			return nil, false
		}
		procs = append(procs, p)
	}
	return procs, true
}

// startOne launches a single shell command and returns its proc.
func startOne(command string) (proc, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return proc{}, fmt.Errorf("pipe for %q: %w", command, err)
	}
	if err := cmd.Start(); err != nil {
		return proc{}, fmt.Errorf("start %q: %w", command, err)
	}
	return proc{cmd: cmd, stdin: stdin}, nil
}

// cleanupStarted closes stdin pipes and waits for already-started procs.
func cleanupStarted(procs []proc) {
	for _, p := range procs {
		p.stdin.Close() // best-effort close
		p.cmd.Wait()    // best-effort wait
	}
}

// writeAll writes the buffered input to each command's stdin and closes
// the pipe. R1.1: write full buffer to each command's stdin.
func writeAll(procs []proc, input []byte) {
	for _, p := range procs {
		p.stdin.Write(input) // best-effort write; command may have exited
		p.stdin.Close()
	}
}

// waitAll waits for all processes to complete and returns 0 if all
// succeeded, 1 if any failed. R1.2, R2.1.
func waitAll(procs []proc) int {
	exitCode := 0
	for _, p := range procs {
		if err := p.cmd.Wait(); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}
