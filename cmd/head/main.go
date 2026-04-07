// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/head: print the first lines of files.
// Implements srd018-head R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "head"

// defaultLines is the number of lines printed when no -n flag is given.
// R1.1: default is 10 lines.
const defaultLines = 10

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the head logic and returns the exit code.
// R4.1: returns 0 when all files processed successfully.
// R4.2: returns 1 when any file cannot be opened or read.
func run(args []string) int {
	lineCount, negative, fileArgs := parseArgs(args)

	// R1.4: read from stdin when no file arguments given.
	if len(fileArgs) == 0 {
		fileArgs = []string{"-"}
	}

	exitCode := 0
	for _, name := range fileArgs {
		if err := processFile(name, lineCount, negative); err != nil {
			reportError(name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens and processes a single file or stdin.
func processFile(name string, lineCount int, negative bool) error {
	r, closer, err := openInput(name)
	if err != nil {
		return err
	}
	defer closer()

	if negative {
		return printAllButLastN(r, lineCount)
	}
	return printFirstN(r, lineCount)
}

// openInput opens a file for reading, or returns stdin for "-".
// R1.4: stdin when file argument is "-".
func openInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// parseArgs extracts -n/--lines flag and file arguments.
// R1.1: defaults to 10 lines.
// R1.2: -n NUM sets line count.
// R1.3: negative NUM (prefixed with '-') sets negative mode.
func parseArgs(args []string) (lineCount int, negative bool, fileArgs []string) {
	lineCount = defaultLines
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			fileArgs = append(fileArgs, args[i+1:]...)
			return
		}
		var numStr string
		switch {
		case arg == "--lines" || arg == "-n":
			if i+1 < len(args) {
				i++
				numStr = args[i]
			}
		case strings.HasPrefix(arg, "--lines="):
			numStr = arg[len("--lines="):]
		case len(arg) > 2 && arg[0] == '-' && arg[1] == 'n':
			numStr = arg[2:]
		default:
			fileArgs = append(fileArgs, arg)
			continue
		}
		if numStr != "" {
			lineCount, negative = parseLineCount(numStr)
		}
	}
	return
}

// parseLineCount parses a line count string, detecting negative prefix.
// R1.2: NUM is a positive integer.
// R1.3: NUM prefixed with '-' enables negative mode.
func parseLineCount(s string) (int, bool) {
	neg := false
	raw := s
	if strings.HasPrefix(s, "-") {
		neg = true
		raw = s[1:]
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid number of lines: %q\n", progName, s)
		return defaultLines, false
	}
	return n, neg
}

// printFirstN writes the first n lines from r to stdout.
// R1.1, R1.2: output first N lines.
// R1.5: a line without a trailing newline is still counted.
func printFirstN(r io.Reader, n int) error {
	br := bufio.NewReader(r)
	for i := 0; i < n; i++ {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := os.Stdout.Write(line); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
	return nil
}

// printAllButLastN writes all lines except the last n from r to stdout.
// R1.3: requires buffering input in a ring buffer.
func printAllButLastN(r io.Reader, n int) error {
	if n <= 0 {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	return drainRingBuffer(bufio.NewReader(r), n)
}

// drainRingBuffer reads lines into a ring buffer of size n, outputting
// evicted lines as new ones arrive.
func drainRingBuffer(br *bufio.Reader, n int) error {
	ring := make([][]byte, n)
	idx := 0
	total := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if total >= n {
				if _, werr := os.Stdout.Write(ring[idx]); werr != nil {
					return werr
				}
			}
			ring[idx] = line
			idx = (idx + 1) % n
			total++
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// reportError prints a GNU-compatible diagnostic to stderr.
func reportError(name string, err error) {
	var pe *os.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(os.Stderr, "%s: cannot open '%s' for reading: %s\n",
			progName, name, pe.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, name, err)
}
