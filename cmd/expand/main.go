// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd024-expand R1.1–R1.4: core tab-to-space expansion.
// R1.1: Default tab expansion with tab stops every 8 columns.
// R1.2: Multiple consecutive tabs each advance independently.
// R1.3: Non-tab characters pass through unchanged.
// R1.4: Newline resets column position to 1.
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
func run(files []string) int {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	if len(files) == 0 {
		expandReader(w, os.Stdin)
		return 0
	}
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile opens a file (or stdin for "-") and expands tabs.
func processFile(w *bufio.Writer, name string) error {
	if name == "-" {
		expandReader(w, os.Stdin)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	expandReader(w, f)
	return nil
}

// expandReader reads from r and replaces tabs with spaces. R1.1–R1.4.
func expandReader(w *bufio.Writer, r io.Reader) {
	br := bufio.NewReader(r)
	col := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		col = expandByte(w, b, col)
	}
}

// expandByte processes one byte and returns the updated column position.
// R1.1: Tab advances to next multiple-of-8 column.
// R1.3: Non-tab bytes pass through, each counting as one column.
// R1.4: Newline resets column to 0 (0-indexed internally; column 1 externally).
func expandByte(w *bufio.Writer, b byte, col int) int {
	switch b {
	case '\t':
		return expandTab(w, col)
	case '\n':
		w.WriteByte('\n')
		return 0
	default:
		w.WriteByte(b)
		return col + 1
	}
}

// expandTab replaces a tab with spaces to reach the next tab stop. R1.1, R1.2.
func expandTab(w *bufio.Writer, col int) int {
	spaces := defaultTabStop - col%defaultTabStop
	for i := 0; i < spaces; i++ {
		w.WriteByte(' ')
	}
	return col + spaces
}

// parseArgs extracts file names from command-line arguments.
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
		// Skip unknown flags gracefully for forward compatibility.
		if strings.HasPrefix(arg, "-t") {
			// -tN form: value is part of this arg, skip it.
			if len(arg) > 2 {
				continue
			}
			// -t N form: skip the next arg too.
			i++
			continue
		}
	}
	return files
}
