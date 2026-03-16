// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd024-expand R1.1-R1.4: cmd/expand converts tab characters to
// the appropriate number of spaces to reach the next tab stop (default every
// 8 columns). Reads from files listed as arguments or stdin when no files are
// given. Treats '-' as stdin. Installs SIGPIPE handler for clean exit on
// broken pipe.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU expand format.
const progName = "expand"

// defaultTabStop is the default tab stop interval in columns (R1.1).
const defaultTabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var files []string

	for i, arg := range args {
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		files = append(files, arg)
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		// R1.1: no file arguments — read from stdin.
		if err := expandReader(os.Stdin, w); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		// R1.1: process each file in argument order.
		for _, name := range files {
			if name == "-" {
				// R1.4: '-' means read from stdin.
				if err := expandReader(os.Stdin, w); err != nil {
					fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
					exitCode = 1
				}
				continue
			}

			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				exitCode = 1
				continue
			}
			if err := expandReader(f, w); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
				exitCode = 1
			}
			f.Close() // best-effort close; read errors already reported
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// expandReader reads from r and writes tab-expanded output to w.
// R1.1: tabs are replaced by spaces to reach the next multiple-of-8 column.
// R1.2: consecutive tabs each advance to the next tab stop independently.
// R1.3: non-tab characters are written unchanged; each byte counts as one column.
// R1.4: newline resets the column position to 0 (0-indexed internally).
func expandReader(r io.Reader, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	col := 0

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch b {
		case '\t':
			// R1.1, R1.2: replace tab with spaces to next tab stop.
			spaces := defaultTabStop - (col % defaultTabStop)
			for range spaces {
				if err := w.WriteByte(' '); err != nil {
					return err
				}
			}
			col += spaces
		case '\n':
			// R1.4: newline resets column position.
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			col = 0
		default:
			// R1.3: non-tab characters pass through unchanged.
			if err := w.WriteByte(b); err != nil {
				return err
			}
			col++
		}
	}
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
