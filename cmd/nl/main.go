// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nl: number lines of files.
// Implements srd022-nl R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4.
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
}

// defaultConfig returns the default nl configuration.
// R1.1: width 6, tab separator, start at 1, increment by 1.
// R2.1: body defaults to t. R2.2: header defaults to n. R2.3: footer defaults to n.
// R3.1: default format is rn (right-justified, no leading zeros).
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
	// R3.1: -n FORMAT (ln, rn, rz)
	numFmt := fs.String("n", "rn", "line number format")
	// R3.2: -w N
	width := fs.Int("w", 6, "line number field width")
	// R3.3: -s SEP
	sep := fs.String("s", "\t", "separator between number and line")
	// R3.4: -v N and -i N
	startVal := fs.Int("v", 1, "initial line number")
	increment := fs.Int("i", 1, "line number increment")

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

	return cfg, fs.Args()
}

// applyFormatFlags validates and applies -n, -w, -s, -v, -i flag values to cfg.
// R3.1: numFmt must be ln, rn, or rz. R3.2: width. R3.3: sep. R3.4: startVal, increment.
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
// R1.3: stdin when filename is "-".
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

// formatOpenError extracts the underlying error to produce GNU-compatible
// error messages: "<name>: <reason>".
func formatOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %s", name, pe.Err)
	}
	return fmt.Errorf("%s: %s", name, err)
}

// emptyPrefix returns spaces matching the width of a numbered line prefix.
// GNU nl pads unnumbered lines with spaces equal to width + len(sep).
func emptyPrefix(cfg config) string {
	return strings.Repeat(" ", cfg.width+len(cfg.sep))
}

// isEmptyLine reports whether line contains only a newline.
func isEmptyLine(line string) bool {
	return line == "\n"
}

// shouldNumber reports whether a line should be numbered given the style.
// R2.1: a=all, t=non-empty, n=none, p=regex match. R2.4: n means no numbering.
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
// R3.1: format per -n. R3.2: width per -w. R3.3: separator per -s.
func writeNumberedLine(w *bufio.Writer, line string, cfg config, lineNum int) error {
	numStr := formatLineNumber(lineNum, cfg.width, cfg.numFormat)
	_, err := fmt.Fprintf(w, "%s%s%s", numStr, cfg.sep, line)
	return err
}

// writeUnnumberedLine writes a line with blank padding instead of a number.
// R1.2, R2.4: unnumbered lines pass through with padding matching the number field.
func writeUnnumberedLine(w *bufio.Writer, line, padding string) error {
	_, err := fmt.Fprintf(w, "%s%s", padding, line)
	return err
}

// numberLines reads lines from r and writes numbered output to w.
// R1.1, R1.2, R2.1-R2.4: numbers lines according to the body style.
// Returns the next line number for continuous numbering across files.
func numberLines(r io.Reader, cfg config, lineNum int, w *bufio.Writer) (int, error) {
	br := bufio.NewReader(r)
	padding := emptyPrefix(cfg)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			// GNU nl appends a newline to the last line if missing.
			if !strings.HasSuffix(line, "\n") {
				line += "\n"
			}
			if writeErr := processLine(w, line, cfg, padding, &lineNum); writeErr != nil {
				return lineNum, writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return lineNum, err
		}
	}
	return lineNum, nil
}

// processLine decides whether to number or pad a line and writes it.
// Uses bodyStyle since section delimiters are not yet implemented (R4).
func processLine(w *bufio.Writer, line string, cfg config, padding string, lineNum *int) error {
	if shouldNumber(line, cfg.bodyStyle) {
		err := writeNumberedLine(w, line, cfg, *lineNum)
		if err == nil {
			*lineNum += cfg.increment
		}
		return err
	}
	return writeUnnumberedLine(w, line, padding)
}

// nlFile reads the named file and writes numbered lines to w.
// R1.3: stdin for "-", otherwise open named file.
// R1.4: lineNum carries across files for continuous numbering.
func nlFile(name string, cfg config, lineNum int, w *bufio.Writer) (int, error) {
	r, err := openInput(name)
	if err != nil {
		return lineNum, err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return numberLines(r, cfg, lineNum, w)
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
	lineNum := cfg.startVal

	// R1.4: continuous numbering across files.
	for _, name := range args {
		var err error
		lineNum, err = nlFile(name, cfg, lineNum, w)
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
