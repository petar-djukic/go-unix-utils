// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd023-fold R1.1-R1.4: cmd/fold wraps long lines to a specified
// width. By default, lines are wrapped at 80 columns, with tab characters
// counted as the number of columns to the next tab stop (every 8 columns).
// Reads from stdin when no file arguments are given; treats '-' as stdin.
// Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU fold format.
const progName = "fold"

// defaultWidth is the default line wrap width in columns (R1.1).
const defaultWidth = 80

// tabStop is the tab stop interval for column counting (R2.2).
const tabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()

	width := defaultWidth
	args := os.Args[1:]
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// -w N or -wN: set width.
		if arg == "-w" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", progName)
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, args[i])
				os.Exit(1)
			}
			width = n
			continue
		}
		if strings.HasPrefix(arg, "-w") {
			val := arg[2:]
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, val)
				os.Exit(1)
			}
			width = n
			continue
		}
		if strings.HasPrefix(arg, "--width=") {
			val := arg[len("--width="):]
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, val)
				os.Exit(1)
			}
			width = n
			continue
		}
		if arg == "--width" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", progName)
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, args[i])
				os.Exit(1)
			}
			width = n
			continue
		}

		fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\n", progName, arg[1:])
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		// R1.1: no file arguments — read from stdin.
		if err := foldReader(os.Stdin, w, width); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		// R1.2: process each file in argument order.
		for _, name := range files {
			if name == "-" {
				// R1.4: '-' means read from stdin.
				if err := foldReader(os.Stdin, w, width); err != nil {
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
			if err := foldReader(f, w, width); err != nil {
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

// foldReader reads from r and writes lines wrapped at width columns to w.
// R1.1: wraps at the specified column width.
// R1.2: lines shorter than or equal to width pass through unchanged.
// R1.3: lines longer than width are split by inserting newlines.
// R1.4: final segment retains original trailing newline.
func foldReader(r io.Reader, w *bufio.Writer, width int) error {
	br := bufio.NewReader(r)

	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			hasNewline := line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}

			if err := foldLine(w, content, width); err != nil {
				return err
			}

			// R1.4: preserve the original trailing newline on the final segment.
			if hasNewline {
				if _, err := w.WriteString("\n"); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// foldLine writes content to w, inserting newlines every width columns.
// Tab characters advance the column to the next tab stop (every 8 columns).
// R1.3: wrapping is applied repeatedly until the remaining portion fits.
func foldLine(w *bufio.Writer, content string, width int) error {
	col := 0

	for i := 0; i < len(content); i++ {
		ch := content[i]

		var advance int
		if ch == '\t' {
			// R2.2: tab advances to next tab stop.
			advance = tabStop - (col % tabStop)
		} else if ch == '\b' {
			// Backspace decrements column (GNU fold behavior), but not below 0.
			if col > 0 {
				col--
			}
			if err := w.WriteByte(ch); err != nil {
				return err
			}
			continue
		} else if ch == '\r' {
			// Carriage return resets column to 0.
			col = 0
			if err := w.WriteByte(ch); err != nil {
				return err
			}
			continue
		} else {
			advance = 1
		}

		// Check if this character would exceed the width.
		if col+advance > width {
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
			col = 0
			// Recalculate advance for tab at column 0.
			if ch == '\t' {
				advance = tabStop - (col % tabStop)
			}
		}

		if err := w.WriteByte(ch); err != nil {
			return err
		}
		col += advance
	}

	return nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
