// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nl implements the nl (number lines) command.
// Implements: prd022-nl R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4
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

// R1.1: Default formatting constants per prd022.
const (
	defaultWidth     = 6
	defaultSeparator = "\t"
	defaultIncrement = 1
	defaultStartNum  = 1
)

// numberStyle represents how lines in a section are numbered.
// R2.1: Styles are a (all), t (non-empty), n (none), pRE (regex match).
type numberStyle int

const (
	styleNonEmpty numberStyle = iota // t: number non-empty lines (default for body)
	styleAll                         // a: number all lines
	styleNone                        // n: number no lines (default for header/footer)
	styleRegex                       // pRE: number lines matching regex
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
	files       []string
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

// parseArgs parses command-line arguments into an nlConfig.
// Handles GNU-style flags where the value can be attached (-ba) or separate (-b a).
func parseArgs(args []string) (nlConfig, error) {
	cfg := nlConfig{
		bodyStyle:   sectionStyle{style: styleNonEmpty}, // R2.1: default body style is t
		headerStyle: sectionStyle{style: styleNone},     // R2.2: default header style is n
		footerStyle: sectionStyle{style: styleNone},     // R2.3: default footer style is n
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

		// Try to match -b, -h, -f with attached or separate value.
		var flagChar byte
		var rest string
		if len(arg) >= 2 && (arg[1] == 'b' || arg[1] == 'h' || arg[1] == 'f') {
			flagChar = arg[1]
			rest = arg[2:]
		} else {
			// Unknown flag — pass as file to match GNU nl behavior (will error on open).
			cfg.files = append(cfg.files, arg)
			i++
			continue
		}

		var styleStr string
		if rest != "" {
			// Attached: -ba, -bt, -bn, -bpRE
			styleStr = rest
		} else {
			// Separate: -b a
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("option requires an argument -- '%c'", flagChar)
			}
			styleStr = args[i]
		}

		style, err := parseStyle(styleStr)
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
	lineNum := defaultStartNum

	if len(cfg.files) == 0 {
		// R1.3: No file arguments — read from stdin.
		if err := nlReader(os.Stdin, &lineNum, cfg.bodyStyle); err != nil {
			fmt.Fprintf(os.Stderr, "nl: %v\n", err)
			os.Exit(1)
		}
	} else {
		// R1.3, R1.4: Process each file in argument order with continuous numbering.
		for _, arg := range cfg.files {
			if err := nlFile(arg, &lineNum, cfg.bodyStyle); err != nil {
				fmt.Fprintf(os.Stderr, "nl: %v\n", err)
				exitCode = 1
			}
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// nlFile opens name and numbers its lines to stdout.
// R1.3: "-" reads from stdin.
func nlFile(name string, lineNum *int, bodyStyle sectionStyle) error {
	if name == "-" {
		return nlReader(os.Stdin, lineNum, bodyStyle)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close() // best-effort cleanup, error ignored
	return nlReader(f, lineNum, bodyStyle)
}

// nlReader reads lines from r and writes them to stdout with line numbering.
//
// R1.1: Non-empty lines are numbered with right-justified numbers in a field
// of width 6, separated from content by a tab.
// R1.2: Empty lines pass through with padding but no number.
// R1.4: lineNum is shared across files for continuous numbering.
// R2.1-R2.4: bodyStyle controls which lines get numbered.
func nlReader(r io.Reader, lineNum *int, bodyStyle sectionStyle) error {
	// R1.2, R2.4: Unnumbered lines get width + len(separator) spaces of padding.
	emptyPrefix := strings.Repeat(" ", defaultWidth+len(defaultSeparator))

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		var err error
		if shouldNumber(bodyStyle, line) {
			// R1.1, R2.1: Number this line per the active body style.
			_, err = fmt.Fprintf(os.Stdout, "%*d%s%s\n", defaultWidth, *lineNum, defaultSeparator, line)
			*lineNum += defaultIncrement
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
