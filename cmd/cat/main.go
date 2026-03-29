// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat implements GNU cat: concatenate files to stdout.
//
// Implements prd006-cat R1.1, R1.2, R1.3, R1.4, R1.5,
// R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3.
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
// R2.1: -n numbers all lines.
// R2.2: -b numbers non-blank lines, implies -n.
// R2.3: -b takes precedence over -n.
// R3.1: -s suppresses repeated blank lines.
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
func parseFlags(opts *catOptions, chars string) {
	for _, ch := range chars {
		switch ch {
		case 'n':
			opts.numberAll = true
		case 'b':
			opts.numberNonBlank = true
		case 's':
			opts.squeeze = true
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
	return opts.numberAll || opts.numberNonBlank || opts.squeeze
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

	bw.Write(line) //nolint:errcheck // error caught on flush

	// If line ends with \n, next chunk starts a new line
	return line[len(line)-1] == '\n'
}

// writeLineNumber writes a line number prefix if appropriate.
// R2.2, R2.3: -b skips numbering blank lines.
// R2.4: blank = line containing only \n.
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
