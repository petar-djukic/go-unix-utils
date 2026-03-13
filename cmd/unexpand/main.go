// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/unexpand implements the unexpand (convert spaces to tabs) command.
// Implements: prd025-unexpand R1.1, R1.2, R1.3, R1.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.4: Default tab stop interval is 8 columns.
const defaultTabStop = 8

func main() {
	// R4.4: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	files := parseArgs(os.Args[1:])

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(files) == 0 {
		// R1.1: No file arguments — read from stdin.
		if err := unexpandReader(os.Stdin, w); err != nil {
			fmt.Fprintf(os.Stderr, "unexpand: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, name := range files {
			if err := unexpandFile(name, w); err != nil {
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

// parseArgs extracts file operands from command-line arguments.
// R1.2, R1.3: Supports file arguments and "-" for stdin.
func parseArgs(args []string) []string {
	var files []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		// For this task (R1.1-R1.4), no flags are implemented.
		// Unknown flags are treated as file arguments to match GNU behavior.
		files = append(files, arg)
	}

	return files
}

// unexpandFile opens name and converts leading spaces to tabs in its contents.
// "-" reads from stdin.
func unexpandFile(name string, w *bufio.Writer) error {
	if name == "-" {
		return unexpandReader(os.Stdin, w)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return unexpandReader(f, w)
}

// unexpandReader reads from r and writes output with leading spaces converted
// to tabs to w.
//
// R1.1: Replace leading spaces with tabs when a run of spaces reaches a tab stop exactly.
// R1.2: Non-leading whitespace is written unchanged.
// R1.3: Partial runs of spaces that do not reach a tab stop are kept as spaces.
// R1.4: Existing tabs in leading whitespace count toward column position.
func unexpandReader(r io.Reader, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	col := 0        // 0-indexed column position
	leading := true // whether we are in leading whitespace
	spaces := 0     // accumulated spaces in current leading run

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				// Flush any trailing spaces that didn't reach a tab stop.
				if err := writeSpaces(w, spaces); err != nil {
					return err
				}
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if leading {
			switch b {
			case ' ':
				// R1.1: Accumulate spaces; emit tab when we reach a tab stop.
				col++
				spaces++
				if col%defaultTabStop == 0 {
					// R1.1: Column is at a tab stop — emit a tab.
					if werr := w.WriteByte('\t'); werr != nil {
						return fmt.Errorf("write error: %w", werr)
					}
					spaces = 0
				}
			case '\t':
				// R1.4: Existing tab in leading whitespace — flush accumulated
				// spaces, then write the tab and advance to the next tab stop.
				if err := writeSpaces(w, spaces); err != nil {
					return err
				}
				spaces = 0
				if werr := w.WriteByte('\t'); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				col = col + (defaultTabStop - col%defaultTabStop)
			case '\n':
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
			default:
				// R1.2: First non-whitespace character — flush remaining spaces
				// and switch to non-leading mode.
				if err := writeSpaces(w, spaces); err != nil {
					return err
				}
				spaces = 0
				if werr := w.WriteByte(b); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				col++
				leading = false
			}
		} else {
			// R1.2: Non-leading characters are written unchanged.
			if b == '\n' {
				if werr := w.WriteByte('\n'); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				col = 0
				leading = true
			} else {
				if werr := w.WriteByte(b); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
				col++
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
