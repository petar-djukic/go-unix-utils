// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expand converts tabs to spaces (prd024-expand R1, R2, R3).
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
	cfg := parseArgs(os.Args[1:])
	os.Exit(run(cfg))
}

type config struct {
	files   []string
	tabSpec string // raw -t value; empty means default
	initial bool   // R3.1: -i / --initial: only expand leading tabs
}

// tabStops holds parsed tab stop configuration.
// R2.4: single value = uniform interval; multiple = explicit positions.
type tabStops struct {
	uniform  int   // >0 when using a uniform interval
	explicit []int // absolute 0-based column positions (sorted, ascending)
}

func parseArgs(args []string) config {
	var cfg config
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if a == "-" || len(a) == 0 || a[0] != '-' {
			cfg.files = append(cfg.files, a)
			continue
		}
		if a == "-i" || a == "--initial" {
			cfg.initial = true
			continue
		}
		if err := parseTabArg(a, args, &i, &cfg); err == nil {
			continue
		}
		die(fmt.Sprintf("invalid option -- '%s'", a[1:]))
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg
}

// parseTabArg handles -t / --tabs arguments. Returns nil if matched.
func parseTabArg(a string, args []string, i *int, cfg *config) error {
	// R2.3: last -t wins
	if a == "-t" || a == "--tabs" {
		if *i+1 >= len(args) {
			die("option requires an argument -- 't'")
		}
		*i++
		cfg.tabSpec = args[*i]
		return nil
	}
	if strings.HasPrefix(a, "-t") {
		cfg.tabSpec = a[2:]
		return nil
	}
	if strings.HasPrefix(a, "--tabs=") {
		cfg.tabSpec = a[7:]
		return nil
	}
	return fmt.Errorf("not a tab arg")
}

// parseTabStops parses the -t value into a tabStops struct.
// R2.1: single integer = uniform interval.
// R2.2: comma-separated or space-separated list = explicit positions.
// R2.4: list with one element = uniform interval.
func parseTabStops(spec string) tabStops {
	if spec == "" {
		return tabStops{uniform: defaultTabStop}
	}
	parts := splitTabSpec(spec)
	if len(parts) == 1 {
		return tabStops{uniform: parsePositiveInt(parts[0])}
	}
	positions := make([]int, len(parts))
	for i, p := range parts {
		v := parsePositiveInt(p)
		positions[i] = v
		if i > 0 && positions[i] <= positions[i-1] {
			die("tab sizes must be ascending")
		}
	}
	return tabStops{explicit: positions}
}

func splitTabSpec(spec string) []string {
	if strings.ContainsRune(spec, ',') {
		return strings.Split(spec, ",")
	}
	return strings.Fields(spec)
}

func parsePositiveInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		die(fmt.Sprintf("tab size contains invalid character(s): '%s'", s))
	}
	return n
}

// run processes all files and returns the exit code.
func run(cfg config) int {
	ts := parseTabStops(cfg.tabSpec)
	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(name, out, ts, cfg.initial); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			exitCode = 1
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFile opens one input and expands its tabs.
func processFile(name string, out *bufio.Writer, ts tabStops, initial bool) error {
	r, err := openInput(name)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return fmt.Errorf("%s: %s", name, pe.Err)
		}
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return expandStream(bufio.NewReader(r), out, ts, initial)
}

func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// expandStream reads input byte by byte and replaces tabs with spaces.
// R1.1-R1.4: default expansion. R2.1-R2.4: custom tab stops.
// R3.1: when initial is true, only leading tabs are expanded.
func expandStream(r *bufio.Reader, out *bufio.Writer, ts tabStops, initial bool) error {
	col := 0
	leading := true
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if werr := handleByte(out, c, &col, &leading, ts, initial); werr != nil {
			return werr
		}
	}
}

// handleByte processes one input byte, expanding tabs when appropriate.
// R3.1: in initial mode, tabs after non-whitespace pass through unchanged.
// R3.2: backspace decrements column position to match GNU expand behavior.
// R3.3: non-tab characters including NUL bytes are written unchanged.
func handleByte(out *bufio.Writer, c byte, col *int, leading *bool, ts tabStops, initial bool) error {
	switch c {
	case '\n':
		*col = 0
		*leading = true
		return out.WriteByte('\n')
	case '\t':
		if !initial || *leading {
			return expandTab(out, col, ts)
		}
		*col++
		return out.WriteByte('\t')
	case '\b':
		// R3.2: backspace decrements column (floor at 0)
		if *col > 0 {
			*col--
		}
		*leading = false
		return out.WriteByte('\b')
	default:
		// R3.1: space preserves leading state; any other char ends it
		if c != ' ' {
			*leading = false
		}
		*col++
		return out.WriteByte(c)
	}
}

// expandTab writes spaces to advance to the next tab stop.
func expandTab(out *bufio.Writer, col *int, ts tabStops) error {
	spaces := computeSpaces(*col, ts)
	for range spaces {
		if err := out.WriteByte(' '); err != nil {
			return err
		}
	}
	*col += spaces
	return nil
}

// computeSpaces returns the number of spaces needed for a tab at col.
// R2.1: uniform interval uses modular arithmetic.
// R2.2: explicit positions find the next position past col.
// R2.2: tab past last explicit stop = single space.
func computeSpaces(col int, ts tabStops) int {
	if ts.uniform > 0 {
		return ts.uniform - col%ts.uniform
	}
	for _, stop := range ts.explicit {
		if stop > col {
			return stop - col
		}
	}
	return 1
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "expand: %s\n", msg)
	os.Exit(1)
}
