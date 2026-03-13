// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd018-head R1.1–R1.5, R3.1–R3.2, R4.1–R4.2
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultLines is the number of lines printed when -n is not specified.
// R1.1: default is 10 lines.
const defaultLines = 10

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	flagN := flag.Int("n", defaultLines, "print the first NUM lines instead of the first 10")
	flag.IntVar(flagN, "lines", defaultLines, "print the first NUM lines instead of the first 10")
	flag.Parse()

	args := flag.Args()
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(args) == 0 {
		// R1.4: no file arguments — read from stdin.
		if err := headLines(w, os.Stdin, *flagN); err != nil {
			fmt.Fprintf(os.Stderr, "head: %v\n", err)
			exitCode = 1
		}
	} else {
		// R3.1, R3.2: print headers when multiple files are given.
		showHeaders := len(args) > 1
		printed := false // tracks whether any file output has been written
		for _, arg := range args {
			r, closer, err := openInput(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "head: %v\n", err)
				exitCode = 1
				continue
			}
			if showHeaders {
				// R3.1: blank line between files (before header of second and subsequent files).
				if printed {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "==> %s <==\n", arg)
			}
			if err := headLines(w, r, *flagN); err != nil {
				fmt.Fprintf(os.Stderr, "head: %v\n", err)
				exitCode = 1
			}
			printed = true
			if closer != nil {
				closer.Close() // best-effort cleanup, error ignored
			}
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "head: write error: %v\n", err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// openInput opens name for reading and returns the reader and an optional closer.
// R1.4: "-" returns stdin with no closer.
func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// headLines reads from r and writes the first n lines to w.
// R1.1, R1.2, R1.3: prints exactly n lines.
// R1.5: a line is terminated by a newline; the last line without a trailing
// newline is still counted.
func headLines(w *bufio.Writer, r io.Reader, n int) error {
	if n <= 0 {
		return nil
	}
	br := bufio.NewReader(r)
	count := 0
	for count < n {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if _, werr := io.WriteString(w, line); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			count++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}
	return nil
}
