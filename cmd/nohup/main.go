// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nohup: run a command immune to hangups.
// Implements srd095-nohup R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "nohup"

const (
	exitInternal = 125
	exitNotExec  = 126
	exitNotFound = 127
)

// TODO: R2 from task skipped — srd095-nohup non_goals states
// "cmd/nohup does not implement --help or --version beyond usage on error."
// Per E6, --help and --version flags are not implemented.

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the nohup command logic. It accepts the command and its
// arguments and returns an exit code.
// R1.1-R1.4, R2.1-R2.3: stub — returns 0.
func run(command []string) int {
	return 0
}

// Ensure imports are used. These variables exist only to satisfy
// the compiler for the contract stub and will be removed when
// the implementation is filled in.
var (
	_ = fmt.Sprintf
	_ = exec.Command
	_ = signal.Ignore
	_ = syscall.SIGHUP
)
