// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the expand utility for converting tabs to spaces.
//
// Implements prd024-expand: default tab expansion (R1), custom tab stops (R2),
// exit codes and SIGPIPE (R3).
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

const defaultTabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()

	tabStops, files := parseArgs(os.Args[1:])

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	exitCode := 0

	processReader := func(r io.Reader) {
		if err := expandReader(r, w, tabStops); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
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
				fmt.Fprintf(os.Stderr, "expand: %v\n", err)
				exitCode = 1
				continue
			}
			processReader(f)
			f.Close()
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
		exitCode = 1
	}
	os.Exit(exitCode)
}

// expandReader reads from r and writes tab-expanded output to w.
func expandReader(r io.Reader, w *bufio.Writer, tabStops []int) error {
	br := bufio.NewReader(r)
	col := 0 // 0-indexed column position

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch b {
		case '\t':
			spaces := nextTabSpaces(col, tabStops)
			for range spaces {
				if wErr := w.WriteByte(' '); wErr != nil {
					return wErr
				}
			}
			col += spaces
		case '\n':
			if wErr := w.WriteByte('\n'); wErr != nil {
				return wErr
			}
			col = 0
		default:
			if wErr := w.WriteByte(b); wErr != nil {
				return wErr
			}
			col++
		}
	}
}

// nextTabSpaces returns the number of spaces to insert for a tab at the given
// 0-indexed column position. R1.1, R2.1, R2.2.
func nextTabSpaces(col int, tabStops []int) int {
	if len(tabStops) == 1 {
		// R2.1: uniform interval.
		interval := tabStops[0]
		return interval - (col % interval)
	}

	// R2.2: explicit tab stop list (0-indexed internally).
	for _, stop := range tabStops {
		if stop > col {
			return stop - col
		}
	}
	// R2.2: past the last explicit stop, replace with a single space.
	return 1
}

// parseArgs parses command-line arguments and returns tab stops and file list.
func parseArgs(args []string) (tabStops []int, files []string) {
	tabStops = []int{defaultTabStop}

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
			case strings.HasPrefix(arg, "--tabs"):
				val := longOptValue(arg, "--tabs", args, &i)
				tabStops = parseTabStops(val)
			case arg == "--help":
				printUsage()
				os.Exit(0)
			case arg == "--version":
				fmt.Println("expand (go-unix-utils)")
				os.Exit(0)
			default:
				fmt.Fprintf(os.Stderr, "expand: unrecognized option '%s'\n", arg)
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
			case 't':
				val := shortOptValue(arg, j, args, &i)
				tabStops = parseTabStops(val)
				j = len(arg)
			case 'i':
				// -i (initial) is accepted by GNU expand but we only need
				// basic support; treat as no-op for flag compatibility.
				j++
			default:
				fmt.Fprintf(os.Stderr, "expand: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
		i++
	}

	return tabStops, files
}

// shortOptValue extracts the value for a short option that takes an argument.
func shortOptValue(arg string, pos int, args []string, idx *int) string {
	rest := arg[pos+1:]
	if rest != "" {
		return rest
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(os.Stderr, "expand: option requires an argument -- '%c'\n", arg[pos])
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
		fmt.Fprintf(os.Stderr, "expand: option '%s' requires an argument\n", prefix)
		os.Exit(1)
	}
	return args[*idx]
}

// parseTabStops parses a tab stop specification. A single number sets a uniform
// interval; a comma-separated list sets absolute positions. R2.1, R2.2, R2.4.
func parseTabStops(s string) []int {
	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		// Single value: uniform interval (R2.1, R2.4).
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "expand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		return []int{n}
	}

	// Multiple values: absolute positions (R2.2).
	// GNU expand positions are 1-indexed; we store them as-is and compare
	// against 0-indexed column with >.
	stops := make([]int, 0, len(parts))
	prev := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "expand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		if n <= prev && len(stops) > 0 {
			fmt.Fprintf(os.Stderr, "expand: tab sizes must be ascending\n")
			os.Exit(1)
		}
		stops = append(stops, n)
		prev = n
	}
	return stops
}

// printUsage prints a brief usage message.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: expand [-t N|LIST] [file ...]\n")
}
