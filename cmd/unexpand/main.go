// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand R1.1-R1.4, R2.1-R2.3, R3.1-R3.3, R4.1-R4.4:
// cmd/unexpand converts sequences of spaces to tabs at tab stop boundaries.
// By default, only leading whitespace is converted. The -a/--all flag converts
// all whitespace in the line. The --first-only flag restores default behavior;
// when both -a and --first-only are given, the last one on the command line wins.
// R2.1-R2.3: -t/--tabs supports a single interval or comma-separated list of
// tab stop positions. Specifying -t implies -a (R3.3).
// R4.1-R4.2: reads from files listed as arguments or stdin when no files are
// given. Treats '-' as stdin. On file open error, prints diagnostic to stderr
// and continues processing remaining files, exiting 1.
// R4.3-R4.4: exits 1 on write error. Installs SIGPIPE handler for clean exit
// on broken pipe.
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
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU unexpand format.
const progName = "unexpand"

// defaultTabStop is the default tab stop interval in columns (R1.1).
const defaultTabStop = 8

// tabStops holds the parsed tab stop configuration.
// R2.1-R2.2: either a single uniform interval or a list of explicit positions.
type tabStops struct {
	// uniform is the tab stop interval when a single value is given (R2.1).
	uniform int
	// positions holds explicit tab stop positions when multiple values are given (R2.2).
	// Empty when uniform mode is used.
	positions []int
}

// isTabStop returns true if col is at a tab stop boundary.
func (ts *tabStops) isTabStop(col int) bool {
	if len(ts.positions) == 0 {
		// R2.1: uniform interval mode.
		return col%ts.uniform == 0
	}
	// R2.2: explicit positions mode.
	return slices.Contains(ts.positions, col)
}

// nextStop returns the column position of the next tab stop after col.
// Returns -1 if there is no next tab stop (past all explicit positions).
func (ts *tabStops) nextStop(col int) int {
	if len(ts.positions) == 0 {
		// R2.1: uniform interval mode.
		return col + (ts.uniform - (col % ts.uniform))
	}
	// R2.2: explicit positions mode.
	for _, pos := range ts.positions {
		if pos > col {
			return pos
		}
	}
	// R3.2: past the last explicit tab stop, no tab can be inserted.
	return -1
}

// parseTabStops parses a comma-separated or single tab stop specification.
// R2.1: a single positive integer sets uniform interval.
// R2.2: multiple comma-separated positive integers set explicit positions.
func parseTabStops(s string) (*tabStops, error) {
	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		// R2.1: single value — uniform interval.
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
			return nil, fmt.Errorf("tab sizes must be ascending")
		}
		positions = append(positions, n)
		prev = n
	}
	return &tabStops{positions: positions}, nil
}

// printHelp prints usage information to stdout (R4.2).
func printHelp() {
	fmt.Print(`Usage: unexpand [OPTION]... [FILE]...
Convert blanks in each FILE (or standard input), writing to standard output.
With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
  -a, --all        convert all blanks, instead of just initial blanks
      --first-only  convert only leading sequences of blanks (overrides -a)
  -t, --tabs=N     have tabs N characters apart instead of 8 (enables -a)
  -t, --tabs=LIST  use comma separated list of tab positions (enables -a)
      --help       display this help and exit
      --version    output version information and exit
`)
}

func main() {
	sys.InstallSIGPIPEHandler()

	ts := &tabStops{uniform: defaultTabStop}
	convertAll := false
	firstOnly := false
	tabsSpecified := false
	args := os.Args[1:]
	var files []string

	// R1.3, R1.4, R2.1-R2.3, R4.1-R4.3: parse flags.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		// R4.1: --version prints version string to stdout and exits 0.
		if arg == "--version" {
			fmt.Printf("%s (go-unix-utils) %s\n", progName, version.Version)
			os.Exit(0)
		}
		// R4.2: --help prints usage to stdout and exits 0.
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "-a" || arg == "--all" {
			convertAll = true
			continue
		}
		if arg == "--first-only" {
			firstOnly = true
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
		} else if strings.HasPrefix(arg, "--") {
			// R4.3: unrecognized long option prints error to stderr and exits 1.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\nTry '%s --help' for more information.\n", progName, arg, progName)
			os.Exit(1)
		} else if strings.HasPrefix(arg, "-") && arg != "-" {
			// R4.3: invalid short option prints error to stderr and exits 1.
			// GNU format: "invalid option -- 'X'"
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\nTry '%s --help' for more information.\n", progName, arg[1:], progName)
			os.Exit(1)
		} else {
			files = append(files, arg)
			continue
		}

		parsed, err := parseTabStops(tabVal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			os.Exit(1)
		}
		ts = parsed
		tabsSpecified = true
	}

	// R3.3: -t implies -a.
	if tabsSpecified {
		convertAll = true
	}

	// R1.4: --first-only overrides -a when both are present.
	if firstOnly {
		convertAll = false
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		// R1.1: no file arguments — read from stdin.
		if err := unexpandReader(os.Stdin, w, ts, convertAll); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		for _, name := range files {
			if name == "-" {
				// R1.1: '-' means read from stdin.
				if err := unexpandReader(os.Stdin, w, ts, convertAll); err != nil {
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
			if err := unexpandReader(f, w, ts, convertAll); err != nil {
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

// unexpandReader reads from r and writes space-to-tab converted output to w.
// R1.1: spaces that reach a tab stop boundary are replaced by a tab character.
// R1.2: in default mode, only leading whitespace is converted; characters after
// the first non-whitespace character pass through unchanged.
// R1.3: when convertAll is true (-a), all space sequences in the line are
// subject to tab conversion, not just leading whitespace.
// R1.4: existing tab characters in the input count toward column position and
// are output as tabs; they do not prevent further tab substitution.
// R2.1-R2.2: uses tabStops for custom tab stop intervals or position lists.
// R3.2: when past the last explicit tab stop, spaces are kept as-is.
func unexpandReader(r io.Reader, w *bufio.Writer, ts *tabStops, convertAll bool) error {
	br := bufio.NewReader(r)
	col := 0
	pending := 0     // number of accumulated spaces not yet output
	inInitial := true // whether we are in the leading blank region of a line

	flushPending := func() error {
		for range pending {
			if err := w.WriteByte(' '); err != nil {
				return err
			}
		}
		pending = 0
		return nil
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return flushPending()
			}
			return err
		}

		converting := convertAll || inInitial

		switch b {
		case ' ':
			if !converting {
				// R1.2: non-leading spaces pass through in default mode.
				if err := w.WriteByte(' '); err != nil {
					return err
				}
				col++
				continue
			}
			pending++
			col++
			// R3.2: check if there is a next tab stop reachable.
			next := ts.nextStop(col - pending)
			if next == -1 {
				// Past all explicit tab stops; flush pending as spaces.
				if err := flushPending(); err != nil {
					return err
				}
				continue
			}
			// R1.1, R2.1: when spaces reach a tab stop, replace with a tab.
			if ts.isTabStop(col) {
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				pending = 0
			}
		case '\t':
			if !converting {
				// R1.2: tabs after non-leading region pass through unchanged.
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				next := ts.nextStop(col)
				if next == -1 {
					col++
				} else {
					col = next
				}
				continue
			}
			// R1.4: existing tab advances to next tab stop.
			// Pending spaces plus this tab all advance to the next tab stop,
			// represented as a single tab character.
			next := ts.nextStop(col)
			if next == -1 {
				// Past all explicit tab stops; flush pending spaces and output tab.
				if err := flushPending(); err != nil {
					return err
				}
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				col++
			} else {
				col = next
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				pending = 0
			}
		case '\n':
			// Flush any pending spaces before the newline.
			if err := flushPending(); err != nil {
				return err
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			col = 0
			inInitial = true
		default:
			// R1.2: first non-blank character ends the initial region.
			if err := flushPending(); err != nil {
				return err
			}
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
