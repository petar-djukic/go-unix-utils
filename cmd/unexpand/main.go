// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand R1.1–R1.4: space-to-tab conversion for leading
// whitespace with default tab stops every 8 columns.
// R1.1: Replace leading spaces with tabs where alignment reaches a tab stop.
// R1.2: Non-leading whitespace passes through unchanged in default mode.
// R1.3: Spaces not reaching a tab stop are kept as spaces.
// R1.4: Existing tabs in leading whitespace advance column position normally.
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

func main() {
	sys.InstallSIGPIPEHandler()
	files := parseArgs(os.Args[1:])
	os.Exit(run(files))
}

// run processes all input sources and returns the exit code.
// R4.1: Returns 0 on success. R4.2: Returns 1 on file open error.
func run(files []string) int {
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	if len(files) == 0 {
		unexpandReader(w, os.Stdin)
	} else {
		exitCode = processFiles(w, files)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFiles iterates over file arguments and unexpands each.
func processFiles(w *bufio.Writer, files []string) int {
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file (or stdin for "-") and unexpands spaces.
func processFile(w *bufio.Writer, name string) error {
	if name == "-" {
		unexpandReader(w, os.Stdin)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("%s: %s", name, osErrorMessage(err))
	}
	defer f.Close()
	unexpandReader(w, f)
	return nil
}

// osErrorMessage extracts the OS-level error message, matching GNU style.
func osErrorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// unexpandReader reads from r and converts leading spaces to tabs.
// Implements R1.1–R1.4.
func unexpandReader(w *bufio.Writer, r io.Reader) {
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
		col, pending, leading = processByte(w, b, col, pending, leading)
	}
}

// processByte handles one byte, dispatching to leading or passthrough mode.
func processByte(w *bufio.Writer, b byte, col, pending int, leading bool) (int, int, bool) {
	if b == '\n' {
		flushSpaces(w, pending)
		w.WriteByte('\n')
		return 0, 0, true
	}
	if !leading {
		w.WriteByte(b)
		return col + 1, 0, false
	}
	return processLeading(w, b, col, pending)
}

// processLeading handles a byte during leading whitespace conversion.
// R1.1: Spaces reaching a tab stop are replaced with a tab.
// R1.3: Spaces not reaching a tab stop are kept.
// R1.4: Tabs advance column to the next tab stop.
func processLeading(w *bufio.Writer, b byte, col, pending int) (int, int, bool) {
	switch b {
	case ' ':
		return processLeadingSpace(w, col, pending)
	case '\t':
		return processLeadingTab(w, col)
	default:
		flushSpaces(w, pending)
		w.WriteByte(b)
		return col + 1, 0, false
	}
}

// processLeadingSpace handles a space in leading whitespace. R1.1, R1.3.
func processLeadingSpace(w *bufio.Writer, col, pending int) (int, int, bool) {
	col++
	pending++
	if col%defaultTabStop == 0 {
		w.WriteByte('\t')
		pending = 0
	}
	return col, pending, true
}

// processLeadingTab handles a tab in leading whitespace. R1.4.
func processLeadingTab(w *bufio.Writer, col int) (int, int, bool) {
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

// parseArgs extracts file names from arguments, skipping known flags.
func parseArgs(args []string) []string {
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
		i = skipFlag(arg, args, i)
	}
	return files
}

// skipFlag advances past a recognized flag, returning the updated index.
func skipFlag(arg string, args []string, i int) int {
	if arg == "-a" || arg == "--all" || arg == "--first-only" {
		return i
	}
	if strings.HasPrefix(arg, "--tabs=") {
		return i
	}
	if strings.HasPrefix(arg, "-t") {
		if len(arg) == 2 && i+1 < len(args) {
			return i + 1
		}
	}
	return i
}
