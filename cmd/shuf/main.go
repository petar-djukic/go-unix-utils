// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/shuf: shuffle lines randomly.
// Implements srd064-shuf R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in diagnostic messages.
const progName = "shuf"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the shuf logic and returns the exit code.
// R1.1: reads lines from file arguments, permutes, writes to stdout.
// R1.2: reads from stdin when no arguments or "-" is given.
func run(args []string) int {
	files := parseArgs(args)
	lines, err := readAllLines(files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		return 1
	}
	// R1.3: uniform random permutation.
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
	if err := writeLines(lines); err != nil {
		return 1
	}
	return 0
}

// parseArgs extracts file arguments, treating "--" as end of options.
func parseArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return args
}

// readAllLines reads all lines from the given files.
// R1.2: empty slice or "-" means stdin.
// R1.4: includes the last line even without a trailing newline.
func readAllLines(files []string) ([]string, error) {
	if len(files) == 0 {
		files = []string{"-"}
	}
	var lines []string
	for _, name := range files {
		fileLines, err := readLinesFromFile(name)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readLinesFromFile reads lines from a single file or stdin.
func readLinesFromFile(name string) ([]string, error) {
	r, closer, err := openInput(name)
	if err != nil {
		return nil, err
	}
	defer closer()
	return scanLines(r)
}

// openInput opens a file for reading, or returns stdin for "-".
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

// scanLines reads all lines from a reader using bufio.Scanner.
// R1.4: each line is terminated by newline; last line without
// trailing newline is still included.
func scanLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// writeLines writes each line to stdout followed by a newline.
func writeLines(lines []string) error {
	w := bufio.NewWriter(os.Stdout)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return w.Flush()
}
