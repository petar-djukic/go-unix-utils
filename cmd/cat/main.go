// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cat: concatenate and display files.
// Implements prd006-cat R1-R5.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// exitCode tracks whether any error occurred during processing.
// R5.1, R5.2: exit 0 on success, exit 1 on any file error.
var exitCode int

// flags holds the parsed command-line flags for cat.
type flags struct {
	numberAll      bool // -n: number all output lines (R2.1)
	numberNonBlank bool // -b: number non-blank lines only (R2.2)
	squeeze        bool // -s: squeeze consecutive blank lines (R3.1)
	showNonPrint   bool // -v: show non-printing characters (R4.1)
	showEnds       bool // -E: show $ at end of lines (R4.3)
	showTabs       bool // -T: show tabs as ^I (R4.4)
}

// needsTransform returns true when any transformation flag is active.
func (f *flags) needsTransform() bool {
	return f.numberAll || f.numberNonBlank || f.squeeze ||
		f.showNonPrint || f.showEnds || f.showTabs
}

func main() {
	// D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared_protocols.
	sys.InstallSIGPIPEHandler()

	fl, files := parseArgs(os.Args[1:])

	if len(files) == 0 {
		files = []string{"-"}
	}

	lineNum := 1
	prevBlank := false

	for _, name := range files {
		var r io.Reader
		if name == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cat: %s: No such file or directory\n", name)
				exitCode = 1
				continue
			}
			defer f.Close() // best-effort cleanup; main exits shortly after loop
			r = f
		}

		if !fl.needsTransform() {
			// R1.1, R1.4: verbatim copy, no transformation.
			if _, err := io.Copy(os.Stdout, r); err != nil {
				fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
				exitCode = 1
			}
			continue
		}

		lineNum, prevBlank = processTransformed(r, fl, lineNum, prevBlank)
	}

	os.Exit(exitCode)
}

// parseArgs parses cat flags from args, supporting combined short flags and --.
// D2: manual flag parsing matching GNU cat semantics.
func parseArgs(args []string) (*flags, []string) {
	fl := &flags{}
	var files []string
	endFlags := false

	for _, arg := range args {
		if endFlags || arg == "-" || (len(arg) > 0 && arg[0] != '-') {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endFlags = true
			continue
		}
		// Parse combined flags like -bvET
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				fl.numberAll = true
			case 'b':
				fl.numberNonBlank = true
			case 's':
				fl.squeeze = true
			case 'v':
				fl.showNonPrint = true
			case 'E':
				fl.showEnds = true
			case 'T':
				fl.showTabs = true
			case 'e': // R4.6: -e implies -vE
				fl.showNonPrint = true
				fl.showEnds = true
			case 't': // R4.7: -t implies -vT
				fl.showNonPrint = true
				fl.showTabs = true
			case 'A': // R4.5: -A implies -vET
				fl.showNonPrint = true
				fl.showEnds = true
				fl.showTabs = true
			case 'u': // R4.8: accepted, no effect
			default:
				fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
	}

	return fl, files
}

// processTransformed reads from r and writes transformed output to stdout.
// It returns the updated line number counter and blank-line state for cross-file continuity.
// R4.9: order of application: squeeze (-s), non-printing (-v/-T), end marker (-E), line number (-n/-b).
func processTransformed(r io.Reader, fl *flags, lineNum int, prevBlank bool) (int, bool) {
	reader := bufio.NewReader(r)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush() // best-effort flush

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break
		}

		hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
		content := line
		if hasNewline {
			content = line[:len(line)-1]
		}

		isBlank := len(content) == 0 && hasNewline

		// R3.1, R3.2: squeeze consecutive blank lines.
		if fl.squeeze && isBlank {
			if prevBlank {
				if err != nil {
					break
				}
				continue
			}
			prevBlank = true
		} else {
			prevBlank = false
		}

		// R2.1-R2.4: line numbering.
		doNumber := fl.numberNonBlank || fl.numberAll
		if doNumber {
			if fl.numberNonBlank && isBlank {
				// R2.2: blank lines get no number and no tab prefix.
			} else {
				fmt.Fprintf(w, "%6d\t", lineNum)
				lineNum++
			}
		}

		// Transform content bytes: -v, -T applied to content (not newline).
		if fl.showNonPrint || fl.showTabs {
			for _, b := range content {
				writeVisibleByte(w, b, fl.showNonPrint, fl.showTabs)
			}
		} else {
			w.Write(content)
		}

		// R4.3: -E appends $ before newline.
		if hasNewline {
			if fl.showEnds {
				w.WriteByte('$')
			}
			w.WriteByte('\n')
		}

		if err != nil {
			break
		}
	}

	return lineNum, prevBlank
}

// writeVisibleByte writes a single byte with non-printing and tab transformations.
// R4.1, R4.2, R4.4: caret/M- notation for non-printing, ^I for tabs.
func writeVisibleByte(w *bufio.Writer, b byte, showNonPrint, showTabs bool) {
	if b == '\t' {
		if showTabs {
			// R4.4: tab displayed as ^I
			w.WriteString("^I")
		} else {
			w.WriteByte(b)
		}
		return
	}

	if !showNonPrint {
		w.WriteByte(b)
		return
	}

	// R4.1: non-printing character display.
	if b >= 128 {
		w.WriteString("M-")
		b -= 128
	}
	if b < 32 {
		// Control characters 0x00-0x1F (except tab/newline already handled).
		w.WriteByte('^')
		w.WriteByte(b + 64)
	} else if b == 127 {
		// DEL character.
		w.WriteString("^?")
	} else {
		w.WriteByte(b)
	}
}
