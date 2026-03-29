// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/unexpand converts spaces to tabs (prd025-unexpand R1, R2).
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
	allMode bool
	help    bool
	version bool
}

func parseArgs(args []string) config {
	var cfg config
	for i := range len(args) {
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
		case "-a", "--all":
			cfg.allMode = true
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
		if err := processFile(name, cfg.allMode, out); err != nil {
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
func processFile(name string, allMode bool, out *bufio.Writer) error {
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
	return unexpandStream(bufio.NewReader(r), out, allMode)
}

func openInput(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// unexpandStream reads input and converts spaces to tabs.
// R1.1-R1.4: default mode converts only leading whitespace.
// R2.1-R2.3: -a mode converts all whitespace throughout the line.
func unexpandStream(r *bufio.Reader, out *bufio.Writer, allMode bool) error {
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
		col, pending, leading, werr = processByte(
			out, c, col, pending, leading, allMode,
		)
		if werr != nil {
			return werr
		}
	}
}

// processByte dispatches a single input byte to the appropriate handler.
func processByte(
	out *bufio.Writer, c byte, col, pending int,
	leading, allMode bool,
) (int, int, bool, error) {
	switch {
	case c == '\n':
		return handleNewline(out, pending)
	case (leading || allMode) && c == ' ':
		return handleSpace(out, col, pending, leading, allMode)
	case (leading || allMode) && c == '\t':
		return handleTab(out, col, leading, allMode)
	case leading:
		return handleEndLeading(out, c, col, pending)
	default:
		return handleNonLeading(out, c, col, pending)
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

// R1.1, R1.3, R2.1, R2.2: Space increments column; emit tab if tab stop reached.
func handleSpace(
	out *bufio.Writer, col, pending int,
	leading, _ bool,
) (int, int, bool, error) {
	col++
	pending++
	if col%defaultTabStop == 0 {
		if err := out.WriteByte('\t'); err != nil {
			return col, 0, leading, err
		}
		return col, 0, leading, nil
	}
	return col, pending, leading, nil
}

// R1.4: Tab advances to next tab stop; pending spaces absorbed.
func handleTab(
	out *bufio.Writer, col int,
	leading, _ bool,
) (int, int, bool, error) {
	next := nextTabStop(col)
	if err := out.WriteByte('\t'); err != nil {
		return next, 0, leading, err
	}
	return next, 0, leading, nil
}

// R1.2: Non-whitespace in leading position flushes pending spaces, exits leading.
func handleEndLeading(
	out *bufio.Writer, c byte, col, pending int,
) (int, int, bool, error) {
	if err := flushSpaces(out, pending); err != nil {
		return col, 0, false, err
	}
	if err := out.WriteByte(c); err != nil {
		return col + 1, 0, false, err
	}
	return col + 1, 0, false, nil
}

// R1.2, R2.3: Non-leading, non-space character passes through.
// In -a mode, pending spaces are flushed before writing.
func handleNonLeading(
	out *bufio.Writer, c byte, col, pending int,
) (int, int, bool, error) {
	if err := flushSpaces(out, pending); err != nil {
		return col, 0, false, err
	}
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

  -a, --all       convert all blanks, instead of just initial blanks
      --help      display this help and exit
      --version   output version information and exit
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
