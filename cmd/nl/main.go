// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4: cmd/nl
// numbers lines from stdin or named files. Supports per-section numbering
// styles via -b (body), -h (header), and -f (footer) flags with styles a
// (all), t (non-empty, default for body), n (none), and pRE (regex match).
// Supports numbering format (-n ln/rn/rz), field width (-w), separator (-s),
// start value (-v), increment (-i), join blank lines (-l), section delimiter
// (-d), and page reset suppression (-p). Recognizes logical page section
// delimiters (\:\:\: header, \:\: body, \: footer). Installs SIGPIPE handler
// for clean exit on broken pipe.
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
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU nl format.
const progName = "nl"

// defaultWidth is the line number field width (R1.1).
const defaultWidth = 6

// defaultSep is the separator between line number and content (R1.1).
const defaultSep = "\t"

// defaultDelim is the two-character section delimiter pair.
// R4.1: default delimiter is \: (backslash-colon).
const defaultDelim = `\:`

// numberStyle represents a line numbering style for a section.
type numberStyle int

const (
	// styleT numbers non-empty lines only (default for body).
	styleT numberStyle = iota
	// styleA numbers all lines including empty ones.
	styleA
	// styleN numbers no lines.
	styleN
	// styleP numbers lines matching a regular expression.
	styleP
)

// numberFormat represents the output format for line numbers.
// R3.1: ln (left-justified), rn (right-justified, default), rz (right-justified, leading zeros).
type numberFormat int

const (
	formatRN numberFormat = iota // right-justified, no leading zeros (default)
	formatLN                     // left-justified, no leading zeros
	formatRZ                     // right-justified, leading zeros
)

// sectionType identifies the current logical page section.
// R4.1: nl processes input in logical pages with header, body, and footer sections.
type sectionType int

const (
	// sectionBody is the body section (default at start of input).
	sectionBody sectionType = iota
	// sectionHeader is the header section.
	sectionHeader
	// sectionFooter is the footer section.
	sectionFooter
)

// sectionStyle holds the parsed numbering style and optional regex for pRE.
type sectionStyle struct {
	style numberStyle
	re    *regexp.Regexp
}

// nlConfig holds the parsed options for nl.
type nlConfig struct {
	bodyStyle   sectionStyle
	headerStyle sectionStyle
	footerStyle sectionStyle
	format      numberFormat
	width       int
	sep         string
	startNum    int
	increment   int
	joinBlanks  int
	noReset     bool
	delim       string
}

// styleForSection returns the numbering style for the given section.
// R2.1-R2.3: -b, -h, -f control per-section numbering style.
func (c nlConfig) styleForSection(s sectionType) sectionStyle {
	switch s {
	case sectionHeader:
		return c.headerStyle
	case sectionFooter:
		return c.footerStyle
	default:
		return c.bodyStyle
	}
}

// formatNumber formats a line number according to the configured format and width.
// R3.1: ln=left-justified, rn=right-justified, rz=right-justified with leading zeros.
func (c nlConfig) formatNumber(n int) string {
	switch c.format {
	case formatLN:
		return fmt.Sprintf("%-*d", c.width, n)
	case formatRZ:
		return fmt.Sprintf("%0*d", c.width, n)
	default: // formatRN
		return fmt.Sprintf("%*d", c.width, n)
	}
}

// parseStyle parses a style string (a, t, n, pRE) into a sectionStyle.
// R2.1: style values are a, t, n, and pRE.
func parseStyle(flag, val string) (sectionStyle, error) {
	switch val {
	case "a":
		return sectionStyle{style: styleA}, nil
	case "t":
		return sectionStyle{style: styleT}, nil
	case "n":
		return sectionStyle{style: styleN}, nil
	}
	if strings.HasPrefix(val, "p") {
		pattern := val[1:]
		re, err := regexp.Compile(pattern)
		if err != nil {
			return sectionStyle{}, fmt.Errorf("invalid regular expression: %s", pattern)
		}
		return sectionStyle{style: styleP, re: re}, nil
	}
	return sectionStyle{}, fmt.Errorf("invalid %s style: %s", flag, val)
}

// shouldNumber returns true if the given line should be numbered under this style.
// R2.1-R2.4: style determines which lines get numbers.
func (s sectionStyle) shouldNumber(line string) bool {
	switch s.style {
	case styleA:
		return true
	case styleT:
		return line != ""
	case styleN:
		return false
	case styleP:
		return s.re.MatchString(line)
	}
	return false
}

// parseIntFlag parses a string as an integer for a numeric flag.
// Returns an error with a diagnostic matching GNU nl format.
func parseIntFlag(flag, val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("invalid %s argument: '%s'", flag, val)
	}
	return n, nil
}

// helpText is the usage message for --help.
const helpText = `Usage: %s [OPTION]... [FILE]...
Write each FILE to standard output, with line numbers added.

With no FILE, or when FILE is -, read standard input.

Mandatory arguments to long options are mandatory for short options too.
  -b, --body-numbering=STYLE      use STYLE for numbering body lines
  -d, --section-delimiter=CC       use CC for logical page delimiters
  -f, --footer-numbering=STYLE    use STYLE for numbering footer lines
  -h, --header-numbering=STYLE    use STYLE for numbering header lines
  -i, --line-increment=NUMBER     line number increment at each line
  -l, --join-blank-lines=NUMBER   group of NUMBER empty lines counted as one
  -n, --number-format=FORMAT      insert line numbers according to FORMAT
  -p, --no-renumber               do not reset line numbers for each section
  -s, --number-separator=STRING   add STRING after (possible) line number
  -v, --starting-line-number=NUMBER  first line number for each section
  -w, --number-width=NUMBER       use NUMBER columns for line numbers
      --help     display this help and exit
      --version  output version information and exit
`

func main() {
	sys.InstallSIGPIPEHandler()

	cfg := nlConfig{
		bodyStyle:   sectionStyle{style: styleT}, // R2.1: default body style is t.
		headerStyle: sectionStyle{style: styleN}, // R2.2: default header style is n.
		footerStyle: sectionStyle{style: styleN}, // R2.3: default footer style is n.
		format:      formatRN,                    // R3.1: default format is rn.
		width:       defaultWidth,                // R3.2: default width is 6.
		sep:         defaultSep,                  // R3.3: default separator is tab.
		startNum:    1,                           // R3.4: default start value.
		increment:   1,                           // R3.4: default increment.
		joinBlanks:  1,                           // R4.4: default join blank lines.
		noReset:     false,                       // R4.3: default resets on header.
		delim:       defaultDelim,                // R4.1: default delimiter pair.
	}

	// Parse flags manually to match GNU nl's flag syntax.
	args := os.Args[1:]
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}

		// R4.2: --help prints usage to stdout and exits 0.
		if arg == "--help" {
			fmt.Fprintf(os.Stdout, helpText, progName) //nolint:errcheck // best-effort output
			os.Exit(0)
		}
		// R4.3: --version prints version to stdout and exits 0.
		if arg == "--version" {
			fmt.Fprintf(os.Stdout, "%s (%s) %s\n", progName, "go-unix-utils", version.Version) //nolint:errcheck // best-effort output
			os.Exit(0)
		}

		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// Long options with = syntax.
		if strings.HasPrefix(arg, "--") {
			if err := parseLongOption(arg, &cfg); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				os.Exit(1)
			}
			continue
		}

		// Short options: handle -b, -h, -f (style flags) and -n, -w, -s, -v, -i, -l, -d (value flags).
		handled := false

		// Style flags: -b, -h, -f.
		for _, f := range []string{"b", "h", "f"} {
			if arg == "-"+f {
				if i+1 >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option requires an argument -- '%s'\n", progName, f)
					os.Exit(1)
				}
				i++
				style, err := parseStyle(f, args[i])
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
					os.Exit(1)
				}
				setStyle(&cfg, f, style)
				handled = true
				break
			}
			if strings.HasPrefix(arg, "-"+f) {
				val := arg[2:]
				style, err := parseStyle(f, val)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
					os.Exit(1)
				}
				setStyle(&cfg, f, style)
				handled = true
				break
			}
		}
		if handled {
			continue
		}

		// -p flag (no value).
		if arg == "-p" {
			cfg.noReset = true
			continue
		}

		// Value flags: -n, -w, -s, -v, -i, -l, -d.
		shortOpt := arg[1:2]
		switch shortOpt {
		case "n":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'n'\n", progName)
				os.Exit(1)
			}
			switch val {
			case "ln":
				cfg.format = formatLN
			case "rn":
				cfg.format = formatRN
			case "rz":
				cfg.format = formatRZ
			default:
				fmt.Fprintf(os.Stderr, "%s: invalid line number format: '%s'\n", progName, val)
				os.Exit(1)
			}
		case "w":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'w'\n", progName)
				os.Exit(1)
			}
			n, err := parseIntFlag("number width", val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				os.Exit(1)
			}
			cfg.width = n
		case "s":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				// Empty separator is valid — means no separator.
				cfg.sep = ""
			} else {
				cfg.sep = val
			}
		case "v":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'v'\n", progName)
				os.Exit(1)
			}
			n, err := parseIntFlag("starting value", val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				os.Exit(1)
			}
			cfg.startNum = n
		case "i":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'i'\n", progName)
				os.Exit(1)
			}
			n, err := parseIntFlag("line number increment", val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				os.Exit(1)
			}
			cfg.increment = n
		case "l":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'l'\n", progName)
				os.Exit(1)
			}
			n, err := parseIntFlag("number of blank lines", val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
				os.Exit(1)
			}
			cfg.joinBlanks = n
		case "d":
			val := getFlagValue(arg, args, &i)
			if val == "" {
				fmt.Fprintf(os.Stderr, "%s: option requires an argument -- 'd'\n", progName)
				os.Exit(1)
			}
			cfg.delim = val
		default:
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\n", progName, arg[1:])
			os.Exit(1)
		}
	}

	exitCode := 0
	lineNum := cfg.startNum
	section := sectionBody
	consecutiveEmpty := 0

	if len(files) == 0 {
		// R1.3: no file arguments — read from stdin.
		var err error
		lineNum, section, consecutiveEmpty, err = nlReader(os.Stdin, lineNum, section, consecutiveEmpty, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
			exitCode = 1
		}
		os.Exit(exitCode)
	}

	// R1.4: process each file in argument order with continuous numbering.
	for _, name := range files {
		var err error
		if name == "-" {
			// R1.3: "-" means read from stdin.
			lineNum, section, consecutiveEmpty, err = nlReader(os.Stdin, lineNum, section, consecutiveEmpty, cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: standard input: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		lineNum, section, consecutiveEmpty, err = nlReader(f, lineNum, section, consecutiveEmpty, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// getFlagValue extracts the value for a short flag. If the flag arg is just
// "-X", the next arg is consumed. If the flag arg is "-Xvalue", the value
// portion is returned directly.
func getFlagValue(arg string, args []string, idx *int) string {
	if len(arg) > 2 {
		return arg[2:]
	}
	if *idx+1 < len(args) {
		*idx++
		return args[*idx]
	}
	return ""
}

// setStyle assigns a parsed sectionStyle to the appropriate field in cfg.
func setStyle(cfg *nlConfig, flag string, style sectionStyle) {
	switch flag {
	case "b":
		cfg.bodyStyle = style
	case "h":
		cfg.headerStyle = style
	case "f":
		cfg.footerStyle = style
	}
}

// parseLongOption handles --key=value long options.
func parseLongOption(arg string, cfg *nlConfig) error {
	parts := strings.SplitN(arg, "=", 2)
	key := parts[0]
	val := ""
	if len(parts) == 2 {
		val = parts[1]
	}

	switch key {
	case "--body-numbering":
		style, err := parseStyle("body numbering", val)
		if err != nil {
			return err
		}
		cfg.bodyStyle = style
	case "--header-numbering":
		style, err := parseStyle("header numbering", val)
		if err != nil {
			return err
		}
		cfg.headerStyle = style
	case "--footer-numbering":
		style, err := parseStyle("footer numbering", val)
		if err != nil {
			return err
		}
		cfg.footerStyle = style
	case "--number-format":
		switch val {
		case "ln":
			cfg.format = formatLN
		case "rn":
			cfg.format = formatRN
		case "rz":
			cfg.format = formatRZ
		default:
			return fmt.Errorf("invalid line number format: '%s'", val)
		}
	case "--number-width":
		n, err := parseIntFlag("number width", val)
		if err != nil {
			return err
		}
		cfg.width = n
	case "--number-separator":
		cfg.sep = val
	case "--starting-line-number":
		n, err := parseIntFlag("starting value", val)
		if err != nil {
			return err
		}
		cfg.startNum = n
	case "--line-increment":
		n, err := parseIntFlag("line number increment", val)
		if err != nil {
			return err
		}
		cfg.increment = n
	case "--join-blank-lines":
		n, err := parseIntFlag("number of blank lines", val)
		if err != nil {
			return err
		}
		cfg.joinBlanks = n
	case "--no-renumber":
		cfg.noReset = true
	case "--section-delimiter":
		cfg.delim = val
	default:
		return fmt.Errorf("unrecognized option '%s'", arg)
	}
	return nil
}

// nlReader reads lines from r, numbering lines according to the per-section
// styles in cfg, starting at lineNum in the given section. Returns the next
// line number, current section, consecutive empty count, and any error.
//
// R1.1: non-empty lines are numbered with configured width and separator.
// R1.2: empty lines are output with whitespace padding but no number (under style t).
// R2.1-R2.4: per-section style determines which lines are numbered.
// R3.1-R3.4: format, width, separator, start, and increment are configurable.
// R4.1: section delimiter lines are replaced with empty lines in output.
// R4.2: header delimiter resets line counter to startNum (unless -p).
// R4.4: -l N join blank lines for numbering with style t.
func nlReader(r io.Reader, lineNum int, section sectionType, consecutiveEmpty int, cfg nlConfig) (int, sectionType, int, error) {
	w := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(r)

	// R4.1: precompute delimiter strings from the configured delimiter pair.
	headerDelim := cfg.delim + cfg.delim + cfg.delim
	bodyDelim := cfg.delim + cfg.delim
	footerDelim := cfg.delim

	// Compute padding for unnumbered lines.
	emptyPadding := strings.Repeat(" ", cfg.width+len(cfg.sep))

	for scanner.Scan() {
		line := scanner.Text()

		// R4.1: check for section delimiter lines before numbering.
		// Check longest first to avoid prefix-match ambiguity.
		if line == headerDelim {
			section = sectionHeader
			// R4.2-R4.3: reset counter unless -p is given.
			if !cfg.noReset {
				lineNum = cfg.startNum
			}
			consecutiveEmpty = 0
			if _, err := fmt.Fprintln(w); err != nil {
				return lineNum, section, consecutiveEmpty, err
			}
			continue
		}
		if line == bodyDelim {
			section = sectionBody
			// GNU nl resets counter on all section delimiters unless -p.
			if !cfg.noReset {
				lineNum = cfg.startNum
			}
			consecutiveEmpty = 0
			if _, err := fmt.Fprintln(w); err != nil {
				return lineNum, section, consecutiveEmpty, err
			}
			continue
		}
		if line == footerDelim {
			section = sectionFooter
			// GNU nl resets counter on all section delimiters unless -p.
			if !cfg.noReset {
				lineNum = cfg.startNum
			}
			consecutiveEmpty = 0
			if _, err := fmt.Fprintln(w); err != nil {
				return lineNum, section, consecutiveEmpty, err
			}
			continue
		}

		// R2.1-R2.3: apply the numbering style for the current section.
		style := cfg.styleForSection(section)

		// R4.4: track consecutive empty lines for -l N join blank lines.
		if line == "" {
			consecutiveEmpty++
		} else {
			consecutiveEmpty = 0
		}

		// Determine if this line should be numbered.
		shouldNum := style.shouldNumber(line)

		// R4.4: when -l N > 1 and the line is empty, group N consecutive
		// empty lines. Lines before the Nth are always unnumbered. The Nth
		// empty line is subject to the normal style check (e.g., style a
		// will number it, style t will not since the line is still empty).
		if line == "" && cfg.joinBlanks > 1 {
			if consecutiveEmpty >= cfg.joinBlanks {
				// Nth or beyond: apply normal style. Reset counter for next group.
				consecutiveEmpty = 0
			} else {
				// Before the Nth: always suppress numbering.
				shouldNum = false
			}
		}

		if shouldNum {
			numStr := cfg.formatNumber(lineNum)
			if _, err := fmt.Fprintf(w, "%s%s%s\n", numStr, cfg.sep, line); err != nil {
				return lineNum, section, consecutiveEmpty, err
			}
			lineNum += cfg.increment
		} else {
			// R2.4: unnumbered line — output with padding, no number or separator.
			if _, err := fmt.Fprintf(w, "%s%s\n", emptyPadding, line); err != nil {
				return lineNum, section, consecutiveEmpty, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return lineNum, section, consecutiveEmpty, err
	}

	if err := w.Flush(); err != nil {
		return lineNum, section, consecutiveEmpty, err
	}

	return lineNum, section, consecutiveEmpty, nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
