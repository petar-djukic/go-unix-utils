// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nl: number lines of files.
// Implements srd022-nl R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sectionKind represents which logical page section is active.
// R4.1: header, body, footer sections delimited by special lines.
type sectionKind int

const (
	sectionBody   sectionKind = iota
	sectionHeader
	sectionFooter
)

// delimiter is the default two-character section delimiter pair.
const delimiter = `\:`

// numberStyle represents a line numbering style for a section.
// R2.1: styles are a (all), t (non-empty), n (none), p RE (regex match).
type numberStyle struct {
	kind  byte           // 'a', 't', 'n', 'p'
	regex *regexp.Regexp // non-nil only when kind == 'p'
}

// config holds nl command-line options.
type config struct {
	numFormat   string      // R3.1: ln, rn, rz (default rn)
	width       int         // R3.2: -w N (default 6)
	sep         string      // R3.3: -s SEP (default tab)
	startVal    int         // R3.4: -v N (default 1)
	increment   int         // R3.4: -i N (default 1)
	bodyStyle   numberStyle // R2.1: -b STYLE (default t)
	headerStyle numberStyle // R2.2: -h STYLE (default n)
	footerStyle numberStyle // R2.3: -f STYLE (default n)
	noReset     bool        // R4.3: -p flag
	joinBlank   int         // R4.4: -l N (default 1)
}

// nlState tracks numbering state across lines.
type nlState struct {
	lineNum    int
	section    sectionKind
	emptyCount int // consecutive empty lines for -l
}

// defaultConfig returns the default nl configuration.
func defaultConfig() config {
	return config{
		numFormat:   "rn",
		width:       6,
		sep:         "\t",
		startVal:    1,
		increment:   1,
		bodyStyle:   numberStyle{kind: 't'},
		headerStyle: numberStyle{kind: 'n'},
		footerStyle: numberStyle{kind: 'n'},
		joinBlank:   1,
	}
}

// parseStyle parses a numbering style string into a numberStyle.
// R2.1: a (all), t (non-empty), n (none), pRE (regex match).
func parseStyle(s string) (numberStyle, error) {
	switch {
	case s == "a":
		return numberStyle{kind: 'a'}, nil
	case s == "t":
		return numberStyle{kind: 't'}, nil
	case s == "n":
		return numberStyle{kind: 'n'}, nil
	case strings.HasPrefix(s, "p"):
		re, err := regexp.Compile(s[1:])
		if err != nil {
			return numberStyle{}, fmt.Errorf("invalid regular expression: %s", s[1:])
		}
		return numberStyle{kind: 'p', regex: re}, nil
	default:
		return numberStyle{}, fmt.Errorf("invalid numbering style: %q", s)
	}
}

// parseFlags parses command-line flags and returns config and file arguments.
func parseFlags() (config, []string) {
	cfg := defaultConfig()
	fs := flag.NewFlagSet("nl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {}

	bodyStr := fs.String("b", "t", "body numbering style")
	headerStr := fs.String("h", "n", "header numbering style")
	footerStr := fs.String("f", "n", "footer numbering style")
	numFmt := fs.String("n", "rn", "line number format")
	width := fs.Int("w", 6, "line number field width")
	sep := fs.String("s", "\t", "separator between number and line")
	startVal := fs.Int("v", 1, "initial line number")
	increment := fs.Int("i", 1, "line number increment")
	noReset := fs.Bool("p", false, "do not reset line counter at logical pages")
	joinBlank := fs.Int("l", 1, "group N consecutive empty lines as one")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	if err := applyFormatFlags(&cfg, *numFmt, *width, *sep, *startVal, *increment); err != nil {
		fmt.Fprintf(os.Stderr, "nl: %s\n", err)
		os.Exit(1)
	}

	if err := applyStyleFlags(&cfg, *bodyStr, *headerStr, *footerStr); err != nil {
		fmt.Fprintf(os.Stderr, "nl: %s\n", err)
		os.Exit(1)
	}

	cfg.noReset = *noReset
	cfg.joinBlank = *joinBlank

	return cfg, fs.Args()
}

// applyFormatFlags validates and applies -n, -w, -s, -v, -i flag values to cfg.
func applyFormatFlags(cfg *config, numFmt string, width int, sep string, startVal, increment int) error {
	if numFmt != "ln" && numFmt != "rn" && numFmt != "rz" {
		return fmt.Errorf("invalid line numbering format: %q (must be ln, rn, or rz)", numFmt)
	}
	cfg.numFormat = numFmt
	cfg.width = width
	cfg.sep = sep
	cfg.startVal = startVal
	cfg.increment = increment
	return nil
}

// applyStyleFlags parses and applies the -b, -h, -f style flag values to cfg.
func applyStyleFlags(cfg *config, body, header, footer string) error {
	var err error
	if cfg.bodyStyle, err = parseStyle(body); err != nil {
		return fmt.Errorf("invalid body numbering style: %s", body)
	}
	if cfg.headerStyle, err = parseStyle(header); err != nil {
		return fmt.Errorf("invalid header numbering style: %s", header)
	}
	if cfg.footerStyle, err = parseStyle(footer); err != nil {
		return fmt.Errorf("invalid footer numbering style: %s", footer)
	}
	return nil
}

// openInput returns os.Stdin for "-", otherwise opens the named file.
func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, formatOpenError(name, err)
	}
	return f, nil
}

// formatOpenError extracts the underlying error for GNU-compatible messages.
func formatOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// isEmptyLine reports whether line contains only a newline.
func isEmptyLine(line string) bool {
	return line == "\n"
}

// shouldNumber reports whether a non-empty line should be numbered given the style.
// R2.1: a=all, t=non-empty, n=none, p=regex match.
func shouldNumber(line string, style numberStyle) bool {
	switch style.kind {
	case 'a':
		return true
	case 't':
		return !isEmptyLine(line)
	case 'n':
		return false
	case 'p':
		content := strings.TrimSuffix(line, "\n")
		return style.regex.MatchString(content)
	}
	return false
}

// formatLineNumber formats a line number according to the configured format.
// R3.1: ln=left-justified, rn=right-justified, rz=right-justified with leading zeros.
func formatLineNumber(lineNum, width int, numFormat string) string {
	switch numFormat {
	case "ln":
		return fmt.Sprintf("%-*d", width, lineNum)
	case "rz":
		return fmt.Sprintf("%0*d", width, lineNum)
	default:
		return fmt.Sprintf("%*d", width, lineNum)
	}
}

// writeNumberedLine writes a line with its line number prefix.
func writeNumberedLine(w *bufio.Writer, line string, cfg config, lineNum int) error {
	numStr := formatLineNumber(lineNum, cfg.width, cfg.numFormat)
	_, err := fmt.Fprintf(w, "%s%s%s", numStr, cfg.sep, line)
	return err
}

// writeUnnumberedLine writes a line with blank padding instead of a number.
// R2.4: unnumbered lines pass through with padding matching the number field.
func writeUnnumberedLine(w *bufio.Writer, line string, cfg config) error {
	padding := strings.Repeat(" ", cfg.width+len(cfg.sep))
	_, err := fmt.Fprintf(w, "%s%s", padding, line)
	return err
}

// detectDelimiter checks if a line is a section delimiter.
// R4.1: \:\:\: = header, \:\: = body, \: = footer.
func detectDelimiter(line string) (sectionKind, bool) {
	content := strings.TrimSuffix(line, "\n")
	switch content {
	case delimiter + delimiter + delimiter:
		return sectionHeader, true
	case delimiter + delimiter:
		return sectionBody, true
	case delimiter:
		return sectionFooter, true
	}
	return 0, false
}

// sectionStyle returns the numbering style for the given section.
func sectionStyle(cfg config, section sectionKind) numberStyle {
	switch section {
	case sectionHeader:
		return cfg.headerStyle
	case sectionFooter:
		return cfg.footerStyle
	default:
		return cfg.bodyStyle
	}
}

// numberLines reads lines from r and writes numbered output to w.
func numberLines(r io.Reader, cfg config, state nlState, w *bufio.Writer) (nlState, error) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if !strings.HasSuffix(line, "\n") {
				line += "\n"
			}
			var writeErr error
			state, writeErr = processLine(w, line, cfg, state)
			if writeErr != nil {
				return state, writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return state, err
		}
	}
	return state, nil
}

// processLine handles one line: delimiter detection, numbering, or padding.
// R4.1: delimiter lines are replaced with a bare newline in output.
func processLine(w *bufio.Writer, line string, cfg config, state nlState) (nlState, error) {
	if newSection, ok := detectDelimiter(line); ok {
		return handleDelimiter(w, cfg, state, newSection)
	}
	style := sectionStyle(cfg, state.section)
	if isEmptyLine(line) {
		return processEmptyLine(w, line, cfg, style, state)
	}
	return processNonEmptyLine(w, line, cfg, style, state)
}

// handleDelimiter updates state for a section delimiter line.
// R4.1: delimiter lines are replaced with a bare newline.
// R4.2: section delimiters reset counter unless -p (R4.3).
func handleDelimiter(w *bufio.Writer, cfg config, state nlState, newSection sectionKind) (nlState, error) {
	_, err := w.WriteString("\n")
	if !cfg.noReset {
		state.lineNum = cfg.startVal
	}
	state.section = newSection
	state.emptyCount = 0
	return state, err
}

// processNonEmptyLine handles numbering for a non-empty content line.
func processNonEmptyLine(w *bufio.Writer, line string, cfg config, style numberStyle, state nlState) (nlState, error) {
	state.emptyCount = 0
	if shouldNumber(line, style) {
		err := writeNumberedLine(w, line, cfg, state.lineNum)
		if err == nil {
			state.lineNum += cfg.increment
		}
		return state, err
	}
	return state, writeUnnumberedLine(w, line, cfg)
}

// shouldNumberEmpty reports whether a blank line should be numbered
// based on the section style and the join-blank counter.
// R4.4: with joinBlank > 1, number every Nth consecutive empty line (style 'a' only).
func shouldNumberEmpty(cfg config, style numberStyle, emptyCount int) bool {
	if style.kind != 'a' {
		return false
	}
	if cfg.joinBlank > 1 {
		return emptyCount >= cfg.joinBlank
	}
	return true
}

// processEmptyLine handles blank line numbering with -l join-blank logic.
// R4.4: -l N groups N consecutive empty lines for numbering with style 'a'.
func processEmptyLine(w *bufio.Writer, line string, cfg config, style numberStyle, state nlState) (nlState, error) {
	state.emptyCount++
	if shouldNumberEmpty(cfg, style, state.emptyCount) {
		state.emptyCount = 0
		err := writeNumberedLine(w, line, cfg, state.lineNum)
		if err == nil {
			state.lineNum += cfg.increment
		}
		return state, err
	}
	return state, writeUnnumberedLine(w, line, cfg)
}

// nlFile reads the named file and writes numbered lines to w.
// R1.3: stdin for "-", otherwise open named file.
// R1.4: state carries across files for continuous numbering.
func nlFile(name string, cfg config, state nlState, w *bufio.Writer) (nlState, error) {
	r, err := openInput(name)
	if err != nil {
		return state, err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return numberLines(r, cfg, state, w)
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args := parseFlags()

	// R1.3: no arguments means read stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0
	state := nlState{lineNum: cfg.startVal, section: sectionBody}

	// R1.4: continuous numbering across files.
	for _, name := range args {
		var err error
		state, err = nlFile(name, cfg, state, w)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: %s\n", err)
			exitCode = 1
		}
	}

	// best-effort flush; SIGPIPE handler covers broken pipe
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "nl: write error\n")
		exitCode = 1
	}

	os.Exit(exitCode)
}
