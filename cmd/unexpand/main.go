// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/unexpand converts spaces to tabs (prd025-unexpand R1).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultTabStop = 8
	programName    = "unexpand"
)

func main() {
	sys.InstallSIGPIPEHandler()
	cfg := parseArgs(os.Args[1:])
	if cfg.help {
		printHelp(os.Stdout)
		os.Exit(0)
	}
	if cfg.version {
		printVersion(os.Stdout)
		os.Exit(0)
	}
	os.Exit(run(cfg))
}

type config struct {
	files   []string
	help    bool
	version bool
}

func parseArgs(args []string) config {
	var cfg config
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		if a == "-" || len(a) == 0 || a[0] != '-' {
			cfg.files = append(cfg.files, a)
			continue
		}
		switch a {
		case "--help":
			cfg.help = true
			return cfg
		case "--version":
			cfg.version = true
			return cfg
		default:
			die(fmt.Sprintf("invalid option -- '%s'", a[1:]))
		}
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg
}

// run processes all files and returns the exit code.
func run(cfg config) int {
	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(name, out); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", programName, err)
		return 1
	}
	return exitCode
}

// processFile opens one input and unexpands its spaces.
func processFile(name string, out *bufio.Writer) error {
	r, err := openInput(name)
	if err != nil {
		if pe, ok := err.(*os.PathError); ok {
			return fmt.Errorf("%s: %s", name, pe.Err)
		}
		return err
	}
	if r != os.Stdin {
		defer r.Close()
	}
	return unexpandStream(bufio.NewReader(r), out)
}

func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// unexpandStream reads input and converts leading spaces to tabs.
// R1.1-R1.4: default mode converts only leading whitespace.
func unexpandStream(r *bufio.Reader, out *bufio.Writer) error {
	col := 0
	pending := 0
	leading := true
	for {
		c, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				return flushSpaces(out, pending)
			}
			return err
		}
		var werr error
		col, pending, leading, werr = processByte(out, c, col, pending, leading)
		if werr != nil {
			return werr
		}
	}
}

// processByte dispatches a single input byte to the appropriate handler.
func processByte(out *bufio.Writer, c byte, col, pending int, leading bool) (int, int, bool, error) {
	switch {
	case c == '\n':
		return handleNewline(out, pending)
	case leading && c == ' ':
		return handleLeadingSpace(out, col, pending)
	case leading && c == '\t':
		return handleLeadingTab(out, col)
	case leading:
		return handleEndLeading(out, c, col, pending)
	default:
		return handleNonLeading(out, c, col)
	}
}

// handleNewline flushes pending spaces and resets state for the next line.
func handleNewline(out *bufio.Writer, pending int) (int, int, bool, error) {
	if err := flushSpaces(out, pending); err != nil {
		return 0, 0, true, err
	}
	if err := out.WriteByte('\n'); err != nil {
		return 0, 0, true, err
	}
	return 0, 0, true, nil
}

// R1.1, R1.3: Leading space increments column; emit tab if tab stop reached.
func handleLeadingSpace(out *bufio.Writer, col, pending int) (int, int, bool, error) {
	col++
	pending++
	if col%defaultTabStop == 0 {
		if err := out.WriteByte('\t'); err != nil {
			return col, 0, true, err
		}
		return col, 0, true, nil
	}
	return col, pending, true, nil
}

// R1.4: Leading tab advances to next tab stop; pending spaces absorbed.
func handleLeadingTab(out *bufio.Writer, col int) (int, int, bool, error) {
	next := nextTabStop(col)
	if err := out.WriteByte('\t'); err != nil {
		return next, 0, true, err
	}
	return next, 0, true, nil
}

// R1.2: Non-whitespace in leading position flushes pending spaces, exits leading mode.
func handleEndLeading(out *bufio.Writer, c byte, col, pending int) (int, int, bool, error) {
	if err := flushSpaces(out, pending); err != nil {
		return col, 0, false, err
	}
	if err := out.WriteByte(c); err != nil {
		return col + 1, 0, false, err
	}
	return col + 1, 0, false, nil
}

// R1.2: Non-leading characters pass through unchanged.
func handleNonLeading(out *bufio.Writer, c byte, col int) (int, int, bool, error) {
	if err := out.WriteByte(c); err != nil {
		return col, 0, false, err
	}
	if c == '\t' {
		return nextTabStop(col), 0, false, nil
	}
	return col + 1, 0, false, nil
}

func flushSpaces(out *bufio.Writer, n int) error {
	for range n {
		if err := out.WriteByte(' '); err != nil {
			return err
		}
	}
	return nil
}

func nextTabStop(col int) int {
	return col + defaultTabStop - col%defaultTabStop
}

// printHelp prints usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s [OPTION]... [FILE]...
Convert blanks in each FILE to tabs, writing to standard output.

With no FILE, or when FILE is -, read standard input.

      --help        display this help and exit
      --version     output version information and exit
`, programName)
}

// printVersion prints version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", programName)
}

func die(msg string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, msg)
	os.Exit(1)
}
