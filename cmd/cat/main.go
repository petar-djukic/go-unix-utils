// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cat command: concatenate and display files.
// Implements prd006-cat (R1-R5).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// catOptions holds the parsed command-line flags for cat.
type catOptions struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only (overrides -n)
	squeezeBlank   bool // -s: suppress repeated blank lines
	showEnds       bool // -E: display $ at end of each line
	showTabs       bool // -T: display TAB as ^I
	showNonPrint   bool // -v: use ^ and M- notation for non-printing chars
}

// parseFlags parses os.Args and returns options and file arguments.
// Returns an error if an unknown flag is encountered.
func parseFlags(args []string) (catOptions, []string) {
	var opts catOptions
	var files []string

	for _, arg := range args {
		if arg == "--" {
			// Everything after -- is a filename.
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
					opts.numberAll = true
				case 'b':
					opts.numberNonBlank = true
				case 's':
					opts.squeezeBlank = true
				case 'E':
					opts.showEnds = true
				case 'T':
					opts.showTabs = true
				case 'v':
					opts.showNonPrint = true
				case 'A': // R4.5: -A equivalent to -vET
					opts.showNonPrint = true
					opts.showEnds = true
					opts.showTabs = true
				case 'e': // R4.6: -e equivalent to -vE
					opts.showNonPrint = true
					opts.showEnds = true
				case 't': // R4.7: -t equivalent to -vT
					opts.showNonPrint = true
					opts.showTabs = true
				case 'u': // R4.8: accepted but no effect
				default:
					fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}
		files = append(files, arg)
	}

	// R2.3: -b overrides -n
	if opts.numberNonBlank {
		opts.numberAll = false
	}

	return opts, files
}

// needsTransform returns true if any transformation flag is active.
func (o catOptions) needsTransform() bool {
	return o.numberAll || o.numberNonBlank || o.squeezeBlank ||
		o.showEnds || o.showTabs || o.showNonPrint
}

// catState tracks state across files for line numbering and blank squeezing.
type catState struct {
	lineNum       int
	prevBlank     bool // true if previous output line was blank
	atLineStart   bool // true if we are at the beginning of a line
	exitCode      int
	stdout        *bufio.Writer
	opts          catOptions
}

// processSimple copies input to output without transformation (R1.4, R1.5).
func (s *catState) processSimple(r io.Reader) error {
	_, err := io.Copy(s.stdout, r)
	return err
}

// processTransform processes input byte-by-byte applying all active flags.
// Order of application (R4.9): squeeze blanks, non-printing display, end-of-line, line number.
func (s *catState) processTransform(r io.Reader) error {
	br := bufio.NewReaderSize(r, 32*1024)

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// R3.1, R3.2: squeeze blank lines
		if b == '\n' && s.atLineStart {
			// This is a blank line (only newline, no other content).
			if s.opts.squeezeBlank && s.prevBlank {
				// Suppress repeated blank line.
				continue
			}
			s.prevBlank = true

			// R2.2: -b skips numbering for blank lines
			if s.opts.numberNonBlank {
				// No line number for blank lines.
			} else if s.opts.numberAll {
				s.lineNum++
				fmt.Fprintf(s.stdout, "%6d\t", s.lineNum)
			}

			if s.opts.showEnds {
				s.stdout.WriteByte('$')
			}
			s.stdout.WriteByte('\n')
			s.atLineStart = true
			continue
		}

		// We have a non-newline character at start of line, or mid-line content.
		if s.atLineStart {
			s.prevBlank = false
			// R2.1, R2.2: prepend line number
			if s.opts.numberAll || s.opts.numberNonBlank {
				s.lineNum++
				fmt.Fprintf(s.stdout, "%6d\t", s.lineNum)
			}
			s.atLineStart = false
		}

		if b == '\n' {
			// End of a non-blank line.
			if s.opts.showEnds {
				s.stdout.WriteByte('$')
			}
			s.stdout.WriteByte('\n')
			s.atLineStart = true
			continue
		}

		if b == '\t' {
			if s.opts.showTabs {
				// R4.4: display TAB as ^I
				s.stdout.WriteByte('^')
				s.stdout.WriteByte('I')
			} else {
				// R4.2: -v alone does not alter TAB
				s.stdout.WriteByte(b)
			}
			continue
		}

		if s.opts.showNonPrint {
			s.writeNonPrint(b)
		} else {
			s.stdout.WriteByte(b)
		}
	}
}

// writeNonPrint writes a byte using caret and M- notation per R4.1.
func (s *catState) writeNonPrint(b byte) {
	if b >= 128 {
		s.stdout.WriteByte('M')
		s.stdout.WriteByte('-')
		b -= 128
	}
	if b < 32 {
		// Control character: ^X
		s.stdout.WriteByte('^')
		s.stdout.WriteByte(b + 64)
	} else if b == 127 {
		// DEL: ^?
		s.stdout.WriteByte('^')
		s.stdout.WriteByte('?')
	} else {
		// Printable character (or printable after M- prefix)
		s.stdout.WriteByte(b)
	}
}

// processFile opens and processes a single file (or stdin for "-").
func (s *catState) processFile(name string) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s: %s\n", name, err.(*os.PathError).Err)
			s.exitCode = 1
			return
		}
		defer f.Close()
		r = f
	}

	var err error
	if s.opts.needsTransform() {
		err = s.processTransform(r)
	} else {
		err = s.processSimple(r)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		s.exitCode = 1
	}
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])

	s := &catState{
		opts:        opts,
		atLineStart: true,
		stdout:      bufio.NewWriterSize(os.Stdout, 32*1024),
	}

	// R1.2: read stdin when no files given
	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, name := range files {
		s.processFile(name)
	}

	if err := s.stdout.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		s.exitCode = 1
	}

	os.Exit(s.exitCode)
}
