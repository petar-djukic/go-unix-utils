// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cat: concatenate and display files.
// Implements: prd006-cat (R1–R5).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is the cat version string matching GNU coreutils format.
const version = "cat (go-unix-utils) 0.1.0"

// catOptions holds the parsed flags for a cat invocation.
type catOptions struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only (overrides -n)
	squeeze        bool // -s: squeeze consecutive blank lines
	showNonPrint   bool // -v: show non-printing characters
	showEnds       bool // -E: show $ at end of lines
	showTabs       bool // -T: show tabs as ^I
}

func main() {
	// D1: install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])
	exitCode := run(opts, files)
	os.Exit(exitCode)
}

// parseArgs parses GNU cat-style flags from args. Returns options and remaining
// file arguments. Handles --, --help, --version, and combined short flags.
func parseArgs(args []string) (catOptions, []string) {
	var opts catOptions
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			flagsDone = true
			continue
		}

		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}

		if arg == "--version" {
			fmt.Println(version)
			os.Exit(0)
		}

		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			for _, ch := range arg[1:] {
				switch ch {
				case 'n':
					opts.numberAll = true
				case 'b':
					opts.numberNonBlank = true
				case 's':
					opts.squeeze = true
				case 'v':
					opts.showNonPrint = true
				case 'E':
					opts.showEnds = true
				case 'T':
					opts.showTabs = true
				case 'A': // R4.5: -A = -vET
					opts.showNonPrint = true
					opts.showEnds = true
					opts.showTabs = true
				case 'e': // R4.6: -e = -vE
					opts.showNonPrint = true
					opts.showEnds = true
				case 't': // R4.7: -t = -vT
					opts.showNonPrint = true
					opts.showTabs = true
				case 'u': // R4.8: accepted, no effect
				default:
					fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}

		files = append(files, arg)
	}

	return opts, files
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

  -A, --show-all           equivalent to -vET
  -b, --number-nonblank    number nonempty output lines, overrides -n
  -e                       equivalent to -vE
  -E, --show-ends          display $ at end of each line
  -n, --number             number all output lines
  -s, --squeeze-blank      suppress repeated empty output lines
  -t                       equivalent to -vT
  -T, --show-tabs          display TAB characters as ^I
  -u                       (ignored)
  -v, --show-nonprinting   use ^ and M- notation, except for LFD and TAB
      --help               display this help and exit
      --version            output version information and exit
`)
}

// run processes all files with the given options. Returns the exit code.
func run(opts catOptions, files []string) int {
	// R1.2: when no files given, read from stdin.
	if len(files) == 0 {
		files = []string{"-"}
	}

	// R2.3: -b overrides -n (blank lines not numbered).
	numberLines := opts.numberAll || opts.numberNonBlank
	skipBlankNumbers := opts.numberNonBlank

	needsTransform := numberLines || opts.squeeze || opts.showNonPrint || opts.showEnds || opts.showTabs

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush on exit

	lineNum := 1
	prevBlank := false
	exitCode := 0

	for _, file := range files {
		ec := processFile(file, w, needsTransform, numberLines, skipBlankNumbers, &opts, &lineNum, &prevBlank)
		if ec < 0 {
			return 1
		}
		if ec > 0 {
			exitCode = 1
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		return 1
	}

	return exitCode
}

// processFile processes a single file. Returns 0 on success, 1 on read error
// (continue processing), and -1 on write error (stop immediately).
func processFile(file string, w *bufio.Writer, needsTransform, numberLines, skipBlankNumbers bool, opts *catOptions, lineNum *int, prevBlank *bool) int {
	var r io.Reader
	if file == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(file)
		if err != nil {
			// R5.2, D3: print error using OS message, continue processing.
			fmt.Fprintf(os.Stderr, "cat: %s: %s\n", file, unwrapOSError(err))
			return 1
		}
		defer f.Close()
		r = f
	}

	if !needsTransform {
		// R1.4: no transformation, copy bytes verbatim.
		if _, err := io.Copy(w, r); err != nil {
			fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
			return -1
		}
		return 0
	}

	// Process byte-by-byte for transformation modes.
	br := bufio.NewReader(r)
	atLineStart := true

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "cat: read error: %v\n", err)
			return 1
		}

		if b == '\n' {
			isBlank := atLineStart

			// R3.1: squeeze consecutive blank lines.
			if opts.squeeze && isBlank && *prevBlank {
				continue
			}
			*prevBlank = isBlank

			// Line numbering for blank lines (only with -n, not -b).
			if atLineStart && numberLines && !skipBlankNumbers {
				fmt.Fprintf(w, "%6d\t", *lineNum)
				*lineNum++
			}

			// R4.3: show $ before newline.
			if opts.showEnds {
				w.WriteByte('$')
			}
			w.WriteByte('\n')
			atLineStart = true
			continue
		}

		// Non-newline character: we're on a non-blank line.
		if atLineStart {
			*prevBlank = false
			if numberLines {
				// R2.1, R2.2: prepend line number.
				fmt.Fprintf(w, "%6d\t", *lineNum)
				*lineNum++
			}
			atLineStart = false
		}

		// R4.4: show tabs as ^I.
		if b == '\t' && opts.showTabs {
			w.WriteByte('^')
			w.WriteByte('I')
			continue
		}

		// R4.1, R4.2: -v does not alter tab or newline (newline handled above).
		if b == '\t' {
			w.WriteByte(b)
			continue
		}

		if opts.showNonPrint {
			writeNonPrinting(w, b)
		} else {
			w.WriteByte(b)
		}
	}

	return 0
}

// unwrapOSError extracts the human-readable error string from an os.PathError,
// matching the format GNU cat uses (which calls strerror(errno)).
func unwrapOSError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// writeNonPrinting writes byte b using caret and M- notation per R4.1.
func writeNonPrinting(w *bufio.Writer, b byte) {
	if b < 32 {
		// Control characters 0x00-0x1F (tab already handled).
		w.WriteByte('^')
		w.WriteByte(b + 64)
	} else if b == 127 {
		// DEL
		w.WriteByte('^')
		w.WriteByte('?')
	} else if b >= 128 {
		w.WriteByte('M')
		w.WriteByte('-')
		if b < 128+32 {
			// 0x80-0x9F: M-^X
			w.WriteByte('^')
			w.WriteByte(b - 128 + 64)
		} else if b == 255 {
			// 0xFF: M-^?
			w.WriteByte('^')
			w.WriteByte('?')
		} else {
			// 0xA0-0xFE: M-X
			w.WriteByte(b - 128)
		}
	} else {
		// Printable ASCII 0x20-0x7E.
		w.WriteByte(b)
	}
}
