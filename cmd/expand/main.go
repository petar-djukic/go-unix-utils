// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/expand: convert tabs to spaces.
// Implements srd024-expand R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
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

// parseTabValue parses a tab stop specification string into a slice of stops.
// R2.1: single number = uniform interval.
// R2.2: comma/space-separated list = absolute positions in strictly increasing order.
// R2.4: single-element list behaves identically to uniform interval.
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

// parseArgs parses command-line arguments into tab stops and file names.
// R2.3: last -t value takes effect when given multiple times.
func parseArgs(args []string) ([]int, []string, error) {
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
		val, skip, err := extractTabFlag(arg, args, i)
		if err != nil {
			return nil, nil, err
		}
		if val == "" {
			return nil, nil, fmt.Errorf("invalid option -- '%s'", arg[1:])
		}
		parsed, err := parseTabValue(val)
		if err != nil {
			return nil, nil, err
		}
		stops = parsed
		i += skip
	}

	if len(files) == 0 {
		files = []string{"-"}
	}
	return stops, files, nil
}

// tabSpaces returns the number of spaces for a tab at 0-indexed column col.
// R2.1/R2.4: single stop = uniform interval.
// R2.2: multiple stops = absolute 1-indexed positions; past last stop → 1 space.
func tabSpaces(col int, stops []int) int {
	if len(stops) == 1 {
		return stops[0] - (col % stops[0])
	}
	for _, s := range stops {
		if s > col {
			return s - col
		}
	}
	return 1
}

// expandReader reads from r, expands tabs to spaces, and writes to w.
// R1.1: tabs are replaced with spaces to reach the next tab stop.
// R1.2: consecutive tabs each advance independently.
// R1.3: non-tab characters pass through unchanged.
// R1.4: newlines reset column position to 0.
func expandReader(r io.Reader, w *bufio.Writer, stops []int) error {
	br := bufio.NewReader(r)
	col := 0
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := expandByte(w, b, &col, stops); err != nil {
			return err
		}
	}
	return nil
}

// expandByte processes a single byte, expanding tabs to spaces.
func expandByte(w *bufio.Writer, b byte, col *int, stops []int) error {
	switch b {
	case '\t':
		spaces := tabSpaces(*col, stops)
		for range spaces {
			if err := w.WriteByte(' '); err != nil {
				return err
			}
		}
		*col += spaces
	case '\n':
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		*col = 0
	default:
		if err := w.WriteByte(b); err != nil {
			return err
		}
		*col++
	}
	return nil
}

// expandFile opens and expands tabs in a named file.
func expandFile(name string, w *bufio.Writer, stops []int) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return expandReader(r, w, stops)
}

func main() {
	sys.InstallSIGPIPEHandler()

	stops, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand: %s\n", err)
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range files {
		if err := expandFile(name, w, stops); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %s\n", err)
			exitCode = 1
		}
	}

	// best-effort flush; SIGPIPE handler covers broken pipe
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error\n")
		exitCode = 1
	}

	os.Exit(exitCode)
}
