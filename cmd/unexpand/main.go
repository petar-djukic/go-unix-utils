// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the unexpand utility for converting spaces to tabs.
//
// Implements prd025-unexpand: default leading-only conversion (R1),
// all-whitespace conversion with -a (R2), custom tab stops with -t (R3),
// exit codes and SIGPIPE (R4).
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

	tabStops, allMode, files := parseArgs(os.Args[1:])

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	exitCode := 0

	processReader := func(r io.Reader) {
		if err := unexpandReader(r, w, tabStops, allMode); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
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
				fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
				exitCode = 1
				continue
			}
			processReader(f)
			f.Close()
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: write error: %v\n", err)
		exitCode = 1
	}
	os.Exit(exitCode)
}

// unexpandReader reads from r and writes space-to-tab converted output to w.
// R1: default mode converts only leading whitespace.
// R2: allMode converts all whitespace runs throughout the line.
func unexpandReader(r io.Reader, w *bufio.Writer, tabStops []int, allMode bool) error {
	br := bufio.NewReader(r)
	col := 0          // 0-indexed column position
	spaceRun := 0     // accumulated spaces in current run
	spaceRunStart := 0 // column where current space run started
	leading := true   // whether we are still in leading whitespace

	flushSpaces := func() error {
		for range spaceRun {
			if wErr := w.WriteByte(' '); wErr != nil {
				return wErr
			}
		}
		spaceRun = 0
		return nil
	}

	// convertSpaces converts a run of spaces into tabs and remaining spaces.
	convertSpaces := func() error {
		startCol := spaceRunStart
		endCol := startCol + spaceRun
		pos := startCol
		for pos < endCol {
			nextStop := nextTabStop(pos, tabStops)
			if nextStop == -1 || nextStop > endCol {
				// No more tab stops reachable; emit remaining as spaces.
				for range endCol - pos {
					if wErr := w.WriteByte(' '); wErr != nil {
						return wErr
					}
				}
				pos = endCol
			} else {
				// R1.1/R2.1: tab stop is reachable, emit a tab.
				if wErr := w.WriteByte('\t'); wErr != nil {
					return wErr
				}
				pos = nextStop
			}
		}
		spaceRun = 0
		return nil
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Flush any pending space run.
				if spaceRun > 0 {
					if leading || allMode {
						if cErr := convertSpaces(); cErr != nil {
							return cErr
						}
					} else {
						if fErr := flushSpaces(); fErr != nil {
							return fErr
						}
					}
				}
				return nil
			}
			return err
		}

		switch b {
		case ' ':
			if spaceRun == 0 {
				spaceRunStart = col
			}
			spaceRun++
			col++
		case '\t':
			// R1.4: existing tabs in whitespace count toward column.
			if spaceRun == 0 {
				spaceRunStart = col
			}
			// Advance column to next tab stop, accounting for accumulated spaces.
			nextStop := nextTabStop(col, tabStops)
			if nextStop == -1 {
				// Past all tab stops; treat as single column advance.
				col++
				spaceRun++
			} else {
				spaceRun += nextStop - col
				col = nextStop
			}
		case '\n':
			// End of line: flush any pending space run then newline.
			if spaceRun > 0 {
				if leading || allMode {
					if cErr := convertSpaces(); cErr != nil {
						return cErr
					}
				} else {
					if fErr := flushSpaces(); fErr != nil {
						return fErr
					}
				}
			}
			if wErr := w.WriteByte('\n'); wErr != nil {
				return wErr
			}
			col = 0
			spaceRun = 0
			leading = true
		default:
			// Non-whitespace character.
			if spaceRun > 0 {
				if leading || allMode {
					if cErr := convertSpaces(); cErr != nil {
						return cErr
					}
				} else {
					if fErr := flushSpaces(); fErr != nil {
						return fErr
					}
				}
			}
			if wErr := w.WriteByte(b); wErr != nil {
				return wErr
			}
			col++
			leading = false
		}
	}
}

// nextTabStop returns the next tab stop position after col, or -1 if past all
// explicit stops in a list. For a uniform interval, always returns the next multiple.
func nextTabStop(col int, tabStops []int) int {
	if len(tabStops) == 1 {
		// Uniform interval.
		interval := tabStops[0]
		return col + (interval - (col % interval))
	}

	// Explicit tab stop list (1-indexed positions stored as-is, compared against
	// 0-indexed col). Mirrors the expand convention from prd024.
	for _, stop := range tabStops {
		if stop > col {
			return stop
		}
	}
	// Past all explicit stops.
	return -1
}

// parseArgs parses command-line arguments and returns tab stops, all-mode flag,
// and file list.
func parseArgs(args []string) (tabStops []int, allMode bool, files []string) {
	tabStops = []int{defaultTabStop}
	customTabs := false

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
				customTabs = true
			case arg == "--all":
				allMode = true
			case arg == "--first-only":
				allMode = false
			case arg == "--help":
				printUsage()
				os.Exit(0)
			case arg == "--version":
				fmt.Println("unexpand (go-unix-utils)")
				os.Exit(0)
			default:
				fmt.Fprintf(os.Stderr, "unexpand: unrecognized option '%s'\n", arg)
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
			case 'a':
				allMode = true
				j++
			case 't':
				val := shortOptValue(arg, j, args, &i)
				tabStops = parseTabStops(val)
				customTabs = true
				j = len(arg)
			default:
				fmt.Fprintf(os.Stderr, "unexpand: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
		i++
	}

	// R3.3: -t implies -a.
	if customTabs {
		allMode = true
	}

	return tabStops, allMode, files
}

// shortOptValue extracts the value for a short option that takes an argument.
func shortOptValue(arg string, pos int, args []string, idx *int) string {
	rest := arg[pos+1:]
	if rest != "" {
		return rest
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(os.Stderr, "unexpand: option requires an argument -- '%c'\n", arg[pos])
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
		fmt.Fprintf(os.Stderr, "unexpand: option '%s' requires an argument\n", prefix)
		os.Exit(1)
	}
	return args[*idx]
}

// parseTabStops parses a tab stop specification. A single number sets a uniform
// interval; a comma-separated list sets absolute positions. R3.1.
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

	stops := make([]int, 0, len(parts))
	prev := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "unexpand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		if n <= prev && len(stops) > 0 {
			fmt.Fprintf(os.Stderr, "unexpand: tab sizes must be ascending\n")
			os.Exit(1)
		}
		stops = append(stops, n)
		prev = n
	}
	return stops
}

// printUsage prints a brief usage message.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: unexpand [-a] [-t N|LIST] [--first-only] [file ...]\n")
}
