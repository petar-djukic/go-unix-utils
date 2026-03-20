// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand R1.1–R1.4, R2.1–R2.3, R3.1–R3.3: space-to-tab
// conversion for leading whitespace (default) or all whitespace (-a),
// with custom tab stop support (-t).
// R1.1: Replace leading spaces with tabs where alignment reaches a tab stop.
// R1.2: Non-leading whitespace passes through unchanged in default mode.
// R1.3: Spaces not reaching a tab stop are kept as spaces.
// R1.4: Existing tabs in leading whitespace advance column position normally.
// R2.1: -a converts all runs of spaces where tabs align, not just leading.
// R2.2: A single space not reaching a tab stop is kept even with -a.
// R2.3: -a processes the entire line past the first non-whitespace character.
// R3.1: -t N sets uniform interval; -t LIST sets absolute positions.
// R3.2: Past the last explicit tab stop in a LIST, spaces are kept as-is.
// R3.3: -t implies -a; custom tab stops convert all whitespace.
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

const defaultTabStop = 8

// tabConfig holds parsed tab stop configuration. R3.1.
type tabConfig struct {
	interval int   // uniform interval; used when stops is nil
	stops    []int // absolute 0-indexed positions; nil means uniform mode
}

// options holds parsed command-line options.
type options struct {
	allMode bool
	tabs    tabConfig
	files   []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts := parseArgs(os.Args[1:])
	os.Exit(run(opts))
}

// run processes all input sources and returns the exit code.
// R4.1: Returns 0 on success. R4.2: Returns 1 on file open error.
func run(opts options) int {
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	if len(opts.files) == 0 {
		unexpandReader(w, os.Stdin, opts)
	} else {
		exitCode = processFiles(w, opts)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFiles iterates over file arguments and unexpands each.
func processFiles(w *bufio.Writer, opts options) int {
	exitCode := 0
	for _, name := range opts.files {
		if err := processFile(w, name, opts); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file (or stdin for "-") and unexpands spaces.
func processFile(w *bufio.Writer, name string, opts options) error {
	if name == "-" {
		unexpandReader(w, os.Stdin, opts)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	defer f.Close()
	unexpandReader(w, f, opts)
	return nil
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// unexpandReader reads from r and converts spaces to tabs.
// R1.1–R1.4: default mode converts leading whitespace only.
// R2.1–R2.3: allMode converts all whitespace in the line.
func unexpandReader(w *bufio.Writer, r io.Reader, opts options) {
	br := bufio.NewReader(r)
	col := 0
	pending := 0
	leading := true
	for {
		b, err := br.ReadByte()
		if err != nil {
			flushSpaces(w, pending)
			return
		}
		col, pending, leading = processByte(
			w, b, col, pending, leading, opts,
		)
	}
}

// processByte handles one byte, dispatching based on mode.
// R1.2: In default mode, non-leading bytes pass through unchanged.
// R2.3: In allMode, all bytes are processed for conversion.
func processByte(w *bufio.Writer, b byte, col, pending int, leading bool, opts options) (int, int, bool) {
	if b == '\n' {
		flushSpaces(w, pending)
		w.WriteByte('\n')
		return 0, 0, true
	}
	if !leading && !opts.allMode {
		w.WriteByte(b)
		return col + 1, 0, false
	}
	return processConvert(w, b, col, pending, opts.tabs)
}

// processConvert handles a byte during whitespace conversion.
// Used for leading whitespace (default) and all whitespace (-a).
func processConvert(w *bufio.Writer, b byte, col, pending int, tc tabConfig) (int, int, bool) {
	switch b {
	case ' ':
		return processSpace(w, col, pending, tc)
	case '\t':
		return processTab(w, col, tc)
	default:
		flushSpaces(w, pending)
		w.WriteByte(b)
		return col + 1, 0, false
	}
}

// processSpace handles a space during conversion. R1.1, R1.3, R2.1, R2.2.
// R3.2: Past the last explicit stop, spaces are kept as-is.
func processSpace(w *bufio.Writer, col, pending int, tc tabConfig) (int, int, bool) {
	col++
	pending++
	if !canTab(col, tc) {
		return col, pending, true
	}
	if isTabStop(col, tc) {
		w.WriteByte('\t')
		pending = 0
	}
	return col, pending, true
}

// isTabStop returns true if col is exactly on a tab stop.
func isTabStop(col int, tc tabConfig) bool {
	if tc.stops == nil {
		return col%tc.interval == 0
	}
	return slices.Contains(tc.stops, col)
}

// canTab returns true if tab insertion is possible at this column.
// R3.2: Past the last explicit stop in a list, no tabs can be inserted.
func canTab(col int, tc tabConfig) bool {
	if tc.stops == nil {
		return true
	}
	return col <= tc.stops[len(tc.stops)-1]
}

// processTab handles a tab during conversion. R1.4.
func processTab(w *bufio.Writer, col int, tc tabConfig) (int, int, bool) {
	col += nextTabWidth(col, tc)
	w.WriteByte('\t')
	return col, 0, true
}

// nextTabWidth returns the number of columns a tab advances from col.
func nextTabWidth(col int, tc tabConfig) int {
	if tc.stops == nil {
		return tc.interval - col%tc.interval
	}
	for _, stop := range tc.stops {
		if stop > col {
			return stop - col
		}
	}
	// Past last stop: tab advances by 1 (matches GNU behavior).
	return 1
}

// flushSpaces writes n space characters to w.
func flushSpaces(w *bufio.Writer, n int) {
	for range n {
		w.WriteByte(' ')
	}
}

// parseArgs extracts options and file names from arguments.
func parseArgs(args []string) options {
	opts := options{}
	var rawStops []int
	hasCustomTabs := false
	endOfFlags := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			opts.files = append(opts.files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		rawStops, hasCustomTabs = parseFlag(
			arg, args, &i, &opts, rawStops, hasCustomTabs,
		)
	}
	opts.tabs = buildTabConfig(rawStops)
	// R3.3: -t implies -a.
	if hasCustomTabs {
		opts.allMode = true
	}
	return opts
}

// parseFlag processes a single flag argument, returning updated stops state.
func parseFlag(arg string, args []string, i *int, opts *options, rawStops []int, hasCustomTabs bool) ([]int, bool) {
	if arg == "-a" || arg == "--all" {
		opts.allMode = true
		return rawStops, hasCustomTabs
	}
	if arg == "--first-only" {
		return rawStops, hasCustomTabs
	}
	if val, ok := strings.CutPrefix(arg, "--tabs="); ok {
		return appendStops(rawStops, val), true
	}
	if strings.HasPrefix(arg, "-t") {
		val := extractTabArg(arg, args, i)
		if val != "" {
			return appendStops(rawStops, val), true
		}
		return rawStops, hasCustomTabs
	}
	return rawStops, hasCustomTabs
}

// extractTabArg handles -tVAL and -t VAL forms, advancing i as needed.
func extractTabArg(arg string, args []string, i *int) string {
	if len(arg) > 2 {
		return arg[2:]
	}
	if *i+1 < len(args) {
		*i++
		return args[*i]
	}
	return ""
}

// appendStops parses a comma-separated tab stop value and appends to stops.
func appendStops(stops []int, val string) []int {
	for p := range strings.SplitSeq(val, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			continue
		}
		stops = append(stops, n)
	}
	return stops
}

// buildTabConfig converts raw stop values into a tabConfig.
// Single value = uniform interval. Multiple values = absolute positions.
func buildTabConfig(stops []int) tabConfig {
	if len(stops) == 0 {
		return tabConfig{interval: defaultTabStop}
	}
	if len(stops) == 1 {
		return tabConfig{interval: stops[0]}
	}
	return tabConfig{stops: stops}
}
