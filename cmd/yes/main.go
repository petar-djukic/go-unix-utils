// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU yes: repeatedly output a string.
// Implements prd012-yes R1-R4.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// minBufSize is the minimum output buffer size per prd012-yes R2.1.
const minBufSize = 8192

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3, R4: Handle --help, --version, and -- separator.
	if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Println("Usage: yes [STRING]...")
			fmt.Println("Repeatedly output a line with all specified STRING(s), or 'y'.")
			os.Exit(0)
		case "--version":
			fmt.Println("yes (go-unix-utils) dev")
			os.Exit(0)
		case "--":
			args = args[1:]
		}
	}

	// R1.1, R1.2: Build the output line.
	line := "y"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}
	line += "\n"

	// R2.1: Buffer output to reduce syscall overhead.
	w := bufio.NewWriterSize(os.Stdout, minBufSize)
	for {
		_, err := w.WriteString(line)
		if err != nil {
			// R2.2: best-effort flush before exit on write error.
			_ = w.Flush() // best-effort, pipe may already be closed
			os.Exit(1)
		}
	}
}
