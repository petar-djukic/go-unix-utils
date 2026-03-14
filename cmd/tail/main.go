// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd055-tail R1.1–R1.4, R2.1–R2.3, R3.1–R3.4, R4.1–R4.4
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

// defaultLines is the number of lines printed when -n is not specified.
// R1.1: default is 10 lines.
const defaultLines = 10

func main() {
	// D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// Parse arguments manually to support GNU-style -n N, -nN, --lines=N,
	// -c N, -cN, --bytes=N, -q, --quiet, --silent, -v, --verbose, --version, --help.
	args := os.Args[1:]

	var (
		countStr   string
		byteStr    string
		quiet      bool
		verbose    bool
		files      []string
		endOfFlags bool
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "--version" {
			fmt.Println("tail (go-unix-utils)")
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if v, ok := strings.CutPrefix(arg, "--lines="); ok {
			countStr = v
			continue
		}
		if v, ok := strings.CutPrefix(arg, "--bytes="); ok {
			byteStr = v
			continue
		}
		if arg == "--quiet" || arg == "--silent" {
			quiet = true
			continue
		}
		if arg == "--verbose" {
			verbose = true
			continue
		}
		if arg == "-q" {
			quiet = true
			continue
		}
		if arg == "-v" {
			verbose = true
			continue
		}
		// -n N or -nN
		if arg == "-n" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "tail: option requires an argument -- 'n'\n")
				os.Exit(1)
			}
			countStr = args[i]
			continue
		}
		if strings.HasPrefix(arg, "-n") {
			countStr = arg[2:]
			continue
		}
		// -c N or -cN
		if arg == "-c" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "tail: option requires an argument -- 'c'\n")
				os.Exit(1)
			}
			byteStr = args[i]
			continue
		}
		if strings.HasPrefix(arg, "-c") {
			byteStr = arg[2:]
			continue
		}
		// Handle combined short flags like -qv or numeric-only args like -5
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// Check if it's a legacy numeric option like -5
			if _, err := strconv.Atoi(arg[1:]); err == nil {
				countStr = arg[1:]
				continue
			}
			// Unknown flag
			fmt.Fprintf(os.Stderr, "tail: invalid option -- '%s'\n", arg[1:])
			os.Exit(1)
		}
		files = append(files, arg)
	}

	// R2.1: determine byte mode.
	byteMode := byteStr != ""
	var byteCount int64
	var byteFromStart bool
	if byteMode {
		var err error
		byteCount, byteFromStart, err = parseCount(byteStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail: invalid number of bytes: '%s'\n", byteStr)
			os.Exit(1)
		}
	}

	// R1.2: parse line count.
	lineCount := int64(defaultLines)
	lineFromStart := false
	if countStr != "" {
		var err error
		lineCount, lineFromStart, err = parseCount(countStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tail: invalid number of lines: '%s'\n", countStr)
			os.Exit(1)
		}
	}

	w := bufio.NewWriter(os.Stdout)

	processInput := func(r io.Reader) error {
		if byteMode {
			if byteFromStart {
				// R2.2: +N starts from byte N (1-based).
				return tailBytesFromStart(w, r, byteCount)
			}
			return tailBytes(w, r, byteCount)
		}
		if lineFromStart {
			// R1.3: +N starts from line N (1-based).
			return tailLinesFromStart(w, r, lineCount)
		}
		return tailLines(w, r, lineCount)
	}

	exitCode := 0

	if len(files) == 0 {
		// R1.4: no file arguments — read from stdin.
		if verbose {
			fmt.Fprintf(w, "==> standard input <==\n")
		}
		if err := processInput(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "tail: %v\n", err)
			exitCode = 1
		}
	} else {
		// R3.1, R3.2: show headers when multiple files.
		// R3.3: -q suppresses headers. R3.4: -v forces headers.
		showHeaders := len(files) > 1
		if quiet {
			showHeaders = false
		}
		if verbose {
			showHeaders = true
		}

		printed := false
		for _, arg := range files {
			r, closer, openErr := openInput(arg)
			if openErr != nil {
				// R4.4: file cannot be opened — no header, print error, continue.
				fmt.Fprintf(os.Stderr, "tail: %v\n", openErr)
				exitCode = 1
				continue
			}

			if showHeaders {
				// R3.1: blank line between files.
				if printed {
					fmt.Fprintln(w)
				}
				name := arg
				if arg == "-" {
					name = "standard input"
				}
				fmt.Fprintf(w, "==> %s <==\n", name)
			}
			printed = true

			// R4.2, R4.4: check if the file is readable (e.g., not a directory).
			// GNU tail prints the header first, then reports the read error.
			if f, ok := r.(*os.File); ok && arg != "-" {
				if err := checkReadable(arg, f); err != nil {
					w.Flush() // best-effort flush before stderr
					fmt.Fprintf(os.Stderr, "tail: %v\n", err)
					exitCode = 1
					closer.Close() // best-effort cleanup
					continue
				}
			}

			if err := processInput(r); err != nil {
				w.Flush() // best-effort flush before stderr
				fmt.Fprintf(os.Stderr, "tail: %v\n", err)
				exitCode = 1
			}
			if closer != nil {
				closer.Close() // best-effort cleanup, error ignored
			}
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "tail: write error: %v\n", err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// openInput opens name for reading and returns the reader and an optional closer.
// R1.4: "-" returns stdin with no closer.
// R4.4: returns a descriptive error when the file cannot be opened.
func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		if pathErr, ok := errors.AsType[*os.PathError](err); ok {
			return nil, nil, fmt.Errorf("cannot open '%s' for reading: %v", name, pathErr.Err)
		}
		return nil, nil, err
	}
	return f, f, nil
}

// checkReadable checks if an opened file is actually readable (not a directory).
// R4.2, R4.4: GNU tail prints "error reading 'name': Is a directory" after the
// header for directory arguments.
func checkReadable(name string, f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("error reading '%s': %v", name, err)
	}
	if info.IsDir() {
		return fmt.Errorf("error reading '%s': Is a directory", name)
	}
	return nil
}

// tailLines reads all input from r and writes the last n lines to w.
// R1.1, R1.2: prints the last n lines.
func tailLines(w *bufio.Writer, r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	br := bufio.NewReader(r)
	var lines []string
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}
	start := max(int64(len(lines))-n, 0)
	for _, line := range lines[start:] {
		if _, err := io.WriteString(w, line); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	return nil
}

// tailLinesFromStart reads from r and writes all lines starting from line n (1-based).
// R1.3: +N starts from line N.
func tailLinesFromStart(w *bufio.Writer, r io.Reader, n int64) error {
	br := bufio.NewReader(r)
	lineNum := int64(0)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			lineNum++
			if lineNum >= n {
				if _, werr := io.WriteString(w, line); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
	}
	return nil
}

// tailBytes reads all input from r and writes the last n bytes to w.
// R2.1: byte-count mode.
func tailBytes(w *bufio.Writer, r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	start := max(int64(len(data))-n, 0)
	if _, werr := w.Write(data[start:]); werr != nil {
		return fmt.Errorf("write error: %w", werr)
	}
	return nil
}

// tailBytesFromStart reads from r and writes all bytes starting from byte n (1-based).
// R2.2: +N starts from byte N.
func tailBytesFromStart(w *bufio.Writer, r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	start := max(n-1, 0)
	if start >= int64(len(data)) {
		return nil
	}
	if _, werr := w.Write(data[start:]); werr != nil {
		return fmt.Errorf("write error: %w", werr)
	}
	return nil
}

// parseCount parses a count string that may include a '+' prefix for from-start
// mode and/or a multiplier suffix. Returns the absolute value, whether the count
// is from-start (+N), and any parse error.
// R2.3: supports GNU coreutils block size suffixes: b (512), kB (1000), K/KiB (1024),
// MB (1000^2), M/MiB (1024^2), GB (1000^3), G/GiB (1024^3), TB (1000^4), T/TiB (1024^4),
// PB (1000^5), P/PiB (1024^5), EB (1000^6), E/EiB (1024^6), ZB (1000^7), Z/ZiB (1024^7),
// YB (1000^8), Y/YiB (1024^8).
func parseCount(s string) (int64, bool, error) {
	fromStart := false
	if strings.HasPrefix(s, "+") {
		fromStart = true
		s = s[1:]
	}

	// Find boundary between digits and suffix.
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false, fmt.Errorf("invalid number: %q", s)
	}

	num, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, false, err
	}

	suffix := s[i:]
	multiplier, ok := suffixMultipliers[suffix]
	if !ok {
		return 0, false, fmt.Errorf("invalid suffix: %q", suffix)
	}

	return num * multiplier, fromStart, nil
}

// suffixMultipliers maps GNU coreutils block size suffixes to their byte multipliers.
// R2.3: SI suffixes (kB, MB, GB, ...) use powers of 1000; binary suffixes
// (K/KiB, M/MiB, G/GiB, ...) use powers of 1024. b=512 is a special case.
var suffixMultipliers = map[string]int64{
	"":    1,
	"b":   512,
	"kB":  1000,
	"K":   1024,
	"KiB": 1024,
	"MB":  1000 * 1000,
	"M":   1024 * 1024,
	"MiB": 1024 * 1024,
	"GB":  1000 * 1000 * 1000,
	"G":   1024 * 1024 * 1024,
	"GiB": 1024 * 1024 * 1024,
	"TB":  1000 * 1000 * 1000 * 1000,
	"T":   1024 * 1024 * 1024 * 1024,
	"TiB": 1024 * 1024 * 1024 * 1024,
	"PB":  1000 * 1000 * 1000 * 1000 * 1000,
	"P":   1024 * 1024 * 1024 * 1024 * 1024,
	"PiB": 1024 * 1024 * 1024 * 1024 * 1024,
	"EB":  1000 * 1000 * 1000 * 1000 * 1000 * 1000,
	"E":   1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	"EiB": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: tail [OPTION]... [FILE]...
Print the last 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes=NUM        output the last NUM bytes; with leading '+',
                         output starting with byte NUM of each file
  -n, --lines=NUM        output the last NUM lines instead of the last 10;
                         with leading '+', output starting with line NUM
  -q, --quiet, --silent  never output headers giving file names
  -v, --verbose          always output headers giving file names
      --help             display this help and exit
      --version          output version information and exit
`)
}
