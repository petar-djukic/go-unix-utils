// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd027-paste R1.1-R1.4, R2.1-R2.3, R3.1-R3.3: cmd/paste merges
// corresponding lines from multiple input files, separating fields with a delimiter
// and writing the result to stdout. Supports custom delimiter lists via -d with
// cycling across columns, including escape sequences (\n, \t, \\, \0). When one
// file is exhausted before others, empty strings are substituted. '-' reads from
// stdin with round-robin consumption across multiple '-' operands. Serial mode (-s)
// processes files one at a time, joining all lines of each file into a single
// output line separated by the delimiter. Installs SIGPIPE handler.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU paste format.
const progName = "paste"

// defaultDelims is the default delimiter list (a single tab) used when -d is not specified.
var defaultDelims = []string{"\t"}

func main() {
	sys.InstallSIGPIPEHandler()

	delims := defaultDelims
	serial := false
	args := os.Args[1:]
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "--help" {
			fmt.Fprintf(os.Stdout,
				"Usage: %s [OPTION]... [FILE]...\n"+
					"Write lines consisting of the sequentially corresponding lines from\n"+
					"each FILE, separated by TABs, to standard output.\n\n"+
					"With no FILE, or when FILE is -, read standard input.\n\n"+
					"  -d, --delimiters=LIST   reuse characters from LIST instead of TABs\n"+
					"  -s, --serial            paste one file at a time instead of in parallel\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n",
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}

		if arg == "--serial" {
			serial = true
			continue
		}
		if arg == "-s" {
			serial = true
			continue
		}
		if strings.HasPrefix(arg, "--delimiters=") {
			delims = parseDelimiterList(arg[len("--delimiters="):])
			continue
		}
		if arg == "--delimiters" || arg == "-d" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'd'\n", progName)
				os.Exit(1)
			}
			i++
			delims = parseDelimiterList(args[i])
			continue
		}

		// Short options: -d with value attached.
		if len(arg) > 2 && arg[0] == '-' && arg[1] == 'd' {
			delims = parseDelimiterList(arg[2:])
			continue
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// Handle combined short flags like -sd or -ds.
		flags := arg[1:]
		for j := 0; j < len(flags); j++ {
			switch flags[j] {
			case 's':
				serial = true
			case 'd':
				val := flags[j+1:]
				if val == "" {
					if i+1 >= len(args) {
						fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'd'\n", progName)
						os.Exit(1)
					}
					i++
					val = args[i]
				}
				delims = parseDelimiterList(val)
				j = len(flags) // consumed rest
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
		}
	}

	// R1.4: when no files are given, read from stdin.
	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)

	var exitCode int
	if serial {
		exitCode = pasteSerial(w, files, delims)
	} else {
		exitCode = pasteParallel(w, files, delims)
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseDelimiterList processes the -d argument into a slice of delimiter strings.
// Each element is one delimiter position in the cycling list. \0 produces an empty
// string (no delimiter at that position), preserving its place in the cycle.
// R2.1: custom delimiter character(s) replacing the default tab.
// R2.2: recognizes \n (newline), \t (tab), \\ (backslash), and \0 (empty string).
// R2.3: the returned slice is cycled per output line, resetting at each new line.
func parseDelimiterList(s string) []string {
	var delims []string
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				delims = append(delims, "\n")
				i++
			case 't':
				delims = append(delims, "\t")
				i++
			case '\\':
				delims = append(delims, "\\")
				i++
			case '0':
				// R2.2: empty string delimiter — no character between fields.
				delims = append(delims, "")
				i++
			default:
				delims = append(delims, string(s[i]))
			}
		} else {
			delims = append(delims, string(s[i]))
		}
	}
	if len(delims) == 0 {
		delims = []string{"\t"}
	}
	return delims
}

// pasteParallel merges corresponding lines from all files side by side.
// R1.1: reads one line from each file per output line, separated by delimiter.
// R1.3: when one file is exhausted, empty strings substitute for its fields.
// R1.4: '-' reads from stdin with round-robin consumption.
// R2.1-R2.3: delimiters cycle across columns and reset each line.
func pasteParallel(w *bufio.Writer, files []string, delims []string) int {
	readers := make([]*bufio.Reader, len(files))
	closers := make([]io.Closer, len(files))
	eof := make([]bool, len(files))
	exitCode := 0

	for i, name := range files {
		if name == "-" {
			readers[i] = bufio.NewReader(os.Stdin)
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				os.Exit(1)
			}
			readers[i] = bufio.NewReader(f)
			closers[i] = f
		}
	}

	// Track a single shared stdin reader for all '-' operands.
	var stdinReader *bufio.Reader
	for i, name := range files {
		if name == "-" {
			if stdinReader == nil {
				stdinReader = readers[i]
			} else {
				readers[i] = stdinReader
			}
		}
	}

	for {
		allDone := true
		fields := make([]string, len(files))

		for i := range files {
			if eof[i] {
				fields[i] = ""
				continue
			}
			line, err := readLine(readers[i])
			if err != nil {
				eof[i] = true
				fields[i] = ""
				continue
			}
			allDone = false
			fields[i] = line
		}

		if allDone {
			break
		}

		// Build output line with cycling delimiters (R2.3: reset each line).
		var buf strings.Builder
		for i, field := range fields {
			if i > 0 {
				delimIdx := (i - 1) % len(delims)
				buf.WriteString(delims[delimIdx])
			}
			buf.WriteString(field)
		}
		buf.WriteByte('\n')

		if _, err := w.WriteString(buf.String()); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
			exitCode = 1
			break
		}
	}

	for _, c := range closers {
		if c != nil {
			c.Close() // best-effort close
		}
	}

	return exitCode
}

// pasteSerial processes files one at a time, joining all lines of each file
// into a single output line separated by the delimiter.
// R3.1: all lines of one file become one output line.
// R3.2: delimiter list cycles across fields within the output line.
func pasteSerial(w *bufio.Writer, files []string, delims []string) int {
	exitCode := 0

	var stdinReader *bufio.Reader

	for _, name := range files {
		var r *bufio.Reader
		var closer io.Closer

		if name == "-" {
			if stdinReader == nil {
				stdinReader = bufio.NewReader(os.Stdin)
			}
			r = stdinReader
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				os.Exit(1)
			}
			r = bufio.NewReader(f)
			closer = f
		}

		var buf strings.Builder
		fieldIdx := 0

		for {
			line, err := readLine(r)
			if err != nil {
				break
			}
			if fieldIdx > 0 {
				delimIdx := (fieldIdx - 1) % len(delims)
				buf.WriteString(delims[delimIdx])
			}
			buf.WriteString(line)
			fieldIdx++
		}

		buf.WriteByte('\n')
		if _, err := w.WriteString(buf.String()); err != nil {
			fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
			exitCode = 1
		}

		if closer != nil {
			closer.Close() // best-effort close
		}
	}

	return exitCode
}

// readLine reads one line from the reader, stripping the trailing newline.
// Returns the line content and an error (io.EOF when no more data).
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if len(line) > 0 {
		// Strip trailing newline.
		if line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		return line, nil
	}
	if err != nil {
		return "", err
	}
	return line, nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
