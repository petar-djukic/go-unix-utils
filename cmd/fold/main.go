// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/fold: wrap long lines to a specified width.
// Implements srd023-fold R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2, R3.3, R3.4.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth = 80
	tabStop      = 8
)

// config holds fold command-line options.
type config struct {
	width     int  // -w N: maximum line width (default 80)
	byteMode  bool // -b: count bytes instead of columns
	spaceMode bool // -s: break at spaces
}

// parseFlags parses command-line flags and returns config and file arguments.
func parseFlags() (config, []string) {
	cfg := config{width: defaultWidth}
	fs := flag.NewFlagSet("fold", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {}

	fs.IntVar(&cfg.width, "w", defaultWidth, "maximum line width")
	fs.BoolVar(&cfg.byteMode, "b", false, "count bytes instead of columns")
	fs.BoolVar(&cfg.spaceMode, "s", false, "break at spaces")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	if cfg.width <= 0 {
		fmt.Fprintf(os.Stderr, "fold: invalid number of columns: %q\n", fmt.Sprintf("%d", cfg.width))
		os.Exit(1)
	}

	return cfg, fs.Args()
}

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
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// columnWidth returns the display column width after appending byte b
// at the given column position. Tab stops are every 8 columns.
// R1.1: columns by default; each byte is one column except tabs.
func columnWidth(col int, b byte) int {
	if b == '\t' {
		return col + tabStop - (col % tabStop)
	}
	if b == '\b' {
		if col > 0 {
			return col - 1
		}
		return 0
	}
	if b == '\r' {
		return 0
	}
	return col + 1
}

// foldLine wraps a single line (without trailing newline) to the writer.
// R1.3: lines longer than width are split by inserting newlines.
// R1.4: hasNewline controls whether a trailing newline is emitted.
func foldLine(w *bufio.Writer, line []byte, cfg config, hasNewline bool) error {
	if cfg.spaceMode {
		return foldLineSpace(w, line, cfg, hasNewline)
	}
	return foldLineHard(w, line, cfg, hasNewline)
}

// foldLineHard wraps a line at exact column/byte boundaries.
// R1.2: lines within width pass through unchanged.
// R1.3: repeated wrapping until remaining fits.
func foldLineHard(w *bufio.Writer, line []byte, cfg config, hasNewline bool) error {
	col := 0
	for i := 0; i < len(line); i++ {
		nextCol := advancePos(col, line[i], cfg.byteMode)
		if nextCol > cfg.width && col > 0 {
			if err := writeByte(w, '\n'); err != nil {
				return err
			}
			col = 0
			nextCol = advancePos(col, line[i], cfg.byteMode)
		}
		if err := writeByte(w, line[i]); err != nil {
			return err
		}
		col = nextCol
	}
	if hasNewline {
		return writeByte(w, '\n')
	}
	return nil
}

// foldLineSpace wraps a line at the last space at or before the wrap column.
// R3.1: break at last space at or before wrap column.
// R3.2: fall back to hard break when no space is found within W columns.
// R3.3: space is written as last char before the inserted newline.
// R3.4: compatible with -b; space detection uses byte positions when -b is active.
func foldLineSpace(w *bufio.Writer, line []byte, cfg config, hasNewline bool) error {
	col := 0
	lastSpace := -1
	segStart := 0

	for i := 0; i < len(line); i++ {
		nextCol := advancePos(col, line[i], cfg.byteMode)
		if line[i] == ' ' {
			lastSpace = i
		}
		if nextCol > cfg.width {
			if err := breakAtSpace(w, line, &segStart, &i, &col, &lastSpace, cfg); err != nil {
				return err
			}
			continue
		}
		col = nextCol
	}

	if err := writeSegment(w, line[segStart:]); err != nil {
		return err
	}
	if hasNewline {
		return writeByte(w, '\n')
	}
	return nil
}

// breakAtSpace handles a line break at a space or hard position.
func breakAtSpace(w *bufio.Writer, line []byte, segStart, i, col, lastSpace *int, cfg config) error {
	if *lastSpace >= *segStart {
		if err := writeSegment(w, line[*segStart:*lastSpace+1]); err != nil {
			return err
		}
		if err := writeByte(w, '\n'); err != nil {
			return err
		}
		*segStart = *lastSpace + 1
		*col = recomputeCol(line, *segStart, *i, cfg.byteMode)
		*lastSpace = -1
		return nil
	}
	// No space found; hard break before current byte.
	if err := writeSegment(w, line[*segStart:*i]); err != nil {
		return err
	}
	if err := writeByte(w, '\n'); err != nil {
		return err
	}
	*segStart = *i
	*col = advancePos(0, line[*i], cfg.byteMode)
	*lastSpace = -1
	return nil
}

// recomputeCol recalculates column position for a range of bytes.
func recomputeCol(line []byte, from, to int, byteMode bool) int {
	col := 0
	for j := from; j <= to; j++ {
		col = advancePos(col, line[j], byteMode)
	}
	return col
}

// advancePos advances the column/byte position by one byte.
func advancePos(col int, b byte, byteMode bool) int {
	if byteMode {
		return col + 1
	}
	return columnWidth(col, b)
}

// writeByte writes a single byte to the writer.
func writeByte(w *bufio.Writer, b byte) error {
	return w.WriteByte(b)
}

// writeSegment writes a slice of bytes to the writer.
func writeSegment(w *bufio.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}

// foldReader reads from r and writes folded output to w.
// R1.1: read each file and wrap lines to width.
func foldReader(r io.Reader, cfg config, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}
			if foldErr := foldLine(w, content, cfg, hasNewline); foldErr != nil {
				return foldErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// foldFile opens and folds a named file.
func foldFile(name string, cfg config, w *bufio.Writer) error {
	r, err := openInput(name)
	if err != nil {
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return foldReader(r, cfg, w)
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args := parseFlags()

	if len(args) == 0 {
		args = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range args {
		if err := foldFile(name, cfg, w); err != nil {
			fmt.Fprintf(os.Stderr, "fold: %s\n", err)
			exitCode = 1
		}
	}

	// best-effort flush; SIGPIPE handler covers broken pipe
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "fold: write error\n")
		exitCode = 1
	}

	os.Exit(exitCode)
}
