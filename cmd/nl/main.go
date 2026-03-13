// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nl implements the nl (number lines) command.
// Implements: prd022-nl R1.1, R1.2, R1.3, R1.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.1: Default formatting constants per prd022.
const (
	defaultWidth     = 6
	defaultSeparator = "\t"
	defaultIncrement = 1
	defaultStartNum  = 1
)

func main() {
	// D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	exitCode := 0
	lineNum := defaultStartNum

	if len(args) == 0 {
		// R1.3: No file arguments — read from stdin.
		if err := nlReader(os.Stdin, &lineNum); err != nil {
			fmt.Fprintf(os.Stderr, "nl: %v\n", err)
			os.Exit(1)
		}
	} else {
		// R1.3, R1.4: Process each file in argument order with continuous numbering.
		for _, arg := range args {
			if err := nlFile(arg, &lineNum); err != nil {
				fmt.Fprintf(os.Stderr, "nl: %v\n", err)
				exitCode = 1
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// nlFile opens name and numbers its lines to stdout.
// R1.3: "-" reads from stdin.
func nlFile(name string, lineNum *int) error {
	if name == "-" {
		return nlReader(os.Stdin, lineNum)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return nlReader(f, lineNum)
}

// nlReader reads lines from r and writes them to stdout with line numbering.
//
// R1.1: Non-empty lines are numbered with right-justified numbers in a field
// of width 6, separated from content by a tab.
// R1.2: Empty lines pass through with padding but no number.
// R1.4: lineNum is shared across files for continuous numbering.
func nlReader(r io.Reader, lineNum *int) error {
	// R1.2: Unnumbered lines get width + len(separator) spaces of padding.
	emptyPrefix := strings.Repeat(" ", defaultWidth+len(defaultSeparator))

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		var err error
		if line == "" {
			// R1.2: Empty line — pass through with padding but no number.
			_, err = fmt.Fprintf(os.Stdout, "%s\n", emptyPrefix)
		} else {
			// R1.1: Number non-empty lines with default format.
			_, err = fmt.Fprintf(os.Stdout, "%*d%s%s\n", defaultWidth, *lineNum, defaultSeparator, line)
			*lineNum += defaultIncrement
		}
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	return nil
}
