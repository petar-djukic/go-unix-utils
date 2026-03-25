// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd107-dir R1.1-R1.4: directory listing in multi-column format
// with C-style backslash escaping of non-printable characters, matching
// GNU dir (ls -C -b) behavior.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultTermWidth is used when stdout is not a TTY.
// R1.1: non-TTY output uses 80 columns.
const defaultTermWidth = 80

// defaultTabSize matches GNU ls tab stop for column alignment.
const defaultTabSize = 8

// filterMode selects which entries to show.
type filterMode int

const (
	filterDefault   filterMode = iota // hide dot-entries
	filterAlmostAll                   // -A: show dot-entries except . and ..
	filterAll                         // -a: show all including . and ..
)

// dirConfig holds parsed command-line options.
type dirConfig struct {
	filter filterMode
	args   []string
}

func main() {
	// R2.4: install SIGPIPE handler.
	sys.InstallSIGPIPEHandler()
	// R1.3: C locale sort order.
	os.Setenv("LC_ALL", "C")
	cfg := parseArgs(os.Args[1:])
	os.Exit(run(cfg))
}

// parseArgs extracts flags and positional arguments.
// R1.4: defaults to current directory when no args given.
func parseArgs(args []string) dirConfig {
	var cfg dirConfig
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			cfg.args = append(cfg.args, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" && !strings.HasPrefix(arg, "--") {
			parseShortFlags(arg[1:], &cfg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			parseLongFlag(arg, &cfg)
			continue
		}
		cfg.args = append(cfg.args, arg)
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"."}
	}
	return cfg
}

// parseShortFlags processes short flag characters.
// R1.3: -a and -A control dot-entry visibility.
func parseShortFlags(flags string, cfg *dirConfig) {
	for _, ch := range flags {
		switch ch {
		case 'a':
			cfg.filter = filterAll
		case 'A':
			cfg.filter = filterAlmostAll
		default:
			fmt.Fprintf(os.Stderr, "dir: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'dir --help' for more information.")
			os.Exit(2)
		}
	}
}

// parseLongFlag processes a long flag.
func parseLongFlag(arg string, cfg *dirConfig) {
	switch arg {
	case "--all":
		cfg.filter = filterAll
	case "--almost-all":
		cfg.filter = filterAlmostAll
	case "--help":
		printHelp()
		os.Exit(0)
	case "--version":
		fmt.Println("dir (go-unix-utils) dev")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "dir: unrecognized option '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'dir --help' for more information.")
		os.Exit(2)
	}
}

// run processes all arguments and returns the exit code.
func run(cfg dirConfig) int {
	exitCode := 0
	var files, dirs []string
	for _, arg := range cfg.args {
		fi, err := os.Stat(arg)
		if err != nil {
			reportError("cannot access", arg, err)
			exitCode = 1
			continue
		}
		if fi.IsDir() {
			dirs = append(dirs, arg)
		} else {
			files = append(files, arg)
		}
	}
	// R1.3: sort in C locale order (byte comparison).
	sort.Strings(files)
	sort.Strings(dirs)
	if len(files) > 0 {
		printColumnar(escapeNames(files))
	}
	exitCode |= listDirs(dirs, cfg, len(files) > 0)
	return exitCode
}

// listDirs lists multiple directory arguments in order.
func listDirs(dirs []string, cfg dirConfig, needBlank bool) int {
	exitCode := 0
	showHeader := needBlank || len(dirs) > 1
	for _, dir := range dirs {
		if needBlank {
			fmt.Println()
		}
		if showHeader {
			fmt.Printf("%s:\n", dir)
		}
		if code := listDir(dir, cfg); code != 0 {
			exitCode = 1
		}
		needBlank = true
	}
	return exitCode
}

// listDir lists the contents of a single directory.
func listDir(dir string, cfg dirConfig) int {
	rawEntries, err := os.ReadDir(dir)
	if err != nil {
		reportError("cannot open directory", dir, err)
		return 1
	}
	names := filterNames(rawEntries, cfg.filter)
	sort.Strings(names)
	printColumnar(escapeNames(names))
	return 0
}

// filterNames extracts and filters entry names from directory entries.
// R1.3: hide dot-entries by default; -a shows all; -A shows all except . and ..
func filterNames(raw []os.DirEntry, filter filterMode) []string {
	var names []string
	if filter == filterAll {
		names = append(names, ".", "..")
	}
	for _, e := range raw {
		name := e.Name()
		if filter == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// escapeNames applies C-style escaping to each name.
func escapeNames(names []string) []string {
	escaped := make([]string, len(names))
	for i, name := range names {
		escaped[i] = escapeFilename(name)
	}
	return escaped
}

// escapeFilename applies C-style backslash escaping to a filename.
// R1.2: escapes backslash, newline, tab, control chars, spaces, and
// non-ASCII bytes using C-style sequences or octal notation.
func escapeFilename(name string) string {
	needsEscape := false
	for i := 0; i < len(name); i++ {
		if mustEscape(name[i]) {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return name
	}
	var buf strings.Builder
	buf.Grow(len(name) * 2)
	for i := 0; i < len(name); i++ {
		writeEscapedByte(&buf, name[i])
	}
	return buf.String()
}

// mustEscape returns true if the byte needs C-style escaping.
func mustEscape(b byte) bool {
	return b == '\\' || b == ' ' || b < 0x20 || b >= 0x7f
}

// writeEscapedByte writes a single byte in C-style escaped form.
// R1.2: uses named escapes for common control chars, octal for others.
func writeEscapedByte(buf *strings.Builder, b byte) {
	switch b {
	case '\\':
		buf.WriteString("\\\\")
	case '\a':
		buf.WriteString("\\a")
	case '\b':
		buf.WriteString("\\b")
	case '\t':
		buf.WriteString("\\t")
	case '\n':
		buf.WriteString("\\n")
	case '\v':
		buf.WriteString("\\v")
	case '\f':
		buf.WriteString("\\f")
	case '\r':
		buf.WriteString("\\r")
	case ' ':
		buf.WriteString("\\ ")
	default:
		if b < 0x20 || b >= 0x7f {
			fmt.Fprintf(buf, "\\%03o", b)
		} else {
			buf.WriteByte(b)
		}
	}
}

// printColumnar prints names in multi-column format.
// R1.1: always uses multi-column layout regardless of TTY status.
func printColumnar(names []string) {
	if len(names) == 0 {
		return
	}
	width := termWidthOrDefault()
	rows := format.Columns(names, width)
	colWidths := computeColumnWidths(rows)
	colStarts := computeColStarts(colWidths)
	for _, row := range rows {
		printColumnarRow(row, colStarts)
	}
}

// termWidthOrDefault returns terminal width or defaultTermWidth.
// R1.1: uses sys.TerminalWidth() for TTY, 80 for non-TTY.
func termWidthOrDefault() int {
	w, err := sys.TerminalWidth()
	if err != nil {
		return defaultTermWidth
	}
	return w
}

// computeColumnWidths returns the max width per column across all rows.
func computeColumnWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	widths := make([]int, maxCols)
	for _, row := range rows {
		for col, e := range row {
			w := utf8.RuneCountInString(e)
			if w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// computeColStarts calculates the starting position of each column.
func computeColStarts(colWidths []int) []int {
	starts := make([]int, len(colWidths))
	pos := 0
	for i, w := range colWidths {
		starts[i] = pos
		pos += w + 2 // width + minimum 2-char gap
	}
	return starts
}

// printColumnarRow prints a single row using tab-based alignment
// matching GNU dir/ls -C output format.
func printColumnarRow(row []string, colStarts []int) {
	pos := 0
	for i, e := range row {
		fmt.Print(e)
		pos += utf8.RuneCountInString(e)
		if i < len(row)-1 {
			pad := tabPad(pos, colStarts[i+1])
			fmt.Print(pad)
			pos = colStarts[i+1]
		}
	}
	fmt.Println()
}

// tabPad returns a mix of tabs and spaces to move from pos to target,
// matching GNU coreutils column alignment behavior.
func tabPad(pos, target int) string {
	if pos >= target {
		return ""
	}
	var buf []byte
	for {
		nextTab := pos + defaultTabSize - pos%defaultTabSize
		if nextTab > target {
			break
		}
		buf = append(buf, '\t')
		pos = nextTab
	}
	for pos < target {
		buf = append(buf, ' ')
		pos++
	}
	return string(buf)
}

// reportError prints a diagnostic to stderr matching GNU dir format.
func reportError(action, path string, err error) {
	msg := err.Error()
	if pe, ok := err.(*os.PathError); ok {
		msg = pe.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "dir: %s '%s': %s\n", action, path, msg)
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: dir [OPTION]... [FILE]...
List directory contents in multi-column format with C-style escaping.
Equivalent to ls -C -b.

  -a, --all            do not ignore entries starting with .
  -A, --almost-all     do not list implied . and ..
      --help           display this help and exit
      --version        output version information and exit
`)
}
