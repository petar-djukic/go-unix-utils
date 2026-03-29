// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/tail implements GNU tail: print the last lines or bytes of files.
//
// Implements prd055-tail R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3,
// R3.1, R3.2, R3.3, R3.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultLines = 10
	programName  = "tail"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// tailOptions holds parsed flag state.
type tailOptions struct {
	count     int64
	fromStart bool // true when +N prefix is used
	byteMode  bool // R2.1: true when -c is used
	quiet     bool
	verbose   bool
}

// run parses flags and processes input files.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", programName, err)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	return processFiles(files, stdin, stdout, stderr, opts)
}

// showHeaders returns true when file headers should be printed.
// R3.1: headers for multiple files. R3.3: -q suppresses. R3.4: -v forces.
func showHeaders(fileCount int, opts tailOptions) bool {
	if opts.quiet {
		return false
	}
	if opts.verbose {
		return true
	}
	return fileCount > 1
}

// processFiles iterates over files and prints tail output for each.
// R4.1: exit 0 when all files succeed. R4.2: exit 1 on any error.
func processFiles(files []string, stdin io.Reader, stdout, stderr io.Writer, opts tailOptions) int {
	exitCode := 0
	headers := showHeaders(len(files), opts)
	for i, name := range files {
		if headers {
			printHeader(stdout, name, i > 0)
		}
		if err := tailFile(name, stdin, stdout, opts); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// printHeader prints the '==> FILENAME <==' header line.
// R3.1: blank line before header when not the first file.
func printHeader(w io.Writer, name string, preceded bool) {
	if preceded {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "==> %s <==\n", fileDisplayName(name))
}

// fileDisplayName returns the display name for a file argument.
// R1.4: "-" is displayed as "standard input".
func fileDisplayName(name string) string {
	if name == "-" {
		return "standard input"
	}
	return name
}

// parseArgs separates flags from file arguments.
func parseArgs(args []string) (tailOptions, []string, error) {
	opts := tailOptions{count: defaultLines}
	var files []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (len(arg) > 0 && arg[0] != '-') {
			files = append(files, arg)
			continue
		}
		if arg == "-" {
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
func parseFlag(opts *tailOptions, flagsDone *bool, args []string, i int) (int, error) {
	arg := args[i]
	switch {
	case arg == "--":
		*flagsDone = true
	case arg == "-q", arg == "--quiet", arg == "--silent":
		opts.quiet = true
		opts.verbose = false
	case arg == "-v", arg == "--verbose":
		opts.verbose = true
		opts.quiet = false
	case strings.HasPrefix(arg, "--lines="):
		return i, parseLinesValue(opts, arg[len("--lines="):])
	case arg == "--lines", arg == "-n":
		return parseNextArg(opts, args, i, arg, parseLinesValue)
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'n':
		return i, parseLinesValue(opts, arg[2:])
	// R2.1: byte-count mode flags
	case strings.HasPrefix(arg, "--bytes="):
		return i, parseBytesValue(opts, arg[len("--bytes="):])
	case arg == "--bytes", arg == "-c":
		return parseNextArg(opts, args, i, arg, parseBytesValue)
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'c':
		return i, parseBytesValue(opts, arg[2:])
	case isLegacyNumArg(arg):
		return i, parseLegacyNum(opts, arg)
	default:
		return i, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
	return i, nil
}

// parseNextArg reads the next argument as the value for a flag.
func parseNextArg(opts *tailOptions, args []string, i int, flag string, parse func(*tailOptions, string) error) (int, error) {
	i++
	if i >= len(args) {
		return i, fmt.Errorf("option '%s' requires an argument", flag)
	}
	return i, parse(opts, args[i])
}

// parseLinesValue parses a line count value which may have a + prefix.
// R1.2: positive integer sets line count.
// R1.3: + prefix means start from that line number.
// R2.1: -n sets byteMode to false (last flag wins).
func parseLinesValue(opts *tailOptions, s string) error {
	raw := s
	fromStart := false
	if strings.HasPrefix(s, "+") {
		fromStart = true
		s = s[1:]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of lines: '%s'", raw)
	}
	opts.count = n
	opts.fromStart = fromStart
	opts.byteMode = false
	return nil
}

// parseBytesValue parses a byte count value with optional + prefix and suffixes.
// R2.1: sets byte mode. R2.2: + prefix for offset from start.
// R2.3: supports multiplier suffixes via sizeparse.
func parseBytesValue(opts *tailOptions, s string) error {
	raw := s
	fromStart := false
	if strings.HasPrefix(s, "+") {
		fromStart = true
		s = s[1:]
	}
	n, err := sizeparse.Parse(s)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid number of bytes: '%s'", raw)
	}
	opts.count = n
	opts.fromStart = fromStart
	opts.byteMode = true
	return nil
}

// isLegacyNumArg checks for legacy -NUM form (e.g., tail -5).
func isLegacyNumArg(arg string) bool {
	return len(arg) > 1 && arg[0] == '-' && arg[1] >= '0' && arg[1] <= '9'
}

// parseLegacyNum parses legacy -NUM form.
func parseLegacyNum(opts *tailOptions, arg string) error {
	n, err := strconv.ParseInt(arg[1:], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid number of lines: '%s'", arg[1:])
	}
	opts.count = n
	opts.fromStart = false
	return nil
}

// tailFile processes a single input file or stdin.
// R1.4: "-" means read from stdin.
func tailFile(name string, stdin io.Reader, stdout io.Writer, opts tailOptions) error {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close() // best-effort close
	}
	if opts.byteMode {
		return tailBytes(r, stdout, opts)
	}
	return tailLines(r, stdout, opts)
}

// tailLines dispatches line-mode output.
func tailLines(r io.Reader, w io.Writer, opts tailOptions) error {
	if opts.fromStart {
		return tailFromLineOffset(r, w, opts.count)
	}
	return tailLastLines(r, w, int(opts.count))
}

// tailBytes dispatches byte-mode output.
// R2.1: last N bytes. R2.2: from byte offset.
func tailBytes(r io.Reader, w io.Writer, opts tailOptions) error {
	if opts.fromStart {
		return tailFromByteOffset(r, w, opts.count)
	}
	return tailLastBytes(r, w, opts.count)
}

// openInput returns a reader and optional closer for the given filename.
// R1.4: "-" means stdin.
// R4.4: open failures use GNU format "cannot open 'FILE' for reading: REASON".
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

// tailLastLines prints the last n lines from r.
// R1.1: default 10 lines. R1.2: configurable via -n.
func tailLastLines(r io.Reader, w io.Writer, n int) error {
	if n <= 0 {
		return nil
	}
	lines, err := readAllLines(r)
	if err != nil {
		return err
	}
	start := max(len(lines)-n, 0)
	return writeLines(w, lines[start:])
}

// tailFromLineOffset prints from line number offset to end of input.
// R1.3: line numbering starts at 1.
func tailFromLineOffset(r io.Reader, w io.Writer, offset int64) error {
	br := bufio.NewReaderSize(r, 64*1024)
	bw := bufio.NewWriter(w)
	var lineNum int64
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			lineNum++
			if lineNum >= offset {
				if _, wErr := bw.Write(line); wErr != nil {
					return wErr
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = bw.Flush() // best-effort flush before returning read error
			return err
		}
	}
	return bw.Flush()
}

// tailLastBytes prints the last n bytes from r.
// R2.1: byte-count mode.
func tailLastBytes(r io.Reader, w io.Writer, n int64) error {
	if n <= 0 {
		return nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	start := max(int64(len(data))-n, 0)
	_, wErr := w.Write(data[start:])
	return wErr
}

// tailFromByteOffset prints from byte offset to end of input.
// R2.2: byte numbering starts at 1.
func tailFromByteOffset(r io.Reader, w io.Writer, offset int64) error {
	if offset > 1 {
		discarded, err := io.CopyN(io.Discard, r, offset-1)
		if err != nil && err != io.EOF {
			return err
		}
		if discarded < offset-1 {
			return nil // offset beyond input
		}
	}
	_, err := io.Copy(w, r)
	return err
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

// writeLines writes a slice of lines to w using buffered I/O.
func writeLines(w io.Writer, lines [][]byte) error {
	bw := bufio.NewWriter(w)
	for _, line := range lines {
		if _, err := bw.Write(line); err != nil {
			return err
		}
	}
	return bw.Flush()
}
