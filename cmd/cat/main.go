// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat implements GNU cat: concatenate files to stdout.
//
// Implements prd006-cat R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
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
	lineNum := 1
	for _, name := range files {
		var err error
		lineNum, err = catFile(name, stdin, stdout, opts, lineNum)
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
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				opts.numberAll = true
			case 'b':
				opts.numberNonBlank = true
			}
		}
	}

	return opts, files
}

// isFlag returns true if the argument looks like a flag.
func isFlag(arg string) bool {
	return len(arg) >= 2 && arg[0] == '-'
}

// needsLineProcessing reports whether flags require line-by-line processing.
func needsLineProcessing(opts catOptions) bool {
	return opts.numberAll || opts.numberNonBlank
}

// catFile copies one file (or stdin if name is "-") to stdout.
// Returns the next line number to use.
func catFile(
	name string, stdin io.Reader, stdout io.Writer,
	opts catOptions, lineNum int,
) (int, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return lineNum, err
	}
	if closer != nil {
		defer closer.Close()
	}

	if !needsLineProcessing(opts) {
		// R1.4, R1.5: pass through without corruption or newline changes
		_, cpErr := io.Copy(stdout, r)
		return lineNum, cpErr
	}

	return catLines(r, stdout, opts, lineNum)
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

// catLines processes input line by line, applying numbering.
// R1.5: does not add or remove newlines.
// R2.1: -n prepends line number to every line.
// R2.2: -b skips numbering for blank lines.
// R2.3: -b overrides -n.
// R2.4: blank line = line containing only a newline.
func catLines(
	r io.Reader, w io.Writer,
	opts catOptions, lineNum int,
) (int, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	bw := bufio.NewWriter(w)
	atLineStart := true

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lineNum, atLineStart = processLine(
				bw, line, opts, lineNum, atLineStart,
			)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return lineNum, err
		}
	}

	return lineNum, bw.Flush()
}

// processLine writes a single chunk (ending with \n or EOF) with optional numbering.
// Returns the next lineNum and whether the next chunk starts a new line.
func processLine(
	bw *bufio.Writer, line []byte,
	opts catOptions, lineNum int, atLineStart bool,
) (int, bool) {
	if atLineStart {
		lineNum = writeLineNumber(bw, line, opts, lineNum)
	}

	bw.Write(line) //nolint:errcheck // error caught on flush

	// If line ends with \n, next chunk starts a new line
	endsWithNewline := line[len(line)-1] == '\n'
	return lineNum, endsWithNewline
}

// writeLineNumber writes a line number prefix if appropriate.
// R2.2, R2.3: -b skips numbering blank lines.
// R2.4: blank = line containing only \n.
func writeLineNumber(
	bw *bufio.Writer, line []byte,
	opts catOptions, lineNum int,
) int {
	isBlank := len(line) == 1 && line[0] == '\n'

	if opts.numberNonBlank {
		if isBlank {
			return lineNum
		}
		fmt.Fprintf(bw, "%6d\t", lineNum)
		return lineNum + 1
	}

	// opts.numberAll must be true (caller checks needsLineProcessing)
	fmt.Fprintf(bw, "%6d\t", lineNum)
	return lineNum + 1
}
