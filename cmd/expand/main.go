// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the expand utility (prd024-expand R1-R3).
// expand converts tab characters to spaces, reading from files or stdin,
// with configurable tab stop positions via the -t flag.
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

// defaultTabStop is the default tab stop interval.
const defaultTabStop = 8

func main() {
	// R3.4: Handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	var tabStops []int
	initialOnly := false

	args := os.Args[1:]
	var files []string

	// Manual flag parsing to match GNU expand behavior.
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "--help" {
			fmt.Println("Usage: expand [OPTION]... [FILE]...")
			fmt.Println("Convert tabs in each FILE to spaces, writing to standard output.")
			fmt.Println()
			fmt.Println("  -i, --initial       do not convert tabs after non blanks")
			fmt.Println("  -t, --tabs=N        have tabs N characters apart, not 8")
			fmt.Println("  -t, --tabs=LIST     use comma separated list of tab positions")
			fmt.Println("      --help          display this help and exit")
			fmt.Println("      --version       output version information and exit")
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("expand (go-unix-utils) 0.1")
			os.Exit(0)
		}
		if arg == "--initial" {
			initialOnly = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--tabs=") {
			val := arg[len("--tabs="):]
			tabStops = parseTabStops(val)
			i++
			continue
		}
		if arg == "--tabs" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "expand: option '--tabs' requires an argument\n")
				os.Exit(1)
			}
			tabStops = parseTabStops(args[i])
			i++
			continue
		}
		if len(arg) == 0 || arg[0] != '-' || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}

		// Parse short flags from the argument.
		j := 1
		for j < len(arg) {
			switch arg[j] {
			case 'i':
				// R3: -i converts only leading tabs.
				initialOnly = true
				j++
			case 't':
				// R2: -t sets tab stops.
				val := arg[j+1:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "expand: option requires an argument -- 't'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				tabStops = parseTabStops(val)
				j = len(arg) // consumed rest of arg
			default:
				fmt.Fprintf(os.Stderr, "expand: invalid option -- '%c'\n", arg[j])
				os.Exit(1)
			}
		}
		i++
	}

	// R1.1: Read stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	// Build effective tab stop configuration.
	if len(tabStops) == 0 {
		tabStops = []int{defaultTabStop}
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	for _, file := range files {
		r, err := openInput(file)
		if err != nil {
			// R3.2: Print error to stderr, continue processing remaining files.
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			exitCode = 1
			continue
		}

		if err := expandInput(r, w, tabStops, initialOnly); err != nil {
			// R3.3: Exit 1 on write error.
			os.Exit(1)
		}

		if closer, ok := r.(io.Closer); ok && file != "-" {
			_ = closer.Close() // best-effort close
		}
	}

	if err := w.Flush(); err != nil {
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// openInput returns a reader for the named file, or stdin if name is "-".
func openInput(name string) (io.Reader, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// parseTabStops parses a tab stop specification: either a single number
// (uniform interval) or a comma-separated list of absolute positions.
func parseTabStops(s string) []int {
	parts := strings.Split(s, ",")
	stops := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "expand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		stops = append(stops, n)
	}
	return stops
}

// nextTabStop returns the next tab stop column position given the current column
// (0-indexed) and the tab stop configuration.
// R2.1: Single value means uniform interval.
// R2.2: Multiple values are absolute positions; past the last stop, a tab becomes one space.
// R2.4: Single value in list behaves as uniform interval.
func nextTabStop(col int, stops []int) int {
	if len(stops) == 1 {
		// Uniform interval.
		interval := stops[0]
		return col + (interval - col%interval)
	}
	// Absolute positions (1-indexed in spec, but we work 0-indexed internally).
	// The stops are given as 1-indexed column positions.
	for _, s := range stops {
		pos := s - 1 // convert to 0-indexed
		if pos > col {
			return pos
		}
	}
	// R2.2: Past the last explicit stop, tab becomes one space.
	return col + 1
}

// expandInput reads from r and writes tab-expanded output to w.
func expandInput(r io.Reader, w *bufio.Writer, stops []int, initialOnly bool) error {
	br := bufio.NewReader(r)
	col := 0
	pastInitial := false

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch {
		case b == '\n':
			// R1.4: Newline resets column position.
			if writeErr := w.WriteByte('\n'); writeErr != nil {
				return writeErr
			}
			col = 0
			pastInitial = false

		case b == '\t' && !(initialOnly && pastInitial):
			// R1.1, R1.2: Replace tab with spaces to reach next tab stop.
			next := nextTabStop(col, stops)
			spaces := next - col
			for range spaces {
				if writeErr := w.WriteByte(' '); writeErr != nil {
					return writeErr
				}
			}
			col = next

		default:
			// R1.3: Non-tab characters pass through unchanged.
			if writeErr := w.WriteByte(b); writeErr != nil {
				return writeErr
			}
			col++
			// Track whether we've passed leading blanks for -i mode.
			if b != ' ' && b != '\t' {
				pastInitial = true
			}
		}
	}
}
