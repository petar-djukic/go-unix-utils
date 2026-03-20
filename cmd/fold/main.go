// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd023-fold R1.1–R1.4, R2.1–R2.3, R3.1–R3.4.
// R1: core line wrapping to at most W columns (default 80).
// R2: -w sets width, -b counts bytes instead of columns.
// R3: -s breaks at last space at or before wrap column.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth = 80
	tabStop      = 8
)

// config holds parsed command-line options.
type config struct {
	width      int
	byteMode   bool
	spaceBreak bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fold: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(cfg, files))
}

// run processes all input sources and returns the exit code.
func run(cfg config, files []string) int {
	w := bufio.NewWriter(os.Stdout)
	if len(files) == 0 {
		foldReader(w, os.Stdin, cfg)
		w.Flush()
		return 0
	}
	exitCode := 0
	for _, name := range files {
		if err := processFile(w, name, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "fold: %v\n", err)
			exitCode = 1
		}
	}
	w.Flush()
	return exitCode
}

// processFile opens a file (or stdin for "-") and folds it.
func processFile(w *bufio.Writer, name string, cfg config) error {
	if name == "-" {
		foldReader(w, os.Stdin, cfg)
		return nil
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	foldReader(w, f, cfg)
	return nil
}

// foldReader reads from r byte by byte and writes folded output to w.
// R3.1–R3.4: when spaceBreak is true, buffers segments to find space breaks.
func foldReader(w *bufio.Writer, r io.Reader, cfg config) {
	br := bufio.NewReader(r)
	if cfg.spaceBreak {
		foldWithSpaceBreak(w, br, cfg)
		return
	}
	col := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		col = processByte(w, b, col, cfg)
	}
}

// processByte handles one byte, returning the updated column position.
func processByte(w *bufio.Writer, b byte, col int, cfg config) int {
	if b == '\n' {
		w.WriteByte('\n')
		return 0
	}
	if cfg.byteMode {
		return processByteMode(w, b, col, cfg.width)
	}
	return processColumnMode(w, b, col, cfg.width)
}

// processByteMode wraps counting each byte as one unit. R2.3.
func processByteMode(w *bufio.Writer, b byte, col, width int) int {
	col++
	if col > width {
		w.WriteByte('\n')
		col = 1
	}
	w.WriteByte(b)
	return col
}

// processColumnMode wraps counting display columns with tab expansion. R2.2.
func processColumnMode(w *bufio.Writer, b byte, col, width int) int {
	switch b {
	case '\t':
		return processTab(w, col, width)
	case '\b':
		w.WriteByte(b)
		if col > 0 {
			col--
		}
		return col
	case '\r':
		w.WriteByte(b)
		return 0
	default:
		col++
		if col > width {
			w.WriteByte('\n')
			col = 1
		}
		w.WriteByte(b)
		return col
	}
}

// processTab handles a tab character with tab-stop column expansion. R2.2.
func processTab(w *bufio.Writer, col, width int) int {
	newCol := col + tabStop - col%tabStop
	if newCol > width {
		w.WriteByte('\n')
		newCol = tabStop
	}
	w.WriteByte('\t')
	return newCol
}

// segByte stores a byte and its column position after the byte is consumed.
type segByte struct {
	b   byte
	col int
}

// foldWithSpaceBreak implements -s: buffer a segment of bytes up to width,
// then break at the last space if possible. R3.1–R3.4.
func foldWithSpaceBreak(w *bufio.Writer, br *bufio.Reader, cfg config) {
	var buf []segByte
	col := 0
	for {
		b, err := br.ReadByte()
		if err != nil {
			flushBuf(w, buf)
			return
		}
		if b == '\n' {
			flushBuf(w, buf)
			buf = buf[:0]
			w.WriteByte('\n')
			col = 0
			continue
		}
		newCol := advanceCol(b, col, cfg)
		if newCol <= cfg.width {
			buf = append(buf, segByte{b: b, col: newCol})
			col = newCol
			continue
		}
		col = emitSegment(w, &buf, b, cfg)
	}
}

// advanceCol computes the column position after consuming byte b. R3.4.
func advanceCol(b byte, col int, cfg config) int {
	if cfg.byteMode {
		return col + 1
	}
	switch b {
	case '\t':
		return col + tabStop - col%tabStop
	case '\b':
		if col > 0 {
			return col - 1
		}
		return 0
	case '\r':
		return 0
	default:
		return col + 1
	}
}

// emitSegment writes the buffered segment, breaking at the last space if
// possible, and returns the new column position. R3.1–R3.3.
func emitSegment(w *bufio.Writer, buf *[]segByte, b byte, cfg config) int {
	spaceIdx := lastBlankIndex(*buf)
	if spaceIdx >= 0 {
		return breakAtSpace(w, buf, b, spaceIdx, cfg)
	}
	// R3.2: no space found, hard break at width.
	flushBuf(w, *buf)
	*buf = (*buf)[:0]
	w.WriteByte('\n')
	startCol := colForByte(b, 0, cfg)
	*buf = append(*buf, segByte{b: b, col: startCol})
	return startCol
}

// breakAtSpace breaks the segment at the last space, writes up to and
// including the space, then carries the remainder forward. R3.3.
func breakAtSpace(w *bufio.Writer, buf *[]segByte, b byte, spaceIdx int, cfg config) int {
	// Write up to and including the space.
	for i := 0; i <= spaceIdx; i++ {
		w.WriteByte((*buf)[i].b)
	}
	w.WriteByte('\n')
	// Carry over bytes after the space, recomputing column positions.
	remainder := (*buf)[spaceIdx+1:]
	*buf = (*buf)[:0]
	col := 0
	for _, sb := range remainder {
		col = colForByte(sb.b, col, cfg)
		*buf = append(*buf, segByte{b: sb.b, col: col})
	}
	// Now add the new byte that caused the overflow.
	col = colForByte(b, col, cfg)
	*buf = append(*buf, segByte{b: b, col: col})
	return col
}

// colForByte returns the column after consuming byte b at position col.
func colForByte(b byte, col int, cfg config) int {
	return advanceCol(b, col, cfg)
}

// lastBlankIndex returns the index of the last blank (space or tab) in buf,
// or -1. GNU fold uses isblank() which matches both space and tab.
func lastBlankIndex(buf []segByte) int {
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i].b == ' ' || buf[i].b == '\t' {
			return i
		}
	}
	return -1
}

// flushBuf writes all buffered bytes to w.
func flushBuf(w *bufio.Writer, buf []segByte) {
	for _, sb := range buf {
		w.WriteByte(sb.b)
	}
}

// parseArgs parses GNU-style command-line arguments into config and files.
func parseArgs(args []string) (config, []string, error) {
	cfg := config{width: defaultWidth}
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
		var err error
		i, err = parseFlag(args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
	}
	return cfg, files, nil
}

// parseFlag parses combined short flags from a single argument.
func parseFlag(args []string, i int, cfg *config) (int, error) {
	arg := args[i]
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'b':
			cfg.byteMode = true
		case 's':
			cfg.spaceBreak = true
		case 'w':
			return parseWidthValue(args, i, j, cfg)
		default:
			return i, fmt.Errorf("invalid option -- '%c'", arg[j])
		}
	}
	return i, nil
}

// parseWidthValue extracts the width number from -wN or -w N form.
func parseWidthValue(args []string, i, j int, cfg *config) (int, error) {
	rest := args[i][j+1:]
	if rest != "" {
		return i, setWidth(cfg, rest)
	}
	i++
	if i >= len(args) {
		return i, fmt.Errorf("option requires an argument -- 'w'")
	}
	return i, setWidth(cfg, args[i])
}

// setWidth validates and sets the width in config. R2.1.
func setWidth(cfg *config, s string) error {
	w, err := strconv.Atoi(s)
	if err != nil || w <= 0 {
		return fmt.Errorf("invalid number of columns: '%s'", s)
	}
	cfg.width = w
	return nil
}
