// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd018-head R1.1-R1.5, R2.1-R2.3, R3.1-R3.5, R4.1-R4.4:
// cmd/head prints the first N lines or bytes of each input file. Defaults to
// 10 lines. Supports -n (line count), -c (byte count), negative counts (all
// but last N), multi-file headers with -q/-v control, multiplier suffixes
// (b, K/KiB, M/MiB, G/GiB), stdin input, error diagnostics for unreadable
// files, and correct exit codes. Installs SIGPIPE handler per ARCHITECTURE.yaml.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU head format.
const progName = "head"

// mode selects between line-count and byte-count operation.
type mode int

const (
	modeLines mode = iota
	modeBytes
)

// headOptions holds the parsed flags for a head invocation.
type headOptions struct {
	mode     mode
	count    int64 // absolute count value (always >= 0)
	negative bool  // true when prefixed with '-' (print all but last N)
	quiet    bool  // -q: suppress headers
	verbose  bool  // -v: force headers
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])
	exitCode := 0

	if len(files) == 0 {
		// R1.4: no file arguments — read from stdin.
		if err := headReader(os.Stdin, opts); err != nil {
			if isEPIPE(err) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
		os.Exit(exitCode)
	}

	showHeaders := len(files) > 1
	if opts.quiet {
		showHeaders = false
	}
	if opts.verbose {
		showHeaders = true
	}

	printed := false // tracks whether any file has produced output
	for _, name := range files {
		if name == "-" {
			if showHeaders {
				if printed {
					fmt.Fprintln(os.Stdout)
				}
				fmt.Fprintf(os.Stdout, "==> standard input <==\n")
			}
			printed = true
			// R1.4: "-" means read from stdin.
			if err := headReader(os.Stdin, opts); err != nil {
				if isEPIPE(err) {
					os.Exit(0)
				}
				fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			// R3.5: print error and continue processing remaining files.
			fmt.Fprintf(os.Stderr, "%s: cannot open '%s' for reading: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		if showHeaders {
			if printed {
				fmt.Fprintln(os.Stdout)
			}
			fmt.Fprintf(os.Stdout, "==> %s <==\n", name)
		}
		printed = true
		if err := headReader(f, opts); err != nil {
			if isEPIPE(err) {
				f.Close() // best-effort close before exit
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: error reading '%s': %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close
	}

	os.Exit(exitCode)
}

// headReader dispatches to the appropriate handler based on mode and negativity.
func headReader(r io.Reader, opts *headOptions) error {
	if opts.mode == modeBytes {
		if opts.negative {
			return headBytesNegative(r, opts.count)
		}
		return headBytesPositive(r, opts.count)
	}
	if opts.negative {
		return headLinesNegative(r, opts.count)
	}
	return headLinesPositive(r, opts.count)
}

// headLinesPositive prints the first n lines from r.
// R1.5: a line is terminated by newline; a final line without newline counts.
func headLinesPositive(r io.Reader, n int64) error {
	if n == 0 {
		return nil
	}
	br := bufio.NewReader(r)
	w := bufio.NewWriter(os.Stdout)
	var count int64
	for count < n {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
			count++
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return w.Flush()
}

// headLinesNegative prints all lines except the last n lines from r.
// R1.3: requires buffering to determine where to stop.
func headLinesNegative(r io.Reader, n int64) error {
	if n == 0 {
		// Print everything.
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	// Buffer all lines, then output all but the last n.
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	var lines [][]byte
	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	end := int64(len(lines)) - n
	if end < 0 {
		end = 0
	}
	w := bufio.NewWriter(os.Stdout)
	for i := int64(0); i < end; i++ {
		if _, err := w.Write(lines[i]); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}

// headBytesPositive prints the first n bytes from r.
func headBytesPositive(r io.Reader, n int64) error {
	if n == 0 {
		return nil
	}
	_, err := io.Copy(os.Stdout, io.LimitReader(r, n))
	return err
}

// headBytesNegative prints all bytes except the last n bytes from r.
// R2.2: requires buffering to determine where to stop.
func headBytesNegative(r io.Reader, n int64) error {
	if n == 0 {
		_, err := io.Copy(os.Stdout, r)
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	end := int64(len(data)) - n
	if end < 0 {
		end = 0
	}
	_, werr := os.Stdout.Write(data[:end])
	return werr
}

// parseArgs separates flags from file arguments. GNU head accepts:
//   - -n NUM, --lines=NUM, --lines NUM
//   - -c NUM, --bytes=NUM, --bytes NUM
//   - -NUM as shorthand for -n NUM
//   - -q/--quiet/--silent, -v/--verbose
func parseArgs(args []string) (*headOptions, []string) {
	opts := &headOptions{
		mode:  modeLines,
		count: 10, // R1.1: default 10 lines
	}
	var files []string
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

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

		// R4.1: --help prints usage to stdout and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, //nolint:errcheck // best-effort output
				"Usage: %s [OPTION]... [FILE]...\n"+
					"Print the first 10 lines of each FILE to standard output.\n"+
					"With more than one FILE, precede each with a header giving the file name.\n\n"+
					"With no FILE, or when FILE is -, read standard input.\n\n"+
					"  -c, --bytes=[-]NUM       print the first NUM bytes of each file;\n"+
					"                             with the leading '-', print all but the last\n"+
					"                             NUM bytes of each file\n"+
					"  -n, --lines=[-]NUM       print the first NUM lines instead of the first 10;\n"+
					"                             with the leading '-', print all but the last\n"+
					"                             NUM lines of each file\n"+
					"  -q, --quiet, --silent    never print headers giving file names\n"+
					"  -v, --verbose            always print headers giving file names\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		// R4.1: --version prints version to stdout and exits 0.
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", //nolint:errcheck // best-effort output
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		// Long options.
		if strings.HasPrefix(arg, "--lines=") {
			opts.mode = modeLines
			opts.count, opts.negative = parseCount(strings.TrimPrefix(arg, "--lines="))
			continue
		}
		if arg == "--lines" {
			if i+1 < len(args) {
				i++
				opts.mode = modeLines
				opts.count, opts.negative = parseCount(args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "--bytes=") {
			opts.mode = modeBytes
			opts.count, opts.negative = parseCount(strings.TrimPrefix(arg, "--bytes="))
			continue
		}
		if arg == "--bytes" {
			if i+1 < len(args) {
				i++
				opts.mode = modeBytes
				opts.count, opts.negative = parseCount(args[i])
			}
			continue
		}
		if arg == "-q" || arg == "--quiet" || arg == "--silent" {
			opts.quiet = true
			opts.verbose = false
			continue
		}
		if arg == "-v" || arg == "--verbose" {
			opts.verbose = true
			opts.quiet = false
			continue
		}

		// Short options: -n NUM, -c NUM.
		if strings.HasPrefix(arg, "-n") {
			val := arg[2:]
			if val == "" {
				if i+1 < len(args) {
					i++
					val = args[i]
				}
			}
			opts.mode = modeLines
			opts.count, opts.negative = parseCount(val)
			continue
		}
		if strings.HasPrefix(arg, "-c") {
			val := arg[2:]
			if val == "" {
				if i+1 < len(args) {
					i++
					val = args[i]
				}
			}
			opts.mode = modeBytes
			opts.count, opts.negative = parseCount(val)
			continue
		}

		// -NUM shorthand for -n NUM (e.g., -5 means first 5 lines).
		if len(arg) > 1 && arg[0] == '-' && isDigit(arg[1]) {
			opts.mode = modeLines
			opts.count, opts.negative = parseCount(arg[1:])
			continue
		}

		files = append(files, arg)
	}

	return opts, files
}

// parseCount parses a count string that may be negative (prefixed with -)
// and may have a multiplier suffix. Returns the absolute count and whether
// the value was negative.
func parseCount(s string) (int64, bool) {
	if s == "" {
		return 10, false
	}
	negative := false
	if s[0] == '-' {
		negative = true
		s = s[1:]
	}

	// R2.3: multiplier suffixes. Check multi-character suffixes (KiB, MiB,
	// GiB) before single-character ones (b, K, M, G) to avoid partial matches.
	multiplier := int64(1)
	if strings.HasSuffix(s, "KiB") {
		multiplier = 1024
		s = s[:len(s)-3]
	} else if strings.HasSuffix(s, "MiB") {
		multiplier = 1048576
		s = s[:len(s)-3]
	} else if strings.HasSuffix(s, "GiB") {
		multiplier = 1073741824
		s = s[:len(s)-3]
	} else if len(s) > 0 {
		last := s[len(s)-1]
		switch last {
		case 'b':
			multiplier = 512
			s = s[:len(s)-1]
		case 'K':
			multiplier = 1024
			s = s[:len(s)-1]
		case 'M':
			multiplier = 1048576
			s = s[:len(s)-1]
		case 'G':
			multiplier = 1073741824
			s = s[:len(s)-1]
		}
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// Invalid count — default to 10.
		return 10, false
	}
	return n * multiplier, negative
}

// isDigit returns true if b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// isEPIPE returns true if err wraps a syscall.EPIPE error.
func isEPIPE(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EPIPE
	}
	return false
}

// unwrapPathError extracts the inner error from an *os.PathError.
func unwrapPathError(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

