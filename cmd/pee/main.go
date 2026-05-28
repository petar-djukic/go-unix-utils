// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd113-pee R1.1, R1.2, R1.3, R2.1, R2.2.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return 0
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pee: %v\n", err)
		return 1
	}

	cmds, pipes := startCommands(args)
	fanOut(pipes, input)
	return waitAll(cmds)
}

func startCommands(args []string) ([]*exec.Cmd, []io.WriteCloser) {
	cmds := make([]*exec.Cmd, 0, len(args))
	pipes := make([]io.WriteCloser, 0, len(args))

	for _, arg := range args {
		cmd := exec.Command("sh", "-c", arg)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		w, err := cmd.StdinPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pee: %v\n", err)
			continue
		}

		if err := cmd.Start(); err != nil {
			w.Close()
			fmt.Fprintf(os.Stderr, "pee: %v\n", err)
			continue
		}

		cmds = append(cmds, cmd)
		pipes = append(pipes, w)
	}

	return cmds, pipes
}

func fanOut(pipes []io.WriteCloser, input []byte) {
	var wg sync.WaitGroup
	wg.Add(len(pipes))
	for _, w := range pipes {
		go func(w io.WriteCloser) {
			defer wg.Done()
			w.Write(input)
			w.Close()
		}(w)
	}
	wg.Wait()
}

func waitAll(cmds []*exec.Cmd) int {
	exitCode := 0
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				if exitErr.ExitCode() != 0 {
					exitCode = 1
				}
			} else {
				exitCode = 1
			}
		}
	}
	return exitCode
}
