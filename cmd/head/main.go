// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/head: print the first lines or bytes of files.
// Implements srd018-head R1.1-R1.5, R2.1-R2.3, R3.1-R3.5, R4.1-R4.4.
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

// progName is used in diagnostic messages.
const progName = "head"

// defaultLines is the number of lines printed when no -n flag is given.
// R1.1: default is 10 lines.
const defaultLines int64 = 10

// countMode distinguishes line-count from byte-count mode.
type countMode int

const (
	modeLines countMode = iota
	modeBytes
)

// config holds parsed command-line options.
type config struct {
	mode     countMode
	count    int64
	negative bool
	quiet    bool
	verbose  bool
	files    []string
	err      bool // R4.4: set when a parse error occurs
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run executes the head logic and returns the exit code.
// R4.1: returns 0 when all files processed successfully.
// R4.2: returns 1 when any file cannot be opened or read.
// R4.4: returns 1 when a non-numeric argument is given for -n or -c.
func run(args []string) int {
	cfg := parseArgs(args)
	if cfg.err {
		return 1
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	showHeader := shouldShowHeader(&cfg)
	exitCode := 0
	printed := 0
	for _, name := range cfg.files {
		if err := processOneFile(name, &cfg, showHeader, &printed); err != nil {
			reportError(name, err)
			exitCode = 1
		}
	}
	return exitCode
}

// shouldShowHeader determines whether file headers should be printed.
// R3.2: no header for single file by default.
// R3.3: -q suppresses all headers.
// R3.4: -v forces headers even for single file.
func shouldShowHeader(cfg *config) bool {
	if cfg.quiet {
		return false
	}
	if cfg.verbose {
		return true
	}
	return len(cfg.files) > 1
}

// displayName returns the user-visible name for a file argument.
// R3.1: stdin ("-") is displayed as "standard input" to match GNU head.
func displayName(name string) string {
	if name == "-" {
		return "standard input"
	}
	return name
}

// printHeader writes the GNU head-format header for a file.
// R3.1: format is "==> FILENAME <==" with blank line between files.
func printHeader(name string, first bool) {
	if !first {
		fmt.Fprintln(os.Stdout)
	}
	fmt.Fprintf(os.Stdout, "==> %s <==\n", displayName(name))
}

// processOneFile opens a file, prints a header if needed, then outputs content.
// R3.5: prints error and continues; header is printed only on successful open.
func processOneFile(name string, cfg *config, showHeader bool, printed *int) error {
	r, closer, err := openInput(name)
	if err != nil {
		return err
	}
	defer closer()
	if showHeader {
		printHeader(name, *printed == 0)
	}
	*printed++
	if cfg.mode == modeBytes {
		return processByteMode(r, cfg.count, cfg.negative)
	}
	return processLineMode(r, cfg.count, cfg.negative)
}

// processLineMode handles line-count output.
func processLineMode(r io.Reader, count int64, negative bool) error {
	if negative {
		return printAllButLastNLines(r, count)
	}
	return printFirstNLines(r, count)
}

// processByteMode handles byte-count output.
// R2.1: -c NUM prints first NUM bytes.
// R2.2: -c -NUM prints all except last NUM bytes.
func processByteMode(r io.Reader, count int64, negative bool) error {
	if negative {
		return printAllButLastNBytes(r, count)
	}
	return printFirstNBytes(r, count)
}

// openInput opens a file for reading, or returns stdin for "-".
// R1.4: stdin when file argument is "-".
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

// parseArgs extracts flags and file arguments.
// R2.1: -c and -n are mutually exclusive; the last one given wins.
func parseArgs(args []string) config {
	cfg := config{count: defaultLines}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			return cfg
		}
		if consumed := parseHeaderFlag(&cfg, arg); consumed > 0 {
			continue
		}
		if consumed := parseCountFlag(&cfg, args, i); consumed > 0 {
			i += consumed - 1
			continue
		}
		cfg.files = append(cfg.files, arg)
	}
	return cfg
}

// parseHeaderFlag handles -q/--quiet/--silent and -v/--verbose flags.
// R3.3: -q suppresses headers.
// R3.4: -v forces headers.
func parseHeaderFlag(cfg *config, arg string) int {
	switch {
	case arg == "-q" || arg == "--quiet" || arg == "--silent":
		cfg.quiet = true
		cfg.verbose = false
		return 1
	case arg == "-v" || arg == "--verbose":
		cfg.verbose = true
		cfg.quiet = false
		return 1
	}
	return 0
}

// parseCountFlag handles -n/--lines and -c/--bytes flags.
func parseCountFlag(cfg *config, args []string, i int) int {
	mode, numStr, consumed := matchCountFlag(args[i], args, i)
	if consumed == 0 {
		return 0
	}
	cfg.mode = mode
	if mode == modeBytes {
		cfg.count, cfg.negative, cfg.err = parseByteCount(numStr)
	} else {
		cfg.count, cfg.negative, cfg.err = parseLineCount(numStr)
	}
	return consumed
}

// matchCountFlag identifies -n/-c flags and extracts the numeric string.
func matchCountFlag(arg string, args []string, i int) (countMode, string, int) {
	switch {
	case arg == "-n" || arg == "--lines":
		if i+1 < len(args) {
			return modeLines, args[i+1], 2
		}
		return modeLines, "", 1
	case strings.HasPrefix(arg, "--lines="):
		return modeLines, arg[len("--lines="):], 1
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'n':
		return modeLines, arg[2:], 1
	case arg == "-c" || arg == "--bytes":
		if i+1 < len(args) {
			return modeBytes, args[i+1], 2
		}
		return modeBytes, "", 1
	case strings.HasPrefix(arg, "--bytes="):
		return modeBytes, arg[len("--bytes="):], 1
	case len(arg) > 2 && arg[0] == '-' && arg[1] == 'c':
		return modeBytes, arg[2:], 1
	}
	return 0, "", 0
}

// parseLineCount parses a line count string, detecting negative prefix.
// R1.2: NUM is a positive integer.
// R1.3: NUM prefixed with '-' enables negative mode.
// R4.4: returns err=true for non-numeric input.
func parseLineCount(s string) (int64, bool, bool) {
	neg := false
	raw := s
	if strings.HasPrefix(s, "-") {
		neg = true
		raw = s[1:]
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid number of lines: '%s'\n", progName, s)
		return 0, false, true
	}
	return n, neg, false
}

// parseByteCount parses a byte count string with optional suffix.
// R2.3: supports b, K/KiB, M/MiB, G/GiB suffixes via sizeparse.
// R4.4: returns err=true for non-numeric input.
func parseByteCount(s string) (int64, bool, bool) {
	neg := false
	raw := s
	if strings.HasPrefix(s, "-") {
		neg = true
		raw = s[1:]
	}
	n, err := sizeparse.Parse(raw)
	if err != nil || n < 0 {
		fmt.Fprintf(os.Stderr, "%s: invalid number of bytes: '%s'\n", progName, s)
		return 0, false, true
	}
	return n, neg, false
}

// printFirstNLines writes the first n lines from r to stdout.
// R1.1, R1.2: output first N lines.
// R1.5: a line without a trailing newline is still counted.
func printFirstNLines(r io.Reader, n int64) error {
	br := bufio.NewReader(r)
	for i := int64(0); i < n; i++ {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := os.Stdout.Write(line); werr != nil {
				return werr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
	return nil
}

// printFirstNBytes writes the first n bytes from r to stdout.
// R2.1: -c NUM prints the first NUM bytes.
func printFirstNBytes(r io.Reader, n int64) error {
	_, err := io.CopyN(os.Stdout, r, n)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// printAllButLastNLines writes all lines except the last n.
// R1.3: requires buffering input in a ring buffer.
func printAllButLastNLines(r io.Reader, n int64) error {
	if n <= 0 {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	return drainLineRing(bufio.NewReader(r), int(n))
}

// drainLineRing reads lines into a ring buffer, outputting evicted lines.
func drainLineRing(br *bufio.Reader, n int) error {
	ring := make([][]byte, n)
	idx := 0
	total := 0
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if total >= n {
				if _, werr := os.Stdout.Write(ring[idx]); werr != nil {
					return werr
				}
			}
			ring[idx] = line
			idx = (idx + 1) % n
			total++
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// printAllButLastNBytes writes all bytes except the last n.
// R2.2: -c -N prints all bytes except the last N.
func printAllButLastNBytes(r io.Reader, n int64) error {
	if n <= 0 {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	end := int64(len(data)) - n
	if end <= 0 {
		return nil
	}
	_, werr := os.Stdout.Write(data[:end])
	return werr
}

// reportError prints a GNU-compatible diagnostic to stderr.
// R3.5: prints error and continues with remaining files.
func reportError(name string, err error) {
	dname := displayName(name)
	var pe *os.PathError
	if errors.As(err, &pe) {
		fmt.Fprintf(os.Stderr, "%s: cannot open '%s' for reading: %s\n",
			progName, dname, pe.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, dname, err)
}
