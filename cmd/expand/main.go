// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/expand: convert tabs to spaces.
// Implements srd024-expand R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

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

// expandReader reads from r, expands tabs to spaces, and writes to w.
// R1.1: tabs are replaced with spaces to reach the next tab stop.
// R1.2: consecutive tabs each advance independently.
// R1.3: non-tab characters pass through unchanged.
// R1.4: newlines reset column position to 0.
func expandReader(r io.Reader, w *bufio.Writer) error {
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
		if err := expandByte(w, b, &col); err != nil {
			return err
		}
	}
	return nil
}

// expandByte processes a single byte, expanding tabs to spaces.
func expandByte(w *bufio.Writer, b byte, col *int) error {
	switch b {
	case '\t':
		spaces := defaultTabStop - (*col % defaultTabStop)
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
func expandFile(name string, w *bufio.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return expandReader(r, w)
}

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range args {
		if err := expandFile(name, w); err != nil {
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
