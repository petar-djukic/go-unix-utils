// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl R1.1-R1.4: cmd/nl numbers lines from stdin or named
// files. By default, numbers non-empty body lines with right-justified width 6
// and a tab separator. Empty lines pass through unnumbered. Line numbering is
// continuous across multiple files. Installs SIGPIPE handler for clean exit on
// broken pipe.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU nl format.
const progName = "nl"

// defaultWidth is the line number field width (R1.1).
const defaultWidth = 6

// defaultSep is the separator between line number and content (R1.1).
const defaultSep = "\t"

func main() {
	sys.InstallSIGPIPEHandler()

	files := os.Args[1:]
	exitCode := 0
	lineNum := 1

	if len(files) == 0 {
		// R1.3: no file arguments — read from stdin.
		var err error
		lineNum, err = nlReader(os.Stdin, lineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
		os.Exit(exitCode)
	}

	// R1.4: process each file in argument order with continuous numbering.
	for _, name := range files {
		var err error
		if name == "-" {
			// R1.3: "-" means read from stdin.
			lineNum, err = nlReader(os.Stdin, lineNum)
			if err != nil {
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
		lineNum, err = nlReader(f, lineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// emptyPadding is the whitespace prefix for unnumbered lines: width + len(separator)
// spaces, matching GNU nl's behavior of replacing the number+separator with spaces.
var emptyPadding = strings.Repeat(" ", defaultWidth+len(defaultSep))

// nlReader reads lines from r, numbering non-empty lines starting at lineNum.
// Returns the next line number to use and any read/write error.
//
// R1.1: non-empty lines are numbered with right-justified width 6, tab separator.
// R1.2: empty lines are output with whitespace padding but no number.
func nlReader(r io.Reader, lineNum int) (int, error) {
	w := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// R1.2: empty line — output padding but no number.
			if _, err := fmt.Fprintf(w, "%s\n", emptyPadding); err != nil {
				return lineNum, err
			}
		} else {
			// R1.1: number non-empty lines.
			if _, err := fmt.Fprintf(w, "%*d%s%s\n", defaultWidth, lineNum, defaultSep, line); err != nil {
				return lineNum, err
			}
			lineNum++
		}
	}

	if err := scanner.Err(); err != nil {
		return lineNum, err
	}

	if err := w.Flush(); err != nil {
		return lineNum, err
	}

	return lineNum, nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
