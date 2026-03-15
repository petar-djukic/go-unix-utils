// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1-R1.5, R2.1-R2.4, R3.1-R3.3, R4.1-R4.9: cmd/cat
// binary with file concatenation, stdin reading, line numbering, blank-line
// squeezing, show-nonprinting, show-ends, show-tabs, combined display flags
// (-A, -e, -t), ignored -u flag, and error handling.

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
	numberAll, numberNonBlank, squeezeBlanks, showNonPrinting, showEnds, showTabs, files := parseArgs(os.Args[1:])

	// R2.3: -b takes precedence over -n.
	if numberNonBlank {
		numberAll = false
	}

	// R1.2: read from stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	// R4.9: determine whether any transformation is needed.
	needsProcessing := numberAll || numberNonBlank || squeezeBlanks || showNonPrinting || showEnds || showTabs

	exitCode := 0
	lineNum := 1
	lastBlank := false
	for _, name := range files {
		var err error
		// R3.2: lastBlank carries across file boundaries for -s.
		lineNum, lastBlank, err = catFile(name, needsProcessing, numberAll, numberNonBlank, squeezeBlanks, showNonPrinting, showEnds, showTabs, lineNum, lastBlank)
		if err != nil {
			// R5.2: print error to stderr and continue with remaining files.
			printError(name, err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseArgs separates flags from file arguments. Supports -n, -b, -s,
// -v, -E, -T, -A, -e, -t, -u and combined short flags (e.g., -nb).
func parseArgs(args []string) (numberAll, numberNonBlank, squeezeBlanks, showNonPrinting, showEnds, showTabs bool, files []string) {
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
			case 'v':
				// R4.1: show non-printing characters.
				showNonPrinting = true
			case 'E':
				// R4.3: show $ at end of lines.
				showEnds = true
			case 'T':
				// R4.4: show tabs as ^I.
				showTabs = true
			case 'A':
				// R4.5: -A is equivalent to -vET.
				showNonPrinting = true
				showEnds = true
				showTabs = true
			case 'e':
				// R4.6: -e is equivalent to -vE.
				showNonPrinting = true
				showEnds = true
			case 't':
				// R4.7: -t is equivalent to -vT.
				showNonPrinting = true
				showTabs = true
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
func catFile(name string, needsProcessing, numberAll, numberNonBlank, squeezeBlanks, showNonPrinting, showEnds, showTabs bool, lineNum int, lastBlank bool) (int, bool, error) {
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
	if !needsProcessing {
		_, err := io.Copy(os.Stdout, r)
		return lineNum, false, err
	}

	// R2.1-R2.4, R3.1-R3.3, R4.1-R4.4: processed path.
	doNumbering := numberAll || numberNonBlank
	return catProcessed(r, doNumbering, numberAll, squeezeBlanks, showNonPrinting, showEnds, showTabs, lineNum, lastBlank)
}

// catProcessed processes input with line numbering, blank-line squeezing,
// and non-printing display transformations.
// R4.9: order of application: squeeze (-s), then non-printing display (-v/-T),
// then end-of-line marker (-E), then line number (-n/-b).
func catProcessed(r io.Reader, doNumbering, numberAll, squeezeBlanks, showNonPrinting, showEnds, showTabs bool, lineNum int, lastBlank bool) (newLineNum int, newLastBlank bool, err error) {
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

		if atLineStart && b == '\n' {
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
			// R4.3: append $ before the newline.
			if showEnds {
				if writeErr := w.WriteByte('$'); writeErr != nil {
					return lineNum, lastBlank, writeErr
				}
			}
			if writeErr := w.WriteByte('\n'); writeErr != nil {
				return lineNum, lastBlank, writeErr
			}
			// atLineStart remains true for next byte.
			continue
		}

		if atLineStart {
			// Non-blank line start.
			lastBlank = false
			if doNumbering {
				fmt.Fprintf(w, "%6d\t", lineNum)
				lineNum++
			}
			atLineStart = false
		}

		if b == '\n' {
			// R4.3: append $ before the newline.
			if showEnds {
				if writeErr := w.WriteByte('$'); writeErr != nil {
					return lineNum, lastBlank, writeErr
				}
			}
			if writeErr := w.WriteByte('\n'); writeErr != nil {
				return lineNum, lastBlank, writeErr
			}
			atLineStart = true
			continue
		}

		// R4.1, R4.4: transform non-newline, non-line-start byte and write.
		if writeErr := writeTransformed(w, b, showNonPrinting, showTabs); writeErr != nil {
			return lineNum, lastBlank, writeErr
		}
	}
}

// writeTransformed writes a single byte through the -v and -T display
// transformations.
// R4.1: non-printing characters use caret notation and M- prefix.
// R4.2: -v does not alter tab (0x09) or newline (0x0A).
// R4.4: -T displays tabs as ^I.
func writeTransformed(w *bufio.Writer, b byte, showNonPrinting, showTabs bool) error {
	// R4.4: tab display as ^I when -T is active.
	if b == '\t' {
		if showTabs {
			_, err := w.WriteString("^I")
			return err
		}
		return w.WriteByte(b)
	}

	if !showNonPrinting {
		return w.WriteByte(b)
	}

	// R4.1: non-printing character display.
	if b < 0x20 {
		// Control characters 0x00-0x1F (tab already handled above,
		// newline never reaches here).
		if _, err := w.WriteString("^"); err != nil {
			return err
		}
		return w.WriteByte(b + 64)
	}
	if b == 0x7F {
		// DEL character.
		_, err := w.WriteString("^?")
		return err
	}
	if b >= 0x80 {
		if _, err := w.WriteString("M-"); err != nil {
			return err
		}
		if b < 0xA0 {
			// 0x80-0x9F: M-^X
			if _, err := w.WriteString("^"); err != nil {
				return err
			}
			return w.WriteByte(b - 0x80 + 64)
		}
		if b == 0xFF {
			// 0xFF: M-^?
			_, err := w.WriteString("^?")
			return err
		}
		// 0xA0-0xFE: M-X
		return w.WriteByte(b - 0x80)
	}

	// Printable ASCII (0x20-0x7E): pass through.
	return w.WriteByte(b)
}

// printHelp writes a usage message to w matching GNU cat --help format.
func printHelp(w io.Writer) error {
	_, err := fmt.Fprintln(w, `Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

  -A, --show-all       equivalent to -vET
  -b                   number nonempty output lines, overrides -n
  -e                   equivalent to -vE
  -E, --show-ends      display $ at end of each line
  -n                   number all output lines
  -s                   suppress repeated empty output lines
  -t                   equivalent to -vT
  -T, --show-tabs      display TAB characters as ^I
  -u                   (ignored)
  -v, --show-nonprinting  use ^ and M- notation, except for LFD and TAB
      --help     display this help and exit
      --version  output version information and exit`)
	return err
}

// printVersion writes version information to w.
func printVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "cat %s\n", version)
	return err
}
