// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nl implements the nl (number lines) command.
// Implements: prd022-nl R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4, R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4
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

// R1.1: Default formatting constants per prd022.
const (
	defaultWidth     = 6
	defaultSeparator = "\t"
	defaultIncrement = 1
	defaultStartNum  = 1
	defaultJoinBlank = 1
)

// defaultDelimiter is the two-character section delimiter.
// R4.1: Delimiter lines use \: repeated 1-3 times to mark sections.
const defaultDelimiter = `\:`

// section represents which logical page section is currently active.
// R4.1: Input can contain header, body, and footer sections.
type section int

const (
	sectionBody   section = iota // default section
	sectionHeader
	sectionFooter
)

// nlState tracks mutable state shared across files.
type nlState struct {
	lineNum    int
	curSection section
	blankCount int
}

// numberStyle represents how lines in a section are numbered.
// R2.1: Styles are a (all), t (non-empty), n (none), pRE (regex match).
type numberStyle int

const (
	styleNonEmpty numberStyle = iota // t: number non-empty lines (default for body)
	styleAll                         // a: number all lines
	styleNone                        // n: number no lines (default for header/footer)
	styleRegex                       // pRE: number lines matching regex
)

// numberFormat represents the line number output format.
// R3.1: Formats are ln (left-justified), rn (right-justified, default), rz (right-justified with zeros).
type numberFormat int

const (
	formatRN numberFormat = iota // rn: right-justified, no leading zeros (default)
	formatLN                     // ln: left-justified, no leading zeros
	formatRZ                     // rz: right-justified, leading zeros
)

// sectionStyle holds a parsed numbering style and optional regex pattern.
type sectionStyle struct {
	style   numberStyle
	pattern *regexp.Regexp
}

// nlConfig holds all parsed options for the nl command.
type nlConfig struct {
	bodyStyle   sectionStyle
	headerStyle sectionStyle
	footerStyle sectionStyle
	format    numberFormat // R3.1: line number format
	width     int          // R3.2: line number field width
	separator string       // R3.3: separator between number and content
	startNum  int          // R3.4: initial line number (-v)
	increment int          // R3.4: line number increment (-i)
	noReset   bool         // R4.3: -p suppresses counter reset on new page
	joinBlank int          // R4.4: -l N consecutive empty lines treated as one
	files     []string
}

// parseStyle parses a style string into a sectionStyle.
// R2.1: Styles are a, t, n, or pRE where RE is a regular expression.
func parseStyle(s string) (sectionStyle, error) {
	switch {
	case s == "a":
		return sectionStyle{style: styleAll}, nil
	case s == "t":
		return sectionStyle{style: styleNonEmpty}, nil
	case s == "n":
		return sectionStyle{style: styleNone}, nil
	case len(s) >= 1 && s[0] == 'p':
		re, err := regexp.Compile(s[1:])
		if err != nil {
			return sectionStyle{}, fmt.Errorf("invalid regex in style %q: %w", s, err)
		}
		return sectionStyle{style: styleRegex, pattern: re}, nil
	default:
		return sectionStyle{}, fmt.Errorf("invalid numbering style: %q", s)
	}
}

// parseFormat parses a number format string into a numberFormat.
// R3.1: Formats are ln, rn, rz.
func parseFormat(s string) (numberFormat, error) {
	switch s {
	case "ln":
		return formatLN, nil
	case "rn":
		return formatRN, nil
	case "rz":
		return formatRZ, nil
	default:
		return formatRN, fmt.Errorf("invalid line number format: %q", s)
	}
}

// consumeValue returns the value for a flag. If rest is non-empty, it is used
// as the attached value. Otherwise the next argument is consumed.
func consumeValue(rest string, args []string, i *int, flagName string) (string, error) {
	if rest != "" {
		return rest, nil
	}
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("option requires an argument -- '%s'", flagName)
	}
	return args[*i], nil
}

// parseArgs parses command-line arguments into an nlConfig.
// Handles GNU-style flags where the value can be attached (-ba) or separate (-b a).
func parseArgs(args []string) (nlConfig, error) {
	cfg := nlConfig{
		bodyStyle:   sectionStyle{style: styleNonEmpty}, // R2.1: default body style is t
		headerStyle: sectionStyle{style: styleNone},     // R2.2: default header style is n
		footerStyle: sectionStyle{style: styleNone},     // R2.3: default footer style is n
		format:      formatRN,                           // R3.1: default format is rn
		width:       defaultWidth,                       // R3.2: default width is 6
		separator:   defaultSeparator,                   // R3.3: default separator is tab
		startNum:    defaultStartNum,                    // R3.4: default start is 1
		increment:   defaultIncrement,                   // R3.4: default increment is 1
		joinBlank:   defaultJoinBlank,                   // R4.4: default is 1
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			// Everything after -- is a file.
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.files = append(cfg.files, arg)
			i++
			continue
		}

		if len(arg) < 2 {
			cfg.files = append(cfg.files, arg)
			i++
			continue
		}

		flagChar := arg[1]
		rest := arg[2:]

		switch flagChar {
		case 'b', 'h', 'f':
			val, err := consumeValue(rest, args, &i, string(flagChar))
			if err != nil {
				return cfg, err
			}
			style, err := parseStyle(val)
			if err != nil {
				return cfg, err
			}
			switch flagChar {
			case 'b':
				cfg.bodyStyle = style
			case 'h':
				cfg.headerStyle = style
			case 'f':
				cfg.footerStyle = style
			}

		case 'n':
			// R3.1: -n FORMAT
			val, err := consumeValue(rest, args, &i, "n")
			if err != nil {
				return cfg, err
			}
			cfg.format, err = parseFormat(val)
			if err != nil {
				return cfg, err
			}

		case 'w':
			// R3.2: -w N
			val, err := consumeValue(rest, args, &i, "w")
			if err != nil {
				return cfg, err
			}
			w, err := strconv.Atoi(val)
			if err != nil {
				return cfg, fmt.Errorf("invalid width: %q", val)
			}
			cfg.width = w

		case 's':
			// R3.3: -s SEP
			val, err := consumeValue(rest, args, &i, "s")
			if err != nil {
				return cfg, err
			}
			cfg.separator = val

		case 'v':
			// R3.4: -v N
			val, err := consumeValue(rest, args, &i, "v")
			if err != nil {
				return cfg, err
			}
			n, err := strconv.Atoi(val)
			if err != nil {
				return cfg, fmt.Errorf("invalid starting line number: %q", val)
			}
			cfg.startNum = n

		case 'i':
			// R3.4: -i N
			val, err := consumeValue(rest, args, &i, "i")
			if err != nil {
				return cfg, err
			}
			n, err := strconv.Atoi(val)
			if err != nil {
				return cfg, fmt.Errorf("invalid line number increment: %q", val)
			}
			cfg.increment = n

		case 'p':
			// R4.3: -p suppresses counter reset on new logical page.
			cfg.noReset = true

		case 'l':
			// R4.4: -l N
			val, err := consumeValue(rest, args, &i, "l")
			if err != nil {
				return cfg, err
			}
			n, err := strconv.Atoi(val)
			if err != nil {
				return cfg, fmt.Errorf("invalid number of blank lines: %q", val)
			}
			cfg.joinBlank = n

		case 'd':
			// -d CC section delimiter (consumed but only default \: is used in R4.1-R4.4)
			_, err := consumeValue(rest, args, &i, "d")
			if err != nil {
				return cfg, err
			}

		default:
			// Unknown flag — pass as file to match GNU nl behavior (will error on open).
			cfg.files = append(cfg.files, arg)
		}

		i++
	}

	return cfg, nil
}

// shouldNumber returns true if line should be numbered under the given style.
// R2.1-R2.3: Style determines which lines get numbered.
// R2.4: Style n means no numbering — lines pass through unnumbered.
func shouldNumber(ss sectionStyle, line string) bool {
	switch ss.style {
	case styleAll:
		return true
	case styleNonEmpty:
		return line != ""
	case styleNone:
		return false
	case styleRegex:
		return ss.pattern.MatchString(line)
	default:
		return false
	}
}

func main() {
	// D1: Install SIGPIPE handler per ARCHITECTURE.yaml shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: %v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	state := &nlState{
		lineNum:    cfg.startNum, // R3.4: initial line number from -v
		curSection: sectionBody,  // R4.1: default section is body
	}

	if len(cfg.files) == 0 {
		// R1.3: No file arguments — read from stdin.
		if err := nlReader(os.Stdin, state, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "nl: %v\n", err)
			os.Exit(1)
		}
	} else {
		// R1.3, R1.4: Process each file in argument order with continuous numbering.
		for _, arg := range cfg.files {
			if err := nlFile(arg, state, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "nl: %v\n", err)
				exitCode = 1
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// formatNumber formats a line number according to the given format and width.
// R3.1: ln = left-justified, rn = right-justified, rz = right-justified with zeros.
// R3.2: width controls the field width.
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

// nlFile opens name and numbers its lines to stdout.
// R1.3: "-" reads from stdin.
func nlFile(name string, state *nlState, cfg nlConfig) error {
	if name == "-" {
		return nlReader(os.Stdin, state, cfg)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return nlReader(f, state, cfg)
}

// sectionStyleFor returns the numbering style for the given section.
func sectionStyleFor(sec section, cfg nlConfig) sectionStyle {
	switch sec {
	case sectionHeader:
		return cfg.headerStyle
	case sectionFooter:
		return cfg.footerStyle
	default:
		return cfg.bodyStyle
	}
}

// nlReader reads lines from r and writes them to stdout with line numbering.
//
// R1.1: Non-empty lines are numbered with configurable format, width, and separator.
// R1.2: Empty lines pass through with padding but no number.
// R1.4: State is shared across files for continuous numbering.
// R2.1-R2.4: Section styles control which lines get numbered.
// R3.1-R3.4: Format, width, separator, and increment are configurable.
// R4.1: Section delimiters switch the active section; delimiter lines are suppressed.
// R4.2: Header delimiter resets counter to startNum unless -p.
// R4.3: -p suppresses counter reset.
// R4.4: -l N groups consecutive empty lines.
func nlReader(r io.Reader, state *nlState, cfg nlConfig) error {
	// R1.2, R2.4: Unnumbered lines get width + len(separator) spaces of padding.
	emptyPrefix := strings.Repeat(" ", cfg.width+len(cfg.separator))

	// R4.1: Build delimiter strings for section detection.
	headerDelim := defaultDelimiter + defaultDelimiter + defaultDelimiter
	bodyDelim := defaultDelimiter + defaultDelimiter
	footerDelim := defaultDelimiter

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		// R4.1: Check for section delimiters (longest match first).
		// Delimiter lines are replaced with an empty line in the output.
		if line == headerDelim {
			state.curSection = sectionHeader
			state.blankCount = 0
			// R4.2: Reset counter unless -p (R4.3).
			if !cfg.noReset {
				state.lineNum = cfg.startNum
			}
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
			continue
		}
		if line == bodyDelim {
			state.curSection = sectionBody
			state.blankCount = 0
			// R4.2: Counter resets at each section boundary unless -p (R4.3).
			if !cfg.noReset {
				state.lineNum = cfg.startNum
			}
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
			continue
		}
		if line == footerDelim {
			state.curSection = sectionFooter
			state.blankCount = 0
			// R4.2: Counter resets at each section boundary unless -p (R4.3).
			if !cfg.noReset {
				state.lineNum = cfg.startNum
			}
			if _, err := fmt.Fprintln(os.Stdout); err != nil {
				return fmt.Errorf("write error: %w", err)
			}
			continue
		}

		style := sectionStyleFor(state.curSection, cfg)
		isEmpty := line == ""

		// Determine whether to number this line.
		var numberThisLine bool
		if isEmpty {
			state.blankCount++
			if style.style == styleAll {
				// R4.4: With -l N and style all, only the Nth consecutive
				// empty line in each group is numbered.
				if state.blankCount >= cfg.joinBlank {
					numberThisLine = true
					state.blankCount = 0
				}
			} else {
				numberThisLine = shouldNumber(style, line)
			}
		} else {
			state.blankCount = 0
			numberThisLine = shouldNumber(style, line)
		}

		var err error
		if numberThisLine {
			// R1.1, R2.1, R3.1-R3.3: Number this line with configured format.
			numStr := formatNumber(state.lineNum, cfg.format, cfg.width)
			_, err = fmt.Fprintf(os.Stdout, "%s%s%s\n", numStr, cfg.separator, line)
			state.lineNum += cfg.increment // R3.4: increment by configured value
		} else {
			// R1.2, R2.4: Line not numbered — pass through with padding.
			_, err = fmt.Fprintf(os.Stdout, "%s%s\n", emptyPrefix, line)
		}
		if err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read error: %w", err)
	}
	return nil
}
