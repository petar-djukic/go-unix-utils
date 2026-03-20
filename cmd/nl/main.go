// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd022-nl R1.1–R1.4, R2.1–R2.4: line numbering with section styles.
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
	width       int
	sep         string
	delim       string
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
	lineNum := 1
	curSection := sectionBody
	exitCode := 0
	for _, name := range cfg.files {
		var procErr error
		lineNum, curSection, procErr = processFile(name, cfg, lineNum, curSection, stdin, w)
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
		width:       defaultWidth,
		sep:         defaultSep,
		delim:       defaultDelim,
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

// applyFlag applies a parsed flag value to the config. R2.1–R2.3.
func applyFlag(cfg *config, ch byte, value string) error {
	target := styleTarget(cfg, ch)
	if target == nil {
		return nil // flags not yet implemented (d, i, l, n, s, v, w)
	}
	style, err := parseStyle(value)
	if err != nil {
		return err
	}
	*target = style
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
func processFile(name string, cfg *config, lineNum int, curSection section, stdin io.Reader, w *bufio.Writer) (int, section, error) {
	r, closer, err := openInput(name, stdin)
	if err != nil {
		return lineNum, curSection, err
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	return numberLines(r, cfg, lineNum, curSection, w)
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
func numberLines(r io.Reader, cfg *config, lineNum int, curSection section, w *bufio.Writer) (int, section, error) {
	br := bufio.NewReader(r)
	padding := strings.Repeat(" ", cfg.width+len(cfg.sep))
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			content := strings.TrimSuffix(line, "\n")
			var writeErr error
			lineNum, curSection, writeErr = processLine(w, cfg, content, lineNum, curSection, padding)
			if writeErr != nil {
				return lineNum, curSection, writeErr
			}
		}
		if err != nil {
			if err != io.EOF {
				return lineNum, curSection, err
			}
			break
		}
	}
	return lineNum, curSection, nil
}

// processLine handles a single line: detects section delimiters or writes content.
func processLine(w *bufio.Writer, cfg *config, content string, lineNum int, curSection section, padding string) (int, section, error) {
	newSec, isDelim := detectSection(content, cfg.delim)
	if isDelim {
		lineNum = 1 // R4.2: reset counter at section boundaries
		curSection = newSec
		_, err := fmt.Fprint(w, "\n")
		return lineNum, curSection, err
	}
	n, err := writeLine(w, cfg, curSection, content, lineNum, padding)
	return n, curSection, err
}

// writeLine writes a single output line, numbered or unnumbered. R2.4.
func writeLine(w *bufio.Writer, cfg *config, sec section, content string, lineNum int, padding string) (int, error) {
	style := styleForSection(cfg, sec)
	if shouldNumber(style, content) {
		_, err := fmt.Fprintf(w, "%*d%s%s\n", cfg.width, lineNum, cfg.sep, content)
		return lineNum + 1, err
	}
	_, err := fmt.Fprintf(w, "%s%s\n", padding, content)
	return lineNum, err
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
