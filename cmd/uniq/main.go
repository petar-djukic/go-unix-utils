// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd028-uniq R1.1-R1.4, R2.1-R2.4: cmd/uniq reads input line by
// line and suppresses adjacent duplicate lines, writing unique lines to stdout
// or an output file. Supports counting (-c), duplicate-only (-d), unique-only
// (-u), and all-repeated (-D) output modes. Reads from stdin when no file
// argument is given, or from a named file. A second positional argument
// specifies the output file. Installs SIGPIPE handler per shared protocol.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU uniq format.
const progName = "uniq"

// options holds the parsed command-line flags for output selection.
// R2.1-R2.4: counting and duplicate filtering modes.
type options struct {
	count       bool   // -c/--count: prefix lines with occurrence count
	repeated    bool   // -d/--repeated: print only duplicate lines (one per group)
	unique      bool   // -u/--unique: print only unique lines
	allRepeated string // -D/--all-repeated: print all duplicate lines; "none", "prepend", or "separate"
}

func main() {
	// R1.4: install SIGPIPE handler for graceful exit when piped.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	var files []string
	var opts options

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if arg == "--help" {
			fmt.Fprintf(os.Stdout,
				"Usage: %s [OPTION]... [INPUT [OUTPUT]]\n"+
					"Filter adjacent matching lines from INPUT (or standard input),\n"+
					"writing to OUTPUT (or standard output).\n\n"+
					"With no options, matching lines are merged to the first occurrence.\n\n"+
					"  -c, --count           prefix lines by the number of occurrences\n"+
					"  -d, --repeated        only print duplicate lines, one for each group\n"+
					"  -D                    print all duplicate lines\n"+
					"      --all-repeated[=METHOD]  like -D, but allow separating groups\n"+
					"                                 with an empty line;\n"+
					"                                 METHOD={none(default),prepend,separate}\n"+
					"  -u, --unique          only print unique lines\n"+
					"      --help     display this help and exit\n"+
					"      --version  output version information and exit\n",
				progName,
			)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n",
				progName, "go-unix-utils", version.Version,
			)
			os.Exit(0)
		}
		if arg == "--count" {
			opts.count = true
			continue
		}
		if arg == "--repeated" {
			opts.repeated = true
			continue
		}
		if arg == "--unique" {
			opts.unique = true
			continue
		}
		// R2.4: --all-repeated with optional method.
		if arg == "--all-repeated" {
			opts.allRepeated = "none"
			continue
		}
		if strings.HasPrefix(arg, "--all-repeated=") {
			method := arg[len("--all-repeated="):]
			switch method {
			case "none", "prepend", "separate":
				opts.allRepeated = method
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid argument '%s' for '--all-repeated'\n", progName, method)
				fmt.Fprintf(os.Stderr, "Valid arguments are:\n  - 'none'\n  - 'prepend'\n  - 'separate'\n")
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
			continue
		}

		// Unrecognized long options.
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", progName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
			os.Exit(1)
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// Short flags: -c, -d, -u, -D and combinations.
		flags := arg[1:]
		for j := 0; j < len(flags); j++ {
			switch flags[j] {
			case 'c':
				opts.count = true
			case 'd':
				opts.repeated = true
			case 'u':
				opts.unique = true
			case 'D':
				// R2.4: -D defaults to "none" delimiter method.
				opts.allRepeated = "none"
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", progName, flags[j])
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
				os.Exit(1)
			}
		}
	}

	// R1.2: first positional arg is input file, second is output file.
	inputName := "-"
	outputName := ""
	if len(files) >= 1 {
		inputName = files[0]
	}
	if len(files) >= 2 {
		outputName = files[1]
	}
	if len(files) > 2 {
		fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", progName, files[2])
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// Open input.
	var input io.Reader
	if inputName == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(inputName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, inputName, unwrapPathError(err))
			os.Exit(1)
		}
		defer f.Close()
		input = f
	}

	// Open output.
	var output io.Writer
	if outputName == "" {
		output = os.Stdout
	} else {
		f, err := os.Create(outputName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progName, outputName, unwrapPathError(err))
			os.Exit(1)
		}
		defer f.Close()
		output = f
	}

	w := bufio.NewWriter(output)
	exitCode := uniqLines(input, w, opts)

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", progName, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// uniqLines reads from r and writes deduplicated adjacent lines to w,
// applying the output selection modes from opts.
// R1.1: suppresses all but the first occurrence of any run of identical
// adjacent lines. R1.2: lines appearing only once are written through.
// R1.4: comparison is case-sensitive and includes the full line content.
// R2.1-R2.4: counting and filtering via opts.
func uniqLines(r io.Reader, w *bufio.Writer, opts options) int {
	br := bufio.NewReader(r)

	// R2.4: -D mode uses different output logic (all lines in duplicate groups).
	if opts.allRepeated != "" {
		return uniqAllRepeated(br, w, opts)
	}

	var prevLine string
	hasPrev := false
	count := 0

	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			content := line
			if line[len(line)-1] == '\n' {
				content = line[:len(line)-1]
			}

			if !hasPrev {
				prevLine = content
				hasPrev = true
				count = 1
			} else if content == prevLine {
				count++
			} else {
				if shouldOutput(count, opts) {
					if werr := writeLine(w, prevLine, count, opts.count); werr != nil {
						return 1
					}
				}
				prevLine = content
				count = 1
			}
		}
		if err != nil {
			if err == io.EOF {
				// Flush last group.
				if hasPrev && shouldOutput(count, opts) {
					if werr := writeLine(w, prevLine, count, opts.count); werr != nil {
						return 1
					}
				}
				return 0
			}
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
			return 1
		}
	}
}

// uniqAllRepeated handles -D/--all-repeated mode, printing every line in
// duplicate groups. R2.4: the delimiter method controls blank line insertion.
func uniqAllRepeated(br *bufio.Reader, w *bufio.Writer, opts options) int {
	var prevLine string
	hasPrev := false
	count := 0
	firstDupGroup := true

	flushGroup := func() error {
		if count <= 1 {
			return nil
		}
		// R2.4: delimiter method controls blank lines between groups.
		if opts.allRepeated == "prepend" || (opts.allRepeated == "separate" && !firstDupGroup) {
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
		}
		firstDupGroup = false
		for i := 0; i < count; i++ {
			if _, err := w.WriteString(prevLine); err != nil {
				return err
			}
			if err := w.WriteByte('\n'); err != nil {
				return err
			}
		}
		return nil
	}

	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			content := line
			if line[len(line)-1] == '\n' {
				content = line[:len(line)-1]
			}

			if !hasPrev {
				prevLine = content
				hasPrev = true
				count = 1
			} else if content == prevLine {
				count++
			} else {
				if ferr := flushGroup(); ferr != nil {
					return 1
				}
				prevLine = content
				count = 1
			}
		}
		if err != nil {
			if err == io.EOF {
				if hasPrev {
					if ferr := flushGroup(); ferr != nil {
						return 1
					}
				}
				return 0
			}
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", progName, err)
			return 1
		}
	}
}

// shouldOutput returns true if a group with the given count should produce
// output based on the filtering options.
// R2.1: -d filters to count > 1. R2.3: -u filters to count == 1.
func shouldOutput(count int, opts options) bool {
	if opts.repeated {
		return count > 1
	}
	if opts.unique {
		return count == 1
	}
	return true
}

// writeLine writes a single output line, optionally prefixed with count.
// R2.4: -c format uses %7d to match GNU uniq right-justified count.
func writeLine(w *bufio.Writer, line string, count int, showCount bool) error {
	if showCount {
		if _, err := fmt.Fprintf(w, "%7d %s\n", count, line); err != nil {
			return err
		}
		return nil
	}
	if _, err := w.WriteString(line); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

// unwrapPathError extracts the inner error from an *os.PathError and
// capitalizes the first letter to match GNU error message format
// (e.g., "No such file or directory" instead of "no such file or directory").
func unwrapPathError(err error) string {
	inner := err
	if pe, ok := err.(*os.PathError); ok {
		inner = pe.Err
	}
	msg := inner.Error()
	if len(msg) == 0 {
		return msg
	}
	runes := []rune(msg)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
