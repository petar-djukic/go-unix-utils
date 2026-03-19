// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd012-yes R1.1-R1.4: core yes output loop,
// R2.1-R2.2: buffered output, R3.1-R3.2: exit codes and signal handling.
package main

import (
	"bufio"
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// outputBufSize is the minimum buffer size for stdout writes.
// R2.1: at least 8192 bytes to avoid per-line write syscalls.
const outputBufSize = 8192

func main() {
	sys.InstallSIGPIPEHandler()

	line := buildLine(os.Args[1:])
	writeLoop(line)
}

// buildLine constructs the output line from command-line arguments.
// R1.1: no arguments produces "y\n".
// R1.2: arguments are joined by spaces with a trailing newline.
// R1.3: "--" separator is handled by skipping it as the first arg.
// R1.4: no stdin is read.
func buildLine(args []string) string {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return "y\n"
	}
	return strings.Join(args, " ") + "\n"
}

// writeLoop writes the line repeatedly to stdout using buffered I/O.
// R2.1: uses bufio.Writer with outputBufSize buffer for throughput.
// R2.2: flushes buffer before exiting on write error.
// R3.1: exits 0 on EPIPE (stdout closed by pipe consumer).
// R3.2: exits 1 on other write errors.
func writeLoop(line string) {
	w := bufio.NewWriterSize(os.Stdout, outputBufSize)
	for {
		_, err := w.WriteString(line)
		if err != nil {
			_ = w.Flush() // R2.2: best-effort flush of partial buffer
			exitOnWriteError(err)
		}
	}
}

// exitOnWriteError exits 0 for EPIPE (normal pipe close) or 1 for
// other write errors. R3.1, R3.2.
func exitOnWriteError(err error) {
	if errors.Is(err, syscall.EPIPE) {
		os.Exit(0)
	}
	_ = os.Stderr.Close() // best-effort, suppress further output
	os.Exit(1)
}
