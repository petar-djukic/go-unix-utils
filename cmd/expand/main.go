// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd024-expand R1.1–R1.4, R2.1–R2.4, R3.1–R3.4: tab-to-space
// expansion with default and custom tab stop support, exit codes, and SIGPIPE.
// R1.1: Default tab expansion with tab stops every 8 columns.
// R1.2: Multiple consecutive tabs each advance independently.
// R1.3: Non-tab characters pass through unchanged.
// R1.4: Newline resets column position to 1.
// R2.1: -t N sets uniform tab stop interval.
// R2.2: -t LIST sets absolute tab stop positions.
// R2.3: Last -t wins; replaces default of 8.
// R2.4: Single-value LIST behaves as uniform interval.
// R3.1: Exit 0 on success.
// R3.2: Exit 1 on file open error; continue processing remaining files.
// R3.3: Exit 1 on stdout write error.
// R3.4: SIGPIPE handled via pkg/sys.InstallSIGPIPEHandler.
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

// tabConfig holds parsed tab stop configuration. R2.1–R2.4.
type tabConfig struct {
	interval int   // uniform interval; used when stops is nil
	stops    []int // absolute 0-indexed positions; nil means uniform mode
}

func main() {
	sys.InstallSIGPIPEHandler()
	tc, files := parseArgs(os.Args[1:])
	os.Exit(run(tc, files))
}

// run processes all input sources and returns the exit code.
// R3.1: Returns 0 on success. R3.2: Returns 1 on file open error.
// R3.3: Returns 1 on stdout write error.
func run(tc tabConfig, files []string) int {
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	if len(files) == 0 {
		expandReader(w, os.Stdin, tc)
	} else {
		exitCode = processFiles(w, files, tc)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFiles iterates over file arguments and expands each. R3.2.
func processFiles(w *bufio.Writer, files []string, tc tabConfig) int {
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name, tc); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file (or stdin for "-") and expands tabs.
func processFile(w *bufio.Writer, name string, tc tabConfig) error {
	if name == "-" {
		expandReader(w, os.Stdin, tc)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	defer f.Close()
	expandReader(w, f, tc)
	return nil
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// expandReader reads from r and replaces tabs with spaces.
func expandReader(w *bufio.Writer, r io.Reader, tc tabConfig) {
	br := bufio.NewReader(r)
	col := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		col = expandByte(w, b, col, tc)
	}
}

// expandByte processes one byte and returns the updated column position.
func expandByte(w *bufio.Writer, b byte, col int, tc tabConfig) int {
	switch b {
	case '\t':
		return expandTab(w, col, tc)
	case '\n':
		w.WriteByte('\n')
		return 0
	default:
		w.WriteByte(b)
		return col + 1
	}
}

// expandTab replaces a tab with spaces to reach the next tab stop.
func expandTab(w *bufio.Writer, col int, tc tabConfig) int {
	spaces := computeSpaces(col, tc)
	for range spaces {
		w.WriteByte(' ')
	}
	return col + spaces
}

// computeSpaces returns the number of spaces for a tab at the given column.
// R2.1: Uniform interval uses modular arithmetic.
// R2.2: Absolute stops finds the first stop past col; single space if past all.
func computeSpaces(col int, tc tabConfig) int {
	if tc.stops == nil {
		return tc.interval - col%tc.interval
	}
	for _, stop := range tc.stops {
		if stop > col {
			return stop - col
		}
	}
	// R2.2: Past the last explicit tab stop, replace with a single space.
	return 1
}

// parseArgs extracts tab config and file names from arguments.
// R2.3: -t replaces the default. Multiple -t values accumulate into a list.
func parseArgs(args []string) (tabConfig, []string) {
	var rawStops []int
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
		if val, ok := strings.CutPrefix(arg, "--tabs="); ok {
			rawStops = appendStops(rawStops, val)
			continue
		}
		if strings.HasPrefix(arg, "-t") {
			val := extractTabArg(arg, args, &i)
			if val != "" {
				rawStops = appendStops(rawStops, val)
			}
			continue
		}
	}
	return buildTabConfig(rawStops), files
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
	parts := strings.Split(val, ",")
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			continue
		}
		stops = append(stops, n)
	}
	return stops
}

// buildTabConfig converts raw stop values into a tabConfig.
// R2.4: Single value = uniform interval. Multiple values = absolute positions.
func buildTabConfig(stops []int) tabConfig {
	if len(stops) == 0 {
		return tabConfig{interval: defaultTabStop}
	}
	if len(stops) == 1 {
		return tabConfig{interval: stops[0]}
	}
	return tabConfig{stops: stops}
}
