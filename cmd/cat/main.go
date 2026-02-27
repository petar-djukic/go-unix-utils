// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cat utility: concatenate and display files.
//
// Implements: prd006-cat (R1-R5)
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// options holds the parsed command-line flags for cat.
type options struct {
	number          bool // -n: number all output lines
	numberNonBlank  bool // -b: number non-blank output lines only
	squeeze         bool // -s: suppress repeated empty output lines
	showNonPrinting bool // -v: display non-printing characters using ^ and M- notation
	showEnds        bool // -E: display $ at end of each line
	showTabs        bool // -T: display TAB characters as ^I
}

// needsTransform returns true when any flag requiring byte-by-byte processing
// is active. When false, io.Copy is used for efficient binary passthrough.
// Per design decision D3.
func (o *options) needsTransform() bool {
	return o.number || o.numberNonBlank || o.squeeze ||
		o.showNonPrinting || o.showEnds || o.showTabs
}

// catState tracks persistent state across multiple input files.
// State must persist across files for correct line numbering (R2.1: numbering
// does not reset per file) and blank-line squeezing (R3.2: squeezing applies
// across file boundaries).
type catState struct {
	lineNum     int  // Current line number counter.
	atLineStart bool // Whether we are at the beginning of a line.
	prevBlank   bool // Whether the previous line was blank (for -s).
}

func main() {
	// SIGPIPE handling: exit 0 silently on broken pipe.
	// Per prd006-cat R5.4 and design decision D2.
	sigpipeCh := make(chan os.Signal, 1)
	signal.Notify(sigpipeCh, syscall.SIGPIPE)
	go func() {
		<-sigpipeCh
		os.Exit(0)
	}()

	opts, files, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "cat: %s\n", err)
		os.Exit(1)
	}

	// Per prd006-cat R2.3: -b takes precedence over -n when both are given.
	if opts.numberNonBlank {
		opts.number = false
	}

	// Per prd006-cat R1.2: read from stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	writer := bufio.NewWriter(os.Stdout)
	state := catState{
		lineNum:     1,
		atLineStart: true,
	}

	for _, file := range files {
		if err := processFile(file, writer, &opts, &state); err != nil {
			fmt.Fprintf(os.Stderr, "cat: %s\n", err)
			exitCode = 1
		}
		if err := writer.Flush(); err != nil {
			// Per prd006-cat R5.3: exit 1 on stdout write error.
			os.Exit(1)
		}
	}

	os.Exit(exitCode)
}

// parseFlags parses cat command-line arguments using manual flag parsing to
// support combined short flags (e.g., -vET) matching GNU cat getopt behavior.
// Per design decision D4 and prd006-cat R4.5-R4.8.
func parseFlags(args []string) (options, []string, error) {
	var opts options
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "-" {
			// Per prd006-cat R1.2: "-" means stdin, not a flag.
			files = append(files, arg)
			continue
		}

		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'n':
					opts.number = true
				case 'b':
					opts.numberNonBlank = true
				case 's':
					opts.squeeze = true
				case 'v':
					opts.showNonPrinting = true
				case 'E':
					opts.showEnds = true
				case 'T':
					opts.showTabs = true
				case 'A':
					// Per prd006-cat R4.5: -A is equivalent to -vET.
					opts.showNonPrinting = true
					opts.showEnds = true
					opts.showTabs = true
				case 'e':
					// Per prd006-cat R4.6: -e is equivalent to -vE.
					opts.showNonPrinting = true
					opts.showEnds = true
				case 't':
					// Per prd006-cat R4.7: -t is equivalent to -vT.
					opts.showNonPrinting = true
					opts.showTabs = true
				case 'u':
					// Per prd006-cat R4.8: accepted but has no effect.
				default:
					return options{}, nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
		} else {
			files = append(files, arg)
		}
	}

	return opts, files, nil
}

// processFile opens and processes a single file (or stdin for "-").
// Per prd006-cat R1.1, R1.2, R5.2.
func processFile(file string, w *bufio.Writer, opts *options, state *catState) error {
	var r io.Reader
	if file == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(file)
		if err != nil {
			if pathErr, ok := err.(*os.PathError); ok {
				return fmt.Errorf("%s: %s", file, pathErr.Err)
			}
			return err
		}
		defer f.Close()
		r = f
	}

	if opts.needsTransform() {
		return processTransformed(r, w, opts, state)
	}
	// Per design decision D3: use io.Copy for efficient binary passthrough
	// when no transformation flags are active.
	_, err := io.Copy(w, r)
	return err
}

// processTransformed reads input byte-by-byte and applies transformations.
// Transformation order per prd006-cat R4.9 and design decision D1:
//  1. squeeze blanks (-s)
//  2. non-printing display (-v/-T)
//  3. end-of-line marker (-E)
//  4. line number prefix (-n/-b)
func processTransformed(r io.Reader, w *bufio.Writer, opts *options, state *catState) error {
	buf := make([]byte, 4096)

	for {
		n, readErr := r.Read(buf)

		for i := 0; i < n; i++ {
			b := buf[i]

			if state.atLineStart {
				if b == '\n' {
					// This is a blank line.
					// Step 1: squeeze check. Per prd006-cat R3.1.
					if opts.squeeze && state.prevBlank {
						continue
					}
					state.prevBlank = true

					// Step 4: line number prefix.
					if opts.numberNonBlank {
						// Per prd006-cat R2.2: blank lines get no number and no tab.
					} else if opts.number {
						if err := writeLineNumber(w, state.lineNum); err != nil {
							return err
						}
						state.lineNum++
					}

					// Step 3: end-of-line marker. Per prd006-cat R4.3.
					if opts.showEnds {
						if err := w.WriteByte('$'); err != nil {
							return err
						}
					}

					if err := w.WriteByte('\n'); err != nil {
						return err
					}
					continue
				}

				// Non-blank line start.
				state.prevBlank = false
				state.atLineStart = false

				// Step 4: line number prefix. Per prd006-cat R2.1, R2.2.
				if opts.number || opts.numberNonBlank {
					if err := writeLineNumber(w, state.lineNum); err != nil {
						return err
					}
					state.lineNum++
				}
			}

			// Process the byte (steps 2 and 3).
			if b == '\n' {
				// End of a non-blank line.
				// Step 3: end-of-line marker. Per prd006-cat R4.3.
				if opts.showEnds {
					if err := w.WriteByte('$'); err != nil {
						return err
					}
				}
				if err := w.WriteByte('\n'); err != nil {
					return err
				}
				state.atLineStart = true
				state.prevBlank = false
				continue
			}

			if b == '\t' {
				// Per prd006-cat R4.2, R4.4: -v does not alter tab. -T shows tab as ^I.
				if opts.showTabs {
					if _, err := w.WriteString("^I"); err != nil {
						return err
					}
				} else {
					if err := w.WriteByte(b); err != nil {
						return err
					}
				}
				continue
			}

			// Step 2: non-printing display. Per prd006-cat R4.1.
			if opts.showNonPrinting {
				if err := writeNonPrinting(w, b); err != nil {
					return err
				}
			} else {
				if err := w.WriteByte(b); err != nil {
					return err
				}
			}
		}

		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// writeLineNumber writes a right-justified line number with a tab separator.
// Format: "%6d\t" per prd006-cat R2.1.
func writeLineNumber(w *bufio.Writer, num int) error {
	_, err := fmt.Fprintf(w, "%6d\t", num)
	return err
}

// writeNonPrinting writes a byte using caret notation and M- prefix for
// non-printing characters. Per prd006-cat R4.1.
//
// This function must NOT be called with tab (0x09) or newline (0x0A) bytes;
// those are handled by the caller before reaching this function, per R4.2.
//
// Byte ranges:
//
//	0x00-0x1F → ^X (where X = byte + 64)
//	0x20-0x7E → pass through (printable ASCII)
//	0x7F      → ^?
//	0x80-0x9F → M-^X (where X = (byte - 128) + 64)
//	0xA0-0xFE → M-X (where X = byte - 128)
//	0xFF      → M-^?
func writeNonPrinting(w *bufio.Writer, b byte) error {
	if b < 32 {
		if err := w.WriteByte('^'); err != nil {
			return err
		}
		return w.WriteByte(b + 64)
	}

	if b < 127 {
		// Printable ASCII: 0x20-0x7E.
		return w.WriteByte(b)
	}

	if b == 127 {
		if err := w.WriteByte('^'); err != nil {
			return err
		}
		return w.WriteByte('?')
	}

	// High bytes (0x80-0xFF): M- prefix.
	if _, err := w.WriteString("M-"); err != nil {
		return err
	}
	stripped := b - 128
	if stripped < 32 {
		// 0x80-0x9F → M-^X
		if err := w.WriteByte('^'); err != nil {
			return err
		}
		return w.WriteByte(stripped + 64)
	}
	if stripped == 127 {
		// 0xFF → M-^?
		if err := w.WriteByte('^'); err != nil {
			return err
		}
		return w.WriteByte('?')
	}
	// 0xA0-0xFE → M-X
	return w.WriteByte(stripped)
}
