// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cat utility for concatenating and displaying files.
//
// Implements prd006-cat (R1-R5).
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// options holds the parsed command-line flags for cat.
type options struct {
	number          bool
	numberNonblank  bool
	squeeze         bool
	showNonprinting bool
	showEnds        bool
	showTabs        bool
}

// needsLineProcessing reports whether any flag requiring line-by-line
// processing is active.
// D4: when false, io.Copy is used for zero-allocation passthrough.
func (o *options) needsLineProcessing() bool {
	return o.number || o.numberNonblank || o.squeeze ||
		o.showNonprinting || o.showEnds || o.showTabs
}

// catState carries mutable state across file boundaries.
type catState struct {
	lineNumber int
	// R3.2: prevBlank persists across files for cross-boundary squeeze.
	prevBlank bool
}

func main() {
	// R5.4: handle SIGPIPE by exiting cleanly.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	var opts options
	flag.BoolVar(&opts.number, "n", false, "number all output lines")
	flag.BoolVar(&opts.numberNonblank, "b", false, "number non-blank output lines")
	flag.BoolVar(&opts.squeeze, "s", false, "squeeze consecutive blank lines")
	flag.BoolVar(&opts.showNonprinting, "v", false, "show non-printing characters")
	flag.BoolVar(&opts.showEnds, "E", false, "display $ at end of each line")
	flag.BoolVar(&opts.showTabs, "T", false, "display TAB characters as ^I")

	showAll := flag.Bool("A", false, "equivalent to -vET")
	shortE := flag.Bool("e", false, "equivalent to -vE")
	shortT := flag.Bool("t", false, "equivalent to -vT")
	// R4.8: -u is accepted but has no effect.
	_ = flag.Bool("u", false, "(ignored)")

	flag.Parse()

	// R4.5: -A is equivalent to -vET.
	if *showAll {
		opts.showNonprinting = true
		opts.showEnds = true
		opts.showTabs = true
	}
	// R4.6: -e is equivalent to -vE.
	if *shortE {
		opts.showNonprinting = true
		opts.showEnds = true
	}
	// R4.7: -t is equivalent to -vT.
	if *shortT {
		opts.showNonprinting = true
		opts.showTabs = true
	}

	// R2.3: -b overrides -n.
	if opts.numberNonblank {
		opts.number = false
	}

	// R1.2: read from stdin when no file arguments are given.
	files := flag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	state := &catState{lineNumber: 1}
	out := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range files {
		if err := catFile(name, out, &opts, state); err != nil {
			reportError(name, err)
			exitCode = 1
		}
	}

	// R5.3: detect write errors on final flush.
	if err := out.Flush(); err != nil {
		exitCode = 1
	}

	os.Exit(exitCode)
}

// reportError writes a cat-style error message to stderr.
// R5.2: per-file error messages to stderr.
func reportError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "cat: %s: %v\n", pathErr.Path, pathErr.Err)
	} else {
		fmt.Fprintf(os.Stderr, "cat: %s: %v\n", name, err)
	}
}

// catFile processes a single file argument. "-" means stdin.
// R1.1: reads named files in argument order.
// R1.2: reads from stdin when "-" appears as a file argument.
func catFile(name string, w *bufio.Writer, opts *options, state *catState) error {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}

	// D4: use io.Copy for zero-allocation passthrough when no flags are active.
	if !opts.needsLineProcessing() {
		_, err := io.Copy(w, r)
		return err
	}

	return processLines(r, w, opts, state)
}

// processLines reads input line by line and applies the active transformations.
// R4.9: order is squeeze, non-printing display, end marker, line number.
func processLines(r io.Reader, w *bufio.Writer, opts *options, state *catState) error {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if writeErr := writeLine(w, line, opts, state); writeErr != nil {
				return writeErr
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

// writeLine applies squeeze, non-printing display, end markers, and line
// numbering to a single line. The line may or may not end with '\n'.
func writeLine(w *bufio.Writer, line []byte, opts *options, state *catState) error {
	hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
	// R2.4: a blank line contains only a newline character.
	isBlank := len(line) == 1 && line[0] == '\n'

	// R3.1: squeeze consecutive blank lines.
	if opts.squeeze && isBlank {
		if state.prevBlank {
			return nil
		}
		state.prevBlank = true
	} else {
		state.prevBlank = false
	}

	// R2.1, R2.2: prepend line number.
	if opts.number || opts.numberNonblank {
		if !(opts.numberNonblank && isBlank) {
			if _, err := fmt.Fprintf(w, "%6d\t", state.lineNumber); err != nil {
				return err
			}
			state.lineNumber++
		}
	}

	// Process content bytes (everything except trailing newline).
	content := line
	if hasNewline {
		content = line[:len(line)-1]
	}

	for _, b := range content {
		if err := displayByte(w, b, opts); err != nil {
			return err
		}
	}

	// R4.3: append $ before newline when -E is active.
	if hasNewline {
		if opts.showEnds {
			if err := w.WriteByte('$'); err != nil {
				return err
			}
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}

	return nil
}

// displayByte writes a single byte to the output, applying non-printing
// display (-v) and tab display (-T) transformations.
// R4.1: caret and M- notation for non-printing characters.
// R4.2: tab and newline exempt from -v (newline handled in writeLine).
// R4.4: tab displayed as ^I when -T is active.
func displayByte(w *bufio.Writer, b byte, opts *options) error {
	if opts.showNonprinting {
		if b >= 128 {
			if _, err := w.WriteString("M-"); err != nil {
				return err
			}
			b -= 128
			// After M- prefix, all control chars use ^X notation without exemption.
			if b < 32 {
				if err := w.WriteByte('^'); err != nil {
					return err
				}
				return w.WriteByte(b + 64)
			}
			if b == 127 {
				if err := w.WriteByte('^'); err != nil {
					return err
				}
				return w.WriteByte('?')
			}
			return w.WriteByte(b)
		}
		// Low bytes: tab exempt from -v unless -T is also active.
		if b < 32 {
			if b == '\t' && !opts.showTabs {
				return w.WriteByte('\t')
			}
			if err := w.WriteByte('^'); err != nil {
				return err
			}
			return w.WriteByte(b + 64)
		}
		if b == 127 {
			if err := w.WriteByte('^'); err != nil {
				return err
			}
			return w.WriteByte('?')
		}
		return w.WriteByte(b)
	}

	// R4.4: -T without -v only affects tabs.
	if opts.showTabs && b == '\t' {
		if err := w.WriteByte('^'); err != nil {
			return err
		}
		return w.WriteByte('I')
	}

	return w.WriteByte(b)
}
