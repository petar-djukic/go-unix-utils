// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the nl utility for numbering lines of files.
//
// Implements prd022-nl: default line numbering (R1), numbering style flags (R2),
// format and numbering options (R3), section delimiters and page reset (R4).
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

// numberStyle describes how lines in a section are numbered.
type numberStyle int

const (
	styleNone    numberStyle = iota // n: no numbering
	styleNonEmpty                   // t: number non-empty lines (default for body)
	styleAll                        // a: number all lines
	styleRegex                      // p RE: number lines matching regex
)

// numberFormat describes how line numbers are formatted.
type numberFormat int

const (
	formatRN numberFormat = iota // rn: right-justified (default)
	formatLN                     // ln: left-justified
	formatRZ                     // rz: right-justified with leading zeros
)

// sectionType identifies the current logical section.
type sectionType int

const (
	sectionBody   sectionType = iota // default
	sectionHeader
	sectionFooter
)

// config holds all parsed command-line options.
type config struct {
	bodyStyle   numberStyle
	headerStyle numberStyle
	footerStyle numberStyle
	bodyRegex   *regexp.Regexp
	headerRegex *regexp.Regexp
	footerRegex *regexp.Regexp
	format      numberFormat
	width       int
	separator   string
	startNum    int
	increment   int
	noReset     bool
	joinBlank   int
	delimiter   string
}

func main() {
	sys.InstallSIGPIPEHandler()

	cfg, files := parseArgs(os.Args[1:])
	exitCode := 0

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	lineNum := cfg.startNum
	currentSection := sectionBody
	blankCount := 0

	processLine := func(line string) {
		// Check for section delimiters. R4.1.
		if sec, ok := matchDelimiter(line, cfg.delimiter); ok {
			currentSection = sec
			if !cfg.noReset {
				// R4.2: reset line counter at section boundaries.
				lineNum = cfg.startNum
			}
			blankCount = 0
			// Delimiter lines are not written to output.
			fmt.Fprint(w, "\n")
			return
		}

		style, re := sectionStyle(cfg, currentSection)
		isEmpty := line == ""

		if isEmpty {
			blankCount++
		} else {
			blankCount = 0
		}

		shouldNumber := false
		switch style {
		case styleAll:
			if isEmpty {
				// R4.4: -l N treats N consecutive blanks as one for numbering.
				shouldNumber = blankCount >= cfg.joinBlank
			} else {
				shouldNumber = true
			}
		case styleNonEmpty:
			if !isEmpty {
				shouldNumber = true
			} else if cfg.joinBlank > 1 && blankCount >= cfg.joinBlank {
				shouldNumber = true
			}
		case styleRegex:
			if re != nil && re.MatchString(line) {
				shouldNumber = true
			}
		case styleNone:
			// no numbering
		}

		if shouldNumber {
			fmt.Fprintf(w, "%s%s%s\n", formatNumber(lineNum, cfg.format, cfg.width), cfg.separator, line)
			lineNum += cfg.increment
		} else {
			// R2.4: unnumbered lines pass through with no number and no separator.
			// GNU nl uses width + len(separator) spaces for the blank prefix.
			fmt.Fprintf(w, "%s%s\n", strings.Repeat(" ", cfg.width+len(cfg.separator)), line)
		}
	}

	processReader := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			processLine(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "nl: read error: %v\n", err)
			exitCode = 1
		}
	}

	if len(files) == 0 {
		processReader(os.Stdin)
	} else {
		for _, name := range files {
			if name == "-" {
				processReader(os.Stdin)
				continue
			}
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "nl: %v\n", err)
				exitCode = 1
				continue
			}
			processReader(f)
			f.Close()
		}
	}

	w.Flush()
	os.Exit(exitCode)
}

// parseArgs parses command-line arguments into config and file names.
func parseArgs(args []string) (config, []string) {
	cfg := config{
		bodyStyle:   styleNonEmpty,
		headerStyle: styleNone,
		footerStyle: styleNone,
		format:      formatRN,
		width:       6,
		separator:   "\t",
		startNum:    1,
		increment:   1,
		joinBlank:   1,
		delimiter:   "\\:",
	}
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch {
			case strings.HasPrefix(arg, "--body-numbering"):
				val := longOptValue(arg, "--body-numbering", args, &i)
				cfg.bodyStyle, cfg.bodyRegex = parseStyle(val)
			case strings.HasPrefix(arg, "--header-numbering"):
				val := longOptValue(arg, "--header-numbering", args, &i)
				cfg.headerStyle, cfg.headerRegex = parseStyle(val)
			case strings.HasPrefix(arg, "--footer-numbering"):
				val := longOptValue(arg, "--footer-numbering", args, &i)
				cfg.footerStyle, cfg.footerRegex = parseStyle(val)
			case strings.HasPrefix(arg, "--number-format"):
				val := longOptValue(arg, "--number-format", args, &i)
				cfg.format = parseFormat(val)
			case strings.HasPrefix(arg, "--number-width"):
				val := longOptValue(arg, "--number-width", args, &i)
				cfg.width = parseInt(val)
			case strings.HasPrefix(arg, "--number-separator"):
				val := longOptValue(arg, "--number-separator", args, &i)
				cfg.separator = val
			case strings.HasPrefix(arg, "--starting-line-number"):
				val := longOptValue(arg, "--starting-line-number", args, &i)
				cfg.startNum = parseInt(val)
			case strings.HasPrefix(arg, "--line-increment"):
				val := longOptValue(arg, "--line-increment", args, &i)
				cfg.increment = parseInt(val)
			case strings.HasPrefix(arg, "--no-renumber"):
				cfg.noReset = true
			case strings.HasPrefix(arg, "--join-blank-lines"):
				val := longOptValue(arg, "--join-blank-lines", args, &i)
				cfg.joinBlank = parseInt(val)
			case strings.HasPrefix(arg, "--section-delimiter"):
				val := longOptValue(arg, "--section-delimiter", args, &i)
				cfg.delimiter = val
			default:
				fmt.Fprintf(os.Stderr, "nl: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			i++
			continue
		}

		// Short options.
		j := 1
		for j < len(arg) {
			ch := arg[j]
			switch ch {
			case 'b':
				val := shortOptValue(arg, j, args, &i)
				cfg.bodyStyle, cfg.bodyRegex = parseStyle(val)
				j = len(arg)
			case 'h':
				val := shortOptValue(arg, j, args, &i)
				cfg.headerStyle, cfg.headerRegex = parseStyle(val)
				j = len(arg)
			case 'f':
				val := shortOptValue(arg, j, args, &i)
				cfg.footerStyle, cfg.footerRegex = parseStyle(val)
				j = len(arg)
			case 'n':
				val := shortOptValue(arg, j, args, &i)
				cfg.format = parseFormat(val)
				j = len(arg)
			case 'w':
				val := shortOptValue(arg, j, args, &i)
				cfg.width = parseInt(val)
				j = len(arg)
			case 's':
				val := shortOptValue(arg, j, args, &i)
				cfg.separator = val
				j = len(arg)
			case 'v':
				val := shortOptValue(arg, j, args, &i)
				cfg.startNum = parseInt(val)
				j = len(arg)
			case 'i':
				val := shortOptValue(arg, j, args, &i)
				cfg.increment = parseInt(val)
				j = len(arg)
			case 'l':
				val := shortOptValue(arg, j, args, &i)
				cfg.joinBlank = parseInt(val)
				j = len(arg)
			case 'p':
				cfg.noReset = true
				j++
			case 'd':
				val := shortOptValue(arg, j, args, &i)
				cfg.delimiter = val
				j = len(arg)
			default:
				fmt.Fprintf(os.Stderr, "nl: invalid option -- '%c'\n", ch)
				os.Exit(1)
			}
		}
		i++
	}

	return cfg, files
}

// shortOptValue extracts the value for a short option that takes an argument.
// The value may be attached (e.g., -ba) or in the next argument (e.g., -b a).
func shortOptValue(arg string, pos int, args []string, idx *int) string {
	rest := arg[pos+1:]
	if rest != "" {
		return rest
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(os.Stderr, "nl: option requires an argument -- '%c'\n", arg[pos])
		os.Exit(1)
	}
	return args[*idx]
}

// longOptValue extracts the value for a long option, either from --opt=val or the next arg.
func longOptValue(arg, prefix string, args []string, idx *int) string {
	if strings.Contains(arg, "=") {
		return arg[strings.Index(arg, "=")+1:]
	}
	*idx++
	if *idx >= len(args) {
		fmt.Fprintf(os.Stderr, "nl: option '%s' requires an argument\n", prefix)
		os.Exit(1)
	}
	return args[*idx]
}

// parseStyle parses a numbering style string into a numberStyle and optional regex.
func parseStyle(s string) (numberStyle, *regexp.Regexp) {
	switch {
	case s == "a":
		return styleAll, nil
	case s == "t":
		return styleNonEmpty, nil
	case s == "n":
		return styleNone, nil
	case strings.HasPrefix(s, "p"):
		pattern := s[1:]
		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: invalid regular expression: %v\n", err)
			os.Exit(1)
		}
		return styleRegex, re
	default:
		fmt.Fprintf(os.Stderr, "nl: invalid numbering style: '%s'\n", s)
		os.Exit(1)
		return styleNone, nil
	}
}

// parseFormat parses a number format string.
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
		return formatRN
	}
}

// parseInt parses a string to int, exiting on error.
func parseInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: invalid number: '%s'\n", s)
		os.Exit(1)
	}
	return n
}

// formatNumber formats a line number according to format and width.
func formatNumber(num int, f numberFormat, width int) string {
	switch f {
	case formatLN:
		return fmt.Sprintf("%-*d", width, num)
	case formatRZ:
		return fmt.Sprintf("%0*d", width, num)
	default: // formatRN
		return fmt.Sprintf("%*d", width, num)
	}
}

// matchDelimiter checks if a line is a section delimiter.
// GNU nl uses the -d characters (default \:) to form delimiters:
// Three repetitions = header, two = body, one = footer.
// The delimiter line must consist of exactly these characters and nothing else.
func matchDelimiter(line, delim string) (sectionType, bool) {
	header := delim + delim + delim
	body := delim + delim
	footer := delim

	switch line {
	case header:
		return sectionHeader, true
	case body:
		return sectionBody, true
	case footer:
		return sectionFooter, true
	}
	return sectionBody, false
}

// sectionStyle returns the numbering style and regex for the given section.
func sectionStyle(cfg config, sec sectionType) (numberStyle, *regexp.Regexp) {
	switch sec {
	case sectionHeader:
		return cfg.headerStyle, cfg.headerRegex
	case sectionFooter:
		return cfg.footerStyle, cfg.footerRegex
	default:
		return cfg.bodyStyle, cfg.bodyRegex
	}
}
