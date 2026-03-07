// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cat utility for concatenating and displaying files.
//
// Implements prd006-cat: file concatenation (R1), line numbering (R2),
// blank-line squeezing (R3), non-printing character display (R4),
// error handling and exit codes (R5).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// flags holds the parsed command-line options.
type flags struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only (overrides -n)
	squeeze        bool // -s: squeeze consecutive blank lines
	showNonprint   bool // -v: show non-printing characters
	showEnds       bool // -E: show $ at end of lines
	showTabs       bool // -T: show tabs as ^I
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, files := parseArgs(os.Args[1:])

	exitCode := 0
	lineNum := 1
	prevBlank := false

	if len(files) == 0 {
		lineNum = processReader(os.Stdout, os.Stdin, f, lineNum, &prevBlank)
		_ = lineNum
	} else {
		for _, name := range files {
			var r io.Reader
			if name == "-" {
				r = os.Stdin
			} else {
				file, err := os.Open(name)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cat: %s: No such file or directory\n", name)
					exitCode = 1
					continue
				}
				r = file
				defer file.Close()
			}
			lineNum = processReader(os.Stdout, r, f, lineNum, &prevBlank)
		}
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and file names.
func parseArgs(args []string) (flags, []string) {
	var f flags
	var files []string

	for _, arg := range args {
		if arg == "--" {
			// Everything after -- is a file argument; handled below.
			files = append(files, args[len(files)+countFlags(args):]...)
			break
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			for _, ch := range arg[1:] {
				switch ch {
				case 'n':
					f.numberAll = true
				case 'b':
					f.numberNonBlank = true
				case 's':
					f.squeeze = true
				case 'v':
					f.showNonprint = true
				case 'E':
					f.showEnds = true
				case 'T':
					f.showTabs = true
				case 'A': // R4.5: -A = -vET
					f.showNonprint = true
					f.showEnds = true
					f.showTabs = true
				case 'e': // R4.6: -e = -vE
					f.showNonprint = true
					f.showEnds = true
				case 't': // R4.7: -t = -vT
					f.showNonprint = true
					f.showTabs = true
				case 'u': // R4.8: accepted, no effect
				default:
					fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
		} else {
			files = append(files, arg)
		}
	}

	// R2.3: -b overrides -n.
	if f.numberNonBlank {
		f.numberAll = false
	}

	return f, files
}

// countFlags counts leading flag arguments (used only by -- handling).
func countFlags(args []string) int {
	n := 0
	for _, arg := range args {
		if arg == "--" {
			return n + 1
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			n++
		} else {
			return n
		}
	}
	return n
}

// needsTransform returns true if any transformation flag is active.
func needsTransform(f flags) bool {
	return f.numberAll || f.numberNonBlank || f.squeeze ||
		f.showNonprint || f.showEnds || f.showTabs
}

// processReader reads from r and writes to w, applying transformations.
// Returns the next line number to use.
func processReader(w io.Writer, r io.Reader, f flags, lineNum int, prevBlank *bool) int {
	// R1.4: When no transformation flags are active, copy verbatim.
	if !needsTransform(f) {
		bw := bufio.NewWriter(w)
		io.Copy(bw, r)
		bw.Flush()
		return lineNum
	}

	// Line-by-line processing for transformation modes.
	reader := bufio.NewReader(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineNum = processLine(bw, line, f, lineNum, prevBlank)
		}
		if err != nil {
			break
		}
	}

	return lineNum
}

// processLine handles a single line (which may or may not end with '\n').
// R4.9: Order is squeeze → non-printing/tabs → end-of-line → line number.
func processLine(w *bufio.Writer, line []byte, f flags, lineNum int, prevBlank *bool) int {
	// Determine if this line is blank (only a newline).
	isBlank := len(line) == 1 && line[0] == '\n'

	// R3.1: Squeeze consecutive blank lines.
	if f.squeeze && isBlank {
		if *prevBlank {
			return lineNum
		}
		*prevBlank = true
	} else {
		*prevBlank = false
	}

	// R2: Line numbering.
	if f.numberNonBlank {
		if !isBlank {
			fmt.Fprintf(w, "%6d\t", lineNum)
			lineNum++
		}
	} else if f.numberAll {
		fmt.Fprintf(w, "%6d\t", lineNum)
		lineNum++
	}

	// Process each byte of the line content.
	hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
	content := line
	if hasNewline {
		content = line[:len(line)-1]
	}

	for _, b := range content {
		writeByte(w, b, f)
	}

	// R4.3: -E appends "$" before the newline.
	if hasNewline {
		if f.showEnds {
			w.WriteByte('$')
		}
		w.WriteByte('\n')
	}

	return lineNum
}

// writeByte writes a single byte with non-printing and tab transformations.
func writeByte(w *bufio.Writer, b byte, f flags) {
	// R4.4: -T shows tabs as ^I.
	if b == '\t' {
		if f.showTabs {
			w.WriteByte('^')
			w.WriteByte('I')
		} else {
			w.WriteByte(b)
		}
		return
	}

	if !f.showNonprint {
		w.WriteByte(b)
		return
	}

	// R4.1: -v non-printing character display.
	if b >= 128 {
		w.WriteByte('M')
		w.WriteByte('-')
		b -= 128
	}

	if b < 32 {
		// Control characters 0x00-0x1F (tab already handled above).
		w.WriteByte('^')
		w.WriteByte(b + 64)
	} else if b == 127 {
		// DEL character.
		w.WriteByte('^')
		w.WriteByte('?')
	} else {
		w.WriteByte(b)
	}
}
