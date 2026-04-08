// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/nl: number lines of files.
// Implements srd022-nl R1.1, R1.2, R1.3, R1.4.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// config holds nl command-line options.
type config struct {
	width     int
	sep       string
	startVal  int
	increment int
}

// defaultConfig returns the default nl configuration.
// R1.1: width 6, tab separator, start at 1, increment by 1.
func defaultConfig() config {
	return config{
		width:     6,
		sep:       "\t",
		startVal:  1,
		increment: 1,
	}
}

// parseFlags parses command-line flags and returns config and file arguments.
func parseFlags() (config, []string) {
	cfg := defaultConfig()
	fs := flag.NewFlagSet("nl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {}
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}
	return cfg, fs.Args()
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

// writeNumberedLine writes a line with its line number prefix.
// R1.1: right-justified number in field of width, separator, then content.
func writeNumberedLine(w *bufio.Writer, line string, cfg config, lineNum int) error {
	_, err := fmt.Fprintf(w, "%*d%s%s", cfg.width, lineNum, cfg.sep, line)
	return err
}

// writeUnnumberedLine writes a line with blank padding instead of a number.
// R1.2: empty lines pass through with padding matching the number field width.
func writeUnnumberedLine(w *bufio.Writer, line, padding string) error {
	_, err := fmt.Fprintf(w, "%s%s", padding, line)
	return err
}

// numberLines reads lines from r and writes numbered output to w.
// R1.1, R1.2: numbers non-empty lines, pads empty lines with spaces.
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
func processLine(w *bufio.Writer, line string, cfg config, padding string, lineNum *int) error {
	if isEmptyLine(line) {
		return writeUnnumberedLine(w, line, padding)
	}
	err := writeNumberedLine(w, line, cfg, *lineNum)
	if err == nil {
		*lineNum += cfg.increment
	}
	return err
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
