// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd023-fold R1.1-R1.4, R2.1-R2.3, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

type options struct {
	width     int
	byteMode  bool
	spaceMode bool
}

func defaultOptions() options {
	return options{
		width: 80,
	}
}

func run(args []string) int {
	opts, files, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fold: %s\n", err)
		return 1
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, f := range files {
		if err := processFile(f, opts, w); err != nil {
			if errors.Is(err, syscall.EPIPE) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "fold: %s\n", err)
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		if errors.Is(err, syscall.EPIPE) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "fold: %s\n", err)
		exitCode = 1
	}
	return exitCode
}

func parseArgs(args []string) (options, []string, error) {
	opts := defaultOptions()
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "-" {
			files = append(files, arg)
			i++
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			consumed, err := parseFlag(arg, args[i+1:], &opts)
			if err != nil {
				return opts, nil, err
			}
			i += 1 + consumed
			continue
		}
		files = append(files, arg)
		i++
	}
	return opts, files, nil
}

func parseFlag(arg string, remaining []string, opts *options) (int, error) {
	flag := arg[1:]
	i := 0
	for i < len(flag) {
		switch flag[i] {
		case 'b':
			opts.byteMode = true
			i++
		case 's':
			opts.spaceMode = true
			i++
		case 'w':
			return setWidth(flag[i+1:], remaining, opts)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", flag[i])
		}
	}
	return 0, nil
}

func setWidth(val string, remaining []string, opts *options) (int, error) {
	if val == "" {
		if len(remaining) > 0 {
			val = remaining[0]
		} else {
			return 0, fmt.Errorf("option requires an argument -- 'w'")
		}
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid number of columns: '%s'", val)
		}
		opts.width = n
		return 1, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid number of columns: '%s'", val)
	}
	opts.width = n
	return 0, nil
}

func processFile(path string, opts options, w *bufio.Writer) error {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
	}
	return processInput(r, opts, w)
}

func processInput(r io.Reader, opts options, w *bufio.Writer) error {
	br := bufio.NewReader(r)
	for {
		line, err := readLine(br)
		if len(line) > 0 {
			if wErr := wrapLine(line, opts, w); wErr != nil {
				return wErr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if len(line) > 0 {
		return line, err
	}
	return nil, err
}

func wrapLine(line []byte, opts options, w *bufio.Writer) error {
	if opts.byteMode {
		return wrapByteMode(line, opts, w)
	}
	return wrapColumnMode(line, opts, w)
}

func wrapByteMode(line []byte, opts options, w *bufio.Writer) error {
	width := opts.width
	for len(line) > 0 {
		if endsWithNewline(line) && len(line)-1 <= width {
			_, err := w.Write(line)
			return err
		}
		if !endsWithNewline(line) && len(line) <= width {
			_, err := w.Write(line)
			return err
		}
		seg := line[:width]
		if opts.spaceMode {
			seg = breakAtSpace(line, width)
		}
		if _, err := w.Write(seg); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		line = line[len(seg):]
	}
	return nil
}

func wrapColumnMode(line []byte, opts options, w *bufio.Writer) error {
	width := opts.width
	col := 0
	start := 0
	lastSpace := -1
	for i := range len(line) {
		b := line[i]
		if b == '\n' {
			if _, err := w.Write(line[start : i+1]); err != nil {
				return err
			}
			start = i + 1
			col = 0
			lastSpace = -1
			continue
		}
		nextCol := advanceColumn(col, b)
		if nextCol > width {
			breakPos := i
			if opts.spaceMode && lastSpace >= start {
				breakPos = lastSpace + 1
			}
			if _, err := w.Write(line[start:breakPos]); err != nil {
				return err
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
			start = breakPos
			col = recalcColumn(line[start:i+1], 0)
			lastSpace = -1
			for j := start; j <= i; j++ {
				if line[j] == ' ' || line[j] == '\t' {
					lastSpace = j
				}
			}
			continue
		}
		if b == ' ' || b == '\t' {
			lastSpace = i
		}
		col = nextCol
	}
	if start < len(line) {
		if _, err := w.Write(line[start:]); err != nil {
			return err
		}
	}
	return nil
}

func advanceColumn(col int, b byte) int {
	if b == '\t' {
		return col + 8 - (col % 8)
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

func recalcColumn(data []byte, startCol int) int {
	col := startCol
	for _, b := range data {
		col = advanceColumn(col, b)
	}
	return col
}

func breakAtSpace(line []byte, width int) []byte {
	if width > len(line) {
		width = len(line)
	}
	seg := line[:width]
	for i := len(seg) - 1; i >= 0; i-- {
		if seg[i] == ' ' || seg[i] == '\t' {
			return seg[:i+1]
		}
	}
	return seg
}

func endsWithNewline(line []byte) bool {
	return len(line) > 0 && line[len(line)-1] == '\n'
}
