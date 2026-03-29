// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uniq implements GNU uniq: report or filter adjacent duplicate lines.
//
// Implements prd028-uniq R1.1 (adjacent-line deduplication),
// R1.2 (input-file and output-file positional arguments),
// R1.3 (dash as stdin), R1.4 (case-sensitive comparison and SIGPIPE).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "uniq"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses positional arguments and processes input.
// R1.2: accepts optional input-file and output-file positional arguments.
// R1.4: exit 0 on success, 1 on error.
func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	inputFile, outputFile := parsePositional(args)
	r, closer, err := openInput(inputFile, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	w, wCloser, err := openOutput(outputFile, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if wCloser != nil {
		defer wCloser.Close() // best-effort close
	}
	if err := dedup(r, w); err != nil {
		fmt.Fprintf(stderr, "%s: write error\n", programName)
		return 1
	}
	return 0
}

// parsePositional extracts input-file and output-file from positional args.
// R1.2: first positional is input, second is output. Both are optional.
func parsePositional(args []string) (string, string) {
	inputFile := "-"
	outputFile := ""
	if len(args) >= 1 {
		inputFile = args[0]
	}
	if len(args) >= 2 {
		outputFile = args[1]
	}
	return inputFile, outputFile
}

// openInput returns a reader and optional closer for the given filename.
// R1.3: "-" means stdin.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: No such file or directory", name)
	}
	return f, f, nil
}

// openOutput returns a writer and optional closer for the given filename.
// Empty string means stdout.
func openOutput(name string, stdout io.Writer) (io.Writer, io.Closer, error) {
	if name == "" {
		return stdout, nil, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %v", name, err)
	}
	return f, f, nil
}

// dedup reads lines and writes each unique run once.
// R1.1: suppresses all but the first occurrence of adjacent identical lines.
// R1.2: non-adjacent duplicates are unaffected.
// R1.4: comparison is case-sensitive and includes the full line content.
func dedup(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	bw := bufio.NewWriter(w)
	prev := ""
	hasPrev := false
	for scanner.Scan() {
		line := scanner.Text()
		if !hasPrev || line != prev {
			if err := writeLine(bw, line); err != nil {
				return err
			}
		}
		prev = line
		hasPrev = true
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

// writeLine writes a single line followed by a newline.
func writeLine(w *bufio.Writer, line string) error {
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
