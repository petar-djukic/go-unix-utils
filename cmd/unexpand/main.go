// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/unexpand converts spaces to tabs (prd025-unexpand R1, R2, R3, R4).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

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
	tabSpec string // raw -t value; empty means default
	help    bool
	version bool
}

// tabStops holds parsed tab stop configuration.
// R3.1: single value = uniform interval; multiple = explicit positions.
type tabStops struct {
	uniform  int   // >0 when using a uniform interval
	explicit []int // absolute 1-based column positions (sorted, ascending)
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
		case "-a", "--all":
			cfg.allMode = true
		default:
			if parseTabArg(a, args, &i, &cfg) {
				continue
			}
			die(fmt.Sprintf("invalid option -- '%s'", a[1:]))
		}
	}
	if len(cfg.files) == 0 {
		cfg.files = []string{"-"}
	}
	return cfg
}

// parseTabArg handles -t / --tabs arguments. Returns true if matched.
func parseTabArg(a string, args []string, i *int, cfg *config) bool {
	if a == "-t" || a == "--tabs" {
		if *i+1 >= len(args) {
			die("option requires an argument -- 't'")
		}
		*i++
		cfg.tabSpec = args[*i]
		return true
	}
	if strings.HasPrefix(a, "-t") {
		cfg.tabSpec = a[2:]
		return true
	}
	if strings.HasPrefix(a, "--tabs=") {
		cfg.tabSpec = a[7:]
		return true
	}
	return false
}

// parseTabStops parses the -t value into a tabStops struct.
// R3.1: single integer = uniform interval.
// R3.1: comma-separated list = explicit positions.
func parseTabStops(spec string) tabStops {
	if spec == "" {
		return tabStops{uniform: defaultTabStop}
	}
	parts := splitTabSpec(spec)
	if len(parts) == 1 {
		return tabStops{uniform: parsePositiveInt(parts[0])}
	}
	positions := make([]int, len(parts))
	for i, p := range parts {
		v := parsePositiveInt(p)
		positions[i] = v
		if i > 0 && positions[i] <= positions[i-1] {
			die("tab sizes must be ascending")
		}
	}
	return tabStops{explicit: positions}
}

func splitTabSpec(spec string) []string {
	if strings.ContainsRune(spec, ',') {
		return strings.Split(spec, ",")
	}
	return strings.Fields(spec)
}

func parsePositiveInt(s string) int {
	trimmed := strings.TrimSpace(s)
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		die(fmt.Sprintf("tab size contains invalid character(s): '%s'", trimmed))
	}
	if n == 0 {
		die("tab size cannot be 0")
	}
	if n < 0 {
		die(fmt.Sprintf("tab size contains invalid character(s): '%s'", trimmed))
	}
	return n
}

// run processes all files and returns the exit code.
func run(cfg config) int {
	ts := parseTabStops(cfg.tabSpec)
	// R3.3: -t implies -a
	allMode := cfg.allMode || cfg.tabSpec != ""
	out := bufio.NewWriter(os.Stdout)
	exitCode := 0
	for _, name := range cfg.files {
		if err := processFile(name, allMode, ts, out); err != nil {
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
func processFile(name string, allMode bool, ts tabStops, out *bufio.Writer) error {
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
	return unexpandStream(bufio.NewReader(r), out, allMode, ts)
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
// R3.1-R3.3: -t sets custom tab stops and implies -a.
func unexpandStream(r *bufio.Reader, out *bufio.Writer, allMode bool, ts tabStops) error {
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
			out, c, col, pending, leading, allMode, ts,
		)
		if werr != nil {
			return werr
		}
	}
}

// processByte dispatches a single input byte to the appropriate handler.
func processByte(
	out *bufio.Writer, c byte, col, pending int,
	leading, allMode bool, ts tabStops,
) (int, int, bool, error) {
	switch {
	case c == '\n':
		return handleNewline(out, pending)
	case (leading || allMode) && c == ' ':
		return handleSpace(out, col, pending, leading, ts)
	case (leading || allMode) && c == '\t':
		return handleTab(out, col, leading, ts)
	case leading:
		return handleEndLeading(out, c, col, pending)
	default:
		return handleNonLeading(out, c, col, pending, ts)
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
// R3.2: Past last explicit stop, spaces are kept as-is.
func handleSpace(
	out *bufio.Writer, col, pending int,
	leading bool, ts tabStops,
) (int, int, bool, error) {
	col++
	pending++
	if !canTabAt(col, ts) {
		return col, pending, leading, nil
	}
	if err := out.WriteByte('\t'); err != nil {
		return col, 0, leading, err
	}
	return col, 0, leading, nil
}

// R1.4: Tab advances to next tab stop; pending spaces absorbed.
func handleTab(
	out *bufio.Writer, col int,
	leading bool, ts tabStops,
) (int, int, bool, error) {
	next := nextTabStop(col, ts)
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
	out *bufio.Writer, c byte, col, pending int, ts tabStops,
) (int, int, bool, error) {
	if err := flushSpaces(out, pending); err != nil {
		return col, 0, false, err
	}
	if err := out.WriteByte(c); err != nil {
		return col, 0, false, err
	}
	if c == '\t' {
		return nextTabStop(col, ts), 0, false, nil
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

// canTabAt returns true if column col is a tab stop where a tab can be emitted.
// R3.2: For explicit tab lists, returns false past the last defined stop.
func canTabAt(col int, ts tabStops) bool {
	if ts.uniform > 0 {
		return col%ts.uniform == 0
	}
	return slices.Contains(ts.explicit, col)
}

// nextTabStop returns the column position of the next tab stop after col.
func nextTabStop(col int, ts tabStops) int {
	if ts.uniform > 0 {
		return col + ts.uniform - col%ts.uniform
	}
	for _, stop := range ts.explicit {
		if stop > col {
			return stop
		}
	}
	// R3.2: past last explicit stop, advance by 1 (tab acts as single space)
	return col + 1
}

// printHelp prints usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s [OPTION]... [FILE]...
Convert blanks in each FILE to tabs, writing to standard output.

With no FILE, or when FILE is -, read standard input.

  -a, --all        convert all blanks, instead of just initial blanks
  -t, --tabs=N     have tabs N characters apart, not 8
  -t, --tabs=LIST  use comma separated list of tab positions
      --help       display this help and exit
      --version    output version information and exit
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
