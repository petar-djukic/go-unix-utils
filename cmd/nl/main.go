// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl R1.1–R1.4: default line numbering behavior.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	progName     = "nl"
	defaultWidth = 6
	defaultSep   = "\t"
)

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes files, returning the exit code.
// R1.3: reads stdin when no file args given. R1.4: continuous numbering.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files := parseFiles(args)
	if len(files) == 0 {
		files = []string{"-"}
	}
	w := bufio.NewWriter(stdout)
	lineNum := 1
	exitCode := 0
	for _, name := range files {
		n, err := processFile(name, lineNum, stdin, w)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(err))
			exitCode = 1
		}
		lineNum = n
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return exitCode
}

// parseFiles extracts file arguments from the argument list.
// Treats "--" as end-of-flags marker and "-" as stdin.
func parseFiles(args []string) []string {
	var files []string
	flagsDone := false
	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) == 0 || arg[0] != '-' {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		// Unknown flags ignored for R1.1–R1.4 scope
		files = append(files, arg)
	}
	return files
}

// processFile reads a single file (or stdin for "-") and numbers its
// lines. Returns the next line number for continuous numbering (R1.4).
func processFile(name string, lineNum int, stdin io.Reader, w *bufio.Writer) (int, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return lineNum, err
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	return numberLines(r, lineNum, w)
}

// openInput returns a reader for the named file or stdin for "-".
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

// emptyPadding is the prefix for unnumbered lines: width spaces + separator
// length spaces, matching GNU nl behavior.
var emptyPadding = strings.Repeat(" ", defaultWidth+len(defaultSep))

// numberLines reads lines from r and writes them with line numbers.
// R1.1: non-empty lines are numbered with right-justified width-6 + tab.
// R1.2: empty lines get space padding matching the number field width.
func numberLines(r io.Reader, lineNum int, w *bufio.Writer) (int, error) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			var writeErr error
			lineNum, writeErr = writeLine(w, line, lineNum)
			if writeErr != nil {
				return lineNum, writeErr
			}
		}
		if err != nil {
			if err != io.EOF {
				return lineNum, err
			}
			break
		}
	}
	return lineNum, nil
}

// writeLine writes a single line, numbering it if non-empty. R1.1, R1.2.
// GNU nl pads unnumbered lines with spaces and always ensures a trailing
// newline on output.
func writeLine(w *bufio.Writer, line string, lineNum int) (int, error) {
	content := strings.TrimSuffix(line, "\n")
	if content == "" {
		_, err := fmt.Fprintf(w, "%s\n", emptyPadding)
		return lineNum, err
	}
	_, err := fmt.Fprintf(w, "%*d%s%s\n", defaultWidth, lineNum, defaultSep, content)
	return lineNum + 1, err
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
