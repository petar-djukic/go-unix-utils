// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-cat R1.1–R1.5 (file concatenation, stdin, binary passthrough,
// no newline modification), R2.1–R2.3 (line numbering with -n and -b).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// catFlags holds parsed command-line flags for cat.
type catFlags struct {
	number         bool // -n: number all output lines. R2.1.
	numberNonblank bool // -b: number non-blank lines only. R2.2.
}

func main() {
	sys.InstallSIGPIPEHandler()

	flags, files := parseArgs(os.Args[1:])

	exitCode := 0

	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)

	// state tracks line numbering across files. R2.1: numbering resets to 1 per invocation, not per file.
	state := &catState{lineNum: 1, atLineStart: true}

	for _, name := range files {
		var r io.Reader
		if name == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(name)
			if err != nil {
				// R5.2: report error, continue with remaining files.
				fmt.Fprintf(os.Stderr, "cat: %s: No such file or directory\n", name)
				exitCode = 1
				continue
			}
			defer f.Close() //nolint:gocritic // deferred close in loop is fine for cat; all files closed at exit.
			r = f
		}

		if err := processFile(r, w, flags, state); err != nil {
			// R5.3: write error on stdout.
			fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// catState tracks state across file boundaries for line numbering.
type catState struct {
	lineNum     int  // next line number to emit
	atLineStart bool // true when next byte is the first byte of a new line
}

// processFile reads from r and writes to w, applying flags. R1.1–R1.5, R2.1–R2.3.
func processFile(r io.Reader, w *bufio.Writer, flags catFlags, state *catState) error {
	needsNumbering := flags.number || flags.numberNonblank

	if !needsNumbering {
		// R1.1–R1.5: verbatim copy, no transformation, no newline modification.
		_, err := io.Copy(w, r)
		return err
	}

	// Line-numbering mode: process byte by byte via buffered reader.
	br := bufio.NewReader(r)

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if state.atLineStart {
			isBlank := (b == '\n')

			if flags.numberNonblank && isBlank {
				// R2.2: blank lines get no number and no tab prefix.
				if writeErr := w.WriteByte('\n'); writeErr != nil {
					return writeErr
				}
				// atLineStart remains true for the next line.
				continue
			}

			// R2.1: right-justified in width 6, followed by tab.
			if _, writeErr := fmt.Fprintf(w, "%6d\t", state.lineNum); writeErr != nil {
				return writeErr
			}
			state.lineNum++
			state.atLineStart = false
		}

		if writeErr := w.WriteByte(b); writeErr != nil {
			return writeErr
		}

		if b == '\n' {
			state.atLineStart = true
		}
	}
}

// parseArgs parses cat command-line arguments and returns flags and file names.
// Supports GNU-style long flags (--number, --number-nonblank) and short flags.
func parseArgs(args []string) (catFlags, []string) {
	var flags catFlags
	var files []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || arg == "-" || !isFlag(arg) {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			switch arg {
			case "--number":
				flags.number = true
			case "--number-nonblank":
				flags.numberNonblank = true
			default:
				fmt.Fprintf(os.Stderr, "cat: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}
		// Short flags: can be combined (e.g., -nb).
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				flags.number = true
			case 'b':
				flags.numberNonblank = true
			case 'u':
				// R4.8: accepted but ignored.
			default:
				fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
	}

	// R2.2: -b implies -n but overrides it (blank lines not numbered).
	// R2.3: when both -b and -n are given, -b takes precedence.
	if flags.numberNonblank {
		flags.number = false
	}

	return flags, files
}

// isFlag returns true if the argument looks like a flag (starts with "-" and is not just "-").
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isLongFlag returns true if the argument is a long flag (starts with "--").
func isLongFlag(arg string) bool {
	return len(arg) > 2 && arg[0] == '-' && arg[1] == '-'
}
