// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/unexpand: convert spaces to tabs.
// Implements srd025-unexpand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultTabStop = 8

// openInput returns os.Stdin for "-", otherwise opens the named file.
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError extracts the underlying error for GNU-compatible messages.
func formatOpenError(name string, err error) error {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// parseTabValue parses a tab stop specification into a slice of stops.
// R3.1: single number = uniform interval; comma/space-separated = absolute positions.
func parseTabValue(s string) ([]int, error) {
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, fmt.Errorf("tab size cannot be 0")
	}
	stops := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("tab size contains invalid character(s): %q", f)
		}
		if n <= 0 {
			return nil, fmt.Errorf("tab sizes must be ascending")
		}
		if len(stops) > 0 && n <= stops[len(stops)-1] {
			return nil, fmt.Errorf("tab sizes must be ascending")
		}
		stops = append(stops, n)
	}
	return stops, nil
}

// extractTabFlag extracts the -t/--tabs value from the current arg.
// Returns the value string, number of extra args consumed, and any error.
func extractTabFlag(arg string, args []string, i int) (string, int, error) {
	if strings.HasPrefix(arg, "--tabs=") {
		return arg[len("--tabs="):], 0, nil
	}
	if arg == "--tabs" {
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("option '--tabs' requires an argument")
		}
		return args[i+1], 1, nil
	}
	if strings.HasPrefix(arg, "-t") {
		rest := arg[2:]
		if rest != "" {
			return rest, 0, nil
		}
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("option requires an argument -- 't'")
		}
		return args[i+1], 1, nil
	}
	return "", 0, nil
}

// handleModeFlag checks if arg is a mode flag (-a, --all, --first-only).
// Returns whether the arg was handled and the new convertAll value.
func handleModeFlag(arg string) (handled bool, convertAll bool) {
	switch arg {
	case "-a", "--all":
		return true, true
	case "--first-only":
		return true, false
	default:
		return false, false
	}
}

// parseArgs parses command-line arguments into conversion mode, tab stops, and files.
func parseArgs(args []string) (bool, []int, []string, error) {
	convertAll := false
	stops := []int{defaultTabStop}
	var files []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if handled, ca := handleModeFlag(arg); handled {
			convertAll = ca
			continue
		}
		val, skip, err := extractTabFlag(arg, args, i)
		if err != nil {
			return false, nil, nil, err
		}
		if val != "" {
			parsed, err := parseTabValue(val)
			if err != nil {
				return false, nil, nil, err
			}
			stops = parsed
			convertAll = true // R3.3: -t implies -a
			i += skip
			continue
		}
		return false, nil, nil, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	return convertAll, stops, files, nil
}

// isAtTabStop reports whether col is exactly at a tab stop position.
// R1.1: used to determine when accumulated spaces can be replaced with a tab.
func isAtTabStop(col int, stops []int) bool {
	if col == 0 {
		return false
	}
	if len(stops) == 1 {
		return col%stops[0] == 0
	}
	return slices.Contains(stops, col)
}

// nextTabStopCol returns the next tab stop column after col.
// Used when an input tab character advances the column position.
func nextTabStopCol(col int, stops []int) int {
	if len(stops) == 1 {
		return col + stops[0] - (col % stops[0])
	}
	for _, s := range stops {
		if s > col {
			return s
		}
	}
	return col + 1
}

// flushSpaces writes n space characters to w.
func flushSpaces(w *bufio.Writer, n int) error {
	for range n {
		if err := w.WriteByte(' '); err != nil {
			return err
		}
	}
	return nil
}

// lineState tracks column position and conversion state within a line.
type lineState struct {
	col       int
	pending   int
	inLeading bool
}

// unexpandReader reads from r, converts spaces to tabs, and writes to w.
// R1.1-R1.4: processes leading whitespace by default, converting space
// runs to tabs at tab stop boundaries.
func unexpandReader(r io.Reader, w *bufio.Writer, stops []int, convertAll bool) error {
	br := bufio.NewReader(r)
	s := lineState{inLeading: true}
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			return flushSpaces(w, s.pending)
		}
		if err != nil {
			return err
		}
		s, err = processByte(b, w, s, convertAll, stops)
		if err != nil {
			return err
		}
	}
}

// processByte dispatches a single byte to the appropriate handler.
func processByte(b byte, w *bufio.Writer, s lineState, convertAll bool, stops []int) (lineState, error) {
	if b == '\n' {
		return processNewline(w, s)
	}
	canConvert := s.inLeading || convertAll
	if canConvert && b == ' ' {
		return processSpace(w, s, stops)
	}
	if canConvert && b == '\t' {
		return processTab(w, s, stops)
	}
	return processOther(b, w, s, stops)
}

// processNewline flushes pending spaces, writes a newline, and resets state.
func processNewline(w *bufio.Writer, s lineState) (lineState, error) {
	if err := flushSpaces(w, s.pending); err != nil {
		return s, err
	}
	if err := w.WriteByte('\n'); err != nil {
		return s, err
	}
	return lineState{inLeading: true}, nil
}

// processSpace handles a space in the converting region.
// R1.1: emits a tab when the column reaches a tab stop.
// R1.3: accumulates spaces that do not reach a tab stop.
func processSpace(w *bufio.Writer, s lineState, stops []int) (lineState, error) {
	s.col++
	s.pending++
	if isAtTabStop(s.col, stops) {
		if err := w.WriteByte('\t'); err != nil {
			return s, err
		}
		s.pending = 0
	}
	return s, nil
}

// processTab handles a tab in the converting region.
// R1.4: existing tabs advance column position and are emitted directly.
func processTab(w *bufio.Writer, s lineState, stops []int) (lineState, error) {
	s.col = nextTabStopCol(s.col, stops)
	s.pending = 0
	if err := w.WriteByte('\t'); err != nil {
		return s, err
	}
	return s, nil
}

// processOther handles non-converting characters: non-whitespace in any mode,
// or whitespace in default mode after leading whitespace has ended.
func processOther(b byte, w *bufio.Writer, s lineState, stops []int) (lineState, error) {
	if err := flushSpaces(w, s.pending); err != nil {
		return s, err
	}
	s.pending = 0
	if err := w.WriteByte(b); err != nil {
		return s, err
	}
	if b == '\t' {
		s.col = nextTabStopCol(s.col, stops)
	} else {
		s.col++
	}
	if b != ' ' && b != '\t' {
		s.inLeading = false
	}
	return s, nil
}

// unexpandFile opens and processes a named file.
func unexpandFile(name string, w *bufio.Writer, stops []int, convertAll bool) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return unexpandReader(r, w, stops, convertAll)
}

func main() {
	sys.InstallSIGPIPEHandler()

	convertAll, stops, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: %s\n", err)
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range files {
		if err := unexpandFile(name, w, stops, convertAll); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %s\n", err)
			exitCode = 1
		}
	}

	// best-effort flush; SIGPIPE handler covers broken pipe
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: write error\n")
		exitCode = 1
	}

	os.Exit(exitCode)
}
