// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1-R1.5, R2.1-R2.4, R3.1-R3.3: cmd/cat binary with
// file concatenation, stdin reading, line numbering, blank-line squeezing, and
// error handling.

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is the project version string, set at build time via
// -ldflags "-X main.version=<tag>". Defaults to "dev" for development builds.
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	// Handle --help and --version as the first argument.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help":
			if err := printHelp(os.Stdout); err != nil {
				os.Exit(1)
			}
			return
		case "--version":
			if err := printVersion(os.Stdout); err != nil {
				os.Exit(1)
			}
			return
		}
	}

	// Parse flags and separate file arguments.
	numberAll, numberNonBlank, squeezeBlanks, files := parseArgs(os.Args[1:])

	// R2.3: -b takes precedence over -n.
	if numberNonBlank {
		numberAll = false
	}

	// R1.2: read from stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	lineNum := 1
	lastBlank := false
	for _, name := range files {
		var err error
		// R3.2: lastBlank carries across file boundaries for -s.
		lineNum, lastBlank, err = catFile(name, numberAll, numberNonBlank, squeezeBlanks, lineNum, lastBlank)
		if err != nil {
			// R5.2: print error to stderr and continue with remaining files.
			printError(name, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseArgs separates flags from file arguments. Supports -n, -b, and
// combined short flags (e.g., -nb). Returns numberAll, numberNonBlank,
// and remaining file arguments.
func parseArgs(args []string) (numberAll, numberNonBlank, squeezeBlanks bool, files []string) {
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
		if arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		// Process each character in the flag group.
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				numberAll = true
			case 'b':
				numberNonBlank = true
			case 's':
				// R3.1: suppress repeated blank lines.
				squeezeBlanks = true
			case 'u':
				// R4.8: -u is accepted but has no effect.
			default:
				fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\nTry 'cat --help' for more information.\n", ch)
				os.Exit(1)
			}
		}
	}
	return
}

// printError writes a GNU-compatible error message to stderr.
// R5.2: error format matches GNU cat: "cat: NAME: REASON".
func printError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "cat: %s: %s\n", name, capitalizeError(pathErr.Err))
	} else {
		fmt.Fprintf(os.Stderr, "cat: %s: %s\n", name, capitalizeError(err))
	}
}

// capitalizeError capitalizes the first letter of an error message to match
// GNU coreutils error output (C strerror() returns capitalized strings on
// macOS, while Go's syscall error table uses lowercase).
func capitalizeError(err error) string {
	s := err.Error()
	if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// catFile reads the contents of a single file and writes them to stdout.
// R1.2: if name is "-", it reads from stdin.
// Returns the updated line number and lastBlank state for cross-file continuity.
func catFile(name string, numberAll, numberNonBlank, squeezeBlanks bool, lineNum int, lastBlank bool) (int, bool, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		// R1.1: open the named file for reading.
		f, err := os.Open(name)
		if err != nil {
			return lineNum, lastBlank, err
		}
		defer f.Close()
		r = f
	}

	// R1.4, R1.5: fast path with no transformation preserves bytes verbatim.
	if !numberAll && !numberNonBlank && !squeezeBlanks {
		_, err := io.Copy(os.Stdout, r)
		return lineNum, false, err
	}

	// R2.1-R2.4, R3.1-R3.3: processed path with numbering and/or squeezing.
	doNumbering := numberAll || numberNonBlank
	return catProcessed(r, doNumbering, numberAll, squeezeBlanks, lineNum, lastBlank)
}

// catProcessed processes input with line numbering and/or blank-line squeezing.
// R2.1: when doNumbering && numberAll, numbers every line with "%6d\t" prefix.
// R2.2: when doNumbering && !numberAll, numbers only non-blank lines.
// R2.4: a blank line contains only a newline character.
// R3.1: when squeezeBlanks is true, consecutive blank lines are collapsed to one.
// R3.3: squeezing is applied before numbering.
func catProcessed(r io.Reader, doNumbering, numberAll, squeezeBlanks bool, lineNum int, lastBlank bool) (newLineNum int, newLastBlank bool, err error) {
	br := bufio.NewReader(r)
	w := bufio.NewWriter(os.Stdout)
	defer func() {
		if flushErr := w.Flush(); flushErr != nil && err == nil {
			err = flushErr
		}
	}()

	atLineStart := true
	for {
		b, readErr := br.ReadByte()
		if readErr != nil {
			if readErr == io.EOF {
				return lineNum, lastBlank, nil
			}
			return lineNum, lastBlank, readErr
		}

		if atLineStart {
			if b == '\n' {
				// R2.4: blank line (only a newline character).
				// R3.1: suppress consecutive blank lines when -s is active.
				if squeezeBlanks && lastBlank {
					continue
				}
				lastBlank = true
				if doNumbering && numberAll {
					// R2.1: number all lines including blank.
					fmt.Fprintf(w, "%6d\t", lineNum)
					lineNum++
				}
				// R2.2: -b skips numbering for blank lines, no prefix added.
			} else {
				lastBlank = false
				if doNumbering {
					// Non-blank line: always number with -n or -b.
					fmt.Fprintf(w, "%6d\t", lineNum)
					lineNum++
				}
			}
			atLineStart = false
		}

		if writeErr := w.WriteByte(b); writeErr != nil {
			return lineNum, lastBlank, writeErr
		}
		if b == '\n' {
			atLineStart = true
		}
	}
}

// printHelp writes a usage message to w matching GNU cat --help format.
func printHelp(w io.Writer) error {
	_, err := fmt.Fprintln(w, `Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

  -b                  number nonempty output lines, overrides -n
  -n                  number all output lines
  -s                  suppress repeated empty output lines
  -u                  (ignored)
      --help     display this help and exit
      --version  output version information and exit`)
	return err
}

// printVersion writes version information to w.
func printVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "cat %s\n", version)
	return err
}
