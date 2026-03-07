// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU paste: merge lines of files.
// Implements prd027-paste R1-R4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	delimiters := "\t"
	serial := false

	files := parseArgs(os.Args[1:], &delimiters, &serial)

	delimList := parseDelimiters(delimiters)

	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)

	var exitCode int
	if serial {
		exitCode = pasteSerial(w, files, delimList)
	} else {
		exitCode = pasteParallel(w, files, delimList)
	}

	w.Flush()
	os.Exit(exitCode)
}

// parseArgs manually parses arguments to support GNU-style -dX (no space).
func parseArgs(args []string, delimiters *string, serial *bool) []string {
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			files = append(files, args[i:]...)
			return files
		}

		if arg == "-s" || arg == "--serial" {
			*serial = true
			i++
			continue
		}

		if arg == "-d" || arg == "--delimiters" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "paste: option requires an argument -- 'd'\n")
				os.Exit(1)
			}
			*delimiters = args[i]
			i++
			continue
		}

		if strings.HasPrefix(arg, "-d") && len(arg) > 2 && arg[1] == 'd' {
			*delimiters = arg[2:]
			i++
			continue
		}

		if strings.HasPrefix(arg, "--delimiters=") {
			*delimiters = arg[len("--delimiters="):]
			i++
			continue
		}

		// Combined short flags like -sd:
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 's':
					*serial = true
					j++
				case 'd':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "paste: option requires an argument -- 'd'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					*delimiters = val
					j = len(arg)
				default:
					fmt.Fprintf(os.Stderr, "paste: invalid option -- '%c'\n", arg[j])
					os.Exit(1)
				}
			}
			i++
			continue
		}

		// Not a flag — rest are filenames.
		break
	}

	files = append(files, args[i:]...)
	return files
}

// parseDelimiters interprets escape sequences in the delimiter string.
// Recognized: \n (newline), \t (tab), \\ (backslash), \0 (empty string).
func parseDelimiters(s string) []string {
	var delims []string
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				delims = append(delims, "\n")
				i += 2
			case 't':
				delims = append(delims, "\t")
				i += 2
			case '\\':
				delims = append(delims, "\\")
				i += 2
			case '0':
				delims = append(delims, "")
				i += 2
			default:
				delims = append(delims, string(s[i]))
				i++
			}
		} else {
			delims = append(delims, string(s[i]))
			i++
		}
	}
	if len(delims) == 0 {
		delims = []string{"\t"}
	}
	return delims
}

// pasteParallel merges corresponding lines from all files.
func pasteParallel(w *bufio.Writer, files []string, delims []string) int {
	readers := make([]*bufio.Reader, len(files))
	done := make([]bool, len(files))
	stdinReader := (*bufio.Reader)(nil)

	for i, name := range files {
		if name == "-" {
			if stdinReader == nil {
				stdinReader = bufio.NewReader(os.Stdin)
			}
			readers[i] = stdinReader
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "paste: %s: %s\n", name, errMsg(err))
				return 1
			}
			defer f.Close()
			readers[i] = bufio.NewReader(f)
		}
	}

	for {
		allDone := true
		var parts []string

		for i, r := range readers {
			if done[i] {
				parts = append(parts, "")
				continue
			}
			line, err := readLine(r)
			if err != nil {
				done[i] = true
				parts = append(parts, "")
				continue
			}
			allDone = false
			parts = append(parts, line)
		}

		if allDone {
			break
		}

		if err := writeLine(w, parts, delims); err != nil {
			fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
			return 1
		}
	}

	return 0
}

// pasteSerial processes each file independently, joining all its lines on one output line.
func pasteSerial(w *bufio.Writer, files []string, delims []string) int {
	stdinReader := (*bufio.Reader)(nil)

	for _, name := range files {
		var r *bufio.Reader
		if name == "-" {
			if stdinReader == nil {
				stdinReader = bufio.NewReader(os.Stdin)
			}
			r = stdinReader
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "paste: %s: %s\n", name, errMsg(err))
				return 1
			}
			defer f.Close()
			r = bufio.NewReader(f)
		}

		var lines []string
		for {
			line, err := readLine(r)
			if err != nil {
				break
			}
			lines = append(lines, line)
		}

		if err := writeLine(w, lines, delims); err != nil {
			fmt.Fprintf(os.Stderr, "paste: write error: %v\n", err)
			return 1
		}
	}

	return 0
}

// writeLine joins parts with cycling delimiters and writes the result plus newline.
func writeLine(w *bufio.Writer, parts []string, delims []string) error {
	for i, p := range parts {
		if i > 0 {
			d := delims[(i-1)%len(delims)]
			if _, err := w.WriteString(d); err != nil {
				return err
			}
		}
		if _, err := w.WriteString(p); err != nil {
			return err
		}
	}
	return w.WriteByte('\n')
}

// readLine reads a single line from the reader, stripping the trailing newline.
func readLine(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			if err == io.EOF && sb.Len() > 0 {
				return sb.String(), nil
			}
			return "", err
		}
		if b == '\n' {
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
}

// errMsg extracts the underlying error message, stripping the os.PathError wrapper.
func errMsg(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
