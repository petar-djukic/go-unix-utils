// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/nl implements GNU nl: number lines of files.
//
// Implements prd022-nl: R1 (default line numbering), R2 (numbering style flags),
// R3 (format and numbering options), R4 (section delimiters and page reset).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// R2.1: -b STYLE sets the body section numbering style (default t).
var bodyStyle = flag.String("b", "t", "body numbering style: a, t, n, or pRE")

// R2.2: -h STYLE sets the header section numbering style (default n).
var headerStyle = flag.String("h", "n", "header numbering style: a, t, n, or pRE")

// R2.3: -f STYLE sets the footer section numbering style (default n).
var footerStyle = flag.String("f", "n", "footer numbering style: a, t, n, or pRE")

// R4.1: -d CC sets the section delimiter characters (default \:).
var sectionDelimiter = flag.String("d", `\:`, "section delimiter characters")

// R3.4: -i N sets the line number increment (default 1).
var increment = flag.Int("i", 1, "line number increment")

// R4.4: -l N sets the number of consecutive blank lines to treat as one (default 1).
var joinBlankLines = flag.Int("l", 1, "number of consecutive empty lines to treat as one")

// R3.1: -n FORMAT sets the line number format (default rn).
var numberFormat = flag.String("n", "rn", "line number format: ln, rn, or rz")

// R4.3: -p suppresses line counter reset at logical page boundaries.
var noReset = flag.Bool("p", false, "do not reset line numbers at logical pages")

// R3.3: -s SEP sets the separator between number and line content (default tab).
var separator = flag.String("s", "\t", "separator between number and text")

// R3.4: -v N sets the starting line number (default 1).
var startingLineNumber = flag.Int("v", 1, "first line number on each logical page")

// R3.2: -w N sets the line number field width (default 6).
var numberWidth = flag.Int("w", 6, "line number field width")

func main() {
	// R3.4 (prd002-sys): handle SIGPIPE gracefully.
	sys.InstallSIGPIPEHandler()

	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := run(files)
	os.Exit(exitCode)
}

// sectionType represents which logical page section is active.
type sectionType int

const (
	sectionBody   sectionType = iota
	sectionHeader
	sectionFooter
)

// numberingStyle holds a parsed numbering style for a section.
type numberingStyle struct {
	mode  byte           // 'a', 't', 'n', or 'p'
	regex *regexp.Regexp // non-nil when mode == 'p'
}

// parseStyle parses a numbering style string per R2.1.
func parseStyle(s string) (numberingStyle, error) {
	switch {
	case s == "a":
		return numberingStyle{mode: 'a'}, nil
	case s == "t":
		return numberingStyle{mode: 't'}, nil
	case s == "n":
		return numberingStyle{mode: 'n'}, nil
	case len(s) > 1 && s[0] == 'p':
		re, err := regexp.Compile(s[1:])
		if err != nil {
			return numberingStyle{}, fmt.Errorf("invalid regex in style %q: %w", s, err)
		}
		return numberingStyle{mode: 'p', regex: re}, nil
	default:
		return numberingStyle{}, fmt.Errorf("invalid numbering style: %q", s)
	}
}

// shouldNumber returns true if the given line should receive a line number.
func (ns numberingStyle) shouldNumber(line string) bool {
	switch ns.mode {
	case 'a':
		return true
	case 't':
		return line != ""
	case 'n':
		return false
	case 'p':
		return ns.regex.MatchString(line)
	}
	return false
}

// formatLineNumber formats a line number per R3.1.
func formatLineNumber(num int, width int, format string) string {
	switch format {
	case "ln":
		return fmt.Sprintf("%-*d", width, num)
	case "rz":
		return fmt.Sprintf("%0*d", width, num)
	default: // "rn"
		return fmt.Sprintf("%*d", width, num)
	}
}

// run processes all files and returns the exit code.
// R1.3: reads stdin when filename is "-".
// R1.4: numbering is continuous across files.
func run(files []string) int {
	body, err := parseStyle(*bodyStyle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: %v\n", err)
		return 1
	}
	header, err := parseStyle(*headerStyle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: %v\n", err)
		return 1
	}
	footer, err := parseStyle(*footerStyle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nl: %v\n", err)
		return 1
	}

	styles := map[sectionType]numberingStyle{
		sectionBody:   body,
		sectionHeader: header,
		sectionFooter: footer,
	}

	lineNum := *startingLineNumber
	blankRun := 0
	currentSection := sectionBody
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)

	for _, name := range files {
		err := processFile(name, styles, &lineNum, &blankRun,
			&currentSection, w)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nl: %v\n", err)
			exitCode = 1
		}
	}
	// best-effort flush
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "nl: write error: %v\n", err)
		exitCode = 1
	}
	return exitCode
}

// processFile reads one file and applies numbering.
func processFile(name string, styles map[sectionType]numberingStyle,
	lineNum *int, blankRun *int, section *sectionType,
	w *bufio.Writer) error {

	r, err := openInput(name)
	if err != nil {
		return err
	}
	defer r.Close()

	return processLines(r, styles, lineNum, blankRun, section, w)
}

// openInput opens the named file or returns stdin for "-".
func openInput(name string) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(name)
}

// delimiterPatterns returns the three section delimiter strings
// derived from the -d flag value.
// R4.1: \:\:\: = header, \:\: = body, \: = footer.
func delimiterPatterns(delim string) (string, string, string) {
	return delim + delim + delim,
		delim + delim,
		delim
}

// processLines reads lines from r and writes numbered output.
func processLines(r io.Reader, styles map[sectionType]numberingStyle,
	lineNum *int, blankRun *int, section *sectionType,
	w *bufio.Writer) error {

	delimHeader, delimBody, delimFooter := delimiterPatterns(*sectionDelimiter)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		if handled := handleDelimiter(line, delimHeader, delimBody,
			delimFooter, lineNum, blankRun, section); handled {
			fmt.Fprintln(w)
			continue
		}

		if err := writeLine(w, line, styles[*section],
			lineNum, blankRun); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// handleDelimiter checks if a line is a section delimiter and
// updates state accordingly. Returns true if the line was a delimiter.
func handleDelimiter(line, delimHeader, delimBody, delimFooter string,
	lineNum *int, blankRun *int, section *sectionType) bool {

	switch line {
	case delimHeader:
		*section = sectionHeader
		*blankRun = 0
		if !*noReset {
			*lineNum = *startingLineNumber
		}
		return true
	case delimBody:
		*section = sectionBody
		*blankRun = 0
		return true
	case delimFooter:
		*section = sectionFooter
		*blankRun = 0
		return true
	}
	return false
}

// writeLine writes a single line with or without a line number.
func writeLine(w *bufio.Writer, line string,
	style numberingStyle, lineNum *int, blankRun *int) error {

	isEmpty := line == ""
	if isEmpty {
		*blankRun++
	} else {
		*blankRun = 0
	}

	if shouldNumberLine(style, line, isEmpty, *blankRun) {
		num := formatLineNumber(*lineNum, *numberWidth, *numberFormat)
		fmt.Fprintf(w, "%s%s%s\n", num, *separator, line)
		*lineNum += *increment
	} else {
		writeUnnumberedLine(w, line)
	}
	return nil
}

// shouldNumberLine determines if the current line should be numbered.
func shouldNumberLine(style numberingStyle, line string,
	isEmpty bool, blankRun int) bool {

	if style.mode == 'n' {
		return false
	}
	if style.mode == 't' && isEmpty {
		// R4.4: -l N treats N consecutive blank lines as one.
		return blankRun > 0 && *joinBlankLines > 1 &&
			blankRun%*joinBlankLines == 0
	}
	return style.shouldNumber(line)
}

// writeUnnumberedLine writes a line with no number.
// R2.4: unnumbered lines have no number and no separator.
func writeUnnumberedLine(w *bufio.Writer, line string) {
	padding := strings.Repeat(" ", *numberWidth+len(*separator))
	fmt.Fprintf(w, "%s%s\n", padding, line)
}
