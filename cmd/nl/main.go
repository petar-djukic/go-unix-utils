// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl: Number Lines of Files.
// Covers R1.1-R1.4 (default line numbering, file/stdin reading),
// R2.1-R2.4 (numbering style flags, section delimiter configuration),
// R3.1-R3.4 (format, width, separator, start, increment, join blank),
// R4.1-R4.4 (section delimiters, page reset, -p, -l).
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

// numberStyle represents the line numbering style for a section.
// R2.1: styles are a (all), t (non-empty, default), n (none), pRE (regex).
type numberStyle int

const (
	styleNonEmpty numberStyle = iota // t: number non-empty lines (default body)
	styleAll                         // a: number all lines
	styleNone                        // n: number no lines
	styleRegex                       // pRE: number lines matching regex
)

// numberFormat represents the line number output format.
// R1.3: formats are ln (left), rn (right, default), rz (right with zeros).
type numberFormat int

const (
	formatRN numberFormat = iota // rn: right-justified (default)
	formatLN                     // ln: left-justified
	formatRZ                     // rz: right-justified with leading zeros
)

// sectionKind identifies which logical page section is active.
// R2.1: header, body, footer sections detected by delimiter lines.
type sectionKind int

const (
	sectionBody   sectionKind = iota
	sectionHeader
	sectionFooter
)

// nlConfig holds parsed command-line options.
type nlConfig struct {
	bodyStyle   numberStyle
	bodyRegex   *regexp.Regexp
	headerStyle numberStyle    // R2.2: -h flag, default n
	headerRegex *regexp.Regexp
	footerStyle numberStyle    // R2.3: -f flag, default n
	footerRegex *regexp.Regexp
	format      numberFormat
	width       int
	separator   string
	delimiter   string // two-character section delimiter (default `\:`)
	startNumber int    // R3.4: -v flag, default 1
	increment   int    // R3.4: -i flag, default 1
	joinBlank   int    // R4.4: -l flag, default 1
	noReset     bool   // R4.3: -p flag, suppress counter reset
}

// nlState holds numbering state across files.
// R1.4: line numbering is continuous across files.
type nlState struct {
	lineNumber int
	section    sectionKind
	emptyCount int // consecutive empty lines seen (for -l)
}

func main() {
	// R5.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files := parseArgs(os.Args[1:])
	exitCode := run(cfg, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseArgs parses nl flags and returns config and file list.
// R1.1: files listed as arguments; stdin when no files or - given.
func parseArgs(args []string) (nlConfig, []string) {
	cfg := nlConfig{
		bodyStyle:   styleNonEmpty,
		headerStyle: styleNone,
		footerStyle: styleNone,
		format:      formatRN,
		width:       6,
		separator:   "\t",
		delimiter:   `\:`,
		startNumber: 1,
		increment:   1,
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
		consumed, err := parseFlag(arg, args, i, &cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: %s\n", err)
			os.Exit(1)
		}
		if consumed > 0 {
			i += consumed
			continue
		}
		files = append(files, arg)
		i++
	}
	return cfg, files
}

// parseFlag tries to parse a single flag starting at args[i].
// Returns the number of args consumed, or 0 if arg is not a flag.
// R3.4: prints error to stderr and exits 1 on invalid flag values.
func parseFlag(arg string, args []string, i int, cfg *nlConfig) (int, error) {
	if len(arg) < 2 || arg[0] != '-' {
		return 0, nil
	}
	return parseFlagByPrefix(arg, args, i, cfg)
}

// parseFlagByPrefix dispatches flag parsing based on the flag prefix.
func parseFlagByPrefix(arg string, args []string, i int, cfg *nlConfig) (int, error) {
	switch {
	case strings.HasPrefix(arg, "-b"):
		return parseFlagValue(arg, args, i, "-b", styleApply(&cfg.bodyStyle, &cfg.bodyRegex))
	case strings.HasPrefix(arg, "-h"):
		return parseFlagValue(arg, args, i, "-h", styleApply(&cfg.headerStyle, &cfg.headerRegex))
	case strings.HasPrefix(arg, "-f"):
		return parseFlagValue(arg, args, i, "-f", styleApply(&cfg.footerStyle, &cfg.footerRegex))
	case strings.HasPrefix(arg, "-n"):
		return parseFlagValue(arg, args, i, "-n", func(v string) error {
			return parseFormat(v, &cfg.format)
		})
	case strings.HasPrefix(arg, "-w"):
		return parseFlagValue(arg, args, i, "-w", parseIntFlag(&cfg.width, "width"))
	case strings.HasPrefix(arg, "-s"):
		return parseFlagValue(arg, args, i, "-s", func(v string) error {
			cfg.separator = v
			return nil
		})
	case strings.HasPrefix(arg, "-d"):
		return parseFlagValue(arg, args, i, "-d", parseDelimiter(cfg))
	case strings.HasPrefix(arg, "-v"):
		return parseFlagValue(arg, args, i, "-v", parseIntFlag(&cfg.startNumber, "starting line number"))
	case strings.HasPrefix(arg, "-i"):
		return parseFlagValue(arg, args, i, "-i", parseIntFlag(&cfg.increment, "line number increment"))
	case strings.HasPrefix(arg, "-l"):
		return parseFlagValue(arg, args, i, "-l", parseIntFlag(&cfg.joinBlank, "number of blank lines"))
	case arg == "-p":
		cfg.noReset = true
		return 1, nil
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
}

// styleApply returns a parser function for a numbering style flag value.
func styleApply(style *numberStyle, re **regexp.Regexp) func(string) error {
	return func(v string) error {
		return parseStyle(v, style, re)
	}
}

// parseIntFlag returns a parser function for an integer flag value.
func parseIntFlag(target *int, name string) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid %s: '%s'", name, v)
		}
		*target = n
		return nil
	}
}

// parseDelimiter returns a parser function for the -d flag value.
// R2.2: one character is padded with ':', two characters used as-is.
func parseDelimiter(cfg *nlConfig) func(string) error {
	return func(v string) error {
		switch len(v) {
		case 0:
			cfg.delimiter = ""
		case 1:
			cfg.delimiter = v + ":"
		default:
			cfg.delimiter = v[:2]
		}
		return nil
	}
}

// parseFlagValue extracts the value for a flag, either attached (-bVALUE)
// or as the next argument (-b VALUE). Calls apply with the value.
func parseFlagValue(arg string, args []string, i int, flag string, apply func(string) error) (int, error) {
	if len(arg) > len(flag) {
		return 1, apply(arg[len(flag):])
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- '%s'", flag[1:])
	}
	return 2, apply(args[i+1])
}

// parseStyle parses a numbering style string (a, t, n, pRE).
// R1.2: body style defaults to t; supports regex via pRE.
func parseStyle(s string, style *numberStyle, re **regexp.Regexp) error {
	switch {
	case s == "a":
		*style = styleAll
	case s == "t":
		*style = styleNonEmpty
	case s == "n":
		*style = styleNone
	case strings.HasPrefix(s, "p"):
		compiled, err := regexp.Compile(s[1:])
		if err != nil {
			return fmt.Errorf("invalid regex: '%s'", s[1:])
		}
		*style = styleRegex
		*re = compiled
	default:
		return fmt.Errorf("invalid numbering style: '%s'", s)
	}
	return nil
}

// parseFormat parses a number format string (ln, rn, rz).
// R1.3: rn is default.
func parseFormat(s string, format *numberFormat) error {
	switch s {
	case "ln":
		*format = formatLN
	case "rn":
		*format = formatRN
	case "rz":
		*format = formatRZ
	default:
		return fmt.Errorf("invalid line numbering format: '%s'", s)
	}
	return nil
}

// run processes all files and returns the exit code.
// R1.1: reads stdin when no files given.
// R1.4: line counter continuous across files.
func run(cfg nlConfig, files []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	state := &nlState{
		lineNumber: cfg.startNumber - cfg.increment,
		section:    sectionBody,
	}
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := processOneFile(name, stdin, bw, stderr, cfg, state); err != nil {
			exitCode = 1
		}
	}
	_ = bw.Flush() // best-effort final flush
	return exitCode
}

// processOneFile opens and processes a single file.
func processOneFile(name string, stdin io.Reader, bw *bufio.Writer, stderr io.Writer, cfg nlConfig, state *nlState) error {
	r, err := openInput(name, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "nl: %s: %s\n", name, err) // best-effort
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return processFile(r, bw, cfg, state)
}

// openInput opens a file or returns stdin for "-".
// R1.1: "-" means stdin.
func openInput(name string, stdin io.Reader) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(stdin), nil
	}
	return os.Open(name)
}

// processFile reads lines from r and writes numbered output.
// R2.1: delimiter lines are consumed and replaced with blank lines.
func processFile(r io.Reader, bw *bufio.Writer, cfg nlConfig, state *nlState) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if handleDelimiter(line, cfg, state) {
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			continue
		}
		if err := writeLine(bw, line, cfg, state); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// handleDelimiter checks if a line is a section delimiter.
// R2.1: \:\:\: = header, \:\: = body, \: = footer.
// R4.2: header delimiter resets counter to startNumber unless -p is given.
func handleDelimiter(line string, cfg nlConfig, state *nlState) bool {
	delim := cfg.delimiter
	if delim == "" {
		return false
	}
	headerDelim := delim + delim + delim
	bodyDelim := delim + delim
	// Check longest first to avoid prefix collisions.
	switch line {
	case headerDelim:
		state.section = sectionHeader
		if !cfg.noReset {
			state.lineNumber = cfg.startNumber - cfg.increment
		}
		state.emptyCount = 0
		return true
	case bodyDelim:
		state.section = sectionBody
		return true
	case delim:
		state.section = sectionFooter
		return true
	}
	return false
}

// writeLine writes one line of output with optional numbering.
// R1.2: unnumbered lines pass through with no number and no separator.
// R3.4: uses configured increment instead of 1.
func writeLine(bw *bufio.Writer, line string, cfg nlConfig, state *nlState) error {
	style := activeStyle(cfg, state.section)
	re := activeRegex(cfg, state.section)
	if shouldNumberLine(style, line, re, cfg, state) {
		state.lineNumber += cfg.increment
		if _, err := bw.WriteString(formatNumber(state.lineNumber, cfg)); err != nil {
			return err
		}
		if _, err := bw.WriteString(cfg.separator); err != nil {
			return err
		}
	} else if line != "" {
		state.emptyCount = 0
	}
	if _, err := bw.WriteString(line); err != nil {
		return err
	}
	return bw.WriteByte('\n')
}

// shouldNumberLine checks if a line should be numbered, accounting for -l.
// R4.4: with -l N, N consecutive empty lines count as one for numbering.
func shouldNumberLine(style numberStyle, line string, re *regexp.Regexp, cfg nlConfig, state *nlState) bool {
	if !shouldNumber(style, line, re) {
		return false
	}
	if line == "" {
		state.emptyCount++
		if state.emptyCount < cfg.joinBlank {
			return false
		}
		state.emptyCount = 0
		return true
	}
	state.emptyCount = 0
	return true
}

// activeStyle returns the numbering style for the current section.
// R2.2: header uses headerStyle; R2.3: footer uses footerStyle.
func activeStyle(cfg nlConfig, section sectionKind) numberStyle {
	switch section {
	case sectionHeader:
		return cfg.headerStyle
	case sectionFooter:
		return cfg.footerStyle
	default:
		return cfg.bodyStyle
	}
}

// activeRegex returns the regex for the current section's style.
func activeRegex(cfg nlConfig, section sectionKind) *regexp.Regexp {
	switch section {
	case sectionHeader:
		return cfg.headerRegex
	case sectionFooter:
		return cfg.footerRegex
	default:
		return cfg.bodyRegex
	}
}

// shouldNumber reports whether a line should receive a number.
// R1.2: style t skips empty lines; style a numbers all; style n skips all.
func shouldNumber(style numberStyle, line string, re *regexp.Regexp) bool {
	switch style {
	case styleAll:
		return true
	case styleNonEmpty:
		return line != ""
	case styleRegex:
		return re != nil && re.MatchString(line)
	default:
		return false
	}
}

// formatNumber formats a line number according to the config.
// R1.3: ln (left), rn (right, default), rz (right with zeros).
func formatNumber(n int, cfg nlConfig) string {
	switch cfg.format {
	case formatLN:
		return fmt.Sprintf("%-*d", cfg.width, n)
	case formatRZ:
		return fmt.Sprintf("%0*d", cfg.width, n)
	default:
		return fmt.Sprintf("%*d", cfg.width, n)
	}
}
