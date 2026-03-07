// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU unexpand: convert spaces to tabs.
// Implements prd025-unexpand R1-R4.
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

// unexpandOptions holds the parsed command-line flags for unexpand.
type unexpandOptions struct {
	tabStops []int // absolute tab stop positions; if single element, used as uniform interval
	allMode  bool  // -a: convert all whitespace, not just leading
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush

	exitCode := 0
	for _, file := range files {
		var r io.Reader
		if file == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "unexpand: %s: No such file or directory\n", file)
				exitCode = 1
				continue
			}
			defer f.Close() // best-effort cleanup
			r = f
		}

		if err := unexpandReader(r, w, opts); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: write error: %v\n", err)
			exitCode = 1
			break
		}
	}

	w.Flush() // best-effort
	os.Exit(exitCode)
}

// unexpandReader reads from r and writes unexpanded output to w.
func unexpandReader(r io.Reader, w *bufio.Writer, opts unexpandOptions) error {
	br := bufio.NewReader(r)
	col := 0        // 0-based column position
	leading := true // whether we are still in leading whitespace
	// spaceRun tracks accumulated spaces that might be converted to tabs.
	spaceRunStart := 0 // column where the current space run started
	spaceCount := 0    // number of spaces in current run

	flushSpaces := func() error {
		for i := 0; i < spaceCount; i++ {
			if werr := w.WriteByte(' '); werr != nil {
				return werr
			}
		}
		spaceCount = 0
		return nil
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Flush any remaining spaces.
				return flushSpaces()
			}
			return err
		}

		converting := leading || opts.allMode

		switch {
		case b == ' ' && converting:
			// R1.1, R2.1: Accumulate spaces for potential tab conversion.
			if spaceCount == 0 {
				spaceRunStart = col
			}
			spaceCount++
			col++

			// Check if we've reached a tab stop.
			next := nextTabStop(spaceRunStart, opts.tabStops)
			if next > 0 && col == next {
				// R1.1: Run of spaces exactly reaches a tab stop — replace with tab.
				if werr := w.WriteByte('\t'); werr != nil {
					return werr
				}
				spaceCount = 0
			}

		case b == '\t' && converting:
			// R1.4: Existing tab in whitespace — discard pending spaces (tab
			// advances past them) and write the tab.
			spaceCount = 0
			if werr := w.WriteByte('\t'); werr != nil {
				return werr
			}
			// Tab advances to next tab stop.
			next := nextTabStop(col, opts.tabStops)
			if next > col {
				col = next
			} else {
				col++
			}

		case b == '\n':
			// Newline: flush pending spaces, reset state.
			if err := flushSpaces(); err != nil {
				return err
			}
			if werr := w.WriteByte('\n'); werr != nil {
				return werr
			}
			col = 0
			leading = true

		case b == '\b':
			// Backspace decrements column (GNU behavior).
			if err := flushSpaces(); err != nil {
				return err
			}
			if werr := w.WriteByte(b); werr != nil {
				return werr
			}
			if col > 0 {
				col--
			}
			leading = false

		default:
			// R1.2: Non-whitespace character — flush pending spaces, write character.
			if err := flushSpaces(); err != nil {
				return err
			}
			if werr := w.WriteByte(b); werr != nil {
				return werr
			}
			col++
			leading = false
		}
	}
}

// nextTabStop returns the next tab stop column (0-based) given the current column.
func nextTabStop(col int, tabStops []int) int {
	if len(tabStops) == 1 {
		// Uniform interval mode (R3.1).
		interval := tabStops[0]
		return ((col / interval) + 1) * interval
	}

	// Absolute position list mode (R3.1).
	// tabStops are 1-based absolute positions in strictly increasing order.
	for _, stop := range tabStops {
		if stop > col {
			return stop
		}
	}

	// R3.2: Past last explicit stop — no tab can be inserted.
	return 0
}

// parseArgs parses unexpand command-line flags manually.
func parseArgs(args []string) (unexpandOptions, []string) {
	opts := unexpandOptions{
		tabStops: []int{8}, // default tab stop every 8 columns
	}

	var files []string
	i := 0
	customTabs := false

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--tabs=") {
			opts.tabStops = parseTabStops(arg[len("--tabs="):])
			customTabs = true
			i++
			continue
		}
		if arg == "--tabs" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "unexpand: option '--tabs' requires an argument\n")
				os.Exit(1)
			}
			opts.tabStops = parseTabStops(args[i])
			customTabs = true
			i++
			continue
		}
		if arg == "--all" {
			opts.allMode = true
			i++
			continue
		}
		if arg == "--first-only" {
			opts.allMode = false
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'a':
					opts.allMode = true
					j++
				case 't':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "unexpand: option requires an argument -- 't'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					opts.tabStops = parseTabStops(val)
					customTabs = true
					j = len(arg) // consumed rest of arg
				default:
					fmt.Fprintf(os.Stderr, "unexpand: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			i++
			continue
		}

		// Not a flag; treat as file argument.
		break
	}

	// R3.3: -t implies -a.
	if customTabs {
		opts.allMode = true
	}

	files = append(files, args[i:]...)
	return opts, files
}

// parseTabStops parses a tab stop specification: either a single integer N
// (uniform interval) or a comma-separated list of absolute positions.
func parseTabStops(s string) []int {
	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "unexpand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		return []int{n}
	}

	// Multiple values are absolute column positions.
	stops := make([]int, 0, len(parts))
	prev := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "unexpand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		if n <= prev {
			fmt.Fprintf(os.Stderr, "unexpand: tab sizes must be ascending\n")
			os.Exit(1)
		}
		stops = append(stops, n)
		prev = n
	}
	return stops
}
