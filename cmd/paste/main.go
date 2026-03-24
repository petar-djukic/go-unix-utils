// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd027-paste: Merge Lines of Files Side by Side.
// Covers R1.1-R1.4 (default parallel merge), R2.1-R2.2 (delimiter configuration).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const defaultDelimiter = "\t"

func main() {
	// R4.4: SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	delims, files, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(delims, files))
}

// run opens all input files and merges them in parallel.
// R1.4: reads stdin when no files given.
// R4.1/R4.2: exit 0 on success, exit 1 on error.
func run(delims []string, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}

	scanners, closers, exitCode := openInputs(files)
	if exitCode != 0 {
		return exitCode
	}
	defer closeAll(closers)

	return mergeParallel(scanners, delims)
}

// openInputs opens all named files and returns scanners.
// R1.3: "-" refers to stdin; multiple "-" share the same reader.
// R4.2: exits on first open error.
func openInputs(files []string) ([]*bufio.Scanner, []io.Closer, int) {
	var scanners []*bufio.Scanner
	var closers []io.Closer
	var stdinScanner *bufio.Scanner

	for _, name := range files {
		s, c, err := openOneInput(name, &stdinScanner)
		if err != nil {
			closeAll(closers)
			reportError(name, err)
			return nil, nil, 1
		}
		scanners = append(scanners, s)
		if c != nil {
			closers = append(closers, c)
		}
	}

	return scanners, closers, 0
}

// openOneInput opens a single file or returns the shared stdin scanner.
func openOneInput(name string, stdinScanner **bufio.Scanner) (*bufio.Scanner, io.Closer, error) {
	if name == "-" {
		if *stdinScanner == nil {
			*stdinScanner = bufio.NewScanner(os.Stdin)
		}
		return *stdinScanner, nil, nil
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return bufio.NewScanner(f), f, nil
}

// mergeParallel reads one line from each file per iteration and writes
// merged output.
// R1.1: lines joined with delimiter.
// R1.2: exhausted files contribute empty strings.
// R2.2: single file outputs lines as-is without delimiter.
func mergeParallel(scanners []*bufio.Scanner, delims []string) int {
	w := bufio.NewWriter(os.Stdout)
	eof := make([]bool, len(scanners))

	for {
		if allEOF(eof) {
			break
		}
		lines, anyData := readOneLine(scanners, eof)
		if !anyData {
			break
		}
		if err := writeMergedLine(w, lines, delims); err != nil {
			fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
			return 1
		}
	}

	if err := checkScanErrors(scanners); err != nil {
		fmt.Fprintf(os.Stderr, "paste: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
		return 1
	}
	return 0
}

// readOneLine reads one line from each scanner, updating eof flags.
// Returns the lines and whether any scanner produced data.
func readOneLine(scanners []*bufio.Scanner, eof []bool) ([]string, bool) {
	lines := make([]string, len(scanners))
	anyData := false

	for i, s := range scanners {
		if eof[i] {
			continue
		}
		if s.Scan() {
			lines[i] = s.Text()
			anyData = true
		} else {
			eof[i] = true
		}
	}

	return lines, anyData
}

// writeMergedLine writes one merged output line with cycling delimiters.
// R2.1: delimiters cycle across columns.
// R2.2: no delimiter for single file.
func writeMergedLine(w *bufio.Writer, lines []string, delims []string) error {
	for i, line := range lines {
		if i > 0 {
			delimIdx := (i - 1) % len(delims)
			if _, err := w.WriteString(delims[delimIdx]); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(line); err != nil {
			return err
		}
	}
	return w.WriteByte('\n')
}

// allEOF reports whether all scanners have reached EOF.
func allEOF(eof []bool) bool {
	for _, done := range eof {
		if !done {
			return false
		}
	}
	return true
}

// checkScanErrors checks all scanners for non-EOF read errors.
func checkScanErrors(scanners []*bufio.Scanner) error {
	for _, s := range scanners {
		if err := s.Err(); err != nil {
			return err
		}
	}
	return nil
}

// closeAll closes all closers (best-effort cleanup).
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		c.Close() // best-effort cleanup, error ignored
	}
}

// reportError writes a paste-style error message to stderr.
// R4.2: diagnostic to stderr for file open failures.
func reportError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "paste: %s: %v\n", name, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "paste: %s: %v\n", name, err)
}

// parseArgs processes command-line flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(args []string) (delims []string, files []string, exit int) {
	delims = []string{defaultDelimiter}
	exit = -1

	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			files = append(files, args[i+1:]...)
			return
		}
		exit = parseOneArg(args, &i, &delims, &files)
		if exit >= 0 {
			return nil, nil, exit
		}
	}

	return
}

// parseOneArg handles a single argument and returns exit code (-1 to continue).
func parseOneArg(args []string, i *int, delims *[]string, files *[]string) int {
	arg := args[*i]
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case arg == "-d" || arg == "--delimiters":
		return consumeDelimArg(args, i, delims)
	case strings.HasPrefix(arg, "-d") && len(arg) > 2:
		*delims = parseDelimiters(arg[2:])
	case strings.HasPrefix(arg, "--delimiters="):
		*delims = parseDelimiters(arg[len("--delimiters="):])
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		fmt.Fprintf(os.Stderr, "paste: unrecognized option '%s'\n", arg)
		return 1
	default:
		*files = append(*files, arg)
	}
	return -1
}

// consumeDelimArg reads the next argument as the delimiter string.
func consumeDelimArg(args []string, i *int, delims *[]string) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"paste: option requires an argument -- 'd'\n")
		return 1
	}
	*i++
	*delims = parseDelimiters(args[*i])
	return -1
}

// parseDelimiters interprets the delimiter string with escape sequences.
// R2.1: \n (newline), \t (tab), \\ (backslash), \0 (empty string).
func parseDelimiters(s string) []string {
	var delims []string
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			d, skip := parseEscape(s[i+1])
			delims = append(delims, d)
			if skip {
				i++
			}
		} else {
			delims = append(delims, string(s[i]))
		}
	}
	if len(delims) == 0 {
		delims = []string{""}
	}
	return delims
}

// parseEscape returns the unescaped delimiter string and whether to
// advance past the escape character.
func parseEscape(c byte) (string, bool) {
	switch c {
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	case '\\':
		return "\\", true
	case '0':
		return "", true
	default:
		return "\\", false
	}
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: paste [OPTION]... [FILE]...
Write lines consisting of the sequentially corresponding lines from
each FILE, separated by TABs, to standard output.

With no FILE, or when FILE is -, read standard input.

  -d, --delimiters=LIST   reuse characters from LIST instead of TABs
      --help               display this help and exit
      --version            output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "paste (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
