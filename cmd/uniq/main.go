// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU uniq: report or filter adjacent duplicate lines.
// Implements prd028-uniq R1-R4.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// errPrefix is the program name used in error messages.
const errPrefix = "uniq"

func main() {
	sys.InstallSIGPIPEHandler()

	opts, inputFile, outputFile := parseArgs(os.Args[1:])

	var reader *bufio.Scanner
	if inputFile == "" || inputFile == "-" {
		reader = bufio.NewScanner(os.Stdin)
	} else {
		f, err := os.Open(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", errPrefix, errMsg(err))
			os.Exit(1)
		}
		defer f.Close()
		reader = bufio.NewScanner(f)
	}
	reader.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var out *bufio.Writer
	if outputFile == "" {
		out = bufio.NewWriter(os.Stdout)
	} else {
		f, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", errPrefix, errMsg(err))
			os.Exit(1)
		}
		defer f.Close()
		out = bufio.NewWriter(f)
	}

	delim := byte('\n')
	if opts.zeroTerminated {
		delim = 0
		reader.Split(scanNULLines)
	}

	exitCode := processLines(reader, out, opts, delim)

	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// options holds parsed command-line flag values.
type options struct {
	count          bool // -c
	repeated       bool // -d
	allRepeated    bool // -D
	unique         bool // -u
	ignoreCase     bool // -i
	skipFields     int  // -f N
	skipChars      int  // -s N
	checkChars     int  // -w N, -1 means unlimited
	checkCharsSet  bool // true if -w was specified
	zeroTerminated bool // -z
}

// parseArgs manually parses arguments, supporting GNU short and long forms.
func parseArgs(args []string) (opts options, inputFile, outputFile string) {
	positional := 0
	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			for i < len(args) {
				switch positional {
				case 0:
					inputFile = args[i]
				case 1:
					outputFile = args[i]
				default:
					fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", errPrefix, args[i])
					os.Exit(1)
				}
				positional++
				i++
			}
			return
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case arg == "--count":
				opts.count = true
			case arg == "--repeated":
				opts.repeated = true
			case arg == "--all-repeated":
				opts.allRepeated = true
			case arg == "--unique":
				opts.unique = true
			case arg == "--ignore-case":
				opts.ignoreCase = true
			case arg == "--zero-terminated":
				opts.zeroTerminated = true
			case arg == "--skip-fields":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--skip-fields' requires an argument\n", errPrefix)
					os.Exit(1)
				}
				opts.skipFields = parseNonNeg(args[i], "skip-fields")
			case strings.HasPrefix(arg, "--skip-fields="):
				opts.skipFields = parseNonNeg(arg[len("--skip-fields="):], "skip-fields")
			case arg == "--skip-chars":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--skip-chars' requires an argument\n", errPrefix)
					os.Exit(1)
				}
				opts.skipChars = parseNonNeg(args[i], "skip-chars")
			case strings.HasPrefix(arg, "--skip-chars="):
				opts.skipChars = parseNonNeg(arg[len("--skip-chars="):], "skip-chars")
			case arg == "--check-chars":
				i++
				if i >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option '--check-chars' requires an argument\n", errPrefix)
					os.Exit(1)
				}
				opts.checkCharsSet = true
				opts.checkChars = parseNonNeg(args[i], "check-chars")
			case strings.HasPrefix(arg, "--check-chars="):
				opts.checkCharsSet = true
				opts.checkChars = parseNonNeg(arg[len("--check-chars="):], "check-chars")
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", errPrefix, arg)
				os.Exit(1)
			}
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'c':
					opts.count = true
					j++
				case 'd':
					opts.repeated = true
					j++
				case 'D':
					opts.allRepeated = true
					j++
				case 'u':
					opts.unique = true
					j++
				case 'i':
					opts.ignoreCase = true
					j++
				case 'z':
					opts.zeroTerminated = true
					j++
				case 'f':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'f'\n", errPrefix)
							os.Exit(1)
						}
						val = args[i]
					}
					opts.skipFields = parseNonNeg(val, "f")
					j = len(arg)
				case 's':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 's'\n", errPrefix)
							os.Exit(1)
						}
						val = args[i]
					}
					opts.skipChars = parseNonNeg(val, "s")
					j = len(arg)
				case 'w':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", errPrefix)
							os.Exit(1)
						}
						val = args[i]
					}
						opts.checkCharsSet = true
					opts.checkChars = parseNonNeg(val, "w")
					j = len(arg)
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", errPrefix, arg[j])
					os.Exit(1)
				}
			}
			i++
			continue
		}

		// Positional argument.
		switch positional {
		case 0:
			inputFile = arg
		case 1:
			outputFile = arg
		default:
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", errPrefix, arg)
			os.Exit(1)
		}
		positional++
		i++
	}
	return
}

// parseNonNeg parses a non-negative integer, exiting on failure.
func parseNonNeg(s, optName string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			fmt.Fprintf(os.Stderr, "%s: invalid number of %s: '%s'\n", errPrefix, optName, s)
			os.Exit(1)
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// compareKey extracts the comparison portion of a line after applying
// field skipping (-f), character skipping (-s), and character limit (-w).
func compareKey(line string, opts options) string {
	s := line
	// R3.2: skip fields — a field is a run of blanks then a run of non-blanks.
	for f := 0; f < opts.skipFields && len(s) > 0; f++ {
		// Skip leading blanks.
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
			s = s[1:]
		}
		// Skip non-blank characters.
		for len(s) > 0 && s[0] != ' ' && s[0] != '\t' {
			s = s[1:]
		}
	}

	// R3.3: skip characters.
	for c := 0; c < opts.skipChars && len(s) > 0; c++ {
		_, size := utf8.DecodeRuneInString(s)
		s = s[size:]
	}

	// R3.4: limit comparison to checkChars characters.
	if opts.checkCharsSet {
		runeCount := 0
		idx := 0
		for idx < len(s) && runeCount < opts.checkChars {
			_, size := utf8.DecodeRuneInString(s[idx:])
			idx += size
			runeCount++
		}
		s = s[:idx]
	}

	// R3.1: case-insensitive comparison.
	if opts.ignoreCase {
		s = strings.ToLower(s)
	}

	return s
}

// processLines reads lines from the scanner and writes filtered output.
func processLines(scanner *bufio.Scanner, w *bufio.Writer, opts options, delim byte) int {
	var prevLine string
	var prevKey string
	count := 0
	first := true

	for scanner.Scan() {
		line := scanner.Text()
		key := compareKey(line, opts)

		if first {
			prevLine = line
			prevKey = key
			count = 1
			first = false
			continue
		}

		if key == prevKey {
			count++
			if opts.allRepeated {
				// -D: print every line of a duplicate run as we go;
				// we'll handle the first occurrence below.
			}
			continue
		}

		// New run — flush previous.
		if err := flushRun(w, prevLine, count, opts, delim); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
			return 1
		}

		prevLine = line
		prevKey = key
		count = 1
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", errPrefix, err)
		return 1
	}

	// Flush final run.
	if !first {
		if err := flushRun(w, prevLine, count, opts, delim); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
			return 1
		}
	}

	return 0
}

// flushRun writes a line according to the output mode flags.
func flushRun(w *bufio.Writer, line string, count int, opts options, delim byte) error {
	shouldPrint := true

	if opts.repeated || opts.allRepeated {
		// -d/-D: only print lines from runs with count > 1.
		if count <= 1 {
			shouldPrint = false
		}
	}

	if opts.unique {
		// -u: only print lines that appear exactly once.
		if count != 1 {
			shouldPrint = false
		}
	}

	if !shouldPrint {
		return nil
	}

	if opts.allRepeated {
		// -D: print all copies.
		for range count {
			if opts.count {
				if _, err := fmt.Fprintf(w, "%7d %s%c", count, line, delim); err != nil {
					return err
				}
			} else {
				if _, err := w.WriteString(line); err != nil {
					return err
				}
				if err := w.WriteByte(delim); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if opts.count {
		if _, err := fmt.Fprintf(w, "%7d %s%c", count, line, delim); err != nil {
			return err
		}
		return nil
	}

	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte(delim)
}

// scanNULLines is a bufio.SplitFunc that splits on NUL bytes instead of newlines.
func scanNULLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := indexOf(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// indexOf returns the index of the first occurrence of b in data, or -1.
func indexOf(data []byte, b byte) int {
	for i, c := range data {
		if c == b {
			return i
		}
	}
	return -1
}

// errMsg extracts the underlying error message, stripping the os.PathError wrapper.
func errMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
