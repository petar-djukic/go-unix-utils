// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/yes: repeatedly output a string.
// Implements srd012-yes R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultOutput is the string printed when no arguments are given.
// R1.1: output "y" followed by a newline.
const defaultOutput = "y"

func main() {
	// R1.4, R3.1: install SIGPIPE handler so yes exits 0 when pipe closes.
	sys.InstallSIGPIPEHandler()

	line := buildLine(os.Args[1:])

	// R2.1: buffer output to avoid per-line write syscalls.
	w := bufio.NewWriter(os.Stdout)

	writeLoop(w, line)
}

// buildLine constructs the output line from command-line arguments.
// R1.1: defaults to "y" with no arguments.
// R1.2: joins arguments with a single space when provided.
// R1.3: arguments after "--" are handled by treating "--" as end-of-flags.
func buildLine(args []string) string {
	// R1.3: skip leading "--" separator to allow arguments starting with "-".
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	if len(args) == 0 {
		return defaultOutput
	}

	return strings.Join(args, " ")
}

// writeLoop repeatedly writes the line to the buffered writer until a write error occurs.
// R1.1, R1.2: output the line followed by a newline, repeatedly.
// R2.2: flush before exiting on write error.
func writeLoop(w *bufio.Writer, line string) {
	for {
		if _, err := w.WriteString(line); err != nil {
			break
		}
		if err := w.WriteByte('\n'); err != nil {
			break
		}
	}

	// R2.2: flush partial buffer before exit.
	_ = w.Flush() // best-effort flush; pipe may already be closed

	// R3.2: exit 1 on write error (SIGPIPE handler catches EPIPE before we get here).
	os.Exit(1)
}
