// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/unexpand implements the unexpand (convert spaces to tabs) command.
// Implements: prd025-unexpand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R4.1, R4.2, R4.3, R4.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.4: Default tab stop interval is 8 columns.
const defaultTabStop = 8

// tabConfig holds the tab stop configuration parsed from -t/--tabs.
type tabConfig struct {
	// uniform is true when a single interval is used (default or -t N).
	uniform bool
	// interval is the uniform tab stop interval (used when uniform is true).
	interval int
	// stops is the list of absolute tab stop positions (used when uniform is false).
	stops []int
}

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	// R4.4: Handle --version and --help before flag parsing so they
	// take precedence and exit 0, matching GNU unexpand behavior.
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--version" {
			fmt.Println("unexpand (go-unix-utils)")
			os.Exit(0)
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
	}

	tc := tabConfig{uniform: true, interval: defaultTabStop}
	allMode := false
	files, err := parseArgs(os.Args[1:], &tc, &allMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(files) == 0 {
		// R1.1: No file arguments — read from stdin.
		if err := unexpandReader(os.Stdin, w, &tc, allMode); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, name := range files {
			if err := unexpandFile(name, w, &tc, allMode); err != nil {
				fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
				exitCode = 1
			}
		}
	}

	// Flush buffered output; exit 1 on write error.
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: write error: %v\n", err)
		os.Exit(1)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// parseArgs parses command-line arguments, extracting -a/--all, --first-only,
// and -t/--tabs flags. Returns the remaining file arguments.
//
// R2.1-R2.3: -a flag for all-whitespace conversion.
// R3.1: -t/--tabs for custom tab stops.
// R3.3: -t implies -a.
func parseArgs(args []string, tc *tabConfig, allMode *bool) ([]string, error) {
	var files []string
	endOfFlags := false
	customTab := false

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
			customTab = true
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
			customTab = true
			continue
		}

		// --all
		if arg == "--all" {
			*allMode = true
			continue
		}

		// --first-only
		if arg == "--first-only" {
			*allMode = false
			continue
		}

		// Short flags: -a, -t, or combined like -at, -t4
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			rest := arg[1:]
			for len(rest) > 0 {
				switch rest[0] {
				case 'a':
					*allMode = true
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
					customTab = true
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", rest[0])
				}
			}
			continue
		}

		files = append(files, arg)
	}

	// R3.3: -t implies -a.
	if customTab {
		*allMode = true
	}

	return files, nil
}

// parseTabStops parses a tab stop specification string.
// R3.1: A single number sets uniform interval. A comma-separated list sets absolute
// positions (must be strictly increasing).
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

	// Validate strictly increasing order.
	for i := 1; i < len(stops); i++ {
		if stops[i] <= stops[i-1] {
			return fmt.Errorf("tab sizes must be ascending")
		}
	}

	if len(stops) == 1 {
		tc.uniform = true
		tc.interval = stops[0]
		tc.stops = nil
	} else {
		tc.uniform = false
		tc.stops = stops
		tc.interval = 0
	}

	return nil
}

// isAtTabStop reports whether col is at a tab stop position.
func isAtTabStop(col int, tc *tabConfig) bool {
	if col == 0 {
		return false
	}
	if tc.uniform {
		return col%tc.interval == 0
	}
	return slices.Contains(tc.stops, col)
}

// advanceTab returns the column after a tab character at the given position.
func advanceTab(col int, tc *tabConfig) int {
	if tc.uniform {
		return col + (tc.interval - col%tc.interval)
	}
	for _, stop := range tc.stops {
		if stop > col {
			return stop
		}
	}
	// Past last stop: tab advances by 1.
	return col + 1
}

// canReachTabStop reports whether there is a tab stop beyond col.
// R3.2: When past the last explicit tab stop, spaces are kept as-is.
func canReachTabStop(col int, tc *tabConfig) bool {
	if tc.uniform {
		return true
	}
	for _, stop := range tc.stops {
		if stop > col {
			return true
		}
	}
	return false
}

// unexpandFile opens name and converts spaces to tabs in its contents.
// "-" reads from stdin.
func unexpandFile(name string, w *bufio.Writer, tc *tabConfig, allMode bool) error {
	if name == "-" {
		return unexpandReader(os.Stdin, w, tc, allMode)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return unexpandReader(f, w, tc, allMode)
}

// unexpandReader reads from r and writes output with spaces converted to tabs to w.
//
// R1.1: Replace leading spaces with tabs when a run of spaces reaches a tab stop exactly.
// R1.2: Non-leading whitespace is written unchanged in default mode.
// R1.3: Partial runs of spaces that do not reach a tab stop are kept as spaces.
// R1.4: Existing tabs in leading whitespace count toward column position.
// R2.1: With allMode, convert all runs of spaces where replacement aligns to a tab stop.
// R2.2: A single space that does not reach a tab stop is kept as a space.
// R2.3: In allMode, the entire line is processed, not just leading whitespace.
func unexpandReader(r io.Reader, w *bufio.Writer, tc *tabConfig, allMode bool) error {
	br := bufio.NewReader(r)
	col := 0        // 0-indexed column position
	leading := true // whether we are in leading whitespace
	spaces := 0     // accumulated spaces in current run

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Flush any trailing spaces that didn't reach a tab stop.
				return writeSpaces(w, spaces)
			}
			return fmt.Errorf("read error: %w", err)
		}

		converting := leading || allMode

		switch {
		case b == '\n':
			// End of line — flush remaining spaces and newline, reset state.
			if err := writeSpaces(w, spaces); err != nil {
				return err
			}
			spaces = 0
			if werr := w.WriteByte('\n'); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			col = 0
			leading = true

		case b == ' ' && converting && canReachTabStop(col, tc):
			// R1.1, R2.1: Accumulate spaces; emit tab when column reaches a tab stop.
			col++
			spaces++
			if isAtTabStop(col, tc) {
				if werr := w.WriteByte('\t'); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				spaces = 0
			}

		case b == '\t' && converting:
			// R1.4: Tab in converting mode — discard accumulated spaces
			// (the tab overshoots past them) and advance to next tab stop.
			spaces = 0
			if werr := w.WriteByte('\t'); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			col = advanceTab(col, tc)

		default:
			// R1.2, R2.2, R3.2: Non-converting characters, spaces past last
			// tab stop, or non-whitespace characters.
			if err := writeSpaces(w, spaces); err != nil {
				return err
			}
			spaces = 0
			if werr := w.WriteByte(b); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			if b == '\t' {
				col = advanceTab(col, tc)
			} else {
				col++
			}
			if b != ' ' && b != '\t' {
				leading = false
			}
		}
	}
}

// writeSpaces writes n space characters to w.
func writeSpaces(w *bufio.Writer, n int) error {
	for range n {
		if err := w.WriteByte(' '); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	return nil
}

// printHelp writes usage information to stdout, matching the format of
// GNU unexpand --help output.
// R4.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: unexpand [OPTION]... [FILE]...
Convert spaces to tabs, writing to standard output.

With no FILE, or when FILE is -, read standard input.

  -a, --all              convert all whitespace, instead of just initial whitespace
      --first-only       convert only leading sequences of whitespace (overrides -a)
  -t, --tabs=N           have tabs N characters apart instead of 8 (enables -a)
  -t, --tabs=LIST        use comma separated list of tab positions (enables -a)
      --help             display this help and exit
      --version          output version information and exit
`)
}
