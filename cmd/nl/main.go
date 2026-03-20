// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4.
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

const (
	progName     = "nl"
	defaultWidth = 6
	defaultSep   = "\t"
	defaultDelim = `\:`
)

// section represents a logical page section.
type section int

const (
	sectionBody   section = iota
	sectionHeader
	sectionFooter
)

// numberFormat represents the line number format (R3.1).
type numberFormat int

const (
	formatRN numberFormat = iota // right-justified, no leading zeros (default)
	formatLN                     // left-justified, no leading zeros
	formatRZ                     // right-justified, leading zeros
)

// numberStyle represents a line numbering style (R2.1).
type numberStyle struct {
	mode  byte           // 'a', 't', 'n', 'p'
	regex *regexp.Regexp // non-nil when mode == 'p'
}

// config holds parsed nl options.
type config struct {
	bodyStyle   numberStyle
	headerStyle numberStyle
	footerStyle numberStyle
	format      numberFormat // R3.1
	width       int          // R3.2
	sep         string       // R3.3
	startNum    int          // R3.4: -v
	increment   int          // R3.4: -i
	delim       string
	noReset     bool // R4.3: -p suppresses page reset
	blankJoin   int  // R4.4: -l N consecutive empty lines as one
	files       []string
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses arguments and processes files, returning the exit code.
// R1.3: reads stdin when no file args given. R1.4: continuous numbering.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", progName, err)
		return 1
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return processAll(cfg, stdin, stdout, stderr)
}

// processAll processes all files with continuous numbering.
func processAll(cfg *config, stdin io.Reader, stdout, stderr io.Writer) int {
	w := bufio.NewWriter(stdout)
	lineNum := cfg.startNum // R3.4: start from -v value
	curSection := sectionBody
	blankCount := 0
	exitCode := 0
	for _, name := range cfg.files {
		var procErr error
		lineNum, curSection, blankCount, procErr = processFile(
			name, cfg, lineNum, curSection, blankCount, stdin, w,
		)
		if procErr != nil {
			fmt.Fprintf(stderr, "%s: %s: %s\n", progName, name, unwrapPathError(procErr))
			exitCode = 1
		}
	}
	if err := w.Flush(); err != nil {
		return 1
	}
	return exitCode
}

// defaultConfig returns the initial configuration with default values.
func defaultConfig() *config {
	return &config{
		bodyStyle:   numberStyle{mode: 't'},
		headerStyle: numberStyle{mode: 'n'},
		footerStyle: numberStyle{mode: 'n'},
		format:      formatRN,
		width:       defaultWidth,
		sep:         defaultSep,
		startNum:    1,
		increment:   1,
		delim:       defaultDelim,
		blankJoin:   1, // R4.4: default, each empty line counted separately
	}
}

// parseArgs parses command-line arguments into a config.
func parseArgs(args []string) (*config, error) {
	cfg := defaultConfig()
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if arg == "-" || len(arg) == 0 || arg[0] != '-' {
			cfg.files = append(cfg.files, arg)
			i++
			continue
		}
		consumed, err := scanFlags(cfg, arg[1:], args, i)
		if err != nil {
			return nil, err
		}
		i += consumed
	}
	return cfg, nil
}

// flagsWithArg lists single-character nl flags that consume an argument.
const flagsWithArg = "bdfhilnsvw"

// scanFlags processes flag characters in a single argument (after '-').
func scanFlags(cfg *config, chars string, args []string, idx int) (int, error) {
	consumed := 1
	j := 0
	for j < len(chars) {
		ch := chars[j]
		if !isFlagWithArg(ch) {
			// R4.3: -p is a boolean flag (no argument)
			if ch == 'p' {
				cfg.noReset = true
			}
			j++
			continue
		}
		value, extra, err := extractValue(chars[j+1:], args, idx+consumed, ch)
		if err != nil {
			return consumed, err
		}
		consumed += extra
		if err := applyFlag(cfg, ch, value); err != nil {
			return consumed, err
		}
		return consumed, nil
	}
	return consumed, nil
}

// isFlagWithArg returns true if the flag character takes an argument.
func isFlagWithArg(ch byte) bool {
	return strings.IndexByte(flagsWithArg, ch) >= 0
}

// extractValue gets the flag's argument value from remaining chars or next arg.
func extractValue(rest string, args []string, nextIdx int, ch byte) (string, int, error) {
	if rest != "" {
		return rest, 0, nil
	}
	if nextIdx < len(args) {
		return args[nextIdx], 1, nil
	}
	return "", 0, fmt.Errorf("option requires an argument -- '%c'", ch)
}

// applyFlag applies a parsed flag value to the config. R2.1–R2.3, R3.1–R3.4.
func applyFlag(cfg *config, ch byte, value string) error {
	target := styleTarget(cfg, ch)
	if target != nil {
		style, err := parseStyle(value)
		if err != nil {
			return err
		}
		*target = style
		return nil
	}
	return applyNonStyleFlag(cfg, ch, value)
}

// applyNonStyleFlag handles flags that are not style flags (R3.1–R3.4, R4.4).
func applyNonStyleFlag(cfg *config, ch byte, value string) error {
	switch ch {
	case 'n':
		return applyFormatFlag(cfg, value)
	case 'w':
		return applyIntFlag(&cfg.width, value, "width")
	case 's':
		cfg.sep = value
		return nil
	case 'v':
		return applyIntFlag(&cfg.startNum, value, "starting line number")
	case 'i':
		return applyIntFlag(&cfg.increment, value, "line number increment")
	case 'd':
		cfg.delim = value
		return nil
	case 'l':
		// R4.4: -l N consecutive empty lines treated as one
		return applyIntFlag(&cfg.blankJoin, value, "number of blank lines")
	}
	return nil
}

// applyFormatFlag parses and sets the -n FORMAT flag (R3.1).
func applyFormatFlag(cfg *config, value string) error {
	switch value {
	case "ln":
		cfg.format = formatLN
	case "rn":
		cfg.format = formatRN
	case "rz":
		cfg.format = formatRZ
	default:
		return fmt.Errorf("invalid line number format: %q", value)
	}
	return nil
}

// applyIntFlag parses an integer value and sets the target field.
func applyIntFlag(target *int, value, name string) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %q", name, value)
	}
	*target = n
	return nil
}

// styleTarget returns a pointer to the style field for the given flag.
func styleTarget(cfg *config, ch byte) *numberStyle {
	switch ch {
	case 'b':
		return &cfg.bodyStyle
	case 'h':
		return &cfg.headerStyle
	case 'f':
		return &cfg.footerStyle
	}
	return nil
}

// parseStyle parses a numbering style string: a, t, n, or pRE. R2.1.
func parseStyle(s string) (numberStyle, error) {
	if len(s) == 0 {
		return numberStyle{}, fmt.Errorf("invalid numbering style: ''")
	}
	switch s[0] {
	case 'a':
		return numberStyle{mode: 'a'}, nil
	case 't':
		return numberStyle{mode: 't'}, nil
	case 'n':
		return numberStyle{mode: 'n'}, nil
	case 'p':
		re, err := regexp.Compile(s[1:])
		if err != nil {
			return numberStyle{}, err
		}
		return numberStyle{mode: 'p', regex: re}, nil
	}
	return numberStyle{}, fmt.Errorf("invalid numbering style: %q", s)
}

// processFile reads a single file and numbers its lines.
func processFile(name string, cfg *config, lineNum int, curSection section, blankCount int, stdin io.Reader, w *bufio.Writer) (int, section, int, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return lineNum, curSection, blankCount, err
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	return numberLines(r, cfg, lineNum, curSection, blankCount, w)
}

// openInput returns a reader for the named file or stdin for "-".
func openInput(name string, stdin io.Reader) (io.Reader, io.Closer, error) {
	if name == "-" {
		return stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// numberLines reads lines and writes them with section-aware numbering.
func numberLines(r io.Reader, cfg *config, lineNum int, curSection section, blankCount int, w *bufio.Writer) (int, section, int, error) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			content := strings.TrimSuffix(line, "\n")
			var writeErr error
			lineNum, curSection, blankCount, writeErr = processLine(
				w, cfg, content, lineNum, curSection, blankCount,
			)
			if writeErr != nil {
				return lineNum, curSection, blankCount, writeErr
			}
		}
		if err != nil {
			if err != io.EOF {
				return lineNum, curSection, blankCount, err
			}
			break
		}
	}
	return lineNum, curSection, blankCount, nil
}

// processLine handles a single line: detects section delimiters or writes content.
// R4.1: delimiter lines replaced with empty output. R4.2: header resets counter.
// R4.3: -p suppresses reset.
func processLine(w *bufio.Writer, cfg *config, content string, lineNum int, curSection section, blankCount int) (int, section, int, error) {
	newSec, isDelim := detectSection(content, cfg.delim)
	if isDelim {
		// R4.2: delimiter resets counter to -v; R4.3: -p suppresses reset
		if !cfg.noReset {
			lineNum = cfg.startNum
		}
		curSection = newSec
		blankCount = 0
		_, err := fmt.Fprint(w, "\n")
		return lineNum, curSection, blankCount, err
	}
	n, bc, err := writeLine(w, cfg, curSection, content, lineNum, blankCount)
	return n, curSection, bc, err
}

// writeLine writes a single output line, numbered or unnumbered.
// R2.4, R3.1–R3.4, R4.4.
func writeLine(w *bufio.Writer, cfg *config, sec section, content string, lineNum int, blankCount int) (int, int, error) {
	style := styleForSection(cfg, sec)
	isEmpty := content == ""
	if isEmpty {
		blankCount++
	} else {
		blankCount = 0
	}
	doNumber := shouldNumber(style, content)
	// R4.4: blank join — suppress numbering for empty lines within a group
	if doNumber && isEmpty && blankCount < cfg.blankJoin {
		doNumber = false
	}
	if doNumber && isEmpty {
		blankCount = 0 // reset after numbering the N-th blank
	}
	return emitLine(w, cfg, content, lineNum, blankCount, doNumber)
}

// emitLine writes the formatted line to the writer.
func emitLine(w *bufio.Writer, cfg *config, content string, lineNum int, blankCount int, doNumber bool) (int, int, error) {
	if doNumber {
		numStr := formatNumber(lineNum, cfg.format, cfg.width)
		_, err := fmt.Fprintf(w, "%s%s%s\n", numStr, cfg.sep, content)
		return lineNum + cfg.increment, blankCount, err
	}
	padding := strings.Repeat(" ", cfg.width+len(cfg.sep))
	_, err := fmt.Fprintf(w, "%s%s\n", padding, content)
	return lineNum, blankCount, err
}

// formatNumber formats a line number according to the format and width. R3.1.
func formatNumber(n int, f numberFormat, width int) string {
	switch f {
	case formatLN:
		return fmt.Sprintf("%-*d", width, n)
	case formatRZ:
		return fmt.Sprintf("%0*d", width, n)
	default:
		return fmt.Sprintf("%*d", width, n)
	}
}

// styleForSection returns the numbering style for the given section. R2.1–R2.3.
func styleForSection(cfg *config, sec section) numberStyle {
	switch sec {
	case sectionHeader:
		return cfg.headerStyle
	case sectionFooter:
		return cfg.footerStyle
	default:
		return cfg.bodyStyle
	}
}

// shouldNumber returns true if the line should receive a number. R2.1, R2.4.
func shouldNumber(style numberStyle, content string) bool {
	switch style.mode {
	case 'a':
		return true
	case 't':
		return content != ""
	case 'n':
		return false
	case 'p':
		return style.regex != nil && style.regex.MatchString(content)
	}
	return false
}

// detectSection checks if a line is a section delimiter.
// R4.1: \:\:\: = header, \:\: = body, \: = footer.
func detectSection(content string, delim string) (section, bool) {
	hdr := delim + delim + delim
	bod := delim + delim
	if content == hdr {
		return sectionHeader, true
	}
	if content == bod {
		return sectionBody, true
	}
	if content == delim {
		return sectionFooter, true
	}
	return sectionBody, false
}

// unwrapPathError extracts the inner error from *os.PathError.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
