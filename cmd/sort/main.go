// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sort implements GNU sort: sort lines of text files.
//
// Implements prd053-sort R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7.
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
	reverse    bool   // -r: reverse sort order
	unique     bool   // R1.5: -u: output only first of equal run
	stable     bool   // R1.7: -s: preserve input order of equal lines
	outputFile string // R1.6: -o FILE: write output to file
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
	if opts.unique {
		lines = dedup(lines)
	}
	if err := writeOutput(opts.outputFile, stdout, lines); err != nil {
		fmt.Fprintf(stderr, "sort: %v\n", err)
		return 2
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (sortOptions, []string, error) {
	var opts sortOptions
	var files []string
	flagsDone := false

	i := 0
	for i < len(args) {
		arg := args[i]
		if flagsDone || arg == "-" || (len(arg) > 0 && arg[0] != '-') {
			files = append(files, arg)
			i++
			continue
		}
		if arg == "--" {
			flagsDone = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--") {
			consumed, err := parseLongFlag(&opts, arg[2:], args[i+1:])
			if err != nil {
				return opts, nil, err
			}
			i += 1 + consumed
			continue
		}
		consumed, err := parseShortFlags(&opts, arg[1:], args[i+1:])
		if err != nil {
			return opts, nil, err
		}
		i += 1 + consumed
	}
	return opts, files, nil
}

// parseLongFlag handles a single --name or --name=value long option.
// Returns the number of extra args consumed from rest.
func parseLongFlag(opts *sortOptions, name string, rest []string) (int, error) {
	if strings.HasPrefix(name, "output=") {
		opts.outputFile = name[len("output="):]
		return 0, nil
	}
	switch name {
	case "reverse":
		opts.reverse = true
	case "unique":
		opts.unique = true
	case "stable":
		opts.stable = true
	case "output":
		if len(rest) == 0 {
			return 0, fmt.Errorf("option '--output' requires an argument")
		}
		opts.outputFile = rest[0]
		return 1, nil
	default:
		return 0, fmt.Errorf("unrecognized option '--%s'", name)
	}
	return 0, nil
}

// parseShortFlags processes flag characters from a single -xyz argument.
// Returns the number of extra args consumed from rest.
func parseShortFlags(opts *sortOptions, chars string, rest []string) (int, error) {
	for idx, ch := range chars {
		switch ch {
		case 'r':
			opts.reverse = true
		case 'u':
			opts.unique = true
		case 's':
			opts.stable = true
		case 'o':
			return consumeFlagArg(chars[idx+1:], rest, &opts.outputFile, 'o')
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}

// consumeFlagArg extracts the argument for a flag that requires a value.
// remaining is the rest of the short-flag cluster after the flag char.
func consumeFlagArg(
	remaining string, rest []string, dest *string, flag byte,
) (int, error) {
	if remaining != "" {
		*dest = remaining
		return 0, nil
	}
	if len(rest) == 0 {
		return 0, fmt.Errorf("option requires an argument -- '%c'", flag)
	}
	*dest = rest[0]
	return 1, nil
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
// R1.7: -s preserves input order of equal lines via stable sort.
func sortLines(lines []string, opts sortOptions) {
	less := func(i, j int) bool {
		if opts.reverse {
			return lines[i] > lines[j]
		}
		return lines[i] < lines[j]
	}
	if opts.stable {
		sort.SliceStable(lines, less)
		return
	}
	// Without -s, use stable sort anyway since equal lines are identical
	// strings under lexicographic comparison (no key fields yet).
	sort.SliceStable(lines, less)
}

// dedup removes consecutive equal lines, keeping only the first of each run.
// R1.5: equality is based on the active sort comparison (full line for now).
func dedup(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	result := lines[:1]
	for _, line := range lines[1:] {
		if line != result[len(result)-1] {
			result = append(result, line)
		}
	}
	return result
}

// writeOutput writes lines to the output file or stdout.
// R1.6: -o FILE writes to file; FILE may be the same as an input file.
func writeOutput(path string, stdout io.Writer, lines []string) error {
	if path == "" {
		return writeLines(stdout, lines)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeLines(f, lines)
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
