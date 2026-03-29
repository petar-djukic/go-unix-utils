// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/head implements GNU head: print the first lines of files.
//
// Implements prd018-head R1.1, R1.2, R1.3, R1.4.
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

const (
	defaultLines = 10
	programName  = "head"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// headOptions holds parsed flag state.
type headOptions struct {
	lines    int
	negative bool
	help     bool
	version  bool
}

// run parses flags and processes input files.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		return 1
	}
	if opts.help {
		printHelp(stdout)
		return 0
	}
	if opts.version {
		printVersion(stdout)
		return 0
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	return processFiles(files, stdin, stdout, stderr, opts)
}

// processFiles iterates over files and prints head output for each.
func processFiles(files []string, stdin io.Reader, stdout, stderr io.Writer, opts headOptions) int {
	exitCode := 0
	for _, name := range files {
		if err := headFile(name, stdin, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs separates flags from file arguments.
// R1.2: -n NUM and --lines=NUM set line count.
// R1.4: --help and --version are recognized.
func parseArgs(args []string) (headOptions, []string, error) {
	opts := headOptions{lines: defaultLines}
	var files []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || arg == "-" || (len(arg) > 0 && arg[0] != '-') {
			files = append(files, arg)
			continue
		}
		var err error
		i, err = parseFlag(&opts, &flagsDone, args, i)
		if err != nil {
			return opts, nil, err
		}
	}
	return opts, files, nil
}

// parseFlag handles a single flag argument starting at args[i].
// Returns the updated index and any error.
func parseFlag(opts *headOptions, flagsDone *bool, args []string, i int) (int, error) {
	arg := args[i]
	switch {
	case arg == "--":
		*flagsDone = true
	case arg == "--help":
		opts.help = true
	case arg == "--version":
		opts.version = true
	case strings.HasPrefix(arg, "--lines="):
		return i, parseLinesValue(opts, arg[len("--lines="):])
	case arg == "--lines" || arg == "-n":
		return parseNextArg(opts, args, i, arg)
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'n':
		return i, parseLinesValue(opts, arg[2:])
	case isLegacyNumArg(arg):
		return i, parseLegacyNum(opts, arg)
	default:
		return i, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
	return i, nil
}

// parseNextArg reads the next argument as the value for -n or --lines.
func parseNextArg(opts *headOptions, args []string, i int, flag string) (int, error) {
	i++
	if i >= len(args) {
		return i, fmt.Errorf("option '%s' requires an argument", flag)
	}
	return i, parseLinesValue(opts, args[i])
}

// parseLinesValue parses a line count value, which may be negative.
// R1.2: positive integer sets line count.
// R1.3: negative prefix means print all except last N lines.
func parseLinesValue(opts *headOptions, s string) error {
	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of lines: '%s'", s)
	}
	opts.lines = n
	opts.negative = negative
	return nil
}

// isLegacyNumArg checks for legacy -NUM form (e.g., head -5).
func isLegacyNumArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-' && arg[1] >= '0' && arg[1] <= '9'
}

// parseLegacyNum parses legacy -NUM form.
func parseLegacyNum(opts *headOptions, arg string) error {
	n, err := strconv.Atoi(arg[1:])
	if err != nil {
		return fmt.Errorf("invalid number of lines: '%s'", arg[1:])
	}
	opts.lines = n
	opts.negative = false
	return nil
}

// headFile processes a single input file or stdin.
// R1.1: default 10 lines. R1.4: stdin when name is "-".
func headFile(name string, stdin io.Reader, stdout io.Writer, opts headOptions) error {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	if opts.negative {
		return headNegativeLines(r, stdout, opts.lines)
	}
	return headPositiveLines(r, stdout, opts.lines)
}

// openInput returns a reader and optional closer for the given filename.
// R1.4: "-" means stdin.
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		if pe, ok := errors.AsType[*os.PathError](err); ok {
			return nil, nil, fmt.Errorf("cannot open '%s' for reading: %v", name, pe.Err)
		}
		return nil, nil, err
	}
	return f, f, nil
}

// headPositiveLines prints the first n lines from r.
// R1.1: default 10 lines. R1.2: configurable via -n.
func headPositiveLines(r io.Reader, w io.Writer, n int) error {
	if n <= 0 {
		return nil
	}
	br := bufio.NewReaderSize(r, 64*1024)
	bw := bufio.NewWriter(w)
	count := 0
	for count < n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, wErr := bw.Write(line); wErr != nil {
				return wErr
			}
			count++
		}
		if err != nil {
			break
		}
	}
	return bw.Flush()
}

// headNegativeLines prints all lines except the last n lines.
// R1.3: negative count means exclude trailing lines.
func headNegativeLines(r io.Reader, w io.Writer, n int) error {
	if n <= 0 {
		_, err := io.Copy(w, r)
		return err
	}
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	end := max(len(lines)-n, 0)
	return writeLines(w, lines[:end])
}

// readAllLines reads all lines from r, preserving line endings.
func readAllLines(r io.Reader) ([][]byte, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var lines [][]byte
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return lines, nil
}

// writeLines writes a slice of lines to w.
func writeLines(w io.Writer, lines [][]byte) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.Write(line); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// printHelp prints usage information to w.
// R1.4: --help prints usage to stdout and exits 0.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s [OPTION]... [FILE]...
Print the first 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -n, --lines=[-]NUM  print the first NUM lines instead of the first 10;
                       with the leading '-', print all but the last
                       NUM lines of each file
      --help        display this help and exit
      --version     output version information and exit
`, programName)
}

// printVersion prints version information to w.
// R1.4: --version prints version to stdout and exits 0.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", programName)
}
