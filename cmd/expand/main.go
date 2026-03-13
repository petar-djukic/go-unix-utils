// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/expand implements the expand (convert tabs to spaces) command.
// Implements: prd024-expand R1.1, R1.2, R1.3, R1.4
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R1.1: Default tab stop interval is 8 columns.
const defaultTabStop = 8

func main() {
	// R3.4, D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	files := os.Args[1:]

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	if len(files) == 0 {
		// R1.1: No file arguments — read from stdin.
		if err := expandReader(os.Stdin, w); err != nil {
			fmt.Fprintf(os.Stderr, "expand: %v\n", err)
			os.Exit(1)
		}
	} else {
		for _, name := range files {
			if err := expandFile(name, w); err != nil {
				fmt.Fprintf(os.Stderr, "expand: %v\n", err)
				exitCode = 1
			}
		}
	}

	// R3.3: Flush buffered output; exit 1 on write error.
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "expand: write error: %v\n", err)
		os.Exit(1)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// expandFile opens name and expands tabs in its contents to the writer.
// D3: "-" reads from stdin.
func expandFile(name string, w *bufio.Writer) error {
	if name == "-" {
		return expandReader(os.Stdin, w)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // best-effort cleanup, error ignored
	return expandReader(f, w)
}

// expandReader reads from r and writes tab-expanded output to w.
//
// R1.1: Tabs are replaced by spaces to advance to the next multiple-of-8 column.
// R1.2: Multiple consecutive tabs each advance to the next tab stop independently.
// R1.3: Non-tab characters are written unchanged; each advances column by one.
// R1.4: Backspace characters are passed through and decrement the column (minimum 0).
func expandReader(r io.Reader, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	col := 0 // 0-indexed column position

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		switch b {
		case '\t':
			// R1.1, R1.2: Replace tab with spaces to reach next tab stop.
			spaces := defaultTabStop - (col % defaultTabStop)
			for range spaces {
				if werr := w.WriteByte(' '); werr != nil {
					return fmt.Errorf("write error: %w", werr)
				}
			}
			col += spaces
		case '\n':
			// R1.3: Newline resets column to 0 (1-indexed: column 1).
			if werr := w.WriteByte('\n'); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			col = 0
		case '\b':
			// R1.4: Backspace decrements column position (minimum 0).
			if werr := w.WriteByte('\b'); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			if col > 0 {
				col--
			}
		default:
			// R1.3: Non-tab characters passed through, each advances column by one.
			if werr := w.WriteByte(b); werr != nil {
				return fmt.Errorf("write error: %w", werr)
			}
			col++
		}
	}
}
