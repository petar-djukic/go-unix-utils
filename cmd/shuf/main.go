// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd064-shuf R1.1–R1.4: default shuffle behavior for files and stdin.
package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "shuf"

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and shuffles input lines, returning the exit code.
// R1.1: reads each named file in argument order.
// R1.2: reads stdin when no args or "-" is given.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	files, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, err := readAllLines(files, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	shuffleLines(lines)
	return writeLines(lines, stdout, stderr)
}

// parseArgs separates flags from file arguments.
// Returns file list and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) ([]string, int) {
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
		code := applyFlag(arg, stdout, stderr)
		if code >= 0 {
			return nil, code
		}
	}
	return files, -1
}

// applyFlag handles recognized flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyFlag(arg string, stdout, stderr io.Writer) int {
	switch arg {
	case "--help":
		printHelp(stdout)
		return 0
	case "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... [FILE]\n", progName)
	fmt.Fprintln(w, "Write a random permutation of the input lines to standard output.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "      --help     display this help and exit")
	fmt.Fprintln(w, "      --version  output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// readAllLines reads all lines from the given files, using stdin for "-".
// R1.4: each line is terminated by newline; last line included without trailing newline.
func readAllLines(files []string, stdin io.Reader) ([]string, error) {
	var lines []string
	for _, name := range files {
		fileLines, err := readFileLines(name, stdin)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fileLines...)
	}
	return lines, nil
}

// readFileLines reads lines from a single file or stdin.
func readFileLines(name string, stdin io.Reader) ([]string, error) {
	if name == "-" {
		return scanLines(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, unwrapPathError(err)
	}
	defer f.Close() // best-effort close on read-only file
	return scanLines(f)
}

// scanLines reads all lines from r, splitting on newline.
// R1.4: includes the last line even if it lacks a trailing newline.
func scanLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// shuffleLines randomly permutes the slice in place.
// R1.3: each input line appears exactly once (Fisher-Yates shuffle).
func shuffleLines(lines []string) {
	rand.Shuffle(len(lines), func(i, j int) {
		lines[i], lines[j] = lines[j], lines[i]
	})
}

// writeLines writes shuffled lines to stdout, one per line.
func writeLines(lines []string, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
			return 1
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "%s: write error: %s\n", progName, err)
		return 1
	}
	return 0
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
