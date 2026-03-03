// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/cat concatenates files and writes to stdout, optionally applying line
// numbering, blank-line squeezing, and non-printing character display, matching
// GNU cat output format under LC_ALL=C.
//
// Implements prd006-cat R1, R2, R3, R4, R5.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "cat"

type config struct {
	numberAll      bool // -n / --number
	numberNonBlank bool // -b / --number-nonblank
	showEnds       bool // -E / --show-ends
	showTabs       bool // -T / --show-tabs
	showNonPrint   bool // -v / --show-nonprinting
	squeezeBlank   bool // -s / --squeeze-blank
}

type outputState struct {
	lineNum     int64
	blankCount  int
	atLineStart bool
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, files := parseArgs(os.Args[1:])

	// No files means read stdin (prd006-cat R1.2).
	if len(files) == 0 {
		files = []string{"-"}
	}

	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	state := outputState{lineNum: 1, atLineStart: true}

	for _, file := range files {
		if err := processFile(out, cfg, file, &state); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
			exitCode = 1
		}
	}

	if err := out.Flush(); err != nil {
		os.Exit(1) // prd006-cat R5.3
	}

	os.Exit(exitCode)
}

// parseArgs parses GNU-style flags including combined short flags (-vET)
// and long flags (--show-all). Cat flags do not take values.
func parseArgs(args []string) (config, []string) {
	var cfg config
	var files []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "-" || !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
			continue
		}

		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			switch name {
			case "number":
				cfg.numberAll = true
			case "number-nonblank":
				cfg.numberNonBlank = true
			case "show-ends":
				cfg.showEnds = true
			case "show-tabs":
				cfg.showTabs = true
			case "show-nonprinting":
				cfg.showNonPrint = true
			case "squeeze-blank":
				cfg.squeezeBlank = true
			case "show-all":
				cfg.showNonPrint = true
				cfg.showEnds = true
				cfg.showTabs = true
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '--%s'\n", progName, name)
				os.Exit(1)
			}
			continue
		}

		// Short flags may be combined: -vET means -v -E -T.
		for _, ch := range arg[1:] {
			switch ch {
			case 'n':
				cfg.numberAll = true
			case 'b':
				cfg.numberNonBlank = true
			case 's':
				cfg.squeezeBlank = true
			case 'v':
				cfg.showNonPrint = true
			case 'E':
				cfg.showEnds = true
			case 'T':
				cfg.showTabs = true
			case 'A':
				cfg.showNonPrint = true
				cfg.showEnds = true
				cfg.showTabs = true
			case 'e':
				cfg.showNonPrint = true
				cfg.showEnds = true
			case 't':
				cfg.showNonPrint = true
				cfg.showTabs = true
			case 'u':
				// Accepted but no effect (prd006-cat R4.8).
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, ch)
				os.Exit(1)
			}
		}
	}

	return cfg, files
}

// needsTransformation reports whether any output transformation flag is active.
func needsTransformation(cfg config) bool {
	return cfg.numberAll || cfg.numberNonBlank || cfg.showEnds ||
		cfg.showTabs || cfg.showNonPrint || cfg.squeezeBlank
}

// processFile opens a single input and writes its content to out. The path "-"
// means stdin. State is carried across files for line numbering and blank
// squeezing (prd006-cat R3.2).
func processFile(out *bufio.Writer, cfg config, path string, state *outputState) error {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			if pe, ok := err.(*os.PathError); ok {
				return fmt.Errorf("%s: %v", path, pe.Err)
			}
			return err
		}
		defer f.Close()
		r = f
	}

	if needsTransformation(cfg) {
		return processInput(out, cfg, r, state)
	}

	_, err := io.Copy(out, r)
	return err
}

// processInput reads bytes from r and applies the configured transformations.
// The order of application follows prd006-cat R4.9: squeeze blanks, then
// non-printing display, then end-of-line marker, then line number prefix.
func processInput(out *bufio.Writer, cfg config, r io.Reader, state *outputState) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := r.Read(buf)
		for i := 0; i < n; i++ {
			b := buf[i]

			if state.atLineStart {
				if b == '\n' {
					// Blank line (prd006-cat R2.4).
					if cfg.squeezeBlank && state.blankCount >= 1 {
						continue
					}
					state.blankCount++

					if cfg.numberAll && !cfg.numberNonBlank {
						fmt.Fprintf(out, "%6d\t", state.lineNum)
						state.lineNum++
					}

					if cfg.showEnds {
						out.WriteByte('$')
					}
					out.WriteByte('\n')
					continue
				}

				// Non-blank line.
				state.blankCount = 0
				if cfg.numberAll || cfg.numberNonBlank {
					fmt.Fprintf(out, "%6d\t", state.lineNum)
					state.lineNum++
				}
				state.atLineStart = false
			}

			if b == '\n' {
				if cfg.showEnds {
					out.WriteByte('$')
				}
				out.WriteByte('\n')
				state.atLineStart = true
			} else if b == '\t' {
				if cfg.showTabs {
					out.WriteByte('^')
					out.WriteByte('I')
				} else {
					out.WriteByte('\t')
				}
			} else if cfg.showNonPrint {
				writeNonPrint(out, b)
			} else {
				out.WriteByte(b)
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

// writeNonPrint writes byte b in caret and M- notation per prd006-cat R4.1.
// Under LC_ALL=C: 0x00-0x1F (except tab/newline handled earlier) -> ^X,
// 0x7F -> ^?, 0x80-0x9F -> M-^X, 0xA0-0xFE -> M-x, 0xFF -> M-^?.
func writeNonPrint(out *bufio.Writer, b byte) {
	if b < 0x20 {
		out.WriteByte('^')
		out.WriteByte(b + 64)
	} else if b == 0x7F {
		out.WriteByte('^')
		out.WriteByte('?')
	} else if b >= 0x80 {
		out.WriteByte('M')
		out.WriteByte('-')
		adj := b - 0x80
		if adj < 0x20 {
			out.WriteByte('^')
			out.WriteByte(adj + 64)
		} else if adj == 0x7F {
			out.WriteByte('^')
			out.WriteByte('?')
		} else {
			out.WriteByte(adj)
		}
	} else {
		out.WriteByte(b)
	}
}
