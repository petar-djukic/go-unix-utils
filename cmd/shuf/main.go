// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/shuf implements prd064-shuf R1.1–R1.4: shuffle input lines randomly.
//
// Reads lines from files or stdin, randomly permutes them, and writes the
// result to stdout. Each input line appears exactly once in the output.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand/v2"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "shuf: %v\n", err)
		os.Exit(1)
	}
}

// run implements the core shuf logic for R1.1–R1.4.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = stderr // reserved for future error reporting
	files := parseArgs(args)
	lines, err := readAllLines(files, stdin)
	if err != nil {
		return err
	}
	shuffleLines(lines)
	return writeLines(stdout, lines)
}

// parseArgs extracts file arguments from args.
// R1.2: no file arguments or "-" means stdin.
func parseArgs(args []string) []string {
	var files []string
	for _, a := range args {
		if a == "--" {
			continue
		}
		files = append(files, a)
	}
	return files
}

// readAllLines reads lines from the given files, or stdin if no files given.
// R1.1: reads all lines from each file.
// R1.2: reads from stdin when no files given or "-" specified.
// R1.4: last line need not end with newline but is included.
func readAllLines(files []string, stdin io.Reader) ([]string, error) {
	if len(files) == 0 {
		return scanLines(stdin)
	}
	var all []string
	for _, name := range files {
		lines, err := readFileLines(name, stdin)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

// readFileLines reads lines from a single file or stdin for "-".
func readFileLines(name string, stdin io.Reader) ([]string, error) {
	if name == "-" {
		return scanLines(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanLines(f)
}

// scanLines reads all lines from r using a scanner.
// R1.4: treats each line as newline-terminated; includes last line
// even without trailing newline.
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

// shuffleLines randomly permutes lines in place.
// R1.3: each line appears exactly once (Fisher-Yates shuffle).
func shuffleLines(lines []string) {
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
}

// writeLines writes each line to w followed by a newline.
func writeLines(w io.Writer, lines []string) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.WriteString(line); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}
