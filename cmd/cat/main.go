// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat concatenates files and writes to stdout with optional line numbering,
// blank-line squeezing, and non-printing character display.
//
// Implements: prd006-cat R1, R2, R3, R4, R5
// Architecture: docs/ARCHITECTURE.yaml (cmd/ component)
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	programName = "cat"
	usageMsg    = "usage: cat [-benstuvAET] [file ...]\n"
)

// flags holds the parsed command-line options for cat.
type flags struct {
	numberAll      bool // -n: number all output lines (R2.1)
	numberNonBlank bool // -b: number non-empty lines, overrides -n (R2.2, R2.3)
	squeezeBlank   bool // -s: squeeze consecutive blank lines (R3.1)
	showNonPrint   bool // -v: non-printing chars as ^ and M- notation (R4.1)
	showEnds       bool // -E: append $ at each line end (R4.3)
	showTabs       bool // -T: show TAB as ^I (R4.4)
}

// needsLineProcessing reports whether any flag requires line-by-line processing
// rather than a raw io.Copy pass-through. (D3)
func (f *flags) needsLineProcessing() bool {
	return f.numberAll || f.numberNonBlank || f.squeezeBlank ||
		f.showNonPrint || f.showEnds || f.showTabs
}

// parseFlags parses POSIX-style short flags from args. Returns parsed flags,
// remaining file arguments, and an error for unrecognized flags.
func parseFlags(args []string) (flags, []string, error) {
	var f flags
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				f.numberAll = true
			case 'b':
				f.numberNonBlank = true
			case 's':
				f.squeezeBlank = true
			case 'v':
				f.showNonPrint = true
			case 'E':
				f.showEnds = true
			case 'T':
				f.showTabs = true
			case 'A': // equivalent to -vET (R4.5)
				f.showNonPrint = true
				f.showEnds = true
				f.showTabs = true
			case 'e': // equivalent to -vE (R4.6)
				f.showNonPrint = true
				f.showEnds = true
			case 't': // equivalent to -vT (R4.7)
				f.showNonPrint = true
				f.showTabs = true
			case 'u':
				// accepted but has no effect (R4.8)
			default:
				return flags{}, nil, fmt.Errorf("invalid option -- '%c'", ch)
			}
		}
	}

	// -b overrides -n (R2.3)
	if f.numberNonBlank {
		f.numberAll = false
	}

	return f, files, nil
}

// writeByte writes a single byte to w, applying -v and -T transformations.
// Tab is only transformed when showTabs is true; newlines are handled at the
// line level and never passed to this function. (R4.1, R4.2, R4.4)
func writeByte(w *bufio.Writer, b byte, showNonPrint, showTabs bool) {
	if b == '\t' {
		if showTabs {
			w.WriteByte('^')
			w.WriteByte('I')
		} else {
			w.WriteByte(b)
		}
		return
	}

	if !showNonPrint {
		w.WriteByte(b)
		return
	}

	// High-bit characters: prefix with M- and process the low 7 bits. (R4.1)
	if b >= 128 {
		w.WriteByte('M')
		w.WriteByte('-')
		b -= 128
	}

	if b < 32 {
		w.WriteByte('^')
		w.WriteByte(b + 64)
	} else if b == 127 {
		w.WriteByte('^')
		w.WriteByte('?')
	} else {
		w.WriteByte(b)
	}
}

// processLines reads from r line by line and writes to w with transformations
// applied in order per R4.9: squeeze blanks, non-printing display, end-of-line
// marker, line number.
func processLines(r io.Reader, w *bufio.Writer, f *flags, lineNum *int, prevBlank *bool) error {
	br := bufio.NewReader(r)
	for {
		line, readErr := br.ReadBytes('\n')
		if len(line) > 0 {
			hasNewline := line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}

			// A blank line contains only a newline character (R2.4).
			isBlank := hasNewline && len(content) == 0

			// Step 1: squeeze consecutive blank lines to one (R3.1).
			if f.squeezeBlank && isBlank && *prevBlank {
				if readErr != nil {
					if readErr == io.EOF {
						return nil
					}
					return readErr
				}
				continue
			}
			*prevBlank = isBlank

			// Step 4: prepend line number (R2.1, R2.2).
			if f.numberAll || (f.numberNonBlank && !isBlank) {
				fmt.Fprintf(w, "%6d\t", *lineNum)
				*lineNum++
			}

			// Step 2: transform content bytes (-v, -T) (R4.1, R4.4).
			if f.showNonPrint || f.showTabs {
				for _, b := range content {
					writeByte(w, b, f.showNonPrint, f.showTabs)
				}
			} else {
				w.Write(content)
			}

			// Step 3: end-of-line marker (-E) then newline (R4.3).
			if hasNewline {
				if f.showEnds {
					w.WriteByte('$')
				}
				w.WriteByte('\n')
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

// run processes the given arguments and returns an exit code. Separating I/O
// from os.Exit allows unit tests to call run directly without spawning a
// subprocess. (D1)
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	f, files, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		fmt.Fprint(stderr, usageMsg)
		return 1, nil
	}

	// Read from stdin when no file arguments are given (R1.2).
	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(stdout)
	exitCode := 0
	lineNum := 1
	prevBlank := false

	for _, file := range files {
		var r io.Reader
		var fd *os.File

		if file == "-" {
			r = stdin
		} else {
			var openErr error
			fd, openErr = os.Open(file)
			if openErr != nil {
				if pathErr, ok := openErr.(*os.PathError); ok {
					fmt.Fprintf(stderr, "%s: %s: %s\n", programName, file, pathErr.Err)
				} else {
					fmt.Fprintf(stderr, "%s: %s\n", programName, openErr)
				}
				exitCode = 1
				continue // continue processing remaining files (R5.2)
			}
			r = fd
		}

		if !f.needsLineProcessing() {
			_, copyErr := io.Copy(w, r)
			if fd != nil {
				fd.Close() // best-effort close on read-only file descriptor
			}
			if copyErr != nil {
				fmt.Fprintf(stderr, "%s: %s: %v\n", programName, file, copyErr)
				exitCode = 1
			}
			continue
		}

		procErr := processLines(r, w, &f, &lineNum, &prevBlank)
		if fd != nil {
			fd.Close() // best-effort close on read-only file descriptor
		}
		if procErr != nil {
			fmt.Fprintf(stderr, "%s: %s: %v\n", programName, file, procErr)
			exitCode = 1
		}
	}

	if flushErr := w.Flush(); flushErr != nil {
		fmt.Fprintf(stderr, "%s: write error: %v\n", programName, flushErr)
		return 1, nil
	}

	return exitCode, nil
}

func main() {
	sys.InstallSIGPIPEHandler() // Exit 0 on SIGPIPE (R5.4, D2).
	code, _ := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
