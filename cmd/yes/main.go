// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/yes: repeatedly output a string to stdout.
// Implements prd012-yes R1, R2, R3.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	binName = "yes"
	version = "0.1.0"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: handle "--" separator.
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	} else if len(args) > 0 {
		switch args[0] {
		case "--help":
			if _, err := fmt.Printf("Usage: %s [STRING]...\n  or:  %s OPTION\nRepeatedly output a line with all specified STRING(s), or 'y'.\n\n      --help        display this help and exit\n      --version     output version information and exit\n", binName, binName); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			if _, err := fmt.Printf("%s (go-unix-utils) %s\n", binName, version); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1: default to "y" when no arguments given.
	// R1.2: join arguments with space.
	line := "y"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}
	line += "\n"

	// R2.1: buffer output to reduce syscall overhead.
	w := bufio.NewWriterSize(os.Stdout, 8192)
	for {
		_, err := w.WriteString(line)
		if err != nil {
			// R2.2: flush before exiting on write error.
			_ = w.Flush() // best-effort flush
			os.Exit(1)
		}
		// Flush when buffer is near capacity to deliver output promptly.
		if w.Available() < len(line) {
			if err := w.Flush(); err != nil {
				os.Exit(1)
			}
		}
	}
}
