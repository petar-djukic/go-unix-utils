// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd006-cat R1.1–R1.5 (file concatenation, stdin, binary passthrough,
// no newline modification), R2.1–R2.4 (line numbering with -n and -b, blank line
// definition), R3.1–R3.3 (squeeze blank lines with -s), R4.1–R4.9 (non-printing
// character display with -v, -E, -T and composite flags -A, -e, -t; flag
// compatibility and application ordering), R5.1–R5.4 (exit codes, error handling,
// and SIGPIPE).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// catFlags holds parsed command-line flags for cat.
type catFlags struct {
	number         bool // -n: number all output lines. R2.1.
	numberNonblank bool // -b: number non-blank lines only. R2.2.
	squeeze        bool // -s: squeeze consecutive blank lines into one. R3.1.
	showNonprinting bool // -v: display non-printing characters. R4.1.
	showEnds       bool // -E: append "$" before each newline. R4.3.
	showTabs       bool // -T: display tabs as "^I". R4.4.
}

func main() {
	// R5.4: exit 0 on SIGPIPE when stdout is closed by a downstream consumer.
	sys.InstallSIGPIPEHandler()

	flags, files := parseArgs(os.Args[1:])

	exitCode := 0

	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)

	// state tracks line numbering across files. R2.1: numbering resets to 1 per invocation, not per file.
	state := &catState{lineNum: 1, atLineStart: true}

	for _, name := range files {
		var r io.Reader
		if name == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(name)
			if err != nil {
				// R5.2: report error to stderr, continue with remaining files.
				fmt.Fprintf(os.Stderr, "cat: %s\n", formatOpenError(name, err))
				exitCode = 1
				continue
			}
			defer f.Close() //nolint:gocritic // deferred close in loop is fine for cat; all files closed at exit.
			r = f
		}

		if err := processFile(r, w, flags, state); err != nil {
			// R5.3: write error on stdout.
			fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "cat: write error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// catState tracks state across file boundaries for line numbering and squeeze.
type catState struct {
	lineNum     int  // next line number to emit
	atLineStart bool // true when next byte is the first byte of a new line
	blankCount  int  // R3.1: consecutive blank lines seen so far
}

// processFile reads from r and writes to w, applying flags.
// R1.1–R1.5, R2.1–R2.4, R3.1–R3.3, R4.1–R4.4.
// R4.9 order: squeeze (-s), then non-printing (-v/-T), then ends (-E), then numbering (-n/-b).
func processFile(r io.Reader, w *bufio.Writer, flags catFlags, state *catState) error {
	needsProcessing := flags.number || flags.numberNonblank || flags.squeeze ||
		flags.showNonprinting || flags.showEnds || flags.showTabs

	if !needsProcessing {
		// R1.1–R1.5: verbatim copy, no transformation, no newline modification.
		_, err := io.Copy(w, r)
		return err
	}

	br := bufio.NewReader(r)

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if state.atLineStart {
			// R2.4: a blank line contains only a newline character.
			isBlank := (b == '\n')

			if isBlank {
				state.blankCount++

				// R3.1: suppress consecutive blank lines beyond the first.
				if flags.squeeze && state.blankCount > 1 {
					// atLineStart remains true for the next line.
					continue
				}
			} else {
				state.blankCount = 0
			}

			if flags.numberNonblank && isBlank {
				// R2.2: blank lines get no number and no tab prefix.
				// R4.3: append "$" before newline if -E is active.
				if flags.showEnds {
					if writeErr := w.WriteByte('$'); writeErr != nil {
						return writeErr
					}
				}
				if writeErr := w.WriteByte('\n'); writeErr != nil {
					return writeErr
				}
				// atLineStart remains true for the next line.
				continue
			}

			if flags.number || flags.numberNonblank {
				// R2.1: right-justified in width 6, followed by tab.
				// R3.3: squeeze applied before numbering; suppressed lines don't consume numbers.
				if _, writeErr := fmt.Fprintf(w, "%6d\t", state.lineNum); writeErr != nil {
					return writeErr
				}
				state.lineNum++
			}
			state.atLineStart = false
		}

		if b == '\n' {
			// R4.3: append "$" before newline.
			if flags.showEnds {
				if writeErr := w.WriteByte('$'); writeErr != nil {
					return writeErr
				}
			}
			if writeErr := w.WriteByte('\n'); writeErr != nil {
				return writeErr
			}
			state.atLineStart = true
			continue
		}

		if b == '\t' && flags.showTabs {
			// R4.4: display tabs as "^I".
			if _, writeErr := w.WriteString("^I"); writeErr != nil {
				return writeErr
			}
			continue
		}

		// R4.2: -v must not alter tab (0x09). Tab is handled above; skip it here.
		if flags.showNonprinting && b != '\t' {
			if writeErr := writeNonprinting(w, b); writeErr != nil {
				return writeErr
			}
			continue
		}

		if writeErr := w.WriteByte(b); writeErr != nil {
			return writeErr
		}
	}
}

// writeNonprinting writes byte b using caret/M- notation per R4.1–R4.2.
// Tab (0x09) and newline (0x0A) are handled by the caller and never reach this function.
func writeNonprinting(w *bufio.Writer, b byte) error {
	switch {
	case b < 0x20: // Control characters 0x00–0x1F (tab/newline handled by caller).
		if writeErr := w.WriteByte('^'); writeErr != nil {
			return writeErr
		}
		return w.WriteByte(b + 64) // ^@ through ^_
	case b == 0x7F: // DEL → ^?
		if writeErr := w.WriteByte('^'); writeErr != nil {
			return writeErr
		}
		return w.WriteByte('?')
	case b >= 0x80 && b <= 0x9F: // M-^@ through M-^_
		if _, writeErr := w.WriteString("M-^"); writeErr != nil {
			return writeErr
		}
		return w.WriteByte(b - 0x80 + 64)
	case b >= 0xA0 && b <= 0xFE: // M-  through M-~
		if _, writeErr := w.WriteString("M-"); writeErr != nil {
			return writeErr
		}
		return w.WriteByte(b - 0x80)
	case b == 0xFF: // M-^?
		if _, writeErr := w.WriteString("M-^"); writeErr != nil {
			return writeErr
		}
		return w.WriteByte('?')
	default:
		// Printable ASCII (0x20–0x7E): pass through.
		return w.WriteByte(b)
	}
}

// parseArgs parses cat command-line arguments and returns flags and file names.
// Supports GNU-style long flags (--number, --number-nonblank) and short flags.
func parseArgs(args []string) (catFlags, []string) {
	var flags catFlags
	var files []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags || arg == "-" || !isFlag(arg) {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			switch arg {
			case "--number":
				flags.number = true
			case "--number-nonblank":
				flags.numberNonblank = true
			case "--squeeze-blank":
				flags.squeeze = true
			case "--show-nonprinting":
				flags.showNonprinting = true
			case "--show-ends":
				flags.showEnds = true
			case "--show-tabs":
				flags.showTabs = true
			case "--show-all":
				// R4.5: -A = -vET.
				flags.showNonprinting = true
				flags.showEnds = true
				flags.showTabs = true
			default:
				fmt.Fprintf(os.Stderr, "cat: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}
		// Short flags: can be combined (e.g., -nb).
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				flags.number = true
			case 'b':
				flags.numberNonblank = true
			case 's':
				flags.squeeze = true
			case 'v':
				// R4.1: show non-printing characters.
				flags.showNonprinting = true
			case 'E':
				// R4.3: show ends.
				flags.showEnds = true
			case 'T':
				// R4.4: show tabs.
				flags.showTabs = true
			case 'A':
				// R4.5: -A = -vET.
				flags.showNonprinting = true
				flags.showEnds = true
				flags.showTabs = true
			case 'e':
				// R4.6: -e = -vE.
				flags.showNonprinting = true
				flags.showEnds = true
			case 't':
				// R4.7: -t = -vT.
				flags.showNonprinting = true
				flags.showTabs = true
			case 'u':
				// R4.8: accepted but ignored.
			default:
				fmt.Fprintf(os.Stderr, "cat: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
	}

	// R2.2: -b implies -n but overrides it (blank lines not numbered).
	// R2.3: when both -b and -n are given, -b takes precedence.
	if flags.numberNonblank {
		flags.number = false
	}

	return flags, files
}

// isFlag returns true if the argument looks like a flag (starts with "-" and is not just "-").
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isLongFlag returns true if the argument is a long flag (starts with "--").
func isLongFlag(arg string) bool {
	return len(arg) > 2 && arg[0] == '-' && arg[1] == '-'
}

// formatOpenError formats an os.Open error to match GNU cat output.
// R5.2: os.Open returns "*os.PathError" whose Error() is "open <path>: <reason>".
// GNU cat outputs "cat: <path>: <Reason>" with the strerror() message capitalized.
// Go's syscall errors are lowercase; we capitalize the first letter of the reason.
func formatOpenError(name string, err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		reason := pathErr.Err.Error()
		if len(reason) > 0 && reason[0] >= 'a' && reason[0] <= 'z' {
			reason = string(reason[0]-32) + reason[1:]
		}
		return fmt.Sprintf("%s: %s", name, reason)
	}
	return fmt.Sprintf("%s: %v", name, err)
}
