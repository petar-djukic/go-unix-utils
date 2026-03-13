// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expand implements the expand (convert tabs to spaces) command.
// Implements: prd024-expand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4
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

// R1.1: Default tab stop interval is 8 columns.
const defaultTabStop = 8

// tabConfig holds the tab stop configuration parsed from -t/--tabs.
type tabConfig struct {
	// uniform is true when a single interval is used (default or -t N).
	uniform bool
	// interval is the uniform tab stop interval (used when uniform is true).
	interval int
	// stops is the list of absolute tab stop positions (0-indexed, used when uniform is false).
	stops []int
}

func main() {
	// R3.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	tc := tabConfig{uniform: true, interval: defaultTabStop}
	initial := false
	files, err := parseArgs(os.Args[1:], &tc, &initial)
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand: %v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(files) == 0 {
		// R1.1: No file arguments — read from stdin.
		if err := expandReader(os.Stdin, w, &tc, initial); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, name := range files {
			if err := expandFile(name, w, &tc, initial); err != nil {
				fmt.Fprintf(os.Stderr, "expand: %v\n", err)
				exitCode = 1
			}
		}
	}

	// R3.3: Flush buffered output; exit 1 on write error.
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
		os.Exit(1)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// parseArgs parses command-line arguments, extracting -t/--tabs and -i/--initial
// flags. Returns the remaining file arguments.
func parseArgs(args []string, tc *tabConfig, initial *bool) ([]string, error) {
	var files []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// --tabs=VALUE
		if strings.HasPrefix(arg, "--tabs=") {
			val := arg[len("--tabs="):]
			if err := parseTabStops(val, tc); err != nil {
				return nil, err
			}
			continue
		}
		if arg == "--tabs" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option '--tabs' requires an argument")
			}
			i++
			if err := parseTabStops(args[i], tc); err != nil {
				return nil, err
			}
			continue
		}

		// --initial
		if arg == "--initial" {
			*initial = true
			continue
		}

		// Short flags: -t, -i, or combined like -it, -ti, -t4
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			rest := arg[1:]
			for len(rest) > 0 {
				switch rest[0] {
				case 'i':
					*initial = true
					rest = rest[1:]
				case 't':
					rest = rest[1:]
					if len(rest) > 0 {
						// -tVALUE (e.g., -t4 or -t4,8)
						if err := parseTabStops(rest, tc); err != nil {
							return nil, err
						}
						rest = ""
					} else {
						// -t VALUE (next arg)
						if i+1 >= len(args) {
							return nil, fmt.Errorf("option requires an argument -- 't'")
						}
						i++
						if err := parseTabStops(args[i], tc); err != nil {
							return nil, err
						}
					}
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", rest[0])
				}
			}
			continue
		}

		files = append(files, arg)
	}

	return files, nil
}

// parseTabStops parses a tab stop specification string.
// R2.1: A single number sets uniform interval.
// R2.2: A comma-separated list sets absolute positions (must be strictly increasing).
// R2.3: Last -t wins (caller passes the same tabConfig pointer).
// R2.4: Single value in a list behaves as uniform interval.
func parseTabStops(spec string, tc *tabConfig) error {
	parts := strings.Split(spec, ",")

	stops := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("tab size contains invalid character(s): '%s'", spec)
		}
		if n <= 0 {
			if n == 0 {
				return fmt.Errorf("tab size cannot be 0")
			}
			return fmt.Errorf("tab size contains invalid character(s): '%s'", spec)
		}
		stops = append(stops, n)
	}

	// R2.2: Validate strictly increasing order.
	for i := 1; i < len(stops); i++ {
		if stops[i] <= stops[i-1] {
			return fmt.Errorf("tab sizes must be ascending")
		}
	}

	// R2.4: Single value behaves as uniform interval.
	if len(stops) == 1 {
		tc.uniform = true
		tc.interval = stops[0]
		tc.stops = nil
	} else {
		tc.uniform = false
		// Tab stop positions are used as-is: column 0 is the first position,
		// so --tabs=4,8 means stops at columns 4 and 8.
		tc.stops = stops
		tc.interval = 0
	}

	return nil
}

// nextTabStop returns the number of spaces to insert for a tab at the given
// 0-indexed column position.
func nextTabStop(col int, tc *tabConfig) int {
	if tc.uniform {
		// R1.1, R2.1: Uniform tab stop interval.
		return tc.interval - (col % tc.interval)
	}

	// R2.2: Absolute tab stop list.
	for _, stop := range tc.stops {
		if stop > col {
			return stop - col
		}
	}

	// R2.2: Tab at or past the last explicit stop → single space.
	return 1
}

// expandFile opens name and expands tabs in its contents to the writer.
// "-" reads from stdin.
func expandFile(name string, w *bufio.Writer, tc *tabConfig, initial bool) error {
	if name == "-" {
		return expandReader(os.Stdin, w, tc, initial)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return expandReader(f, w, tc, initial)
}

// expandReader reads from r and writes tab-expanded output to w.
//
// R1.1: Tabs are replaced by spaces to advance to the next tab stop.
// R1.2: Multiple consecutive tabs each advance to the next tab stop independently.
// R1.3: Non-tab characters are written unchanged; each advances column by one.
// R1.4: Backspace characters are passed through and decrement the column (minimum 0).
// R2.1-R2.4: Custom tab stops via tabConfig.
func expandReader(r io.Reader, w *bufio.Writer, tc *tabConfig, initial bool) error {
	br := bufio.NewReader(r)
	col := 0       // 0-indexed column position
	leading := true // whether we are still in leading whitespace (for --initial)

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		switch b {
		case '\t':
			if !initial || leading {
				// R1.1, R1.2, R2.1-R2.4: Replace tab with spaces.
				spaces := nextTabStop(col, tc)
				for range spaces {
					if werr := w.WriteByte(' '); werr != nil {
						return fmt.Errorf("write error: %w", werr)
					}
				}
				col += spaces
			} else {
				// --initial: non-leading tab passed through unchanged.
				if werr := w.WriteByte('\t'); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				col++ // tab character occupies one column when not expanded
			}
		case '\n':
			// R1.3: Newline resets column to 0.
			if werr := w.WriteByte('\n'); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			col = 0
			leading = true
		case '\b':
			// R1.4: Backspace decrements column position (minimum 0).
			if werr := w.WriteByte('\b'); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			if col > 0 {
				col--
			}
		default:
			// R1.3: Non-tab characters passed through, each advances column by one.
			if werr := w.WriteByte(b); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			col++
			if b != ' ' {
				leading = false
			}
		}
	}
}
