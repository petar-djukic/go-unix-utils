// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU fold: wrap each input line to fit in specified width.
// Implements prd023-fold R1-R4.
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

// foldOptions holds the parsed command-line flags for fold.
type foldOptions struct {
	width      int  // -w: maximum line width (default 80)
	byteMode   bool // -b: count bytes instead of columns
	spaceBreak bool // -s: break at last space within width
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush

	exitCode := 0
	for _, file := range files {
		var r io.Reader
		if file == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "fold: %s: No such file or directory\n", file)
				exitCode = 1
				continue
			}
			defer f.Close() // best-effort cleanup
			r = f
		}

		if err := foldReader(r, w, opts); err != nil {
			fmt.Fprintf(os.Stderr, "fold: write error: %v\n", err)
			exitCode = 1
			break
		}
	}

	w.Flush() // best-effort
	os.Exit(exitCode)
}

// foldReader reads from r and writes folded output to w.
func foldReader(r io.Reader, w *bufio.Writer, opts foldOptions) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if werr := foldLine(w, line, opts); werr != nil {
				return werr
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

// foldLine wraps a single line (which may or may not end with '\n') to the specified width.
func foldLine(w *bufio.Writer, line string, opts foldOptions) error {
	// Strip trailing newline; we'll add it back at the end if present.
	hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
	if hasNewline {
		line = line[:len(line)-1]
	}

	width := opts.width

	for len(line) > 0 {
		// Find the break point for the current segment.
		breakIdx := findBreak(line, width, opts)

		if breakIdx >= len(line) {
			// Remaining portion fits within width.
			if _, err := w.WriteString(line); err != nil {
				return err
			}
			break
		}

		segment := line[:breakIdx]

		if opts.spaceBreak {
			// R3.1: Look for last space at or before the break point.
			spaceIdx := strings.LastIndex(segment, " ")
			if spaceIdx >= 0 {
				// R3.3: Space is written as the last character before the inserted newline.
				if _, err := w.WriteString(line[:spaceIdx+1]); err != nil {
					return err
				}
				if err := w.WriteByte('\n'); err != nil {
					return err
				}
				line = line[spaceIdx+1:]
				continue
			}
			// R3.2: No space found; fall through to hard break.
		}

		if _, err := w.WriteString(segment); err != nil {
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		line = line[breakIdx:]
	}

	if hasNewline {
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return nil
}

// findBreak returns the byte index in line where the width limit is reached.
// In byte mode, this is simply min(width, len(line)).
// In column mode, tabs expand to the next 8-column boundary.
func findBreak(line string, width int, opts foldOptions) int {
	if opts.byteMode {
		// R2.3: Each byte counts as 1.
		if width >= len(line) {
			return len(line)
		}
		return width
	}

	// Column mode: track display column position.
	col := 0
	for i := 0; i < len(line); i++ {
		if line[i] == '\t' {
			// R2.2: Tab expands to next tab stop (every 8 columns).
			newCol := col + 8 - (col % 8)
			if newCol > width {
				return i
			}
			col = newCol
		} else if line[i] == '\b' {
			// Backspace decreases column by 1 (matching GNU fold).
			if col > 0 {
				col--
			}
		} else {
			col++
			if col > width {
				return i
			}
		}
	}
	return len(line)
}

// parseArgs parses fold command-line flags manually.
func parseArgs(args []string) (foldOptions, []string) {
	opts := foldOptions{
		width: 80,
	}

	var files []string
	i := 0

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--width=") {
			opts.width = parseWidth(arg[len("--width="):])
			i++
			continue
		}
		if arg == "--width" {
			i++
			if i >= len(args) {
				fmt.Fprintf(os.Stderr, "fold: option '--width' requires an argument\n")
				os.Exit(1)
			}
			opts.width = parseWidth(args[i])
			i++
			continue
		}
		if arg == "--bytes" {
			opts.byteMode = true
			i++
			continue
		}
		if arg == "--spaces" {
			opts.spaceBreak = true
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			j := 1
			for j < len(arg) {
				ch := arg[j]
				switch ch {
				case 'b':
					opts.byteMode = true
					j++
				case 's':
					opts.spaceBreak = true
					j++
				case 'w':
					val := arg[j+1:]
					if val == "" {
						i++
						if i >= len(args) {
							fmt.Fprintf(os.Stderr, "fold: option requires an argument -- 'w'\n")
							os.Exit(1)
						}
						val = args[i]
					}
					opts.width = parseWidth(val)
					j = len(arg) // consumed rest of arg
				default:
					// Check for numeric width: -N is equivalent to -w N.
					if ch >= '0' && ch <= '9' {
						val := arg[j:]
						opts.width = parseWidth(val)
						j = len(arg)
					} else {
						fmt.Fprintf(os.Stderr, "fold: invalid option -- '%c'\n", ch)
						os.Exit(1)
					}
				}
			}
			i++
			continue
		}

		// Not a flag; treat as file argument.
		break
	}

	files = append(files, args[i:]...)
	return opts, files
}

// parseWidth parses a width string as a positive integer.
func parseWidth(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "fold: invalid number of columns: '%s'\n", s)
		os.Exit(1)
	}
	return n
}
