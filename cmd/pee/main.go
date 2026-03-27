// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/pee implements the moreutils pee command: tee stdin to pipe commands.
// Implements prd113-pee R1.1, R1.2, R1.3, R2.1, R2.2.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pee: %v\n", err)
		os.Exit(1)
	}

	// R2.2: with no command arguments, read stdin to completion and exit 0.
	if len(os.Args) < 2 {
		os.Exit(0)
	}

	commands := os.Args[1:]
	os.Exit(runCommands(input, commands))
}

// runCommands executes each command via sh -c, piping input to each command's
// stdin in parallel. Returns 0 if all commands succeed, 1 if any fails.
// R1.1: reads stdin and writes to each command in parallel via sh -c.
// R1.2: waits for all commands to complete before returning.
// R1.3: stdout of each command goes to os.Stdout (interleaved).
// R2.1: exits 0 when all commands exit 0, 1 when any exits non-zero.
func runCommands(input []byte, commands []string) int {
	var wg sync.WaitGroup
	errs := make(chan error, len(commands))

	for _, cmdStr := range commands {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			errs <- execCommand(input, c)
		}(cmdStr)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return 1
		}
	}
	return 0
}

// execCommand runs a single command via sh -c, writing input to its stdin.
// R1.1: each command is executed via sh -c COMMAND.
// R1.3: stdout and stderr are inherited from the parent process.
func execCommand(input []byte, cmdStr string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
