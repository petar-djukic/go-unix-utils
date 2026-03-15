// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd006-cat R1.1-R1.5, R2.1-R2.4, R3.1-R3.3, R4.1-R4.9:
// cmd/cat core concatenation with line numbering, blank-line squeezing,
// and non-printing display. Concatenates files to stdout, reads stdin
// when no arguments or "-" is given, and reports errors to stderr in
// GNU format.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU cat format.
const progName = "cat"

// catOptions holds the parsed flags for a cat invocation.
type catOptions struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only (overrides -n)
	squeezeBlanks  bool // -s: suppress repeated blank lines
	showNonPrinting bool // -v: display non-printing chars with caret/M- notation
	showEnds       bool // -E: append "$" before each newline
	showTabs       bool // -T: display tabs as ^I
}

// needsLineProcessing returns true when any flag requires line-by-line processing.
func (o *catOptions) needsLineProcessing() bool {
	return o.numberAll || o.numberNonBlank || o.squeezeBlanks ||
		o.showNonPrinting || o.showEnds || o.showTabs
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])
	exitCode := 0

	// lineNum persists across files per R2.1: numbering resets to 1 per
	// invocation, not per file.
	lineNum := 1
	// R3.2: prevBlank persists across file boundaries so consecutive blank
	// lines spanning two files are still squeezed.
	prevBlank := false

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		var err error
		lineNum, prevBlank, err = catFile(os.Stdin, opts, lineNum, prevBlank)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: stdin: %v\n", progName, err)
			exitCode = 1
		}
		_, _ = lineNum, prevBlank // suppress unused warnings at end of invocation
		os.Exit(exitCode)
	}

	// R1.1, R1.3: concatenate named files in argument order.
	for _, name := range files {
		if name == "-" {
			// R1.2: "-" means read from stdin.
			var err error
			lineNum, prevBlank, err = catFile(os.Stdin, opts, lineNum, prevBlank)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			// R5.2: print error to stderr in GNU format, continue processing.
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		lineNum, prevBlank, err = catFile(f, opts, lineNum, prevBlank)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// parseArgs separates flags from file arguments. GNU cat accepts flags
// before the first non-flag argument or after "--". Single-char flags
// can be grouped: -nb is equivalent to -n -b.
func parseArgs(args []string) (*catOptions, []string) {
	opts := &catOptions{}
	var files []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for _, ch := range arg[1:] {
				switch ch {
				case 'n':
					opts.numberAll = true
				case 'b':
					opts.numberNonBlank = true
				case 's':
					opts.squeezeBlanks = true
				case 'v':
					// R4.1: display non-printing characters.
					opts.showNonPrinting = true
				case 'E':
					// R4.3: append "$" before each newline.
					opts.showEnds = true
				case 'T':
					// R4.4: display tabs as ^I.
					opts.showTabs = true
				case 'A':
					// R4.5: -A is equivalent to -v -E -T.
					opts.showNonPrinting = true
					opts.showEnds = true
					opts.showTabs = true
				case 'e':
					// R4.6: -e is equivalent to -v -E.
					opts.showNonPrinting = true
					opts.showEnds = true
				case 't':
					// R4.7: -t is equivalent to -v -T.
					opts.showNonPrinting = true
					opts.showTabs = true
				case 'u':
					// R4.8: accepted but ignored.
				default:
					// Unknown flags silently accepted.
				}
			}
		} else {
			files = append(files, arg)
		}
	}

	// R2.3: -b overrides -n.
	if opts.numberNonBlank {
		opts.numberAll = false
	}

	return opts, files
}

// catFile processes a single reader according to opts. Returns the next line
// number, the prevBlank state for squeeze continuity, and any write error.
func catFile(r io.Reader, opts *catOptions, lineNum int, prevBlank bool) (int, bool, error) {
	// R1.4, R1.5: when no transformation flags are active, use io.Copy
	// to pass bytes through without modification.
	if !opts.needsLineProcessing() {
		_, err := io.Copy(os.Stdout, r)
		return lineNum, false, err
	}

	return catLines(r, opts, lineNum, prevBlank)
}

// catLines processes input line by line, applying transformations per R4.9
// order: squeeze blanks (-s), then non-printing display (-v/-T), then
// end-of-line marker (-E), then line number (-n/-b). R3.3: squeezing is
// applied before numbering so suppressed blank lines do not consume line
// numbers. R1.5: does not add or remove newlines.
func catLines(r io.Reader, opts *catOptions, lineNum int, prevBlank bool) (int, bool, error) {
	w := bufio.NewWriter(os.Stdout)
	br := bufio.NewReader(r)

	// atLineStart tracks whether we are at the start of a new line.
	// GNU cat numbers the first line of input even before any bytes arrive.
	atLineStart := true

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			hasNewline := line[len(line)-1] == '\n'
			content := line
			if hasNewline {
				content = line[:len(line)-1]
			}

			// R2.4: blank line is zero non-newline bytes followed by newline.
			isBlank := len(content) == 0 && hasNewline

			// R3.1: suppress repeated blank lines. The first blank line is
			// written; subsequent consecutive blank lines are discarded.
			if opts.squeezeBlanks && isBlank && prevBlank {
				continue
			}
			prevBlank = isBlank

			// R4.9 step 2: apply non-printing display (-v/-T) to content.
			if opts.showNonPrinting || opts.showTabs {
				content = transformContent(content, opts.showNonPrinting, opts.showTabs)
			}

			// R4.9 step 4: prepend line number (-n/-b).
			if atLineStart {
				if opts.numberNonBlank {
					// R2.2: -b numbers only non-blank lines.
					if !isBlank {
						if _, werr := fmt.Fprintf(w, "%6d\t", lineNum); werr != nil {
							return lineNum, prevBlank, werr
						}
						lineNum++
					}
				} else if opts.numberAll {
					// R2.1: -n numbers every line.
					if _, werr := fmt.Fprintf(w, "%6d\t", lineNum); werr != nil {
						return lineNum, prevBlank, werr
					}
					lineNum++
				}
			}

			// Write transformed content.
			if _, werr := w.Write(content); werr != nil {
				return lineNum, prevBlank, werr
			}
			if hasNewline {
				// R4.3/R4.9 step 3: append "$" before newline.
				if opts.showEnds {
					if werr := w.WriteByte('$'); werr != nil {
						return lineNum, prevBlank, werr
					}
				}
				if werr := w.WriteByte('\n'); werr != nil {
					return lineNum, prevBlank, werr
				}
				atLineStart = true
			} else {
				atLineStart = false
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return lineNum, prevBlank, err
		}
	}

	return lineNum, prevBlank, w.Flush()
}

// transformContent applies -v and -T byte transformations to content bytes.
// R4.1: -v uses caret notation and M- prefix for non-printing characters.
// R4.2: -v does not alter tab or newline (newline is not in content).
// R4.4: -T displays tabs as ^I.
func transformContent(content []byte, showNonPrinting, showTabs bool) []byte {
	// Pre-allocate with same capacity; most bytes pass through unchanged.
	buf := make([]byte, 0, len(content))
	for _, b := range content {
		if b == '\t' {
			if showTabs {
				// R4.4: display tab as ^I.
				buf = append(buf, '^', 'I')
			} else {
				buf = append(buf, b)
			}
			continue
		}
		if !showNonPrinting {
			buf = append(buf, b)
			continue
		}
		// R4.1: non-printing display.
		if b < 32 {
			// Control character (tab handled above, newline not in content).
			buf = append(buf, '^', b+64)
		} else if b == 127 {
			// DEL -> ^?
			buf = append(buf, '^', '?')
		} else if b >= 128 {
			if b < 128+32 {
				// 0x80-0x9F -> M-^X
				buf = append(buf, 'M', '-', '^', b-128+64)
			} else if b == 255 {
				// 0xFF -> M-^?
				buf = append(buf, 'M', '-', '^', '?')
			} else {
				// 0xA0-0xFE -> M-X
				buf = append(buf, 'M', '-', b-128)
			}
		} else {
			// Printable ASCII (0x20-0x7E).
			buf = append(buf, b)
		}
	}
	return buf
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
