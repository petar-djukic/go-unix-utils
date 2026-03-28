// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd025-unexpand: Convert Spaces to Tabs.
// Covers R1.1-R1.4 (default leading whitespace conversion),
// R2.1-R2.3 (--first-only default, -a/--all convert all whitespace),
// R3.1-R3.3 (-t custom tab stops, list mode, -t implies -a),
// R4.1-R4.4 (exit codes, file error handling, write error, SIGPIPE).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const defaultTabStop = 8

// unexpandConfig holds parsed command-line options.
type unexpandConfig struct {
	allMode  bool  // R2.1: convert all whitespace, not just leading
	tabStops []int // R3.1: explicit tab stop positions (empty = uniform)
	uniform  int   // R3.1: uniform tab interval (0 = use tabStops list)
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
	cfg := unexpandConfig{uniform: defaultTabStop}
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
		consumed, err := parseFlag(arg, args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		i += consumed
	}
	return cfg, files, nil
}

// parseFlag parses a single flag and returns args consumed.
func parseFlag(arg string, args []string, i int, cfg *unexpandConfig) (int, error) {
	switch {
	case arg == "-a" || arg == "--all":
		cfg.allMode = true
		return 1, nil
	case arg == "--first-only":
		cfg.allMode = false
		return 1, nil
	case strings.HasPrefix(arg, "-t"):
		return parseShortTabs(arg, args, i, cfg)
	case strings.HasPrefix(arg, "--tabs"):
		return parseLongTabs(arg, args, i, cfg)
	default:
		return 0, fmt.Errorf("invalid option -- '%s'", arg[1:])
	}
}

// parseShortTabs parses -t N or -tN form.
func parseShortTabs(arg string, args []string, i int, cfg *unexpandConfig) (int, error) {
	if len(arg) > 2 {
		return 1, applyTabs(arg[2:], cfg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 't'")
	}
	return 2, applyTabs(args[i+1], cfg)
}

// parseLongTabs parses --tabs N or --tabs=N form.
func parseLongTabs(arg string, args []string, i int, cfg *unexpandConfig) (int, error) {
	if strings.HasPrefix(arg, "--tabs=") {
		return 1, applyTabs(arg[len("--tabs="):], cfg)
	}
	if arg != "--tabs" {
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option '--tabs' requires an argument")
	}
	return 2, applyTabs(args[i+1], cfg)
}

// applyTabs parses a tab stop specification.
// R3.1: single number = uniform interval, comma list = absolute positions.
// R3.3: -t implies -a.
func applyTabs(val string, cfg *unexpandConfig) error {
	// R3.3: custom tab stops imply -a mode.
	cfg.allMode = true
	parts := strings.Split(strings.ReplaceAll(val, " ", ","), ",")
	if len(parts) == 1 {
		return applyUniformTab(parts[0], cfg)
	}
	return applyTabList(parts, cfg)
}

// applyUniformTab sets a uniform tab interval.
func applyUniformTab(s string, cfg *unexpandConfig) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("tab size contains invalid character(s): '%s'", s)
	}
	cfg.uniform = n
	cfg.tabStops = nil
	return nil
}

// applyTabList sets explicit tab stop positions (must be ascending).
func applyTabList(parts []string, cfg *unexpandConfig) error {
	stops := make([]int, 0, len(parts))
	prev := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			return fmt.Errorf("tab size contains invalid character(s): '%s'", p)
		}
		if n <= prev {
			return fmt.Errorf("tab sizes must be ascending")
		}
		stops = append(stops, n)
		prev = n
	}
	cfg.tabStops = stops
	cfg.uniform = 0
	return nil
}

// run processes all files and returns the exit code.
// R4.1: exit 0 when all inputs processed successfully.
// R4.2: exit 1 when any file cannot be opened; continue remaining.
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
// R1.1: replaces leading spaces with tabs at configured tab stops.
// R1.2: by default only converts leading whitespace.
// R2.1: -a mode converts all whitespace.
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
		col, pending, converting, err = processByte(
			c, col, pending, converting, bw, cfg,
		)
		if err != nil {
			return err
		}
	}
}

// processByte dispatches a single byte through the unexpand state machine.
func processByte(c byte, col, pending int, converting bool, bw *bufio.Writer, cfg unexpandConfig) (int, int, bool, error) {
	switch {
	case c == '\n':
		if err := flushAndWrite(bw, pending, '\n'); err != nil {
			return 0, 0, true, err
		}
		return 0, 0, true, nil
	case !converting:
		if err := bw.WriteByte(c); err != nil {
			return col, 0, false, err
		}
		return col + 1, 0, false, nil
	case c == ' ':
		return handleSpace(col, pending, bw, cfg)
	case c == '\t':
		return handleTab(col, bw, cfg)
	default:
		return handleNonWS(c, col, pending, bw, cfg)
	}
}

// handleSpace accumulates a space and emits a tab when a tab stop is reached.
// R1.1: tabs preferred when a run reaches a tab stop exactly.
// R1.3: spaces that do not reach a tab stop are kept as spaces.
// R3.2: past last explicit stop, spaces are kept as-is.
func handleSpace(col, pending int, bw *bufio.Writer, cfg unexpandConfig) (int, int, bool, error) {
	col++
	pending++
	ns := nextStop(col-1, cfg)
	// R3.2: if no valid stop exists, keep spaces as-is.
	if ns < 0 {
		return col, pending, true, nil
	}
	if col == ns {
		if err := bw.WriteByte('\t'); err != nil {
			return col, 0, true, err
		}
		return col, 0, true, nil
	}
	return col, pending, true, nil
}

// handleTab processes an existing tab in the input.
// R1.4: existing tabs count toward column position.
func handleTab(col int, bw *bufio.Writer, cfg unexpandConfig) (int, int, bool, error) {
	if err := bw.WriteByte('\t'); err != nil {
		return col, 0, true, err
	}
	ns := nextStop(col, cfg)
	if ns < 0 {
		return col + 1, 0, true, nil
	}
	return ns, 0, true, nil
}

// handleNonWS flushes pending spaces and writes a non-whitespace character.
// R1.2: in default mode, stops converting after first non-whitespace.
// R2.1/R2.3: in -a mode, continues converting after non-whitespace.
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
// Returns -1 when using explicit stops and col is past the last stop.
// R3.1: -t N sets uniform interval, -t LIST sets explicit positions.
// R3.2: past last explicit stop, returns -1 (no tab insertion).
func nextStop(col int, cfg unexpandConfig) int {
	if cfg.uniform > 0 {
		return (col/cfg.uniform + 1) * cfg.uniform
	}
	for _, s := range cfg.tabStops {
		if s > col {
			return s
		}
	}
	// R3.2: past last explicit stop — no more tab stops.
	return -1
}
