// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand R1.1-R1.4: cmd/unexpand converts sequences of
// spaces to tabs at tab stop boundaries. By default, only leading whitespace
// is converted. The -a/--all flag converts all whitespace in the line.
// The --first-only flag restores default behavior; when both -a and --first-only
// are given, the last one on the command line wins.
// Reads from files listed as arguments or stdin when no files are given.
// Treats '-' as stdin. Installs SIGPIPE handler for clean exit on broken pipe.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU unexpand format.
const progName = "unexpand"

// defaultTabStop is the default tab stop interval in columns (R1.1).
const defaultTabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()

	convertAll := false
	firstOnly := false
	args := os.Args[1:]
	var files []string

	// R1.3, R1.4: parse flags. --first-only overrides -a regardless of order.
	for i, arg := range args {
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-a" || arg == "--all" {
			convertAll = true
			continue
		}
		if arg == "--first-only" {
			firstOnly = true
			continue
		}
		files = append(files, arg)
	}

	// R1.4: --first-only overrides -a when both are present.
	if firstOnly {
		convertAll = false
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		// R1.1: no file arguments — read from stdin.
		if err := unexpandReader(os.Stdin, w, defaultTabStop, convertAll); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		for _, name := range files {
			if name == "-" {
				// R1.1: '-' means read from stdin.
				if err := unexpandReader(os.Stdin, w, defaultTabStop, convertAll); err != nil {
					fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
					exitCode = 1
				}
				continue
			}

			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
				exitCode = 1
				continue
			}
			if err := unexpandReader(f, w, defaultTabStop, convertAll); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
				exitCode = 1
			}
			f.Close() // best-effort close; read errors already reported
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// unexpandReader reads from r and writes space-to-tab converted output to w.
// R1.1: spaces that reach a tab stop boundary are replaced by a tab character.
// R1.2: in default mode, only leading whitespace is converted; characters after
// the first non-whitespace character pass through unchanged.
// R1.3: when convertAll is true (-a), all space sequences in the line are
// subject to tab conversion, not just leading whitespace.
// R1.4: existing tab characters in the input count toward column position and
// are output as tabs; they do not prevent further tab substitution.
func unexpandReader(r io.Reader, w *bufio.Writer, tabStop int, convertAll bool) error {
	br := bufio.NewReader(r)
	col := 0
	pending := 0    // number of accumulated spaces not yet output
	inInitial := true // whether we are in the leading blank region of a line

	flushPending := func() error {
		for range pending {
			if err := w.WriteByte(' '); err != nil {
				return err
			}
		}
		pending = 0
		return nil
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return flushPending()
			}
			return err
		}

		converting := convertAll || inInitial

		switch b {
		case ' ':
			if !converting {
				// R1.2: non-leading spaces pass through in default mode.
				if err := w.WriteByte(' '); err != nil {
					return err
				}
				col++
				continue
			}
			pending++
			col++
			// R1.1: when spaces reach a tab stop, replace with a tab.
			if col%tabStop == 0 {
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				pending = 0
			}
		case '\t':
			if !converting {
				// R1.2: tabs after non-leading region pass through unchanged.
				if err := w.WriteByte('\t'); err != nil {
					return err
				}
				col += tabStop - (col % tabStop)
				continue
			}
			// R1.4: existing tab advances to next tab stop.
			// Pending spaces plus this tab all advance to the next tab stop,
			// represented as a single tab character.
			col += tabStop - (col % tabStop)
			if err := w.WriteByte('\t'); err != nil {
				return err
			}
			pending = 0
		case '\n':
			// Flush any pending spaces before the newline.
			if err := flushPending(); err != nil {
				return err
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			col = 0
			inInitial = true
		default:
			// R1.2: first non-blank character ends the initial region.
			if err := flushPending(); err != nil {
				return err
			}
			if err := w.WriteByte(b); err != nil {
				return err
			}
			col++
			inInitial = false
		}
	}
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
