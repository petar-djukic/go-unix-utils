// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the fold utility (prd023-fold R1-R4).
// fold wraps each input line to fit a specified width (default 80 columns),
// reading from files or stdin, with optional byte-mode (-b) and
// word-boundary breaking (-s).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// tabStop is the column interval for tab stops.
const tabStop = 8

func main() {
	// R4.4: Handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	width := 80
	byteMode := false
	spaceBreak := false

	args := os.Args[1:]
	var files []string

	// Manual flag parsing to match GNU fold behavior.
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if len(arg) == 0 || arg[0] != '-' || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}

		// Parse flags from the argument.
		j := 1
		for j < len(arg) {
			switch arg[j] {
			case 'b':
				byteMode = true
				j++
			case 's':
				spaceBreak = true
				j++
			case 'w':
				// R2.1: -w N sets width.
				val := arg[j+1:]
				if val == "" {
					i++
					if i < len(args) {
						val = args[i]
					}
				}
				n, err := strconv.Atoi(val)
				if err != nil || n <= 0 {
					fmt.Fprintf(os.Stderr, "fold: invalid number of columns: '%s'\n", val)
					os.Exit(1)
				}
				width = n
				j = len(arg) // consumed rest of arg
			default:
				fmt.Fprintf(os.Stderr, "fold: invalid option -- '%c'\n", arg[j])
				os.Exit(1)
			}
		}
		i++
	}

	// R1.1: Read stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	for _, file := range files {
		r, err := openInput(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fold: %v\n", err)
			exitCode = 1
			continue
		}

		if err := foldInput(r, w, width, byteMode, spaceBreak); err != nil {
			// R4.3: Exit 1 on write error.
			os.Exit(1)
		}

		if closer, ok := r.(io.Closer); ok && file != "-" {
			_ = closer.Close() // best-effort close
		}
	}

	if err := w.Flush(); err != nil {
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// openInput returns a reader for the named file, or stdin if name is "-".
func openInput(name string) (io.Reader, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// foldInput reads from r and writes folded output to w.
func foldInput(r io.Reader, w *bufio.Writer, width int, byteMode, spaceBreak bool) error {
	br := bufio.NewReader(r)
	for {
		line, err := readLine(br)
		if len(line) > 0 {
			if writeErr := foldLine(w, line, width, byteMode, spaceBreak); writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// readLine reads a single line from the reader, including the trailing newline if present.
// Returns the line bytes and any error.
func readLine(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, isPrefix, err := r.ReadLine()
		line = append(line, chunk...)
		if err != nil {
			return line, err
		}
		if !isPrefix {
			// ReadLine strips the newline; add it back.
			line = append(line, '\n')
			return line, nil
		}
	}
}

// foldLine wraps a single line (which may end with '\n') and writes it to w.
func foldLine(w *bufio.Writer, line []byte, width int, byteMode, spaceBreak bool) error {
	// R1.4: Detect and preserve (or not) trailing newline.
	hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
	content := line
	if hasNewline {
		content = line[:len(line)-1]
	}

	for len(content) > 0 {
		// Measure the segment up to width units.
		// writeEnd: bytes to write for this segment.
		// nextStart: where the next segment begins (may skip a space with -s).
		writeEnd, nextStart := findBreakPos(content, width, byteMode, spaceBreak)

		if writeEnd >= len(content) {
			// R1.2: Remaining content fits; write it and the trailing newline if any.
			if _, err := w.Write(content); err != nil {
				return err
			}
			if hasNewline {
				if err := w.WriteByte('\n'); err != nil {
					return err
				}
			}
			return nil
		}

		// R1.3: Write the segment and insert a newline.
		if _, err := w.Write(content[:writeEnd]); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		content = content[nextStart:]
	}

	// Empty content but had a newline.
	if hasNewline {
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

// findBreakPos determines where to break content.
// Returns (writeEnd, nextStart): writeEnd is the end of the current segment to write,
// nextStart is where the next segment begins (may differ when -s consumes a space).
func findBreakPos(content []byte, width int, byteMode, spaceBreak bool) (int, int) {
	if byteMode {
		return findBreakByte(content, width, spaceBreak)
	}
	return findBreakColumn(content, width, spaceBreak)
}

// findBreakByte finds the break position using byte counting.
// Returns (writeEnd, nextStart).
func findBreakByte(content []byte, width int, spaceBreak bool) (int, int) {
	if len(content) <= width {
		return len(content), len(content)
	}

	if spaceBreak {
		// Check if the byte at the width boundary is a space; consume it.
		if content[width] == ' ' {
			return width, width + 1
		}
		// R3.1: Find the last space at or before the width.
		lastSpace := -1
		for i := 0; i < width; i++ {
			if content[i] == ' ' {
				lastSpace = i
			}
		}
		// R3.3: Break after the space character.
		if lastSpace >= 0 {
			return lastSpace + 1, lastSpace + 1
		}
		// R3.2: If no space found, fall back to hard break at width.
	}

	return width, width
}

// findBreakColumn finds the break position using column counting with tab expansion.
// Returns (writeEnd, nextStart).
func findBreakColumn(content []byte, width int, spaceBreak bool) (int, int) {
	col := 0
	lastSpace := -1

	for i := 0; i < len(content); i++ {
		ch := content[i]

		var newCol int
		if ch == '\t' {
			// R2.2: Tab expands to next tab stop.
			newCol = col + (tabStop - col%tabStop)
		} else {
			newCol = col + 1
		}

		if newCol > width {
			// This character pushes past the width.
			if spaceBreak {
				// If this character is a space, consume it as the break delimiter.
				if ch == ' ' {
					return i, i + 1
				}
				if lastSpace >= 0 {
					return lastSpace + 1, lastSpace + 1
				}
			}
			return i, i
		}

		if ch == ' ' {
			lastSpace = i
		}

		col = newCol

		if col == width {
			// Exactly at the width boundary.
			if spaceBreak {
				if i+1 < len(content) && content[i+1] == ' ' {
					// Next character is a space; consume it as the break delimiter.
					return i + 1, i + 2
				}
				if lastSpace >= 0 {
					return lastSpace + 1, lastSpace + 1
				}
			}
			return i + 1, i + 1
		}
	}

	return len(content), len(content)
}
