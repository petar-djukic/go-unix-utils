// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cat: concatenate and display files.
// Implements srd006-cat R1.1, R1.2, R1.3, R1.4, R1.5, R2.1, R2.2, R2.3.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds parsed command-line flags.
type options struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only
}

// needsLineProcessing returns true when flags require line-by-line processing.
func (o *options) needsLineProcessing() bool {
	return o.numberAll || o.numberNonBlank
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) < 2 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if err := parseFlags(&opts, arg[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", err)
			os.Exit(1)
		}
	}
	return opts, files
}

// parseFlags processes the characters in a combined flag string.
func parseFlags(opts *options, flags string) error {
	for _, ch := range flags {
		switch ch {
		case 'n':
			opts.numberAll = true
		case 'b':
			opts.numberNonBlank = true
		case 'u':
			// R4.8: accepted but ignored.
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// catPassthrough copies file contents verbatim to stdout.
// R1.1: writes contents verbatim. R1.4, R1.5: preserves all bytes and newlines.
func catPassthrough(name string, w io.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	_, err = io.Copy(w, r)
	return err
}

// catNumbered processes a file with line numbering.
// R2.1: -n numbers every line. R2.2, R2.3: -b numbers non-blank only.
func catNumbered(name string, w io.Writer, lineNum *int, nonBlankOnly bool) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return numberLines(r, w, lineNum, nonBlankOnly)
}

// numberLines reads from r line by line and writes numbered output to w.
func numberLines(r io.Reader, w io.Writer, lineNum *int, nonBlankOnly bool) error {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if werr := writeLine(w, line, lineNum, nonBlankOnly); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// writeLine writes a single line with optional numbering prefix.
// R2.1: format "%6d\t" for numbered lines.
// R2.2: blank lines (only newline) get no prefix when nonBlankOnly is set.
func writeLine(w io.Writer, line []byte, lineNum *int, nonBlankOnly bool) error {
	// R2.4: blank = line containing only a newline character.
	if nonBlankOnly && len(line) == 1 && line[0] == '\n' {
		_, err := w.Write(line)
		return err
	}
	if _, err := fmt.Fprintf(w, "%6d\t", *lineNum); err != nil {
		return err
	}
	*lineNum++
	_, err := w.Write(line)
	return err
}

// openInput returns os.Stdin for "-", otherwise opens the named file.
// R1.2: stdin when filename is "-".
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, args := parseArgs(os.Args[1:])

	// R1.2: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	// R2.3: -b takes precedence over -n when both are given.
	nonBlankOnly := opts.numberNonBlank

	exitCode := 0
	lineNum := 1
	for _, name := range args {
		var err error
		if opts.needsLineProcessing() {
			err = catNumbered(name, os.Stdout, &lineNum, nonBlankOnly)
		} else {
			err = catPassthrough(name, os.Stdout)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}
