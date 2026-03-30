// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pee implements moreutils pee: tee standard input to pipes.
//
// Implements prd113-pee R1.1, R1.2, R1.3.
package main

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses arguments and executes the pee logic. Returns exit code.
// R1.1: each arg is a COMMAND executed via sh -c.
// R1.2: waits for all commands before returning.
// R1.3: stdout of each command goes to stdout, interleaved.
func run(args []string, stdin io.Reader, stdout, stderr *os.File) int {
	if len(args) == 0 {
		return 0
	}
	cmds, pipes, err := startCommands(args, stdout, stderr)
	if err != nil {
		return 1
	}
	distributeInput(stdin, pipes)
	return waitAll(cmds)
}

// startCommands launches each command via sh -c and returns the
// command handles and their stdin pipes.
func startCommands(args []string, stdout, stderr *os.File) ([]*exec.Cmd, []io.WriteCloser, error) {
	cmds := make([]*exec.Cmd, 0, len(args))
	pipes := make([]io.WriteCloser, 0, len(args))
	for _, arg := range args {
		cmd, pipe, err := startOneCommand(arg, stdout, stderr)
		if err != nil {
			closePipes(pipes)
			return nil, nil, err
		}
		cmds = append(cmds, cmd)
		pipes = append(pipes, pipe)
	}
	return cmds, pipes, nil
}

// startOneCommand creates and starts a single sh -c command.
// R1.1: commands are executed via the shell.
// R1.3: stdout goes to the caller's stdout.
func startOneCommand(arg string, stdout, stderr *os.File) (*exec.Cmd, io.WriteCloser, error) {
	cmd := exec.Command("sh", "-c", arg)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		pipe.Close() // best-effort close
		return nil, nil, err
	}
	return cmd, pipe, nil
}

// distributeInput reads from stdin and writes to all command pipes.
// R1.1: input is written to each command in parallel.
func distributeInput(stdin io.Reader, pipes []io.WriteCloser) {
	buf := make([]byte, 32*1024)
	for {
		n, readErr := stdin.Read(buf)
		if n > 0 {
			writeToAll(pipes, buf[:n])
		}
		if readErr != nil {
			break
		}
	}
	closePipes(pipes)
}

// writeToAll writes data to every pipe concurrently.
func writeToAll(pipes []io.WriteCloser, data []byte) {
	var wg sync.WaitGroup
	wg.Add(len(pipes))
	for i := range pipes {
		go func(w io.WriteCloser) {
			defer wg.Done()
			w.Write(data) //nolint:errcheck // best-effort write
		}(pipes[i])
	}
	wg.Wait()
}

// closePipes closes all pipe writers.
func closePipes(pipes []io.WriteCloser) {
	for _, p := range pipes {
		p.Close() // best-effort close
	}
}

// waitAll waits for all commands to finish.
// R1.2: blocks until all commands complete.
// Returns 0 if all exit 0, 1 if any exit non-zero.
func waitAll(cmds []*exec.Cmd) int {
	exitCode := 0
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			exitCode = 1
		}
	}
	return exitCode
}
