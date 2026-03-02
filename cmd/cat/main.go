// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cat command, which concatenates and displays files.
// It reads named files or stdin and writes to stdout, optionally applying line
// numbering, blank-line squeezing, and non-printing character display.
//
// Implements: prd006-cat R1, R2, R3, R4, R5
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// options holds the parsed flag configuration for a cat invocation.
type options struct {
	numberAll      bool // -n: number all output lines
	numberNonblank bool // -b: number non-blank lines only (overrides -n)
	squeeze        bool // -s: squeeze consecutive blank lines
	showEnds       bool // -E: show $ at end of lines
	showTabs       bool // -T: show tabs as ^I
	showNonprint   bool // -v: show non-printing characters
}

// needsTransform reports whether any output transformation is active.
func (o *options) needsTransform() bool {
	return o.numberAll || o.numberNonblank || o.squeeze ||
		o.showEnds || o.showTabs || o.showNonprint
}

// catState holds mutable state that persists across files during a single
// cat invocation.
type catState struct {
	lineNum     int  // next line number to assign
	blankCount  int  // consecutive blank lines seen (for -s)
	atLineStart bool // whether next byte starts a new line
}

// run parses flags and processes inputs. Returns 0 on success, 1 on error.
//
// Implements: prd006-cat R1, R2, R4, R5
func run(args []string) int {
	// Install SIGPIPE handler (R5.4).
	installSIGPIPEHandler()

	fs := flag.NewFlagSet("cat", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		flagNumber   = fs.Bool("n", false, "number all output lines")
		flagNumberNB = fs.Bool("b", false, "number non-blank output lines, overrides -n")
		flagSqueeze  = fs.Bool("s", false, "squeeze consecutive blank output lines")
		flagNonprint = fs.Bool("v", false, "display non-printing characters")
		flagEnds     = fs.Bool("E", false, "display $ at end of each line")
		flagTabs     = fs.Bool("T", false, "display TAB characters as ^I")
		flagAll      = fs.Bool("A", false, "equivalent to -vET")
		flagEndNP    = fs.Bool("e", false, "equivalent to -vE")
		flagTabNP    = fs.Bool("t", false, "equivalent to -vT")
	)

	// -u is accepted but ignored (R4.8).
	fs.Bool("u", false, "(ignored)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Build options, expanding combination flags (R4.5, R4.6, R4.7).
	opts := &options{
		numberAll:      *flagNumber,
		numberNonblank: *flagNumberNB,
		squeeze:        *flagSqueeze,
		showEnds:       *flagEnds,
		showTabs:       *flagTabs,
		showNonprint:   *flagNonprint,
	}

	if *flagAll {
		opts.showNonprint = true
		opts.showEnds = true
		opts.showTabs = true
	}
	if *flagEndNP {
		opts.showNonprint = true
		opts.showEnds = true
	}
	if *flagTabNP {
		opts.showNonprint = true
		opts.showTabs = true
	}

	// -b overrides -n (R2.3).
	if opts.numberNonblank {
		opts.numberAll = false
	}

	// Collect input files (R1.1, R1.2).
	files := fs.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	// Process each input in order (R1.1, R1.3).
	exitCode := 0
	st := &catState{lineNum: 1, atLineStart: true}
	w := bufio.NewWriter(os.Stdout)

	for _, file := range files {
		var r io.Reader
		var f *os.File

		if file == "-" {
			r = os.Stdin
		} else {
			var err error
			f, err = os.Open(file)
			if err != nil {
				printFileError(file, err)
				exitCode = 1
				continue
			}
			r = f
		}

		var err error
		if opts.needsTransform() {
			err = processTransform(r, w, opts, st)
		} else {
			err = processSimple(r, w)
		}

		if f != nil {
			f.Close()
		}

		if err != nil {
			printFileError(file, err)
			exitCode = 1
		}
	}

	// Flush remaining buffered output (R5.3).
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		exitCode = 1
	}

	return exitCode
}

// installSIGPIPEHandler sets up a signal handler for SIGPIPE that exits
// cleanly. This prevents non-zero exit when stdout is closed by a downstream
// consumer.
//
// Implements: prd006-cat R5.4
func installSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

// processSimple copies input to output with no transformations. Used when no
// flags are active, ensuring binary data passes through without corruption.
//
// Implements: prd006-cat R1.4
func processSimple(r io.Reader, w *bufio.Writer) error {
	_, err := io.Copy(w, r)
	return err
}

// processTransform reads input byte-by-byte and applies active transformations.
// The transformation order follows R4.9: squeeze blanks, non-printing display,
// end-of-line marker, line number.
//
// Implements: prd006-cat R2, R3, R4
func processTransform(r io.Reader, w *bufio.Writer, opts *options, st *catState) error {
	br := bufio.NewReader(r)

	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// At the start of a line, handle squeezing and numbering.
		if st.atLineStart {
			isBlank := (b == '\n')

			// Squeeze consecutive blank lines (R3.1, R3.2).
			if opts.squeeze {
				if isBlank {
					st.blankCount++
					if st.blankCount > 1 {
						continue
					}
				} else {
					st.blankCount = 0
				}
			}

			// Line numbering (R2.1, R2.2, R2.3).
			if opts.numberNonblank {
				if !isBlank {
					fmt.Fprintf(w, "%6d\t", st.lineNum)
					st.lineNum++
				}
			} else if opts.numberAll {
				fmt.Fprintf(w, "%6d\t", st.lineNum)
				st.lineNum++
			}

			st.atLineStart = false
		}

		// Apply byte-level transformations.
		if b == '\n' {
			// End-of-line marker (R4.3).
			if opts.showEnds {
				w.WriteByte('$')
			}
			w.WriteByte('\n')
			st.atLineStart = true
		} else if b == '\t' {
			// Tab display (R4.4).
			if opts.showTabs {
				w.WriteByte('^')
				w.WriteByte('I')
			} else {
				w.WriteByte(b)
			}
		} else if opts.showNonprint {
			// Non-printing character display (R4.1, R4.2).
			writeNonprint(w, b)
		} else {
			w.WriteByte(b)
		}
	}
}

// writeNonprint writes a single byte to w using caret notation for control
// characters and M- notation for bytes above 127. Tab (0x09) and newline
// (0x0A) are handled by the caller before reaching this function.
//
// Implements: prd006-cat R4.1
func writeNonprint(w *bufio.Writer, b byte) {
	if b < 32 {
		// Control characters 0x00-0x1F (tab/newline handled by caller).
		w.WriteByte('^')
		w.WriteByte(b + 64)
	} else if b == 127 {
		// DEL character.
		w.WriteByte('^')
		w.WriteByte('?')
	} else if b >= 128 {
		// High bytes: M- prefix.
		w.WriteByte('M')
		w.WriteByte('-')
		adj := b - 128
		if adj < 32 {
			w.WriteByte('^')
			w.WriteByte(adj + 64)
		} else if adj == 127 {
			w.WriteByte('^')
			w.WriteByte('?')
		} else {
			w.WriteByte(adj)
		}
	} else {
		// Normal printable characters 0x20-0x7E.
		w.WriteByte(b)
	}
}

// printFileError writes a file access error to stderr (R5.2).
//
// Implements: prd006-cat R5.2
func printFileError(file string, err error) {
	if pe, ok := err.(*os.PathError); ok {
		fmt.Fprintf(os.Stderr, "cat: %s: %s\n", pe.Path, pe.Err)
	} else {
		fmt.Fprintf(os.Stderr, "cat: %s: %v\n", file, err)
	}
}
