// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the yes utility that repeatedly outputs a string.
//
// Implements prd012-yes: core output behavior (R1), output buffering (R2),
// exit codes and signal handling (R3).
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const version = "yes (go-unix-utils) 1.0"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: Handle "--" separator.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	} else if len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprintln(os.Stdout, "Usage: yes [STRING]...\n  or:  yes OPTION\nRepeatedly output a line with all specified STRING(s), or 'y'.\n\n      --help     display this help and exit\n      --version  output version information and exit")
			os.Exit(0)
		case "--version":
			fmt.Fprintln(os.Stdout, version)
			os.Exit(0)
		}
	}

	// R1.1, R1.2: Determine the output line.
	line := "y"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}
	line += "\n"

	// R2.1: Buffer output to avoid per-line syscalls.
	w := bufio.NewWriterSize(os.Stdout, 8192)
	for {
		_, err := w.WriteString(line)
		if err != nil {
			// R2.2: Flush before exiting.
			// best-effort flush; ignoring error since we're already failing
			w.Flush()
			os.Exit(1)
		}
		// Flush when the buffer is sufficiently full.
		if w.Available() < len(line) {
			if err := w.Flush(); err != nil {
				os.Exit(1)
			}
		}
	}
}
