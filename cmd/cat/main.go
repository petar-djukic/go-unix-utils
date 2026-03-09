// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cat — file concatenation and display.
// Implements prd006-cat (R1-R5): default passthrough, line numbering (-n, -b),
// blank squeezing (-s), non-printing display (-v, -E, -T, -A, -e, -t),
// error handling, and SIGPIPE handling.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

const (
	version    = "cat (go-unix-utils) 0.1"
	lineNumFmt = "%6d\t"
)

// flags holds parsed command-line flags for cat.
type flags struct {
	numberAll      bool // -n: number all output lines
	numberNonBlank bool // -b: number non-blank lines only (overrides -n)
	squeeze        bool // -s: squeeze consecutive blank lines
	showNonPrint   bool // -v: show non-printing characters
	showEnds       bool // -E: show $ at end of lines
	showTabs       bool // -T: show tabs as ^I
	showHelp       bool // --help
	showVersion    bool // --version
}

func main() {
	installSIGPIPEHandler()

	f, files := parseFlags(os.Args[1:])

	if f.showHelp {
		printHelp()
		os.Exit(0)
	}
	if f.showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	exitCode := run(f, files)
	os.Exit(exitCode)
}

// installSIGPIPEHandler sets up SIGPIPE handling so cat exits 0 when a
// downstream consumer closes its stdin. R5.4: use signal.Notify with a
// buffered channel; exit 0 in a goroutine.
func installSIGPIPEHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGPIPE)
	go func() {
		<-c
		os.Exit(0)
	}()
}

// parseFlags parses GNU-style short and long flags from args. Returns the
// parsed flags and remaining file arguments.
func parseFlags(args []string) (flags, []string) {
	var f flags
	var files []string

	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if arg == "--help" {
			f.showHelp = true
			continue
		}
		if arg == "--version" {
			f.showVersion = true
			continue
		}
		if arg == "--number" {
			f.numberAll = true
			continue
		}
		if arg == "--number-nonblank" {
			f.numberNonBlank = true
			continue
		}
		if arg == "--squeeze-blank" {
			f.squeeze = true
			continue
		}
		if arg == "--show-all" {
			f.showNonPrint = true
			f.showEnds = true
			f.showTabs = true
			continue
		}
		if arg == "--show-nonprinting" {
			f.showNonPrint = true
			continue
		}
		if arg == "--show-ends" {
			f.showEnds = true
			continue
		}
		if arg == "--show-tabs" {
			f.showTabs = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			// Short flags, possibly combined (e.g., -vET)
			for _, ch := range arg[1:] {
				switch ch {
				case 'n':
					f.numberAll = true
				case 'b':
					f.numberNonBlank = true
				case 's':
					f.squeeze = true
				case 'v':
					f.showNonPrint = true
				case 'E':
					f.showEnds = true
				case 'T':
					f.showTabs = true
				case 'A':
					f.showNonPrint = true
					f.showEnds = true
					f.showTabs = true
				case 'e':
					f.showNonPrint = true
					f.showEnds = true
				case 't':
					f.showNonPrint = true
					f.showTabs = true
				case 'u':
					// R4.8: accepted but no effect
				default:
					fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}
		files = append(files, arg)
	}

	return f, files
}

// needsLineProcessing returns true if any flag requires line-by-line processing.
func (f flags) needsLineProcessing() bool {
	return f.numberAll || f.numberNonBlank || f.squeeze ||
		f.showNonPrint || f.showEnds || f.showTabs
}

// run processes all files with the given flags and returns the exit code.
func run(f flags, files []string) int {
	// If no files specified, read from stdin. R1.2.
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	lineNum := 1
	lastBlank := false

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush() // best-effort flush on exit

	for _, name := range files {
		var r io.Reader
		if name == "-" {
			r = os.Stdin
		} else {
			file, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cat: %s: No such file or directory\n", name)
				exitCode = 1 // D4: set exit code but continue
				continue
			}
			defer file.Close() // best-effort close after loop iteration
			r = file
		}

		if !f.needsLineProcessing() {
			// D2: raw byte pass-through with io.Copy
			if _, err := io.Copy(out, r); err != nil {
				fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
				exitCode = 1
			}
			continue
		}

		// D2: line-by-line processing with bufio
		lineNum, lastBlank = processLines(out, r, f, lineNum, lastBlank, &exitCode)
	}

	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		exitCode = 1
	}

	return exitCode
}

// processLines reads from r line-by-line, applying transformation flags.
// Returns the updated line number and blank state for cross-file continuity.
// R4.9 transformation order: squeeze (-s), non-printing (-v/-T), ends (-E), number (-n/-b).
// Uses bufio.Reader.ReadBytes to preserve whether input ends with a newline (R1.5).
func processLines(out *bufio.Writer, r io.Reader, f flags, lineNum int, lastBlank bool, exitCode *int) (int, bool) {
	br := bufio.NewReaderSize(r, 64*1024)

	for {
		line, err := br.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "cat: read error: %v\n", err)
				*exitCode = 1
			}
			break
		}

		// Determine if line ends with newline and separate content from delimiter
		hasNewline := len(line) > 0 && line[len(line)-1] == '\n'
		content := line
		if hasNewline {
			content = line[:len(line)-1]
		}

		blank := len(content) == 0

		// R3.1: squeeze consecutive blank lines
		if f.squeeze && blank && hasNewline {
			if lastBlank {
				continue
			}
			lastBlank = true
		} else {
			lastBlank = blank && hasNewline
		}

		// R2: line numbering
		if f.numberNonBlank {
			// R2.2: number non-blank lines only
			if !blank {
				fmt.Fprintf(out, lineNumFmt, lineNum)
				lineNum++
			}
		} else if f.numberAll {
			// R2.1: number all lines
			fmt.Fprintf(out, lineNumFmt, lineNum)
			lineNum++
		}

		// Apply non-printing display and tabs to line content
		if f.showNonPrint || f.showTabs {
			writeTransformed(out, content, f.showNonPrint, f.showTabs)
		} else {
			out.Write(content)
		}

		// R4.3: show $ before newline
		if hasNewline && f.showEnds {
			out.WriteByte('$')
		}

		if hasNewline {
			out.WriteByte('\n')
		}

		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "cat: read error: %v\n", err)
				*exitCode = 1
			}
			break
		}
	}

	return lineNum, lastBlank
}

// writeTransformed writes bytes with non-printing character display (-v)
// and/or tab display (-T) applied.
// R4.1: caret notation for control chars, M- prefix for high bytes.
// R4.2: tab and newline are exempt from -v (newline handled by caller).
// R4.4: tab displayed as ^I when -T is active.
func writeTransformed(out *bufio.Writer, data []byte, showNonPrint, showTabs bool) {
	for _, b := range data {
		if b == '\t' {
			if showTabs {
				// R4.4: display tab as ^I
				out.WriteByte('^')
				out.WriteByte('I')
			} else {
				out.WriteByte(b)
			}
			continue
		}

		if !showNonPrint {
			out.WriteByte(b)
			continue
		}

		// R4.1: non-printing character display
		if b < 32 {
			// Control characters 0x00-0x1F (tab already handled above)
			out.WriteByte('^')
			out.WriteByte(b + 64)
		} else if b == 127 {
			// DEL character
			out.WriteByte('^')
			out.WriteByte('?')
		} else if b >= 128 {
			out.WriteByte('M')
			out.WriteByte('-')
			if b < 128+32 {
				// 0x80-0x9F: M-^X
				out.WriteByte('^')
				out.WriteByte(b - 128 + 64)
			} else if b == 255 {
				// 0xFF: M-^?
				out.WriteByte('^')
				out.WriteByte('?')
			} else {
				// 0xA0-0xFE: M-X
				out.WriteByte(b - 128)
			}
		} else {
			// Printable ASCII 0x20-0x7E
			out.WriteByte(b)
		}
	}
}

func printHelp() {
	fmt.Print(`Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s) to standard output.

With no FILE, or when FILE is -, read standard input.

  -A, --show-all           equivalent to -vET
  -b, --number-nonblank    number nonempty output lines, overrides -n
  -e                       equivalent to -vE
  -E, --show-ends          display $ at end of each line
  -n, --number             number all output lines
  -s, --squeeze-blank      suppress repeated empty output lines
  -t                       equivalent to -vT
  -T, --show-tabs          display TAB characters as ^I
  -u                       (ignored)
  -v, --show-nonprinting   use ^ and M- notation, except for LFD and TAB
      --help               display this help and exit
      --version            output version information and exit
`)
}
