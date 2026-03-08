// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the fold utility for wrapping input lines to a
// specified width.
//
// Implements prd023-fold: core line wrapping (R1), width and byte-mode flags (R2),
// space-break mode (R3), exit codes and SIGPIPE (R4).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultWidth = 80
const tabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()

	width, byteMode, spaceBreak, files := parseArgs(os.Args[1:])

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	exitCode := 0

	processReader := func(r io.Reader) {
		if err := foldReader(r, w, width, byteMode, spaceBreak); err != nil {
			fmt.Fprintf(os.Stderr, "fold: %v\n", err)
			exitCode = 1
		}
	}

	if len(files) == 0 {
		processReader(os.Stdin)
	} else {
		for _, name := range files {
			if name == "-" {
				processReader(os.Stdin)
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "fold: %v\n", err)
				exitCode = 1
				continue
			}
			processReader(f)
			f.Close()
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "fold: write error: %v\n", err)
		exitCode = 1
	}
	os.Exit(exitCode)
}

// foldReader reads from r and writes folded output to w.
func foldReader(r io.Reader, w *bufio.Writer, width int, byteMode, spaceBreak bool) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if writeErr := foldLine(w, line, width, byteMode, spaceBreak); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// foldLine wraps a single line (which may or may not end with '\n') to the
// given width and writes the result to w.
func foldLine(w *bufio.Writer, line string, width int, byteMode, spaceBreak bool) error {
	// Strip trailing newline; we'll add it back at the end if present.
	hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
	if hasNewline {
		line = line[:len(line)-1]
	}

	for len(line) > 0 {
		// Find the break point.
		breakIdx := findBreak(line, width, byteMode, spaceBreak)
		segment := line[:breakIdx]
		line = line[breakIdx:]

		if _, err := w.WriteString(segment); err != nil {
			return err
		}

		if len(line) > 0 {
			// More content remains; insert a newline at the wrap point.
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
		}
	}

	if hasNewline {
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

// findBreak returns the byte index at which to break the line. The returned
// index is at most len(line).
func findBreak(line string, width int, byteMode, spaceBreak bool) int {
	if byteMode {
		return findBreakBytes(line, width, spaceBreak)
	}
	return findBreakColumns(line, width, spaceBreak)
}

// findBreakBytes returns the break index using byte counting.
func findBreakBytes(line string, width int, spaceBreak bool) int {
	if len(line) <= width {
		return len(line)
	}
	if spaceBreak {
		// R3.1: find last space at or before width.
		lastSpace := -1
		for i := 0; i < width; i++ {
			if line[i] == ' ' {
				lastSpace = i
			}
		}
		// R3.3: break after the space (space is last char of current segment).
		if lastSpace >= 0 {
			return lastSpace + 1
		}
	}
	// R3.2: fall back to hard break.
	return width
}

// findBreakColumns returns the break index using column counting with tab
// expansion.
func findBreakColumns(line string, width int, spaceBreak bool) int {
	col := 0
	lastSpace := -1

	for i := range len(line) {
		ch := line[i]

		var advance int
		if ch == '\t' {
			// R2.2: tab expands to next tab stop.
			advance = tabStop - (col % tabStop)
		} else if ch == '\b' {
			// Backspace: GNU fold decrements column if > 0.
			if col > 0 {
				col--
			}
			// Backspace itself doesn't exceed width; continue.
			if spaceBreak && ch == ' ' {
				lastSpace = i
			}
			continue
		} else if ch == '\r' {
			// Carriage return: GNU fold resets column to 0.
			col = 0
			continue
		} else {
			advance = 1
		}

		if col+advance > width {
			// This character would exceed the width.
			if spaceBreak && lastSpace >= 0 {
				return lastSpace + 1
			}
			return i
		}

		if spaceBreak && ch == ' ' {
			lastSpace = i
		}

		col += advance

		if col == width {
			// Exactly at width boundary.
			if i+1 >= len(line) {
				// End of line; no break needed.
				return len(line)
			}
			if spaceBreak && lastSpace >= 0 {
				return lastSpace + 1
			}
			return i + 1
		}
	}

	return len(line)
}

// parseArgs parses command-line arguments.
func parseArgs(args []string) (width int, byteMode, spaceBreak bool, files []string) {
	width = defaultWidth

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case strings.HasPrefix(arg, "--width"):
				val := longOptValue(arg, "--width", args, &i)
				width = parseWidth(val)
			case arg == "--bytes":
				byteMode = true
			case arg == "--spaces":
				spaceBreak = true
			case arg == "--help":
				printUsage()
				os.Exit(0)
			case arg == "--version":
				fmt.Println("fold (go-unix-utils)")
				os.Exit(0)
			default:
				fmt.Fprintf(os.Stderr, "fold: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			i++
			continue
		}

		// Short options.
		j := 1
		for j < len(arg) {
			ch := arg[j]
			switch ch {
			case 'w':
				val := shortOptValue(arg, j, args, &i)
				width = parseWidth(val)
				j = len(arg)
			case 'b':
				byteMode = true
				j++
			case 's':
				spaceBreak = true
				j++
			default:
				fmt.Fprintf(os.Stderr, "fold: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
		i++
	}

	return width, byteMode, spaceBreak, files
}

// shortOptValue extracts the value for a short option that takes an argument.
func shortOptValue(arg string, pos int, args []string, idx *int) string {
	rest := arg[pos+1:]
	if rest != "" {
		return rest
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(os.Stderr, "fold: option requires an argument -- '%c'\n", arg[pos])
		os.Exit(1)
	}
	return args[*idx]
}

// longOptValue extracts the value for a long option.
func longOptValue(arg, prefix string, args []string, idx *int) string {
	if strings.Contains(arg, "=") {
		return arg[strings.Index(arg, "=")+1:]
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(os.Stderr, "fold: option '%s' requires an argument\n", prefix)
		os.Exit(1)
	}
	return args[*idx]
}

// parseWidth parses the width value, exiting on invalid input.
func parseWidth(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "fold: invalid number of columns: '%s'\n", s)
		os.Exit(1)
	}
	return n
}

// printUsage prints a brief usage message.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: fold [-bs] [-w width] [file ...]\n")
}
