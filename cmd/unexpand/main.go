// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand: Convert Spaces to Tabs.
// Covers R1.1-R1.4 (default leading whitespace conversion, stdin/file reading, stdout output),
// R2.1-R2.2 (--first-only default, -a/--all convert all whitespace).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultTabStop = 8

// unexpandConfig holds parsed command-line options.
type unexpandConfig struct {
	allMode bool // R2.1/R2.2: convert all whitespace, not just leading
}

func main() {
	// R4.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpand: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(cfg, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseArgs parses unexpand flags and returns config and file list.
func parseArgs(args []string) (unexpandConfig, []string, error) {
	var cfg unexpandConfig
	var files []string
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			files = append(files, arg)
			i++
			continue
		}
		consumed, err := parseFlag(arg, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		i += consumed
	}
	return cfg, files, nil
}

// parseFlag parses a single flag and returns args consumed.
func parseFlag(arg string, cfg *unexpandConfig) (int, error) {
	switch arg {
	case "-a", "--all":
		cfg.allMode = true
		return 1, nil
	case "--first-only":
		cfg.allMode = false
		return 1, nil
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
}

// run processes all files and returns the exit code.
// R1.4: stdin when no files or "-" given.
func run(cfg unexpandConfig, files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	for _, name := range files {
		if err := processFile(name, stdin, bw, cfg); err != nil {
			fmt.Fprintf(stderr, "unexpand: %s\n", err)
			exitCode = 1
		}
	}
	if err := bw.Flush(); err != nil {
		exitCode = 1
	}
	return exitCode
}

// processFile opens and unexpands a single file.
func processFile(name string, stdin io.Reader, bw *bufio.Writer, cfg unexpandConfig) error {
	r, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return unexpandInput(r, bw, cfg)
}

// openInput opens a file or returns stdin for "-".
func openInput(name string, stdin io.Reader) (io.ReadCloser, error) {
	if name == "-" {
		return io.NopCloser(stdin), nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", name, unwrapOSError(err))
	}
	return f, nil
}

// unwrapOSError extracts the underlying error message from an os.PathError.
func unwrapOSError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// unexpandInput reads from r and writes unexpanded output to bw.
// R1.1: replaces leading spaces with tabs at default 8-column stops.
// R1.2: by default only converts leading whitespace.
// R1.3: preserves content after first non-blank unchanged (default mode).
func unexpandInput(r io.Reader, bw *bufio.Writer, cfg unexpandConfig) error {
	br := bufio.NewReader(r)
	var col, pending int
	converting := true
	for {
		c, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return flushSpaces(bw, pending)
			}
			return err
		}
		switch {
		case c == '\n':
			if err := flushAndWrite(bw, pending, '\n'); err != nil {
				return err
			}
			col, pending, converting = 0, 0, true
		case !converting:
			if err := bw.WriteByte(c); err != nil {
				return err
			}
		case c == ' ':
			col, pending, err = handleSpace(col, pending, bw)
			if err != nil {
				return err
			}
		case c == '\t':
			col, pending, err = handleTab(col, bw)
			if err != nil {
				return err
			}
		default:
			col, pending, converting, err = handleNonWS(c, col, pending, bw, cfg)
			if err != nil {
				return err
			}
		}
	}
}

// handleSpace accumulates a space and emits a tab when a tab stop is reached.
// R1.1: tabs are preferred over spaces when a run reaches a tab stop exactly.
// R1.3: spaces that do not reach a tab stop are kept as spaces.
func handleSpace(col, pending int, bw *bufio.Writer) (int, int, error) {
	col++
	pending++
	if col%defaultTabStop == 0 {
		if err := bw.WriteByte('\t'); err != nil {
			return col, 0, err
		}
		return col, 0, nil
	}
	return col, pending, nil
}

// handleTab processes an existing tab in the input.
// R1.4: existing tabs count toward column position and do not prevent further substitution.
func handleTab(col int, bw *bufio.Writer) (int, int, error) {
	if err := bw.WriteByte('\t'); err != nil {
		return col, 0, err
	}
	return nextStop(col), 0, nil
}

// handleNonWS flushes pending spaces and writes a non-whitespace character.
// R1.2: in default mode, stops converting after first non-whitespace.
// R2.1/R2.2: in -a mode, continues converting after non-whitespace.
func handleNonWS(c byte, col, pending int, bw *bufio.Writer, cfg unexpandConfig) (int, int, bool, error) {
	if err := flushSpaces(bw, pending); err != nil {
		return col, 0, false, err
	}
	if err := bw.WriteByte(c); err != nil {
		return col + 1, 0, false, err
	}
	return col + 1, 0, cfg.allMode, nil
}

// flushSpaces writes n space characters to the writer.
func flushSpaces(bw *bufio.Writer, n int) error {
	for i := 0; i < n; i++ {
		if err := bw.WriteByte(' '); err != nil {
			return err
		}
	}
	return nil
}

// flushAndWrite flushes pending spaces then writes a single byte.
func flushAndWrite(bw *bufio.Writer, pending int, c byte) error {
	if err := flushSpaces(bw, pending); err != nil {
		return err
	}
	return bw.WriteByte(c)
}

// nextStop returns the next tab stop column after col.
func nextStop(col int) int {
	return (col/defaultTabStop + 1) * defaultTabStop
}
