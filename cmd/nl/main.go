// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU nl: number lines of files.
// Implements prd022-nl R1-R4.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// numberStyle represents the numbering style for a section.
type numberStyle int

const (
	styleNonEmpty numberStyle = iota // t: number non-empty lines (default body)
	styleAll                         // a: number all lines
	styleNone                        // n: number no lines
	styleRegex                       // pRE: number lines matching regex
)

// numberFormat represents the output format for line numbers.
type numberFormat int

const (
	formatRN numberFormat = iota // rn: right-justified, no leading zeros (default)
	formatLN                    // ln: left-justified, no leading zeros
	formatRZ                    // rz: right-justified, leading zeros
)

// sectionType identifies which logical page section a line belongs to.
type sectionType int

const (
	sectionBody   sectionType = iota // default section
	sectionHeader                    // after \:\:\: delimiter
	sectionFooter                    // after \: delimiter
)

// nlOptions holds the parsed command-line flags for nl.
type nlOptions struct {
	bodyStyle   numberStyle  // -b: body numbering style (default t)
	headerStyle numberStyle  // -h: header numbering style (default n)
	footerStyle numberStyle  // -f: footer numbering style (default n)
	bodyRegex   *regexp.Regexp
	headerRegex *regexp.Regexp
	footerRegex *regexp.Regexp
	format      numberFormat // -n: number format (default rn)
	width       int          // -w: number field width (default 6)
	separator   string       // -s: separator between number and line (default \t)
	startNum    int          // -v: starting line number (default 1)
	increment   int          // -i: line number increment (default 1)
	noRenumber  bool         // -p: do not reset counter at logical pages
	joinBlank   int          // -l: consecutive empty lines treated as one (default 1)
	delimiter   string       // -d: section delimiter characters (default \:)
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	if len(files) == 0 {
		files = []string{"-"}
	}

	lineNum := opts.startNum
	currentSection := sectionBody
	consecutiveEmpty := 0

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
				fmt.Fprintf(os.Stderr, "nl: %s: No such file or directory\n", file)
				exitCode = 1
				continue
			}
			defer f.Close() // best-effort cleanup
			r = f
		}

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()

			// R4.1: Check for section delimiters.
			if newSection, ok := detectSection(line, opts.delimiter); ok {
				currentSection = newSection
				// R4.2: Header delimiter resets line counter unless -p.
				if newSection == sectionHeader && !opts.noRenumber {
					lineNum = opts.startNum
				}
				consecutiveEmpty = 0
				// Delimiter lines output as empty lines.
				fmt.Fprintln(w)
				continue
			}

			style := sectionStyle(opts, currentSection)
			regex := sectionRegex(opts, currentSection)

			isEmpty := line == ""

			if shouldNumber(line, style, regex, isEmpty, &consecutiveEmpty, opts.joinBlank) {
				printNumbered(w, lineNum, line, opts)
				lineNum += opts.increment
			} else {
				printUnnumbered(w, line)
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "nl: read error: %v\n", err)
			exitCode = 1
		}
	}

	w.Flush() // best-effort
	os.Exit(exitCode)
}

// detectSection checks if a line is a section delimiter and returns the new section type.
// R4.1: \:\:\: = header, \:\: = body, \: = footer.
func detectSection(line, delim string) (sectionType, bool) {
	// The delimiter is a two-character pair (default \:).
	// Header: delimiter repeated 3 times, body: 2 times, footer: 1 time.
	headerDelim := strings.Repeat(delim, 3)
	bodyDelim := strings.Repeat(delim, 2)
	footerDelim := delim

	// Must check longest first.
	if line == headerDelim {
		return sectionHeader, true
	}
	if line == bodyDelim {
		return sectionBody, true
	}
	if line == footerDelim {
		return sectionFooter, true
	}
	return sectionBody, false
}

// sectionStyle returns the numbering style for the given section.
func sectionStyle(opts nlOptions, section sectionType) numberStyle {
	switch section {
	case sectionHeader:
		return opts.headerStyle
	case sectionFooter:
		return opts.footerStyle
	default:
		return opts.bodyStyle
	}
}

// sectionRegex returns the compiled regex for the given section, if applicable.
func sectionRegex(opts nlOptions, section sectionType) *regexp.Regexp {
	switch section {
	case sectionHeader:
		return opts.headerRegex
	case sectionFooter:
		return opts.footerRegex
	default:
		return opts.bodyRegex
	}
}

// shouldNumber determines whether a line should receive a line number.
func shouldNumber(line string, style numberStyle, re *regexp.Regexp, isEmpty bool, consecutiveEmpty *int, joinBlank int) bool {
	switch style {
	case styleAll:
		if isEmpty {
			*consecutiveEmpty++
		} else {
			*consecutiveEmpty = 0
		}
		return true
	case styleNone:
		*consecutiveEmpty = 0
		return false
	case styleRegex:
		*consecutiveEmpty = 0
		if re != nil {
			return re.MatchString(line)
		}
		return false
	default: // styleNonEmpty
		if isEmpty {
			*consecutiveEmpty++
			if joinBlank > 1 && *consecutiveEmpty == joinBlank {
				*consecutiveEmpty = 0
				return true
			}
			return false
		}
		*consecutiveEmpty = 0
		return true
	}
}

// printNumbered writes a numbered line to w.
func printNumbered(w *bufio.Writer, num int, line string, opts nlOptions) {
	numStr := formatLineNumber(num, opts.format, opts.width)
	fmt.Fprintf(w, "%s%s%s\n", numStr, opts.separator, line)
}

// printUnnumbered writes an unnumbered line to w.
func printUnnumbered(w *bufio.Writer, line string) {
	fmt.Fprintln(w, line)
}

// formatLineNumber formats a line number according to the format and width.
func formatLineNumber(num int, format numberFormat, width int) string {
	switch format {
	case formatLN:
		// Left-justified, no leading zeros.
		return fmt.Sprintf("%-*d", width, num)
	case formatRZ:
		// Right-justified, leading zeros.
		return fmt.Sprintf("%0*d", width, num)
	default: // formatRN
		// Right-justified, no leading zeros.
		return fmt.Sprintf("%*d", width, num)
	}
}

// parseArgs parses nl command-line flags manually.
func parseArgs(args []string) (nlOptions, []string) {
	opts := nlOptions{
		bodyStyle:   styleNonEmpty,
		headerStyle: styleNone,
		footerStyle: styleNone,
		format:      formatRN,
		width:       6,
		separator:   "\t",
		startNum:    1,
		increment:   1,
		joinBlank:   1,
		delimiter:   `\:`,
	}

	var files []string
	i := 0

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}

		// Handle --help and --version.
		if arg == "--help" {
			fmt.Println("Usage: nl [OPTION]... [FILE]...")
			fmt.Println("Write each FILE to standard output, with line numbers added.")
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println("nl (go-unix-utils) dev")
			os.Exit(0)
		}

		// Long options.
		if strings.HasPrefix(arg, "--body-numbering=") {
			opts.bodyStyle, opts.bodyRegex = parseStyle(arg[len("--body-numbering="):])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--header-numbering=") {
			opts.headerStyle, opts.headerRegex = parseStyle(arg[len("--header-numbering="):])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--footer-numbering=") {
			opts.footerStyle, opts.footerRegex = parseStyle(arg[len("--footer-numbering="):])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--number-format=") {
			opts.format = parseFormat(arg[len("--number-format="):])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--number-width=") {
			opts.width = parseInt(arg[len("--number-width="):], "number-width")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--number-separator=") {
			opts.separator = arg[len("--number-separator="):]
			i++
			continue
		}
		if strings.HasPrefix(arg, "--starting-line-number=") {
			opts.startNum = parseInt(arg[len("--starting-line-number="):], "starting-line-number")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--line-increment=") {
			opts.increment = parseInt(arg[len("--line-increment="):], "line-increment")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--join-blank-lines=") {
			opts.joinBlank = parseInt(arg[len("--join-blank-lines="):], "join-blank-lines")
			i++
			continue
		}
		if strings.HasPrefix(arg, "--section-delimiter=") {
			opts.delimiter = arg[len("--section-delimiter="):]
			i++
			continue
		}
		if arg == "--no-renumber" || arg == "-p" {
			opts.noRenumber = true
			i++
			continue
		}

		// Short options.
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			ch := arg[1]
			switch ch {
			case 'b':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'b'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.bodyStyle, opts.bodyRegex = parseStyle(val)
				i++
				continue
			case 'h':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'h'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.headerStyle, opts.headerRegex = parseStyle(val)
				i++
				continue
			case 'f':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'f'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.footerStyle, opts.footerRegex = parseStyle(val)
				i++
				continue
			case 'n':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'n'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.format = parseFormat(val)
				i++
				continue
			case 'w':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'w'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.width = parseInt(val, "number-width")
				i++
				continue
			case 's':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 's'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.separator = val
				i++
				continue
			case 'v':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'v'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.startNum = parseInt(val, "starting-line-number")
				i++
				continue
			case 'i':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'i'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.increment = parseInt(val, "line-increment")
				i++
				continue
			case 'l':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'l'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.joinBlank = parseInt(val, "join-blank-lines")
				i++
				continue
			case 'd':
				val := arg[2:]
				if val == "" {
					i++
					if i >= len(args) {
						fmt.Fprintf(os.Stderr, "nl: option requires an argument -- 'd'\n")
						os.Exit(1)
					}
					val = args[i]
				}
				opts.delimiter = val
				i++
				continue
			case 'p':
				opts.noRenumber = true
				// Check if there are more flags after 'p' in combined form.
				if len(arg) > 2 {
					// Rewrite the remaining flags and re-process.
					args[i] = "-" + arg[2:]
					continue
				}
				i++
				continue
			default:
				fmt.Fprintf(os.Stderr, "nl: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}

		// Not a flag; treat as file argument.
		break
	}

	files = append(files, args[i:]...)
	return opts, files
}

// parseStyle parses a numbering style string (a, t, n, or pRE).
func parseStyle(s string) (numberStyle, *regexp.Regexp) {
	switch s {
	case "a":
		return styleAll, nil
	case "t":
		return styleNonEmpty, nil
	case "n":
		return styleNone, nil
	default:
		if strings.HasPrefix(s, "p") {
			pattern := s[1:]
			re, err := regexp.Compile(pattern)
			if err != nil {
				fmt.Fprintf(os.Stderr, "nl: invalid regular expression: %s\n", pattern)
				os.Exit(1)
			}
			return styleRegex, re
		}
		fmt.Fprintf(os.Stderr, "nl: invalid body numbering style: '%s'\n", s)
		os.Exit(1)
		return styleNonEmpty, nil // unreachable
	}
}

// parseFormat parses a number format string (ln, rn, or rz).
func parseFormat(s string) numberFormat {
	switch s {
	case "ln":
		return formatLN
	case "rn":
		return formatRN
	case "rz":
		return formatRZ
	default:
		fmt.Fprintf(os.Stderr, "nl: invalid line number format: '%s'\n", s)
		os.Exit(1)
		return formatRN // unreachable
	}
}

// parseInt parses a string as an integer for option values.
func parseInt(s, optName string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: invalid %s: '%s'\n", optName, s)
		os.Exit(1)
	}
	return n
}
