// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/fold wraps long lines to a specified width (prd023-fold R1.1-R1.4, R2, R3, R4).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultWidth = 80
	tabStop      = 8
)

type config struct {
	width      int
	byteMode   bool
	spaceBreak bool
	files      []string
}

type foldState struct {
	out   *bufio.Writer
	cfg   config
	col   int
	buf   []byte
	blank int // index of last space in buf, -1 if none
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg := parseArgs(os.Args[1:])
	os.Exit(run(cfg))
}

func run(cfg config) int {
	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(name, cfg, out); err != nil {
			fmt.Fprintf(os.Stderr, "fold: %v\n", err)
			exitCode = 1
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "fold: write error: %v\n", err)
		return 1
	}
	return exitCode
}

// processFile opens and folds one input file.
// R3.1: multiple files are processed in order by the caller.
// R3.3: empty input produces no output (foldStream returns nil on immediate EOF).
func processFile(name string, cfg config, out *bufio.Writer) error {
	r, err := openInput(name)
	if err != nil {
		// R3.2: format matches GNU fold: "fold: <name>: <reason>"
		if pe, ok := err.(*os.PathError); ok {
			return fmt.Errorf("%s: %s", name, pe.Err)
		}
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return foldStream(bufio.NewReader(r), cfg, out)
}

func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

func foldStream(r *bufio.Reader, cfg config, out *bufio.Writer) error {
	s := &foldState{out: out, cfg: cfg, blank: -1}
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return s.flush()
			}
			return err
		}
		if werr := s.processByte(c); werr != nil {
			return werr
		}
	}
}

// processByte handles one input byte, folding when the column exceeds width.
// R1.2: lines within width pass through unchanged.
// R1.3: lines exceeding width are split by inserting newlines.
// R2.1: -b counts bytes instead of columns.
// R2.2: tabs advance to next tab stop; backspace decrements column.
// R2.3: carriage return resets column to zero.
func (s *foldState) processByte(c byte) error {
	if c == '\n' {
		s.buf = append(s.buf, '\n')
		return s.writeBuf()
	}
	newCol := columnAfter(s.col, c, s.cfg.byteMode)
	if newCol > s.cfg.width && len(s.buf) > 0 {
		return s.handleFold(c)
	}
	if s.cfg.spaceBreak && c == ' ' {
		s.blank = len(s.buf)
	}
	s.buf = append(s.buf, c)
	s.col = newCol
	return nil
}

func (s *foldState) handleFold(c byte) error {
	if s.cfg.spaceBreak {
		return s.handleSpaceFold(c)
	}
	return s.hardFold(c)
}

// handleSpaceFold implements -s: prefer breaking at the last space.
// R3.1: break at last blank within width.
// R3.2: if no blank exists within width, fall back to hard break.
// R3.4: compatible with -b; space detection uses byte positions when -b is active.
func (s *foldState) handleSpaceFold(c byte) error {
	if s.blank >= 0 {
		return s.breakAtBlank(c)
	}
	if c == ' ' {
		s.buf = append(s.buf, '\n')
		return s.writeBuf()
	}
	return s.hardFold(c)
}

// breakAtBlank splits the pending buffer at the last saved space position.
// R3.3: the space is written as the last character of the current output line.
func (s *foldState) breakAtBlank(c byte) error {
	breakIdx := s.blank + 1
	if _, err := s.out.Write(s.buf[:breakIdx]); err != nil {
		return err
	}
	if err := s.out.WriteByte('\n'); err != nil {
		return err
	}
	rest := append([]byte(nil), s.buf[breakIdx:]...)
	s.buf = rest
	s.col = recalcCol(rest, s.cfg.byteMode)
	s.blank = lastSpaceIn(rest)
	return s.processByte(c)
}

// hardFold writes the pending buffer followed by a newline, then reprocesses c.
func (s *foldState) hardFold(c byte) error {
	s.buf = append(s.buf, '\n')
	if err := s.writeBuf(); err != nil {
		return err
	}
	return s.processByte(c)
}

func (s *foldState) writeBuf() error {
	_, err := s.out.Write(s.buf)
	s.buf = s.buf[:0]
	s.col = 0
	s.blank = -1
	return err
}

// flush writes any remaining pending bytes (for input without trailing newline).
// R1.4: final segment retains the original newline presence or absence.
func (s *foldState) flush() error {
	if len(s.buf) > 0 {
		_, err := s.out.Write(s.buf)
		s.buf = s.buf[:0]
		return err
	}
	return nil
}

// columnAfter returns the display column after outputting byte c at column col.
// R2.1: in byte mode, every byte counts as 1.
// R2.2: tabs advance to the next tab stop (every 8 columns); backspace
// decrements the column by 1 (minimum 0).
// R2.3: carriage return resets the column to zero.
func columnAfter(col int, c byte, byteMode bool) int {
	if byteMode {
		return col + 1
	}
	switch c {
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

func recalcCol(data []byte, byteMode bool) int {
	col := 0
	for _, c := range data {
		col = columnAfter(col, c, byteMode)
	}
	return col
}

func lastSpaceIn(data []byte) int {
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] == ' ' {
			return i
		}
	}
	return -1
}

func parseArgs(args []string) config {
	cfg := config{width: defaultWidth}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if a == "-" || len(a) == 0 || a[0] != '-' {
			cfg.files = append(cfg.files, a)
			continue
		}
		i = parseFlags(a[1:], args, i, &cfg)
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg
}

func parseFlags(flags string, args []string, idx int, cfg *config) int {
	for j := 0; j < len(flags); j++ {
		switch flags[j] {
		case 'b':
			cfg.byteMode = true
		case 's':
			cfg.spaceBreak = true
		case 'w':
			rest := flags[j+1:]
			if rest == "" {
				idx++
				if idx >= len(args) {
					die("option requires an argument -- 'w'")
				}
				rest = args[idx]
			}
			w, err := strconv.Atoi(rest)
			if err != nil || w <= 0 {
				die(fmt.Sprintf("invalid number of columns: '%s'", rest))
			}
			cfg.width = w
			return idx
		default:
			die(fmt.Sprintf("invalid option -- '%c'", flags[j]))
		}
	}
	return idx
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "fold: %s\n", msg)
	os.Exit(1)
}
