// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cat utility for concatenating and displaying files.
// Implements prd006-cat (R1, R2, R3, R4, R5).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// flags holds the parsed command-line flags for cat.
type flags struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only
	squeeze        bool // -s: squeeze repeated blank lines
	showNonPrint   bool // -v: show non-printing characters
	showEnds       bool // -E: show $ at end of each line
	showTabs       bool // -T: show tabs as ^I
}

// needsTransform returns true if any transformation flag is active.
func (f *flags) needsTransform() bool {
	return f.numberAll || f.numberNonBlank || f.squeeze ||
		f.showNonPrint || f.showEnds || f.showTabs
}

func main() {
	// R5.4: install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	fl, files := parseArgs(os.Args[1:])

	exitCode := 0
	if len(files) == 0 {
		files = []string{"-"}
	}

	if !fl.needsTransform() {
		// Fast path: simple copy with no transformations.
		for _, name := range files {
			if err := catSimple(name); err != nil {
				fmt.Fprintf(os.Stderr, "cat: %s\n", err)
				exitCode = 1
			}
		}
	} else {
		// Slow path: line-by-line processing with transformations.
		state := &transformState{}
		for _, name := range files {
			if err := catTransform(name, fl, state); err != nil {
				fmt.Fprintf(os.Stderr, "cat: %s\n", err)
				exitCode = 1
			}
		}
	}

	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into flags and file names.
// R4.8: -u is accepted but has no effect.
func parseArgs(args []string) (flags, []string) {
	var fl flags
	var files []string

	for _, arg := range args {
		if arg == "--" {
			// Everything after -- is a file name.
			continue
		}
		if arg == "-" {
			files = append(files, "-")
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'n':
					fl.numberAll = true
				case 'b':
					fl.numberNonBlank = true
				case 's':
					fl.squeeze = true
				case 'v':
					fl.showNonPrint = true
				case 'E':
					fl.showEnds = true
				case 'T':
					fl.showTabs = true
				case 'A':
					// R4.5: -A is equivalent to -vET.
					fl.showNonPrint = true
					fl.showEnds = true
					fl.showTabs = true
				case 'e':
					// R4.6: -e is equivalent to -vE.
					fl.showNonPrint = true
					fl.showEnds = true
				case 't':
					// R4.7: -t is equivalent to -vT.
					fl.showNonPrint = true
					fl.showTabs = true
				case 'u':
					// R4.8: accepted but no effect.
				default:
					fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}
		files = append(files, arg)
	}

	// R2.3: -b overrides -n.
	if fl.numberNonBlank {
		fl.numberAll = false
	}

	return fl, files
}

// openInput returns a reader for the given file name. "-" means stdin.
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", name, err.(*os.PathError).Err)
	}
	return f, nil
}

// catSimple copies input to stdout with no transformations.
// R1.4: binary data passes through without corruption.
func catSimple(name string) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	defer r.Close() // best-effort close

	_, err = io.Copy(os.Stdout, r)
	return err
}

// transformState tracks state across files for line numbering and blank squeezing.
type transformState struct {
	lineNum       int  // current line number (persists across files)
	prevBlank     bool // previous output line was blank (for -s)
	atLineStart   bool // at the start of a line (for numbering)
	initialized   bool // whether we've started processing
	pendingSqueze bool // we're in a run of blank lines being squeezed
}

// catTransform processes a file with transformation flags applied.
// R4.9: order is squeeze blanks, non-printing display, end-of-line marker, line number.
func catTransform(name string, fl flags, state *transformState) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	defer r.Close() // best-effort close

	if !state.initialized {
		state.lineNum = 0
		state.atLineStart = true
		state.initialized = true
	}

	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush() // best-effort flush

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
		content := line
		if hasNewline {
			content = line[:len(line)-1]
		}

		isBlank := len(content) == 0 && hasNewline

		// R3.1: squeeze repeated blank lines.
		if fl.squeeze && isBlank {
			if state.prevBlank {
				if err == io.EOF {
					return nil
				}
				continue
			}
		}
		state.prevBlank = isBlank

		// Line numbering.
		if state.atLineStart {
			if fl.numberNonBlank {
				// R2.2: number non-blank lines only.
				if !isBlank {
					state.lineNum++
					fmt.Fprintf(writer, "%6d\t", state.lineNum)
				}
			} else if fl.numberAll {
				// R2.1: number all lines.
				state.lineNum++
				fmt.Fprintf(writer, "%6d\t", state.lineNum)
			}
		}

		// Write content with transformations.
		for _, b := range content {
			if fl.showNonPrint {
				if b >= 128 {
					writer.WriteString("M-") //nolint:errcheck
					b -= 128
				}
				if b < 32 && b != '\t' {
					// R4.1: control characters as ^X.
					writer.WriteByte('^')           //nolint:errcheck
					writer.WriteByte(b + '@')       //nolint:errcheck
					continue
				}
				if b == 127 {
					// R4.1: DEL as ^?.
					writer.WriteByte('^')     //nolint:errcheck
					writer.WriteByte('?')     //nolint:errcheck
					continue
				}
				if fl.showTabs && b == '\t' {
					// R4.4: tab as ^I.
					writer.WriteByte('^')     //nolint:errcheck
					writer.WriteByte('I')     //nolint:errcheck
					continue
				}
				writer.WriteByte(b) //nolint:errcheck
			} else if fl.showTabs && b == '\t' {
				// R4.4: tab as ^I (without -v).
				writer.WriteByte('^') //nolint:errcheck
				writer.WriteByte('I') //nolint:errcheck
			} else {
				writer.WriteByte(b) //nolint:errcheck
			}
		}

		if hasNewline {
			// R4.3: show $ before newline.
			if fl.showEnds {
				writer.WriteByte('$') //nolint:errcheck
			}
			writer.WriteByte('\n') //nolint:errcheck
			state.atLineStart = true
		} else {
			state.atLineStart = false
		}

		if err == io.EOF {
			return nil
		}
	}
}
