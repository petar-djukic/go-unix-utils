// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat implements GNU cat: concatenate files to stdout.
//
// Implements prd006-cat R1.1, R1.2, R1.3, R1.4, R1.5,
// R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3,
// R4.1, R4.2, R4.3, R4.4, R4.5, R4.6, R4.7, R4.8.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// catOptions holds parsed flag state.
type catOptions struct {
	numberAll      bool // -n: number all lines
	numberNonBlank bool // -b: number non-blank lines (overrides -n)
	squeeze        bool // -s: suppress repeated blank lines
	showNonPrint   bool // -v: show non-printing characters
	showEnds       bool // -E: show $ at end of each line
	showTabs       bool // -T: show tabs as ^I
}

// catState tracks mutable state across file boundaries.
// R3.2: prevBlank persists across files for cross-boundary squeezing.
type catState struct {
	lineNum   int
	prevBlank bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and concatenates files to stdout.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	opts, files := parseArgs(args)

	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	state := catState{lineNum: 1}
	for _, name := range files {
		err := catFile(name, stdin, stdout, opts, &state)
		if err != nil {
			fmt.Fprintf(stderr, "cat: %s: %v\n", name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
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
		if arg == "-" || !isFlag(arg) {
			files = append(files, arg)
			continue
		}
		parseFlags(&opts, arg[1:])
	}

	return opts, files
}

// parseFlags processes flag characters from a single argument.
// R4.1: -v shows non-printing characters.
// R4.3: -E shows $ at end of lines.
// R4.4: -T shows tabs as ^I.
func parseFlags(opts *catOptions, chars string) {
	for _, ch := range chars {
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
		case 'A':
			// R4.5: -A is equivalent to -v -E -T
			opts.showNonPrint = true
			opts.showEnds = true
			opts.showTabs = true
		case 'e':
			// R4.6: -e is equivalent to -v -E
			opts.showNonPrint = true
			opts.showEnds = true
		case 't':
			// R4.7: -t is equivalent to -v -T
			opts.showNonPrint = true
			opts.showTabs = true
		case 'u':
			// R4.8: accepted, no effect
		}
	}
}

// isFlag returns true if the argument looks like a flag.
func isFlag(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-'
}

// needsLineProcessing reports whether flags require line-by-line processing.
func needsLineProcessing(opts catOptions) bool {
	return opts.numberAll || opts.numberNonBlank || opts.squeeze ||
		opts.showNonPrint || opts.showEnds || opts.showTabs
}

// catFile copies one file (or stdin if name is "-") to stdout.
func catFile(
	name string, stdin io.Reader, stdout io.Writer,
	opts catOptions, state *catState,
) error {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	if !needsLineProcessing(opts) {
		// R1.4, R1.5: pass through without corruption or newline changes
		_, cpErr := io.Copy(stdout, r)
		return cpErr
	}

	return catLines(r, stdout, opts, state)
}

// openInput returns a reader and optional closer for the given filename.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// catLines processes input line by line, applying squeezing and numbering.
// R1.5: does not add or remove newlines.
// R3.1: suppresses repeated blank lines.
// R3.3: squeeze is applied before numbering.
func catLines(
	r io.Reader, w io.Writer,
	opts catOptions, state *catState,
) error {
	br := bufio.NewReaderSize(r, 64*1024)
	bw := bufio.NewWriter(w)
	atLineStart := true

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			atLineStart = processLine(bw, line, opts, state, atLineStart)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return bw.Flush()
}

// isBlankLine reports whether a line is blank.
// R2.4: blank = line containing only \n (zero non-newline bytes).
func isBlankLine(line []byte) bool {
	return len(line) == 1 && line[0] == '\n'
}

// processLine writes a single chunk with optional squeezing and numbering.
// R3.1: suppress consecutive blank lines when squeeze is active.
// R3.3: squeezing before numbering.
// R4.9: order is squeeze → transform (-v/-T) → end marker (-E) → number.
// Returns whether the next chunk starts a new line.
func processLine(
	bw *bufio.Writer, line []byte,
	opts catOptions, state *catState, atLineStart bool,
) bool {
	blank := isBlankLine(line)

	// R3.1: suppress repeated blank lines
	if opts.squeeze && blank && state.prevBlank {
		return true // still at line start, line suppressed
	}
	state.prevBlank = blank

	if atLineStart {
		state.lineNum = writeLineNumber(bw, opts, state.lineNum, blank)
	}

	writeLine(bw, line, opts)

	// If line ends with \n, next chunk starts a new line
	return line[len(line)-1] == '\n'
}

// writeLineNumber writes a line number prefix if appropriate.
// R2.2, R2.3: -b skips numbering blank lines.
func writeLineNumber(
	bw *bufio.Writer, opts catOptions,
	lineNum int, blank bool,
) int {
	if opts.numberNonBlank {
		if blank {
			return lineNum
		}
		fmt.Fprintf(bw, "%6d\t", lineNum)
		return lineNum + 1
	}

	if opts.numberAll {
		fmt.Fprintf(bw, "%6d\t", lineNum)
		return lineNum + 1
	}

	return lineNum
}

// writeLine writes a line with optional -v, -T, -E transformations.
// R4.1: non-printing chars shown as ^X / M-^X / M-X.
// R4.2: -v exempts tab and newline.
// R4.3: -E appends $ before newline.
// R4.4: -T shows tab as ^I.
func writeLine(bw *bufio.Writer, line []byte, opts catOptions) {
	needsTransform := opts.showNonPrint || opts.showEnds || opts.showTabs
	if !needsTransform {
		bw.Write(line) //nolint:errcheck // error caught on flush
		return
	}

	for _, b := range line {
		writeTransformedByte(bw, b, opts)
	}
}

// writeTransformedByte writes a single byte with -v/-E/-T transformations.
// R4.1: caret and M- notation for non-printing characters.
// R4.2: tab and newline exempt from -v.
// R4.3: $ before newline when -E active.
// R4.4: tab shown as ^I when -T active.
func writeTransformedByte(bw *bufio.Writer, b byte, opts catOptions) {
	if b == '\n' {
		if opts.showEnds {
			bw.WriteByte('$') //nolint:errcheck
		}
		bw.WriteByte('\n') //nolint:errcheck
		return
	}

	if b == '\t' {
		if opts.showTabs {
			bw.WriteString("^I") //nolint:errcheck
		} else {
			bw.WriteByte('\t') //nolint:errcheck
		}
		return
	}

	if !opts.showNonPrint {
		bw.WriteByte(b) //nolint:errcheck
		return
	}

	writeNonPrintByte(bw, b)
}

// writeNonPrintByte writes a byte using caret/M- notation per R4.1.
// 0x00-0x1F (except tab/newline handled above): ^X
// 0x7F: ^?
// 0x80-0x9F: M-^X
// 0xA0-0xFE: M-X
// 0xFF: M-^?
func writeNonPrintByte(bw *bufio.Writer, b byte) {
	if b < 0x20 {
		bw.WriteByte('^')          //nolint:errcheck
		bw.WriteByte(b + '@')     //nolint:errcheck
		return
	}
	if b == 0x7F {
		bw.WriteString("^?") //nolint:errcheck
		return
	}
	if b >= 0x80 {
		bw.WriteString("M-") //nolint:errcheck
		if b < 0xA0 {
			// 0x80-0x9F: M-^X
			bw.WriteByte('^')              //nolint:errcheck
			bw.WriteByte(b - 0x80 + '@')  //nolint:errcheck
		} else if b == 0xFF {
			// 0xFF: M-^?
			bw.WriteString("^?") //nolint:errcheck
		} else {
			// 0xA0-0xFE: M-X
			bw.WriteByte(b - 0x80) //nolint:errcheck
		}
		return
	}
	// Printable ASCII 0x20-0x7E
	bw.WriteByte(b) //nolint:errcheck
}
