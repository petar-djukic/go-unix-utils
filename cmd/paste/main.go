// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the paste utility.
// Implements prd027-paste R1 (parallel merge), R2 (delimiter configuration),
// R3 (serial mode), R4 (exit codes, SIGPIPE).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "paste"

func main() {
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fatal(err.Error())
	}

	w := bufio.NewWriter(os.Stdout)
	var exitCode int
	if opts.serial {
		exitCode = runSerial(w, opts)
	} else {
		exitCode = runParallel(w, opts)
	}
	if err := w.Flush(); err != nil {
		exitCode = 1
	}
	os.Exit(exitCode)
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	os.Exit(1)
}

type options struct {
	delimiters []byte
	serial     bool
	files      []string
}

func parseArgs(args []string) (*options, error) {
	opts := &options{
		delimiters: []byte{'\t'},
	}
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags || !strings.HasPrefix(arg, "-") {
			opts.files = append(opts.files, arg)
			continue
		}

		if arg == "-" {
			opts.files = append(opts.files, "-")
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("paste (go-unix-utils)")
			os.Exit(0)
		}
		if arg == "--serial" {
			opts.serial = true
			continue
		}
		if val, ok := strings.CutPrefix(arg, "--delimiters="); ok {
			opts.delimiters = parseDelimiters(val)
			continue
		}
		if arg == "--delimiters" {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("option '--delimiters' requires an argument")
			}
			opts.delimiters = parseDelimiters(args[i])
			continue
		}

		// Short flags
		j := 1
		for j < len(arg) {
			ch := arg[j]
			switch ch {
			case 's':
				opts.serial = true
				j++
			case 'd':
				rest := arg[j+1:]
				if rest != "" {
					opts.delimiters = parseDelimiters(rest)
					j = len(arg)
				} else {
					i++
					if i >= len(args) {
						return nil, fmt.Errorf("option requires an argument -- 'd'")
					}
					opts.delimiters = parseDelimiters(args[i])
					j = len(arg)
				}
			default:
				return nil, fmt.Errorf("invalid option -- '%c'", ch)
			}
		}
	}

	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}

	return opts, nil
}

// parseDelimiters interprets backslash escapes in the delimiter string.
// R2.2: \n (newline), \t (tab), \\ (backslash), \0 (empty string).
func parseDelimiters(s string) []byte {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result = append(result, '\n')
				i += 2
			case 't':
				result = append(result, '\t')
				i += 2
			case '\\':
				result = append(result, '\\')
				i += 2
			case '0':
				// \0 means empty string — we use 0 byte as sentinel
				result = append(result, 0)
				i += 2
			default:
				result = append(result, s[i])
				i++
			}
		} else {
			result = append(result, s[i])
			i++
		}
	}
	if len(result) == 0 {
		result = []byte{'\t'}
	}
	return result
}

// inputSource wraps a scanner with EOF tracking.
type inputSource struct {
	scanner *bufio.Scanner
	closer  io.Closer // nil for stdin
	done    bool
}

// openInputs opens all file operands. "-" uses the shared stdinReader.
func openInputs(files []string, stdinReader io.Reader) ([]*inputSource, error) {
	sources := make([]*inputSource, len(files))
	for i, name := range files {
		var r io.Reader
		var closer io.Closer
		if name == "-" {
			r = stdinReader
		} else {
			f, err := os.Open(name)
			if err != nil {
				// Close already-opened files
				for j := range i {
					if sources[j].closer != nil {
						sources[j].closer.Close() // best-effort close
					}
				}
				return nil, err
			}
			r = f
			closer = f
		}
		s := bufio.NewScanner(r)
		sources[i] = &inputSource{scanner: s, closer: closer}
	}
	return sources, nil
}

func closeInputs(sources []*inputSource) {
	for _, src := range sources {
		if src.closer != nil {
			src.closer.Close() // best-effort close
		}
	}
}

// runParallel implements the default parallel merge mode (R1).
func runParallel(w *bufio.Writer, opts *options) int {
	sources, err := openInputs(opts.files, os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	defer closeInputs(sources)

	for {
		anyActive := false
		var line strings.Builder
		for i, src := range sources {
			if i > 0 {
				delimIdx := (i - 1) % len(opts.delimiters)
				d := opts.delimiters[delimIdx]
				if d != 0 { // \0 means empty string
					line.WriteByte(d)
				}
			}
			if !src.done {
				if src.scanner.Scan() {
					line.WriteString(src.scanner.Text())
					anyActive = true
				} else {
					src.done = true
					// Exhausted file contributes empty field (R1.2)
				}
			}
			// Already-done sources contribute empty field (delimiter written above)
		}

		if !anyActive {
			break
		}

		fmt.Fprintln(w, line.String())
	}

	return 0
}

// runSerial implements the -s serial mode (R3).
func runSerial(w *bufio.Writer, opts *options) int {
	for _, name := range opts.files {
		var r io.Reader
		var closer io.Closer
		if name == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
				return 1
			}
			closer = f
			r = f
		}

		scanner := bufio.NewScanner(r)
		first := true
		delimIdx := 0
		for scanner.Scan() {
			if !first {
				d := opts.delimiters[delimIdx]
				if d != 0 { // \0 means empty string
					w.WriteByte(d)
				}
				delimIdx = (delimIdx + 1) % len(opts.delimiters)
			}
			w.WriteString(scanner.Text())
			first = false
		}
		fmt.Fprintln(w)

		if closer != nil {
			closer.Close() // best-effort close
		}
	}

	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [OPTION]... [FILE]...
Write lines consisting of the sequentially corresponding lines from
each FILE, separated by TABs, to standard output.

With no FILE, or when FILE is -, read standard input.

  -d LIST        reuse characters from LIST instead of TABs
  -s, --serial   paste one file at a time instead of in parallel
      --help     display this help and exit
      --version  output version information and exit
`, programName)
}
