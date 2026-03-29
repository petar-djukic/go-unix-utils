// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sort implements GNU sort: sort lines of text files.
//
// Implements prd053-sort R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sortOptions holds parsed flag state.
type sortOptions struct {
	reverse bool // -r: reverse sort order
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run parses flags, reads input, sorts, and writes output.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "sort: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	lines, exitCode := readAllLines(files, stdin, stderr)
	sortLines(lines, opts)
	if err := writeLines(stdout, lines); err != nil {
		return 2
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
// R1.4: recognizes -r and --reverse.
func parseArgs(args []string) (sortOptions, []string, error) {
	var opts sortOptions
	var files []string
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
		if arg == "-" {
			files = append(files, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := parseLongFlag(&opts, arg[2:]); err != nil {
				return opts, nil, err
			}
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			if err := parseShortFlags(&opts, arg[1:]); err != nil {
				return opts, nil, err
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files, nil
}

// parseLongFlag handles a single --name long option.
func parseLongFlag(opts *sortOptions, name string) error {
	switch name {
	case "reverse":
		opts.reverse = true
	default:
		return fmt.Errorf("unrecognized option '--%s'", name)
	}
	return nil
}

// parseShortFlags processes flag characters from a single -xyz argument.
func parseShortFlags(opts *sortOptions, chars string) error {
	for _, ch := range chars {
		switch ch {
		case 'r':
			opts.reverse = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// readAllLines reads lines from all files, combining into a single slice.
// R1.2: "-" means stdin. R1.3: multiple files merged.
func readAllLines(
	files []string, stdin io.Reader, stderr io.Writer,
) ([]string, int) {
	var lines []string
	exitCode := 0
	for _, name := range files {
		fileLines, err := readFileLines(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "sort: cannot read: %s: %v\n",
				name, unwrapErr(err))
			exitCode = 2
			continue
		}
		lines = append(lines, fileLines...)
	}
	return lines, exitCode
}

// readFileLines reads all lines from a single file or stdin.
func readFileLines(name string, stdin io.Reader) ([]string, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer.Close()
	}
	return scanLines(r)
}

// openInput returns a reader and optional closer for the given filename.
// R1.2: "-" reads from stdin.
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

// scanLines reads all lines from a reader using a buffered scanner.
func scanLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// sortLines sorts lines lexicographically by byte value.
// R1.1: default lexicographic sort under LC_ALL=C.
// R1.4: reverse with -r.
func sortLines(lines []string, opts sortOptions) {
	sort.SliceStable(lines, func(i, j int) bool {
		if opts.reverse {
			return lines[i] > lines[j]
		}
		return lines[i] < lines[j]
	})
}

// writeLines writes all sorted lines to the writer, each followed by a newline.
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

// unwrapErr extracts the underlying syscall error from os.PathError.
func unwrapErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
