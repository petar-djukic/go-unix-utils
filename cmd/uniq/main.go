// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq R1.1–R1.4: default adjacent-duplicate suppression,
// stdin/file input, optional output file, case-sensitive comparison.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "uniq"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes input, returning the exit code.
// R1.3: reads stdin when no file argument given; accepts optional output file.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	inputFile, outputFile, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	reader, closer, err := openInput(inputFile, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	writer, wCloser, err := openOutput(outputFile, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if wCloser != nil {
		defer wCloser.Close() // best-effort close on output file
	}
	return processInput(reader, writer, stderr)
}

// parseArgs extracts input and output file arguments.
// Returns empty strings for stdin/stdout defaults.
func parseArgs(args []string) (string, string, error) {
	// Filter out "--" and handle flags
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(arg) > 0 && arg[0] == '-' && arg != "-" {
			return "", "", fmt.Errorf("invalid option -- '%s'", arg[1:])
		}
		positional = append(positional, arg)
	}
	if len(positional) > 2 {
		return "", "", fmt.Errorf("extra operand '%s'", positional[2])
	}
	inputFile := ""
	outputFile := ""
	if len(positional) >= 1 {
		inputFile = positional[0]
	}
	if len(positional) >= 2 {
		outputFile = positional[1]
	}
	return inputFile, outputFile, nil
}

// openInput opens the input source. Returns the reader and an optional closer.
// R1.3: stdin when no file or "-" is given.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "" || name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, unwrapPathError(err)
	}
	return f, f, nil
}

// openOutput opens the output destination. Returns the writer and an optional closer.
// R1.3: stdout when no output file is given.
func openOutput(name string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if name == "" {
		return stdout, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, unwrapPathError(err)
	}
	return f, f, nil
}

// processInput reads lines and suppresses adjacent duplicates.
// R1.1: writes first occurrence of each run.
// R1.2: non-adjacent duplicates are unaffected.
// R1.4: comparison is case-sensitive, includes newline.
func processInput(reader io.Reader, writer io.Writer, stderr io.Writer) int {
	scanner := bufio.NewScanner(reader)
	w := bufio.NewWriter(writer)
	var prevLine []byte
	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		if first || !bytes.Equal(line, prevLine) {
			if err := writeLine(w, line); err != nil {
				fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
				return 1
			}
			prevLine = make([]byte, len(line))
			copy(prevLine, line)
			first = false
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
	}
	return 0
}

// writeLine writes a line followed by a newline to the buffered writer.
func writeLine(w *bufio.Writer, line []byte) error {
	if _, err := w.Write(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
