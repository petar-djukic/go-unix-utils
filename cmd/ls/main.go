// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ls utility for listing directory contents.
//
// Implements prd008-ls (R1.1, R1.2, R1.3, R1.4, R3.1, R3.2, R4.1, R4.2, R4.3, R4.4, R6.1, R6.2, R7.1).
// This implementation provides multi-column output when stdout is a terminal,
// one-per-line output when piped, -1 and -C flag support, alphabetical
// sorting under LC_ALL=C, hidden-file filtering (-a, -A), multi-argument
// headers, and non-directory file argument handling.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultTermWidth is the fallback terminal width used when -C is given but
// TerminalWidth returns an error (stdout is not a terminal).
const defaultTermWidth = 80

// columnGap is the number of spaces between adjacent columns, matching the
// gap used by pkg/format.Columns when computing layout.
const columnGap = 2

// options holds the parsed command-line flags for ls.
type options struct {
	showAll       bool // -a: show all entries including . and ..
	showAlmostAll bool // -A: show entries starting with . except . and ..
	singleColumn  bool // -1: force single-column output
	forceColumns  bool // -C: force multi-column output
}

// showHidden returns true when hidden entries should be included.
// R3.4: when both -a and -A are given, the last one on the command line
// takes precedence. Since Go's flag package uses last-wins semantics for
// repeated flags, the values here reflect the final state.
// However, with separate bool flags, both may be true if both are given.
// We resolve: -a takes precedence over -A when both are set, matching the
// behavior where -a is the broader filter.
func (o *options) showHidden() bool {
	return o.showAll || o.showAlmostAll
}

func (o *options) includeDotAndDotDot() bool {
	return o.showAll
}

// lsEntry pairs an entry name with its file type mode for color rendering.
// D2: avoids a separate stat call at render time by capturing the mode during collection.
type lsEntry struct {
	Name string
	Mode os.FileMode
}

// reportError writes an ls-style error message to stderr.
// R6.2: per-path error messages to stderr.
func reportError(path string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "ls: %s: %v\n", path, pathErr.Err)
	} else {
		fmt.Fprintf(os.Stderr, "ls: %s: %v\n", path, err)
	}
}

// collectEntries reads directory entries, applies filtering and sorting,
// and returns the sorted lsEntry slice with file type modes.
// R1.3: entries sorted in C locale byte order.
// R1.4: entries starting with "." excluded unless -a or -A is given.
// D2: captures os.DirEntry.Type() during collection for color rendering.
func collectEntries(dirPath string, opts *options) ([]lsEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	result := make([]lsEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !opts.showHidden() && len(name) > 0 && name[0] == '.' {
			continue
		}
		if opts.showAlmostAll && !opts.showAll {
			// -A: include dotfiles but exclude . and ..
			if name == "." || name == ".." {
				continue
			}
		}
		result = append(result, lsEntry{Name: name, Mode: e.Type()})
	}

	// R3.1: -a includes . and .. in the listing.
	if opts.includeDotAndDotDot() {
		result = append(result, lsEntry{Name: ".", Mode: os.ModeDir})
		result = append(result, lsEntry{Name: "..", Mode: os.ModeDir})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// writeMultiColumn writes entries in multi-column layout using
// pkg/format.Columns and pkg/format.PadRight.
// R1.1: multi-column output via terminal width detection.
// R4: color wrapping applied at render time after layout computation.
// D3: layout computed from plain names; color codes applied after padding.
func writeMultiColumn(w *bufio.Writer, entries []lsEntry, termWidth int) error {
	// D3: extract plain names for grid layout computation.
	names := make([]string, len(entries))
	modeMap := make(map[string]os.FileMode, len(entries))
	for i, e := range entries {
		names[i] = e.Name
		modeMap[e.Name] = e.Mode
	}

	grid := format.Columns(names, termWidth)

	// Compute per-column widths from the grid.
	numCols := 0
	for _, row := range grid {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	colWidths := make([]int, numCols)
	for _, row := range grid {
		for c, entry := range row {
			dw := utf8.RuneCountInString(entry)
			if dw > colWidths[c] {
				colWidths[c] = dw
			}
		}
	}

	for _, row := range grid {
		for c, name := range row {
			colorCode := format.FileTypeColor(modeMap[name])
			resetCode := format.Reset()
			if c < len(row)-1 {
				// Non-final column: pad plain name, then wrap with color.
				padded := format.PadRight(name, colWidths[c]+columnGap)
				if _, err := fmt.Fprintf(w, "%s%s%s", colorCode, padded, resetCode); err != nil {
					return err
				}
			} else {
				// Last entry in row: no trailing spaces.
				if _, err := fmt.Fprintf(w, "%s%s%s\n", colorCode, name, resetCode); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// writeSingleColumn writes entries one per line with optional color.
// R1.2: single-column output, one entry per line.
// R4.3: entry names wrapped with color codes when color is active.
func writeSingleColumn(w *bufio.Writer, entries []lsEntry) error {
	for _, e := range entries {
		colorCode := format.FileTypeColor(e.Mode)
		resetCode := format.Reset()
		if _, err := fmt.Fprintf(w, "%s%s%s\n", colorCode, e.Name, resetCode); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	// R7.1: handle SIGPIPE by exiting cleanly with code 0.
	sigpipe := make(chan os.Signal, 1)
	signal.Notify(sigpipe, syscall.SIGPIPE)
	go func() {
		<-sigpipe
		os.Exit(0)
	}()

	var opts options
	flag.BoolVar(&opts.showAll, "a", false, "show all entries including . and ..")
	flag.BoolVar(&opts.showAlmostAll, "A", false, "show entries starting with . except . and ..")
	flag.BoolVar(&opts.singleColumn, "1", false, "force single-column output")
	flag.BoolVar(&opts.forceColumns, "C", false, "force multi-column output")
	// R4.1: --color flag with auto/always/never values, defaulting to auto.
	colorFlag := flag.String("color", "auto", "colorize output: auto, always, never")
	flag.Parse()

	// R4.1, R4.2, R4.3: configure color mode based on --color flag.
	switch *colorFlag {
	case "always":
		format.SetColorEnabled(true)
	case "never":
		format.SetColorEnabled(false)
	case "auto":
		// R4.2: pkg/format detects TTY automatically via colorActive.
	default:
		fmt.Fprintf(os.Stderr, "ls: invalid argument '%s' for '--color'\n", *colorFlag)
		os.Exit(2)
	}

	// D2: determine output mode before processing arguments.
	// -1 forces single-column; -C forces multi-column; otherwise detect terminal.
	multiColumn := false
	termWidth := defaultTermWidth

	if opts.singleColumn {
		multiColumn = false
	} else if opts.forceColumns {
		multiColumn = true
		if w, err := sys.TerminalWidth(int(os.Stdout.Fd())); err == nil {
			termWidth = w
		}
	} else {
		// R1.1/R1.2: multi-column when stdout is a terminal, single-column otherwise.
		if w, err := sys.TerminalWidth(int(os.Stdout.Fd())); err == nil {
			multiColumn = true
			termWidth = w
		}
	}

	// R2: default to "." when no arguments are given.
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	out := bufio.NewWriter(os.Stdout)
	exitCode := 0

	// Separate file arguments from directory arguments, preserving order.
	// R2: non-directory files print their name as a single line without a header.
	// R5: file arguments receive color based on os.Lstat mode.
	var fileArgs []lsEntry
	var dirArgs []string

	for _, arg := range args {
		info, err := os.Lstat(arg)
		if err != nil {
			reportError(arg, err)
			exitCode = 1
			continue
		}
		if info.IsDir() {
			dirArgs = append(dirArgs, arg)
		} else {
			fileArgs = append(fileArgs, lsEntry{Name: arg, Mode: info.Mode()})
		}
	}

	needSeparator := false

	// Print non-directory file arguments first, one per line.
	// R2: file arguments print as single lines without headers.
	// R5: file type color applied via format.FileTypeColor.
	for _, e := range fileArgs {
		colorCode := format.FileTypeColor(e.Mode)
		resetCode := format.Reset()
		if _, err := fmt.Fprintf(out, "%s%s%s\n", colorCode, e.Name, resetCode); err != nil {
			exitCode = 1
		}
		needSeparator = true
	}

	// R2: when listing more than one directory, print headers.
	// Headers are also printed when there are file arguments mixed with directories.
	showHeaders := len(dirArgs) > 1 || (len(fileArgs) > 0 && len(dirArgs) > 0)

	for _, dirPath := range dirArgs {
		// R2: blank line between groups.
		if needSeparator {
			fmt.Fprintln(out)
		}
		needSeparator = true

		if showHeaders {
			fmt.Fprintf(out, "%s:\n", dirPath)
		}

		// D3: collect filtered and sorted entries, then render in the
		// appropriate output mode.
		entries, err := collectEntries(dirPath, &opts)
		if err != nil {
			reportError(dirPath, err)
			exitCode = 1
			continue
		}

		if multiColumn && len(entries) > 0 {
			if err := writeMultiColumn(out, entries, termWidth); err != nil {
				exitCode = 1
			}
		} else {
			if err := writeSingleColumn(out, entries); err != nil {
				exitCode = 1
			}
		}
	}

	// R5: detect write errors on final flush.
	if err := out.Flush(); err != nil {
		exitCode = 1
	}

	os.Exit(exitCode)
}
