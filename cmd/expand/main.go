// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd024-expand R1.1-R1.4, R2.1-R2.4, R3.1-R3.4: cmd/expand converts
// tab characters to the appropriate number of spaces to reach the next tab stop.
// Supports -t/--tabs for custom tab stop intervals or explicit position lists.
// Supports -i/--initial to convert only leading tabs on each line.
// Reads from files listed as arguments or stdin when no files are given.
// Treats '-' as stdin. Installs SIGPIPE handler for clean exit on broken pipe.
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

// progName is the name used in error messages to match GNU expand format.
const progName = "expand"

// defaultTabStop is the default tab stop interval in columns (R1.1).
const defaultTabStop = 8

// tabStops holds the parsed tab stop configuration.
// R2.1-R2.4: either a single uniform interval or a list of explicit positions.
type tabStops struct {
	// uniform is the tab stop interval when a single value is given (R2.1, R2.4).
	uniform int
	// positions holds explicit tab stop positions when multiple values are given (R2.2).
	// Empty when uniform mode is used.
	positions []int
}

// nextStop returns the number of spaces needed to reach the next tab stop
// from the given 0-indexed column position.
func (ts *tabStops) nextStop(col int) int {
	if len(ts.positions) == 0 {
		// R2.1, R2.4: uniform interval mode.
		return ts.uniform - (col % ts.uniform)
	}
	// R2.2: explicit positions mode.
	for _, pos := range ts.positions {
		if pos > col {
			return pos - col
		}
	}
	// R2.2: past the last explicit tab stop, replace with a single space.
	return 1
}

func main() {
	sys.InstallSIGPIPEHandler()

	ts := &tabStops{uniform: defaultTabStop}
	initialOnly := false
	args := os.Args[1:]
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		// R3.1: parse -i/--initial flag.
		if arg == "-i" || arg == "--initial" {
			initialOnly = true
			continue
		}
		// R2.1-R2.3: parse -t/--tabs option.
		var tabVal string
		if arg == "-t" || arg == "--tabs" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 't'\n", progName)
				os.Exit(1)
			}
			i++
			tabVal = args[i]
		} else if strings.HasPrefix(arg, "-t") {
			tabVal = arg[2:]
		} else if strings.HasPrefix(arg, "--tabs=") {
			tabVal = arg[7:]
		} else {
			files = append(files, arg)
			continue
		}

		parsed, err := parseTabStops(tabVal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			os.Exit(1)
		}
		// R2.3: last -t value takes effect.
		ts = parsed
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		// R3.3: no file arguments — read from stdin.
		if err := expandReader(os.Stdin, w, ts, initialOnly); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		// R3.4: process each file in argument order as a concatenated stream.
		for _, name := range files {
			if name == "-" {
				// R3.3: '-' means read from stdin.
				if err := expandReader(os.Stdin, w, ts, initialOnly); err != nil {
					fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
					exitCode = 1
				}
				continue
			}

			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				exitCode = 1
				continue
			}
			if err := expandReader(f, w, ts, initialOnly); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
				exitCode = 1
			}
			f.Close() // best-effort close; read errors already reported
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseTabStops parses a comma-separated or single tab stop specification.
// R2.1: a single positive integer sets uniform interval.
// R2.2: multiple comma-separated positive integers set explicit positions.
// R2.4: validates that values are strictly increasing positive integers.
func parseTabStops(s string) (*tabStops, error) {
	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		// R2.1, R2.4: single value — uniform interval.
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("tab size contains invalid character(s): '%s'", s)
		}
		if n == 0 {
			return nil, fmt.Errorf("tab size cannot be 0")
		}
		return &tabStops{uniform: n}, nil
	}

	// R2.2: multiple values — explicit tab stop positions.
	positions := make([]int, 0, len(parts))
	prev := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("tab size contains invalid character(s): '%s'", s)
		}
		if n == 0 {
			return nil, fmt.Errorf("tab size cannot be 0")
		}
		if n <= prev {
			// R2.4: not strictly increasing.
			return nil, fmt.Errorf("tab sizes must be ascending")
		}
		positions = append(positions, n)
		prev = n
	}
	return &tabStops{positions: positions}, nil
}

// expandReader reads from r and writes tab-expanded output to w.
// R1.1: tabs are replaced by spaces to reach the next tab stop.
// R1.2: consecutive tabs each advance to the next tab stop independently.
// R1.3: non-tab characters are written unchanged; each byte counts as one column.
// R1.4: newline resets the column position to 0 (0-indexed internally).
// R3.1: when initialOnly is true, only leading tabs (before the first non-blank
// character on each line) are expanded; tabs after non-blank content pass through.
func expandReader(r io.Reader, w *bufio.Writer, ts *tabStops, initialOnly bool) error {
	br := bufio.NewReader(r)
	col := 0
	// R3.1: track whether we are still in the leading blank region of a line.
	inInitial := true

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
			// R3.1, R3.2: when initialOnly is set and we have passed the leading
			// blank region, output the tab character unchanged.
			if initialOnly && !inInitial {
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				col++
				continue
			}
			// R1.1, R1.2, R2.1-R2.2: replace tab with spaces to next tab stop.
			spaces := ts.nextStop(col)
			for range spaces {
				if err := w.WriteByte(' '); err != nil {
					return err
				}
			}
			col += spaces
		case '\n':
			// R1.4: newline resets column position.
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			col = 0
			inInitial = true
		case ' ':
			// Space is a blank character; does not end the initial region.
			if err := w.WriteByte(' '); err != nil {
				return err
			}
			col++
		default:
			// R1.3: non-tab, non-blank characters pass through unchanged.
			// R3.1: first non-blank character ends the initial region.
			if err := w.WriteByte(b); err != nil {
				return err
			}
			col++
			inInitial = false
		}
	}
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
