// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU expand: convert tabs to spaces.
// Implements prd024-expand R1-R3.
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

// expandOptions holds the parsed command-line flags for expand.
type expandOptions struct {
	tabStops []int // absolute tab stop positions; if single element, used as uniform interval
	initial  bool  // -i: convert only leading tabs
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
				fmt.Fprintf(os.Stderr, "expand: %s: No such file or directory\n", file)
				exitCode = 1
				continue
			}
			defer f.Close() // best-effort cleanup
			r = f
		}

		if err := expandReader(r, w, opts); err != nil {
			fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
			exitCode = 1
			break
		}
	}

	w.Flush() // best-effort
	os.Exit(exitCode)
}

// expandReader reads from r and writes expanded output to w.
func expandReader(r io.Reader, w *bufio.Writer, opts expandOptions) error {
	br := bufio.NewReader(r)
	col := 0       // 0-based column position
	leading := true // whether we are still in leading blanks

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
			if opts.initial && !leading {
				// R1: --initial: non-leading tab passes through unchanged.
				if werr := w.WriteByte('\t'); werr != nil {
					return werr
				}
				col++ // tab counts as 1 column when not expanded
			} else {
				// R1.1, R2: Expand tab to spaces.
				nextStop := nextTabStop(col, opts.tabStops)
				spaces := nextStop - col
				if spaces <= 0 {
					// R2.2: Past last explicit stop, insert one space.
					spaces = 1
				}
				for i := 0; i < spaces; i++ {
					if werr := w.WriteByte(' '); werr != nil {
						return werr
					}
				}
				col += spaces
			}
		case '\n':
			// R1.4: Newline resets column position.
			if werr := w.WriteByte('\n'); werr != nil {
				return werr
			}
			col = 0
			leading = true
		default:
			// R1.3: Non-tab characters pass through unchanged.
			if b != ' ' {
				leading = false
			}
			if werr := w.WriteByte(b); werr != nil {
				return werr
			}
			col++
		}
	}
}

// nextTabStop returns the next tab stop column (0-based) given the current column.
func nextTabStop(col int, tabStops []int) int {
	if len(tabStops) == 1 {
		// Uniform interval mode (R2.1).
		interval := tabStops[0]
		return ((col / interval) + 1) * interval
	}

	// Absolute position list mode (R2.2).
	// tabStops are 0-based absolute positions in strictly increasing order.
	for _, stop := range tabStops {
		if stop > col {
			return stop
		}
	}

	// Past last explicit stop: return col+1 to produce one space (R2.2).
	return col + 1
}

// parseArgs parses expand command-line flags manually.
func parseArgs(args []string) (expandOptions, []string) {
	opts := expandOptions{
		tabStops: []int{8}, // R1.1: default tab stop every 8 columns
	}

	var files []string
	i := 0

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--tabs=") {
			opts.tabStops = parseTabStops(arg[len("--tabs="):])
			i++
			continue
		}
		if arg == "--tabs" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "expand: option '--tabs' requires an argument\n")
				os.Exit(1)
			}
			opts.tabStops = parseTabStops(args[i])
			i++
			continue
		}
		if arg == "--initial" {
			opts.initial = true
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'i':
					opts.initial = true
					j++
				case 't':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "expand: option requires an argument -- 't'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					opts.tabStops = parseTabStops(val)
					j = len(arg) // consumed rest of arg
				default:
					// Check for numeric: -N is equivalent to -t N.
					if ch >= '0' && ch <= '9' {
						val := arg[j:]
						opts.tabStops = parseTabStops(val)
						j = len(arg)
					} else {
						fmt.Fprintf(os.Stderr, "expand: invalid option -- '%c'\n", ch)
						os.Exit(1)
					}
				}
			}
			i++
			continue
		}

		// Not a flag; treat as file argument.
		break
	}

	files = append(files, args[i:]...)
	return opts, files
}

// parseTabStops parses a tab stop specification: either a single integer N
// (uniform interval) or a comma-separated list of absolute positions.
func parseTabStops(s string) []int {
	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		// R2.4: Single value behaves as uniform interval.
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "expand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		return []int{n}
	}

	// R2.2: Multiple values are absolute column positions used directly with 0-based column counter.
	stops := make([]int, 0, len(parts))
	prev := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "expand: tab size contains invalid character(s): '%s'\n", s)
			os.Exit(1)
		}
		if n <= prev {
			fmt.Fprintf(os.Stderr, "expand: tab sizes must be ascending\n")
			os.Exit(1)
		}
		stops = append(stops, n)
		prev = n
	}
	return stops
}
