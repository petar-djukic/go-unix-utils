// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd024-expand: Convert Tabs to Spaces.
// Covers R1.1-R1.4 (default tab expansion, stdin/file reading, stdout output),
// R2.1-R2.4 (-t custom tab stops, comma-separated list, last-wins, single-value uniform),
// R2.2 (-i initial-only mode),
// R3.1-R3.4 (exit codes, file error handling, write error handling, SIGPIPE).
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

// expandConfig holds parsed command-line options.
type expandConfig struct {
	tabStops []int // R2.1: explicit tab stop positions (empty = uniform)
	uniform  int   // R2.1: uniform tab interval (0 = use tabStops list)
	initial  bool  // R2.2: convert only leading tabs
}

func main() {
	// R1.4: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	cfg, files, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand: %s\n", err)
		os.Exit(1)
	}
	exitCode := run(cfg, files, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// parseArgs parses expand flags and returns config and file list.
// R1.2: stdin when no files or "-" given.
func parseArgs(args []string) (expandConfig, []string, error) {
	cfg := expandConfig{uniform: defaultTabStop}
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
		consumed, err := parseExpandFlag(arg, args, i, &cfg)
		if err != nil {
			return cfg, nil, err
		}
		i += consumed
	}
	return cfg, files, nil
}

// parseExpandFlag parses a single flag and returns args consumed.
func parseExpandFlag(arg string, args []string, i int, cfg *expandConfig) (int, error) {
	switch {
	case arg == "-i" || arg == "--initial":
		cfg.initial = true
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
func parseShortTabs(arg string, args []string, i int, cfg *expandConfig) (int, error) {
	if len(arg) > 2 {
		return 1, applyTabs(arg[2:], cfg)
	}
	if i+1 >= len(args) {
		return 0, fmt.Errorf("option requires an argument -- 't'")
	}
	return 2, applyTabs(args[i+1], cfg)
}

// parseLongTabs parses --tabs N or --tabs=N form.
func parseLongTabs(arg string, args []string, i int, cfg *expandConfig) (int, error) {
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
// R2.1: single number = uniform interval.
// R2.3: replaces default of 8; last -t wins (caller overwrites each time).
// R2.4: single value in comma-separated form treated as uniform interval.
func applyTabs(val string, cfg *expandConfig) error {
	parts := strings.Split(strings.ReplaceAll(val, " ", ","), ",")
	// R2.4: single value behaves identically to -t N (uniform interval).
	if len(parts) == 1 {
		return applyUniformTab(parts[0], cfg)
	}
	// R2.3: comma-separated list of absolute tab stop positions.
	return applyTabList(parts, cfg)
}

// applyUniformTab sets a uniform tab interval.
func applyUniformTab(s string, cfg *expandConfig) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("tab size contains invalid character(s): '%s'", s)
	}
	cfg.uniform = n
	cfg.tabStops = nil
	return nil
}

// applyTabList sets explicit tab stop positions (must be ascending).
func applyTabList(parts []string, cfg *expandConfig) error {
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
// R3.1: exit 0 when all inputs processed successfully.
// R3.2: stdin when no files given; process multiple files in order.
// R3.3: exit 1 when any file cannot be opened; error to stderr, continue.
// R3.4: exit 1 when a write error occurs on stdout.
func run(cfg expandConfig, files []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// R3.2: stdin when no files or "-" given.
	if len(files) == 0 {
		files = []string{"-"}
	}
	bw := bufio.NewWriter(stdout)
	exitCode := 0
	// R3.2: process multiple files in order, concatenating output.
	for _, name := range files {
		if err := processFile(name, stdin, bw, cfg); err != nil {
			// R3.3: write error to stderr and continue with remaining files.
			fmt.Fprintf(stderr, "expand: %s\n", err)
			exitCode = 1
		}
	}
	// R3.4: exit 1 on stdout write error (detected at flush).
	if err := bw.Flush(); err != nil {
		exitCode = 1
	}
	// R3.1: exit 0 when all inputs processed successfully.
	return exitCode
}

// processFile opens and expands a single file.
func processFile(name string, stdin io.Reader, bw *bufio.Writer, cfg expandConfig) error {
	r, err := openInput(name, stdin)
	if err != nil {
		return err
	}
	if name != "-" {
		defer r.Close()
	}
	return expandInput(r, bw, cfg)
}

// openInput opens a file or returns stdin for "-".
// R1.2: "-" means stdin.
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

// expandInput reads from r and writes expanded output to bw.
func expandInput(r io.Reader, bw *bufio.Writer, cfg expandConfig) error {
	br := bufio.NewReader(r)
	col := 0        // R1.1: 0-indexed column position
	leading := true // R2.2: true while in leading whitespace
	for {
		c, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		col, leading, err = expandByte(c, col, leading, bw, cfg)
		if err != nil {
			return err
		}
	}
}

// expandByte processes one byte, returns updated column and leading state.
func expandByte(c byte, col int, leading bool, bw *bufio.Writer, cfg expandConfig) (int, bool, error) {
	if c == '\n' {
		// R1.4: newline resets column position.
		err := bw.WriteByte('\n')
		return 0, true, err
	}
	if c == '\t' {
		return expandTab(col, leading, bw, cfg)
	}
	// R1.3: non-tab characters pass through unchanged.
	if err := bw.WriteByte(c); err != nil {
		return 0, false, err
	}
	if c != ' ' {
		leading = false
	}
	return col + 1, leading, nil
}

// expandTab expands a tab to spaces or passes it through.
// R2.2: -i mode preserves non-leading tabs.
func expandTab(col int, leading bool, bw *bufio.Writer, cfg expandConfig) (int, bool, error) {
	if cfg.initial && !leading {
		if err := bw.WriteByte('\t'); err != nil {
			return 0, false, err
		}
		return col + 1, false, nil
	}
	target := nextStop(col, cfg)
	spaces := target - col
	if spaces <= 0 {
		spaces = 1
	}
	for i := 0; i < spaces; i++ {
		if err := bw.WriteByte(' '); err != nil {
			return 0, false, err
		}
	}
	return col + spaces, leading, nil
}

// nextStop returns the next tab stop column after col.
// R1.1: default uniform stops at multiples of 8.
// R2.1: -t N sets uniform interval, -t LIST sets explicit positions.
func nextStop(col int, cfg expandConfig) int {
	if cfg.uniform > 0 {
		return (col/cfg.uniform + 1) * cfg.uniform
	}
	// Explicit tab stop list: find first stop > col.
	for _, s := range cfg.tabStops {
		if s > col {
			return s
		}
	}
	// Past last explicit stop: single space.
	return col + 1
}
