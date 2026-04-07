// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/ls: list directory contents.
// Implements srd008-ls R1.1, R1.2, R1.3, R1.4 (output formats and layout).
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// filterMode controls which entries are shown.
type filterMode int

const (
	filterDefault filterMode = iota // skip dot-files
	filterAlmostAll                 // -A: show dot-files except . and ..
	filterAll                       // -a: show all including . and ..
)

// options holds parsed command-line flags.
type options struct {
	onePerLine bool       // -1: force single-column output
	filter     filterMode // -a / -A
}

// parseArgs separates flags from path arguments.
// R4.3: invalid option exits 2.
func parseArgs(args []string) (options, []string) {
	var opts options
	var paths []string
	flagsDone := false

	for _, arg := range args {
		if flagsDone || arg == "-" || len(arg) < 2 || arg[0] != '-' {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if err := parseFlags(&opts, arg[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "ls: %s\n", err)
			os.Exit(2)
		}
	}
	return opts, paths
}

// parseFlags processes combined short flags.
// R2.4: last of -a/-A wins.
func parseFlags(opts *options, flags string) error {
	for _, ch := range flags {
		switch ch {
		case '1':
			opts.onePerLine = true
		case 'a':
			opts.filter = filterAll
		case 'A':
			opts.filter = filterAlmostAll
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return nil
}

// listDir reads and prints the contents of a single directory.
func listDir(path string, opts *options) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	names := filterEntries(entries, opts.filter)
	// R1.3: sort in C locale byte order.
	sort.Strings(names)
	printEntries(names, opts)
	return nil
}

// filterEntries extracts entry names and applies the filter mode.
// R1.4: default hides dot-files. R2.1: -a includes . and ..
// R2.2: -A includes dot-files except . and ..
func filterEntries(entries []os.DirEntry, mode filterMode) []string {
	var names []string
	if mode == filterAll {
		names = append(names, ".", "..")
	}
	for _, e := range entries {
		name := e.Name()
		if mode == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	return names
}

// printEntries outputs entry names in the appropriate format.
// R1.2: single-column when stdout is not a TTY.
// R1.5/R4: -1 forces single-column.
func printEntries(names []string, opts *options) {
	if len(names) == 0 {
		return
	}
	if opts.onePerLine || !sys.IsTerminal(os.Stdout.Fd()) {
		printOnePerLine(names)
		return
	}
	printColumns(names)
}

// printOnePerLine writes one entry per line.
func printOnePerLine(names []string) {
	for _, name := range names {
		fmt.Println(name)
	}
}

// printColumns formats entries in multi-column layout.
// R1.1: uses pkg/format.Columns when stdout is a TTY.
func printColumns(names []string) {
	width, err := sys.TerminalWidth()
	if err != nil || width <= 0 {
		width = 80
	}
	rows := columnLayout(names, width)
	for _, row := range rows {
		printRow(row, names, width)
	}
}

// columnLayout computes multi-column layout for entries.
// Returns row slices of entry names, sorted column-down.
func columnLayout(names []string, termWidth int) [][]string {
	n := len(names)
	if n == 0 {
		return nil
	}
	bestCols := findMaxCols(names, n, termWidth)
	return buildRows(names, bestCols, n)
}

// findMaxCols determines the maximum number of columns that fit.
func findMaxCols(names []string, n, termWidth int) int {
	for numCols := n; numCols > 1; numCols-- {
		numRows := (n + numCols - 1) / numCols
		if totalWidth(names, numCols, numRows, n) <= termWidth {
			return numCols
		}
	}
	return 1
}

// totalWidth computes the display width for a given column layout.
// Uses 2-space gap between columns.
func totalWidth(names []string, numCols, numRows, n int) int {
	total := 0
	for col := range numCols {
		maxW := 0
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			w := len(names[idx])
			if w > maxW {
				maxW = w
			}
		}
		total += maxW
		if col < numCols-1 {
			total += 2
		}
	}
	return total
}

// buildRows arranges entries into rows for column-down layout.
func buildRows(names []string, numCols, n int) [][]string {
	numRows := (n + numCols - 1) / numCols
	rows := make([][]string, numRows)
	for row := range numRows {
		var r []string
		for col := range numCols {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			r = append(r, names[idx])
		}
		rows[row] = r
	}
	return rows
}

// printRow prints a single row with per-column padding.
func printRow(row []string, allNames []string, termWidth int) {
	n := len(allNames)
	numCols := findMaxCols(allNames, n, termWidth)
	numRows := (n + numCols - 1) / numCols
	colWidths := computeColWidths(allNames, numCols, numRows, n)

	for i, name := range row {
		if i > 0 {
			fmt.Print("  ")
		}
		if i < len(row)-1 {
			fmt.Print(padRight(name, colWidths[i]))
		} else {
			fmt.Print(name)
		}
	}
	fmt.Println()
}

// computeColWidths returns the max width for each column.
func computeColWidths(names []string, numCols, numRows, n int) []int {
	widths := make([]int, numCols)
	for col := range numCols {
		for row := range numRows {
			idx := col*numRows + row
			if idx >= n {
				break
			}
			w := len(names[idx])
			if w > widths[col] {
				widths[col] = w
			}
		}
	}
	return widths
}

// padRight pads s with spaces to reach width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// listPath handles a single argument: file or directory.
func listPath(path string, opts *options) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		fmt.Println(info.Name())
		return nil
	}
	return listDir(path, opts)
}

// formatError produces GNU ls-compatible error messages.
func formatError(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("ls: cannot access '%s': %s", pe.Path, pe.Err)
	}
	return fmt.Sprintf("ls: cannot access '%s': %s", path, err)
}

func main() {
	// R4.4: install SIGPIPE handler.
	sys.InstallSIGPIPEHandler()

	opts, paths := parseArgs(os.Args[1:])

	// R1.1: default to current directory.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	exitCode := 0
	for _, path := range paths {
		if err := listPath(path, &opts); err != nil {
			fmt.Fprintln(os.Stderr, formatError(path, err))
			exitCode = 1
		}
	}
	// R4.1: exit 0 on success. R4.2: exit 1 on minor error.
	os.Exit(exitCode)
}
