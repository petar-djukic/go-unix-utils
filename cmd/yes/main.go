// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd012-yes R1.1–R1.4
package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	// R1.4, D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R1.1: default output is "y".
	line := "y"

	// R1.2: join arguments with spaces when provided.
	// R1.3: consume "--" separator so flag-like args can be passed.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}

	// D2: buffer output for efficient repeated writes.
	w := bufio.NewWriter(os.Stdout)

	// R1.1, R1.2: write line repeatedly until write error or SIGPIPE.
	for {
		_, err := w.WriteString(line)
		if err != nil {
			// D4: exit 0 on broken pipe (SIGPIPE handled by signal handler).
			break
		}
		err = w.WriteByte('\n')
		if err != nil {
			break
		}
		// R2.2 (prd012): flush so partial buffer reaches pipe reader.
		if err := w.Flush(); err != nil {
			break
		}
	}
}
