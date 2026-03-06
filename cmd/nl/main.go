// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the nl utility (prd022-nl R1-R4).
// nl reads lines from stdin or named files and writes them to stdout with
// line numbers, supporting configurable numbering styles per section (header,
// body, footer), format options, and logical page delimiters.
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

// Numbering style constants.
const (
	styleAll      = "a" // Number all lines.
	styleNone     = "n" // Number no lines.
	styleNonEmpty = "t" // Number non-empty lines (default for body).
	styleRegex    = "p" // Number lines matching a regex.
)

// Number format constants.
const (
	formatLN = "ln" // Left-justified, no leading zeros.
	formatRN = "rn" // Right-justified, no leading zeros (default).
	formatRZ = "rz" // Right-justified, leading zeros.
)

// Default section delimiter pair.
const defaultDelimiter = `\:`

// numberingStyle holds a parsed numbering style for a section.
type numberingStyle struct {
	mode  string
	regex *regexp.Regexp
}

// config holds all parsed command-line options.
type config struct {
	bodyStyle   numberingStyle
	headerStyle numberingStyle
	footerStyle numberingStyle
	format      string
	width       int
	separator   string
	startNum    int
	increment   int
	delimiter   string
	noReset     bool
	joinBlank   int
}

func main() {
	// D1: Handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	cfg, files := parseArgs(os.Args[1:])

	// R1.3: Read stdin when no file arguments are given.
	if len(files) == 0 {
		files = []string{"-"}
	}

	lineNum := cfg.startNum
	// Start in body section by default (no explicit delimiter needed).
	currentStyle := cfg.bodyStyle
	consecutiveEmpty := 0

	w := bufio.NewWriter(os.Stdout)

	for _, file := range files {
		r, err := openInput(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: %v\n", err)
			os.Exit(1)
		}

		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()

			// R4.1: Check for section delimiters.
			if section, ok := matchDelimiter(line, cfg.delimiter); ok {
				switch section {
				case "header":
					// R4.2: Reset counter on header unless -p.
					if !cfg.noReset {
						lineNum = cfg.startNum
					}
					currentStyle = cfg.headerStyle
				case "body":
					currentStyle = cfg.bodyStyle
				case "footer":
					currentStyle = cfg.footerStyle
				}
				consecutiveEmpty = 0
				// R4.1: Delimiter lines are consumed; not written to output.
				continue
			}

			isEmpty := len(line) == 0

			if shouldNumber(line, currentStyle, isEmpty, consecutiveEmpty, cfg.joinBlank) {
				fmt.Fprintf(w, "%s%s%s\n", formatNumber(lineNum, cfg.format, cfg.width), cfg.separator, line)
				lineNum += cfg.increment
				consecutiveEmpty = 0
			} else {
				if isEmpty {
					consecutiveEmpty++
					// R1.2: Empty lines pass through with no number and no separator.
					fmt.Fprint(w, "\n")
				} else {
					consecutiveEmpty = 0
					// R2.4: Style n — pass through with no number, but with width+separator spacing.
					fmt.Fprintf(w, "%s%s%s\n", strings.Repeat(" ", cfg.width), cfg.separator, line)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "nl: %v\n", err)
			os.Exit(1)
		}

		if closer, ok := r.(io.Closer); ok && file != "-" {
			closer.Close() // best-effort close
		}
	}

	if err := w.Flush(); err != nil {
		os.Exit(1)
	}
}

// openInput returns a reader for the named file, or stdin if name is "-".
func openInput(name string) (io.Reader, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// matchDelimiter checks if a line is a section delimiter.
// Returns the section name ("header", "body", "footer") and true if matched.
func matchDelimiter(line, delim string) (string, bool) {
	header := delim + delim + delim
	body := delim + delim
	footer := delim

	if line == header {
		return "header", true
	}
	if line == body {
		return "body", true
	}
	if line == footer {
		return "footer", true
	}
	return "", false
}

// shouldNumber determines whether a line should be numbered based on the
// current section's numbering style.
func shouldNumber(line string, style numberingStyle, isEmpty bool, consecutiveEmpty, joinBlank int) bool {
	switch style.mode {
	case styleAll:
		return true
	case styleNone:
		return false
	case styleNonEmpty:
		if isEmpty {
			// R4.4: With -l N, N consecutive empty lines trigger numbering.
			if joinBlank > 1 && (consecutiveEmpty+1) >= joinBlank {
				return true
			}
			return false
		}
		return true
	case styleRegex:
		if style.regex != nil {
			return style.regex.MatchString(line)
		}
		return false
	}
	return false
}

// formatNumber formats a line number according to the specified format and width.
func formatNumber(num int, format string, width int) string {
	s := strconv.Itoa(num)
	switch format {
	case formatLN:
		// Left-justified, padded with spaces on the right.
		if len(s) < width {
			return s + strings.Repeat(" ", width-len(s))
		}
		return s
	case formatRZ:
		// Right-justified, padded with leading zeros.
		if len(s) < width {
			return strings.Repeat("0", width-len(s)) + s
		}
		return s
	default: // formatRN
		// Right-justified, padded with spaces on the left.
		if len(s) < width {
			return strings.Repeat(" ", width-len(s)) + s
		}
		return s
	}
}

// parseArgs parses command-line arguments into a config and remaining file arguments.
// Uses manual parsing to match GNU nl flag syntax (e.g., -b a, -ba, -bpRE).
func parseArgs(args []string) (config, []string) {
	cfg := config{
		bodyStyle:   numberingStyle{mode: styleNonEmpty},
		headerStyle: numberingStyle{mode: styleNone},
		footerStyle: numberingStyle{mode: styleNone},
		format:      formatRN,
		width:       6,
		separator:   "\t",
		startNum:    1,
		increment:   1,
		delimiter:   defaultDelimiter,
		joinBlank:   1,
	}

	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if len(arg) == 0 || arg[0] != '-' || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}

		// Parse flag.
		flag := arg[1:]
		switch {
		case strings.HasPrefix(flag, "b"):
			cfg.bodyStyle = parseStyleArg(flag[1:], args, &i)
		case strings.HasPrefix(flag, "h"):
			cfg.headerStyle = parseStyleArg(flag[1:], args, &i)
		case strings.HasPrefix(flag, "f"):
			cfg.footerStyle = parseStyleArg(flag[1:], args, &i)
		case strings.HasPrefix(flag, "n"):
			cfg.format = getArgValue(flag[1:], args, &i)
		case strings.HasPrefix(flag, "w"):
			val := getArgValue(flag[1:], args, &i)
			n, err := strconv.Atoi(val)
			if err == nil {
				cfg.width = n
			}
		case strings.HasPrefix(flag, "s"):
			cfg.separator = getArgValue(flag[1:], args, &i)
		case strings.HasPrefix(flag, "v"):
			val := getArgValue(flag[1:], args, &i)
			n, err := strconv.Atoi(val)
			if err == nil {
				cfg.startNum = n
			}
		case strings.HasPrefix(flag, "i"):
			val := getArgValue(flag[1:], args, &i)
			n, err := strconv.Atoi(val)
			if err == nil {
				cfg.increment = n
			}
		case strings.HasPrefix(flag, "d"):
			cfg.delimiter = getArgValue(flag[1:], args, &i)
		case flag == "p":
			cfg.noReset = true
		case strings.HasPrefix(flag, "l"):
			val := getArgValue(flag[1:], args, &i)
			n, err := strconv.Atoi(val)
			if err == nil {
				cfg.joinBlank = n
			}
		default:
			// Unknown flag — treat as file.
			files = append(files, arg)
		}
		i++
	}

	return cfg, files
}

// parseStyleArg parses a numbering style value (a, t, n, pRE).
func parseStyleArg(rest string, args []string, idx *int) numberingStyle {
	val := rest
	if val == "" {
		*idx++
		if *idx < len(args) {
			val = args[*idx]
		}
	}

	switch {
	case val == styleAll:
		return numberingStyle{mode: styleAll}
	case val == styleNone:
		return numberingStyle{mode: styleNone}
	case val == styleNonEmpty:
		return numberingStyle{mode: styleNonEmpty}
	case strings.HasPrefix(val, styleRegex):
		pattern := val[1:]
		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: invalid regular expression: %s\n", pattern)
			os.Exit(1)
		}
		return numberingStyle{mode: styleRegex, regex: re}
	default:
		return numberingStyle{mode: styleNonEmpty}
	}
}

// getArgValue extracts the value for a flag, either from the remaining
// characters after the flag letter or from the next argument.
func getArgValue(rest string, args []string, idx *int) string {
	if rest != "" {
		return rest
	}
	*idx++
	if *idx < len(args) {
		return args[*idx]
	}
	return ""
}
