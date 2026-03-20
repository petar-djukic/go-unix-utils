// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd004-ts R1.1, R1.2, R1.3, R1.4: default timestamp behavior
// for the ts utility (timestamp stdin lines).
package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultFormat is the Go time layout equivalent of strftime "%b %d %H:%M:%S".
// R1.2: default timestamp format.
const defaultFormat = "Jan 02 15:04:05"

func main() {
	sys.InstallSIGPIPEHandler()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ts: %v\n", err)
		os.Exit(1)
	}
}

// run reads stdin line by line and writes each line prefixed with a timestamp.
// R1.1: line-by-line processing. R1.3: flush after each line.
func run() error {
	writer := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if err := writeLine(writer, line); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// writeLine formats and writes a single timestamped line to the writer.
// R1.2: timestamp evaluated at time each line is received.
// R1.4: preserves original newline, does not add extra.
func writeLine(w *bufio.Writer, line string) error {
	ts := time.Now().Format(defaultFormat)
	// R1.1: prefix with timestamp and single space
	fmt.Fprintf(w, "%s %s\n", ts, line)
	// R1.3: flush after each line
	return w.Flush()
}
