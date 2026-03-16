// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl R1.1-R1.4, R2.1-R2.4: cmd/nl numbers lines from stdin
// or named files. Supports per-section numbering styles via -b (body), -h
// (header), and -f (footer) flags with styles a (all), t (non-empty, default
// for body), n (none), and pRE (regex match). When style is n, lines pass
// through with no number and no separator. Installs SIGPIPE handler for clean
// exit on broken pipe.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU nl format.
const progName = "nl"

// defaultWidth is the line number field width (R1.1).
const defaultWidth = 6

// defaultSep is the separator between line number and content (R1.1).
const defaultSep = "\t"

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

func main() {
	sys.InstallSIGPIPEHandler()

	cfg := nlConfig{
		bodyStyle:   sectionStyle{style: styleT}, // R2.1: default body style is t.
		headerStyle: sectionStyle{style: styleN}, // R2.2: default header style is n.
		footerStyle: sectionStyle{style: styleN}, // R2.3: default footer style is n.
	}

	// Parse flags manually to match GNU nl's flag syntax (e.g., -ba, -b a, -bt, -pRE).
	args := os.Args[1:]
	var files []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			continue
		}

		// Handle -b, -h, -f flags.
		var flag string
		var val string
		var handled bool

		for _, f := range []string{"b", "h", "f"} {
			if arg == "-"+f {
				// Value is the next argument.
				flag = f
				if i+1 >= len(args) {
					fmt.Fprintf(os.Stderr, "%s: option requires an argument -- '%s'\n", progName, f)
					os.Exit(1)
				}
				i++
				val = args[i]
				handled = true
				break
			}
			if strings.HasPrefix(arg, "-"+f) {
				// Value is concatenated: -ba, -bt, -bn, -bpRE.
				flag = f
				val = arg[2:]
				handled = true
				break
			}
		}

		if !handled {
			// Unknown flag — pass through to match GNU behavior (GNU nl will
			// report its own error). For now, report and exit.
			fmt.Fprintf(os.Stderr, "%s: invalid option -- '%s'\n", progName, arg[1:])
			os.Exit(1)
		}

		style, err := parseStyle(flag, val)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			os.Exit(1)
		}
		switch flag {
		case "b":
			cfg.bodyStyle = style
		case "h":
			cfg.headerStyle = style
		case "f":
			cfg.footerStyle = style
		}
	}

	exitCode := 0
	lineNum := 1

	// Compute padding for unnumbered lines.
	emptyPadding := strings.Repeat(" ", defaultWidth+len(defaultSep))

	if len(files) == 0 {
		// R1.3: no file arguments — read from stdin.
		var err error
		lineNum, err = nlReader(os.Stdin, lineNum, cfg.bodyStyle, emptyPadding)
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
			lineNum, err = nlReader(os.Stdin, lineNum, cfg.bodyStyle, emptyPadding)
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
		lineNum, err = nlReader(f, lineNum, cfg.bodyStyle, emptyPadding)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
		}
		f.Close() // best-effort close; read errors already reported
	}

	os.Exit(exitCode)
}

// nlReader reads lines from r, numbering lines according to the given style
// starting at lineNum. Returns the next line number to use and any error.
//
// R1.1: non-empty lines are numbered with right-justified width 6, tab separator.
// R1.2: empty lines are output with whitespace padding but no number (under style t).
// R2.1-R2.4: style determines which lines are numbered.
func nlReader(r io.Reader, lineNum int, style sectionStyle, emptyPadding string) (int, error) {
	w := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if style.shouldNumber(line) {
			// Number this line.
			if _, err := fmt.Fprintf(w, "%*d%s%s\n", defaultWidth, lineNum, defaultSep, line); err != nil {
				return lineNum, err
			}
			lineNum++
		} else {
			// R2.4: unnumbered line — output with padding, no number or separator.
			if _, err := fmt.Fprintf(w, "%s%s\n", emptyPadding, line); err != nil {
				return lineNum, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return lineNum, err
	}

	if err := w.Flush(); err != nil {
		return lineNum, err
	}

	return lineNum, nil
}

// unwrapPathError extracts the inner error from an *os.PathError to produce
// messages like "No such file or directory" instead of "open foo: no such ...".
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
