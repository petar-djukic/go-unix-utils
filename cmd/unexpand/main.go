// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand R1.1–R1.4, R2.1–R2.3: space-to-tab conversion
// for leading whitespace (default) or all whitespace (-a).
// R1.1: Replace leading spaces with tabs where alignment reaches a tab stop.
// R1.2: Non-leading whitespace passes through unchanged in default mode.
// R1.3: Spaces not reaching a tab stop are kept as spaces.
// R1.4: Existing tabs in leading whitespace advance column position normally.
// R2.1: -a converts all runs of spaces where tabs align, not just leading.
// R2.2: A single space not reaching a tab stop is kept even with -a.
// R2.3: -a processes the entire line past the first non-whitespace character.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultTabStop = 8

// options holds parsed command-line options.
type options struct {
	allMode bool
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
		unexpandReader(w, os.Stdin, opts.allMode)
	} else {
		exitCode = processFiles(w, opts.files, opts.allMode)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFiles iterates over file arguments and unexpands each.
func processFiles(w *bufio.Writer, files []string, allMode bool) int {
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name, allMode); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file (or stdin for "-") and unexpands spaces.
func processFile(w *bufio.Writer, name string, allMode bool) error {
	if name == "-" {
		unexpandReader(w, os.Stdin, allMode)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	defer f.Close()
	unexpandReader(w, f, allMode)
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
func unexpandReader(w *bufio.Writer, r io.Reader, allMode bool) {
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
		col, pending, leading = processByte(w, b, col, pending, leading, allMode)
	}
}

// processByte handles one byte, dispatching based on mode.
// R1.2: In default mode, non-leading bytes pass through unchanged.
// R2.3: In allMode, all bytes are processed for conversion.
func processByte(w *bufio.Writer, b byte, col, pending int, leading, allMode bool) (int, int, bool) {
	if b == '\n' {
		flushSpaces(w, pending)
		w.WriteByte('\n')
		return 0, 0, true
	}
	if !leading && !allMode {
		w.WriteByte(b)
		return col + 1, 0, false
	}
	return processConvert(w, b, col, pending)
}

// processConvert handles a byte during whitespace conversion.
// Used for leading whitespace (default) and all whitespace (-a).
func processConvert(w *bufio.Writer, b byte, col, pending int) (int, int, bool) {
	switch b {
	case ' ':
		return processSpace(w, col, pending)
	case '\t':
		return processTab(w, col)
	default:
		flushSpaces(w, pending)
		w.WriteByte(b)
		return col + 1, 0, false
	}
}

// processSpace handles a space during conversion. R1.1, R1.3, R2.1, R2.2.
func processSpace(w *bufio.Writer, col, pending int) (int, int, bool) {
	col++
	pending++
	if col%defaultTabStop == 0 {
		w.WriteByte('\t')
		pending = 0
	}
	return col, pending, true
}

// processTab handles a tab during conversion. R1.4.
func processTab(w *bufio.Writer, col int) (int, int, bool) {
	col += defaultTabStop - col%defaultTabStop
	w.WriteByte('\t')
	return col, 0, true
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
		i = parseFlag(arg, args, i, &opts)
	}
	return opts
}

// parseFlag processes a single flag argument, returning the updated index.
func parseFlag(arg string, args []string, i int, opts *options) int {
	if arg == "-a" || arg == "--all" {
		opts.allMode = true
		return i
	}
	if arg == "--first-only" {
		return i
	}
	if strings.HasPrefix(arg, "--tabs=") {
		return i
	}
	if strings.HasPrefix(arg, "-t") && len(arg) == 2 && i+1 < len(args) {
		return i + 1
	}
	return i
}
