// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd012-yes R1.1 (print string + newline in infinite loop),
// R1.2 (join multiple args with space), R1.3 (-- separator passthrough),
// R1.4 (must not read stdin), R2.1 (buffered output, at least 8192 bytes),
// R2.2 (flush on write error), R3.1 (SIGPIPE → exit 0),
// R3.2 (exit 1 on non-EPIPE write error).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	// R3.1, ARCHITECTURE shared_protocols: exit 0 on SIGPIPE.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.3: scan for the "--" separator. Everything after it is the output string,
	// preventing misinterpretation of leading-dash values as flags.
	separatorIdx := -1
	for i, a := range args {
		if a == "--" {
			separatorIdx = i
			break
		}
	}

	if separatorIdx >= 0 {
		// After "--": remaining args form the output string; no flag parsing.
		args = args[separatorIdx+1:]
	} else if len(args) > 0 {
		// D2: follow cmd/true and cmd/false --help/--version flag structure.
		switch args[0] {
		case "--help":
			if _, err := fmt.Fprintln(os.Stdout, "Usage: yes [STRING]..."); err != nil {
				os.Exit(1)
			}
			if _, err := fmt.Fprintln(os.Stdout, "Repeatedly output a line with all specified STRING(s), or 'y'."); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		case "--version":
			if _, err := fmt.Fprintf(os.Stdout, "yes %s\n", version); err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// R1.1, R1.2: default to "y"; join multiple args with a single space.
	output := "y"
	if len(args) > 0 {
		output = strings.Join(args, " ")
	}

	// R2.1: buffer output to avoid per-line write syscalls; use at least 8192 bytes.
	w := bufio.NewWriterSize(os.Stdout, 8192)
	line := output + "\n"

	// R1.1: print STRING followed by a newline in an infinite loop.
	for {
		if _, err := w.WriteString(line); err != nil {
			// R2.2: flush buffer before exiting so the last partial buffer reaches the reader.
			_ = w.Flush() // best-effort; original error takes precedence.
			// R3.2: EPIPE means the pipe reader closed normally — exit 0; all other errors exit 1.
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			os.Exit(1)
		}
	}
}
