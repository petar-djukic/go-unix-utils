// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd018-head R1.1–R1.5, R2.1–R2.3, R3.1–R3.5, R4.1–R4.3
package main

import (
	"bufio"
	"errors"
	"flag"
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

	// R4.1, R4.2: Handle --version and --help before flag parsing so they
	// take precedence and exit 0, matching GNU head behavior.
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--version" {
			fmt.Println("head (go-unix-utils)")
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
	}

	flagN := flag.Int("n", defaultLines, "print the first NUM lines instead of the first 10")
	flag.IntVar(flagN, "lines", defaultLines, "print the first NUM lines instead of the first 10")
	flagC := flag.String("c", "", "print the first NUM bytes")
	flag.StringVar(flagC, "bytes", "", "print the first NUM bytes")
	flagQ := flag.Bool("q", false, "never print headers giving file names")
	flag.BoolVar(flagQ, "quiet", false, "never print headers giving file names")
	flag.BoolVar(flagQ, "silent", false, "never print headers giving file names")
	flagV := flag.Bool("v", false, "always print headers giving file names")
	flag.BoolVar(flagV, "verbose", false, "always print headers giving file names")
	flag.Parse()

	// R2.1: determine byte mode. D2: -c takes precedence if both specified.
	byteMode := *flagC != ""
	var byteCount int64
	var byteNegative bool
	if byteMode {
		var err error
		byteCount, byteNegative, err = parseCount(*flagC)
		if err != nil {
			fmt.Fprintf(os.Stderr, "head: invalid number of bytes: %s\n", quote(*flagC))
			os.Exit(1)
		}
	}

	w := bufio.NewWriter(os.Stdout)

	processInput := func(r io.Reader) error {
		if byteMode {
			if byteNegative {
				// R2.2: negative byte count — all bytes except last N.
				return headBytesNegative(w, r, byteCount)
			}
			return headBytes(w, r, byteCount)
		}
		return headLines(w, r, *flagN)
	}

	args := flag.Args()
	exitCode := 0

	if len(args) == 0 {
		// R1.4: no file arguments — read from stdin.
		// R3.4: -v forces headers even for single input.
		if *flagV {
			fmt.Fprintf(w, "==> standard input <==\n")
		}
		if err := processInput(os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "head: %v\n", err)
			exitCode = 1
		}
	} else {
		// R3.1, R3.2: show headers when multiple files.
		// R3.3: -q suppresses headers. R3.4: -v forces headers.
		showHeaders := len(args) > 1
		if *flagQ {
			showHeaders = false
		}
		if *flagV {
			showHeaders = true
		}

		printed := false // tracks whether any file output has been written
		for _, arg := range args {
			r, closer, err := openInput(arg)
			if err != nil {
				// R3.5: print error and continue.
				fmt.Fprintf(os.Stderr, "head: %v\n", err)
				exitCode = 1
				continue
			}
			if showHeaders {
				// R3.1: blank line between files (before header of second and subsequent files).
				if printed {
					fmt.Fprintln(w)
				}
				name := arg
				if arg == "-" {
					name = "standard input"
				}
				fmt.Fprintf(w, "==> %s <==\n", name)
			}
			if err := processInput(r); err != nil {
				fmt.Fprintf(os.Stderr, "head: %v\n", err)
				exitCode = 1
			}
			printed = true
			if closer != nil {
				closer.Close() // best-effort cleanup, error ignored
			}
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "head: write error: %v\n", err)
		os.Exit(1)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// quote wraps s in single quotes for error messages, matching GNU head format.
func quote(s string) string {
	return "'" + s + "'"
}

// openInput opens name for reading and returns the reader and an optional closer.
// R1.4: "-" returns stdin with no closer.
// R3.1, R3.2: formats errors to match GNU head: cannot open 'NAME' for reading: REASON.
func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return nil, nil, fmt.Errorf("cannot open '%s' for reading: %v", name, pathErr.Err)
		}
		return nil, nil, err
	}
	return f, f, nil
}

// headLines reads from r and writes the first n lines to w.
// R1.1, R1.2, R1.3: prints exactly n lines.
// R1.5: a line is terminated by a newline; the last line without a trailing
// newline is still counted.
func headLines(w *bufio.Writer, r io.Reader, n int) error {
	if n <= 0 {
		return nil
	}
	br := bufio.NewReader(r)
	count := 0
	for count < n {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if _, werr := io.WriteString(w, line); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			count++
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

// headBytes reads from r and writes the first n bytes to w.
// R2.1: byte-count mode.
func headBytes(w *bufio.Writer, r io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}
	_, err := io.CopyN(w, r, n)
	if err != nil && err != io.EOF {
		return fmt.Errorf("copy error: %w", err)
	}
	return nil
}

// headBytesNegative reads all input from r and writes all bytes except the
// last n to w.
// R2.2: negative byte count.
func headBytesNegative(w *bufio.Writer, r io.Reader, n int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	end := int64(len(data)) - n
	if end <= 0 {
		return nil
	}
	if _, werr := w.Write(data[:end]); werr != nil {
		return fmt.Errorf("write error: %w", werr)
	}
	return nil
}

// parseCount parses a count string that may include a negative prefix ("-")
// and/or a multiplier suffix. Returns the absolute value, whether the count
// is negative, and any parse error.
// R2.3: supports b (512), K/KiB (1024), M/MiB (1048576), G/GiB (1073741824).
func parseCount(s string) (int64, bool, error) {
	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
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
	var multiplier int64
	switch suffix {
	case "":
		multiplier = 1
	case "b":
		multiplier = 512
	case "K", "KiB":
		multiplier = 1024
	case "M", "MiB":
		multiplier = 1048576
	case "G", "GiB":
		multiplier = 1073741824
	default:
		return 0, false, fmt.Errorf("invalid suffix: %q", suffix)
	}

	return num * multiplier, negative, nil
}

// printHelp writes usage information to stdout, matching the structure of
// GNU head --help output.
// R4.2: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: head [OPTION]... [FILE]...
Print the first 10 lines of each FILE to standard output.
With more than one FILE, precede each with a header giving the file name.

With no FILE, or when FILE is -, read standard input.

  -c, --bytes=NUM        print the first NUM bytes; with leading '-',
                         print all but the last NUM bytes of each file
  -n, --lines=NUM        print the first NUM lines instead of the first 10;
                         with leading '-', print all but the last NUM lines
  -q, --quiet, --silent  never print headers giving file names
  -v, --verbose          always print headers giving file names
      --help             display this help and exit
      --version          output version information and exit
`)
}
