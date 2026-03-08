// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the whoami utility for printing the effective username.
//
// Implements prd042-whoami: default behavior (R1), error handling (R2).
package main

import (
	"fmt"
	"os"
	"os/user"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	for _, arg := range args {
		if arg == "--help" || arg == "--version" {
			continue
		}
		fmt.Fprintf(os.Stderr, "whoami: extra operand '%s'\n", arg)
		os.Exit(1)
	}

	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "whoami: cannot find name for user ID %d\n", os.Geteuid())
		os.Exit(1)
	}

	fmt.Println(u.Username)
}
