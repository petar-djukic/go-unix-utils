// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd004-ts R1.1, R1.2, R1.3, R1.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultFormat is the strftime-equivalent Go time format for "%b %d %H:%M:%S".
// R1.2: The default timestamp format is evaluated at the time each line is received.
const defaultFormat = "Jan 02 15:04:05"

func main() {
	// R1.3 (via SIGPIPE): install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// R1.1: Read stdin line by line, prepend timestamp, write to stdout.
	w := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// R1.2: Evaluate timestamp at the time each line is received.
		ts := time.Now().Format(defaultFormat)
		line := scanner.Text()
		// R1.1: Each line is prefixed by timestamp and a single space.
		// R1.4: Preserve the original newline; scanner strips it, we add it back.
		if _, err := fmt.Fprintf(w, "%s %s\n", ts, line); err != nil {
			fmt.Fprintf(os.Stderr, "ts: write error: %v\n", err)
			os.Exit(1)
		}
		// R1.3: Flush stdout after each line for real-time downstream consumption.
		if err := w.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "ts: write error: %v\n", err)
			os.Exit(1)
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "ts: read error: %v\n", err)
		os.Exit(1)
	}

	// R1.4: Exit 0 on successful completion.
}
