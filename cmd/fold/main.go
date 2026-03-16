// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd023-fold R1.1-R1.4, R2.1-R2.3, R3.1-R3.4: cmd/fold wraps long
// lines to a specified width. By default, lines are wrapped at 80 columns, with
// tab characters counted as the number of columns to the next tab stop (every
// 8 columns). -b counts bytes instead of columns. -s breaks at the last space
// within the width. Reads from stdin when no file arguments are given; treats
// '-' as stdin. Installs SIGPIPE handler for clean exit on broken pipe.
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

// progName is the name used in error messages to match GNU fold format.
const progName = "fold"

// defaultWidth is the default line wrap width in columns (R1.1).
const defaultWidth = 80

// tabStop is the tab stop interval for column counting (R2.2).
const tabStop = 8

func main() {
	sys.InstallSIGPIPEHandler()

	width := defaultWidth
	byteMode := false
	spaceBreak := false
	args := os.Args[1:]
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// Handle combined short flags (e.g., -bs, -bsw10).
		if len(arg) > 1 && arg[1] != '-' {
			flags := arg[1:]
			consumed := false
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'b':
					byteMode = true
				case 's':
					spaceBreak = true
				case 'w':
					// Remainder of flags is the width value, or next arg.
					val := flags[j+1:]
					if val == "" {
						if i+1 >= len(args) {
							fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", progName)
							os.Exit(1)
						}
						i++
						val = args[i]
					}
					n, err := strconv.Atoi(val)
					if err != nil || n <= 0 {
						fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, val)
						os.Exit(1)
					}
					width = n
					consumed = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
					os.Exit(1)
				}
				if consumed {
					break
				}
			}
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--width=") {
			val := arg[len("--width="):]
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, val)
				os.Exit(1)
			}
			width = n
			continue
		}
		if arg == "--width" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", progName)
				os.Exit(1)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "%s: invalid number of columns: '%s'\n", progName, args[i])
				os.Exit(1)
			}
			width = n
			continue
		}
		if arg == "--bytes" {
			byteMode = true
			continue
		}
		if arg == "--spaces" {
			spaceBreak = true
			continue
		}

		fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\n", progName, arg[1:])
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	if len(files) == 0 {
		// R1.1: no file arguments — read from stdin.
		if err := foldReader(os.Stdin, w, width, byteMode, spaceBreak); err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		// R1.2: process each file in argument order.
		for _, name := range files {
			if name == "-" {
				// R1.4: '-' means read from stdin.
				if err := foldReader(os.Stdin, w, width, byteMode, spaceBreak); err != nil {
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
			if err := foldReader(f, w, width, byteMode, spaceBreak); err != nil {
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

// foldReader reads from r and writes lines wrapped at width to w.
// R1.1: wraps at the specified width.
// R1.2: lines shorter than or equal to width pass through unchanged.
// R1.3: lines longer than width are split by inserting newlines.
// R1.4: final segment retains original trailing newline.
// R2.3: byteMode counts bytes instead of columns.
// R3.1-R3.4: spaceBreak breaks at the last space within the width.
func foldReader(r io.Reader, w *bufio.Writer, width int, byteMode, spaceBreak bool) error {
	br := bufio.NewReader(r)

	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			hasNewline := line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}

			if err := foldLine(w, content, width, byteMode, spaceBreak); err != nil {
				return err
			}

			// R1.4: preserve the original trailing newline on the final segment.
			if hasNewline {
				if _, err := w.WriteString("\n"); err != nil {
					return err
				}
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

// foldLine writes content to w, inserting newlines every width units.
// In column mode (default), tab characters advance to the next tab stop.
// In byte mode (-b), each byte counts as 1 unit (R2.3).
// In space-break mode (-s), breaks at the last space within the width (R3.1).
// R1.3: wrapping is applied repeatedly until the remaining portion fits.
func foldLine(w *bufio.Writer, content string, width int, byteMode, spaceBreak bool) error {
	if !spaceBreak {
		return foldLineSimple(w, content, width, byteMode)
	}
	return foldLineSpace(w, content, width, byteMode)
}

// foldLineSimple wraps content at exact width boundaries without space-breaking.
func foldLineSimple(w *bufio.Writer, content string, width int, byteMode bool) error {
	col := 0

	for i := 0; i < len(content); i++ {
		ch := content[i]

		var advance int
		if byteMode {
			// R2.3: in byte mode, every byte counts as 1 unit.
			advance = 1
		} else if ch == '\t' {
			// R2.2: tab advances to next tab stop.
			advance = tabStop - (col % tabStop)
		} else if ch == '\b' {
			if col > 0 {
				col--
			}
			if err := w.WriteByte(ch); err != nil {
				return err
			}
			continue
		} else if ch == '\r' {
			col = 0
			if err := w.WriteByte(ch); err != nil {
				return err
			}
			continue
		} else {
			advance = 1
		}

		if col+advance > width {
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
			col = 0
			if !byteMode && ch == '\t' {
				advance = tabStop
			}
		}

		if err := w.WriteByte(ch); err != nil {
			return err
		}
		col += advance
	}

	return nil
}

// foldLineSpace wraps content breaking at the last space within the width (R3.1-R3.4).
// R3.2: falls back to exact column break if no space is found.
// R3.3: the space is the last char before the newline; next line starts after it.
// R3.4: uses byte positions when byteMode is active.
func foldLineSpace(w *bufio.Writer, content string, width int, byteMode bool) error {
	i := 0
	for i < len(content) {
		// Scan one segment of up to width units.
		col := 0
		lastSpaceIdx := -1 // index relative to start of content (absolute)
		breakIdx := -1     // where to hard-break if no space found

		j := i
		for j < len(content) {
			ch := content[j]

			var advance int
			if byteMode {
				advance = 1
			} else if ch == '\t' {
				advance = tabStop - (col % tabStop)
			} else if ch == '\b' {
				if col > 0 {
					col--
				}
				j++
				continue
			} else if ch == '\r' {
				col = 0
				lastSpaceIdx = -1
				j++
				continue
			} else {
				advance = 1
			}

			if col+advance > width {
				breakIdx = j
				break
			}

			if ch == ' ' {
				lastSpaceIdx = j
			}

			col += advance
			j++
		}

		if breakIdx < 0 {
			// Remaining content fits within width.
			if _, err := w.WriteString(content[i:]); err != nil {
				return err
			}
			break
		}

		// R3.1: prefer breaking at the last space.
		if lastSpaceIdx >= i {
			// R3.3: write up to and including the space, then newline.
			if _, err := w.WriteString(content[i : lastSpaceIdx+1]); err != nil {
				return err
			}
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
			i = lastSpaceIdx + 1
		} else {
			// R3.2: no space found, fall back to exact break.
			if _, err := w.WriteString(content[i:breakIdx]); err != nil {
				return err
			}
			if _, err := w.WriteString("\n"); err != nil {
				return err
			}
			i = breakIdx
		}
	}

	return nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
