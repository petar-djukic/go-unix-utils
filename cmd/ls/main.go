// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the ls utility: list directory contents.
//
// Implements: prd008-ls (R1, R3, R6, R7)
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// filterMode controls which directory entries are shown.
type filterMode int

const (
	filterDefault   filterMode = iota // exclude entries whose names start with "."
	filterAll                         // -a: include . and .. and all dotfiles
	filterAlmostAll                   // -A: include dotfiles, exclude . and ..
)

// options holds the parsed command-line flags for ls (R1, R3, R6, R7 scope).
type options struct {
	filter     filterMode
	dirAsEntry bool // -d: list directory arguments themselves, not their contents
}

// currentTermWidth holds the most recent terminal column count, updated by
// the SIGWINCH handler registered in main. Atomic for safe concurrent access.
var currentTermWidth atomic.Int32

func main() {
	// Per prd008-ls R7.1: suppress broken-pipe errors when piped to head etc.
	sys.InstallSIGPIPEHandler()

	// Per prd008-ls R7.2: re-query terminal width when the terminal is resized.
	sys.OnTerminalResize(func(w int) {
		currentTermWidth.Store(int32(w))
	})

	// Initialize terminal width; ignore error when stdout is not a TTY.
	if w, err := sys.TerminalWidth(); err == nil {
		currentTermWidth.Store(int32(w))
	}

	opts, paths, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ls: %s\n", err)
		os.Exit(2)
	}

	// Per prd008-ls R1.1, R1.2: default to the current directory.
	if len(paths) == 0 {
		paths = []string{"."}
	}

	out := bufio.NewWriter(os.Stdout)
	code := runLS(opts, paths, out)
	if err := out.Flush(); err != nil {
		os.Exit(1)
	}
	os.Exit(code)
}

// parseFlags parses ls command-line arguments using manual flag parsing to
// support combined short flags (e.g., -aA, -Ad) and the -- end-of-flags
// sentinel. Follows the pattern from cmd/cat/main.go and cmd/wc/main.go.
// Per design decision D1.
func parseFlags(args []string) (options, []string, error) {
	var opts options
	var paths []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}

		// Long options: only -- end-of-flags is supported in this scope.
		// Any other --foo is an unrecognized option. Per prd008-ls R6.3.
		if strings.HasPrefix(arg, "--") {
			return options{}, nil, fmt.Errorf("unrecognized option '%s'", arg)
		}

		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					// Per prd008-ls R3.1, R3.4: -a includes . and ..;
					// last of -a/-A wins.
					opts.filter = filterAll
				case 'A':
					// Per prd008-ls R3.2, R3.4: -A includes dotfiles, excludes
					// . and ..; last of -a/-A wins.
					opts.filter = filterAlmostAll
				case 'd':
					// Per prd008-ls R3.3: list directory arguments as entries.
					opts.dirAsEntry = true
				default:
					return options{}, nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
		} else {
			paths = append(paths, arg)
		}
	}

	return opts, paths, nil
}

// runLS executes the listing logic for the given paths and returns an exit code.
// Per prd008-ls R6.1-R6.3.
func runLS(opts options, paths []string, w *bufio.Writer) int {
	exitCode := 0
	isTTY := sys.IsTerminal(os.Stdout.Fd())

	// -d: emit each argument path without descending into directories.
	// Per prd008-ls R3.3.
	if opts.dirAsEntry {
		for _, p := range paths {
			if _, err := os.Lstat(p); err != nil {
				printAccessError(p, err)
				exitCode = 1
				continue
			}
			fmt.Fprintln(w, p)
		}
		return exitCode
	}

	// Classify each argument: collect valid paths, separating files from dirs.
	type entry struct {
		path  string
		isDir bool
	}

	var valid []entry
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			// Per prd008-ls R6.2: report error, continue with remaining args.
			printAccessError(p, err)
			exitCode = 1
			continue
		}
		valid = append(valid, entry{path: p, isDir: info.IsDir()})
	}

	if len(valid) == 0 {
		return exitCode
	}

	var files []string
	var dirs []string
	for _, e := range valid {
		if e.isDir {
			dirs = append(dirs, e.path)
		} else {
			files = append(files, e.path)
		}
	}

	// Non-directory file arguments are listed first (sorted), one per line or
	// multi-column when stdout is a TTY.
	if len(files) > 0 {
		sort.Strings(files)
		printEntries(files, isTTY, w)
	}

	// Directory arguments are listed with "path:" headers when there are
	// multiple directories or when mixed with file arguments.
	needHeaders := len(dirs) > 1 || (len(files) > 0 && len(dirs) > 0)
	for i, d := range dirs {
		if needHeaders {
			// Blank line before each directory block after the first section.
			if i > 0 || len(files) > 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s:\n", d)
		}
		if err := listDir(d, opts, isTTY, w); err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': %s\n", d, sysErrMsg(err))
			exitCode = 1
		}
	}

	return exitCode
}

// listDir reads a directory, applies the filter, sorts entries in C locale
// order, and writes them to w. Per prd008-ls R1.2, R1.3, R1.4.
func listDir(dir string, opts options, isTTY bool, w *bufio.Writer) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	names := filterEntries(entries, opts.filter)
	// Per prd008-ls R1.3: sort in C locale order (byte-lexicographic).
	sort.Strings(names)
	printEntries(names, isTTY, w)
	return nil
}

// filterEntries returns entry names after applying the filter mode.
// Per prd008-ls R1.4, R3.1, R3.2.
func filterEntries(entries []os.DirEntry, filter filterMode) []string {
	var names []string

	// -a adds the . and .. special entries; os.ReadDir never returns them.
	// Per prd008-ls R3.1.
	if filter == filterAll {
		names = append(names, ".", "..")
	}

	for _, entry := range entries {
		name := entry.Name()
		// Default mode excludes names starting with ".". Per prd008-ls R1.4.
		if filter == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}

	return names
}

// printEntries outputs names as single-column or multi-column output based on
// whether stdout is a TTY. Per prd008-ls R1.1, R1.2.
func printEntries(names []string, isTTY bool, w *bufio.Writer) {
	if len(names) == 0 {
		return
	}
	if isTTY {
		printMultiColumn(names, w)
	} else {
		// Per prd008-ls R1.2: one entry per line when stdout is not a TTY.
		for _, name := range names {
			fmt.Fprintln(w, name)
		}
	}
}

// printMultiColumn formats names into a grid layout that fits the terminal
// width. Entries are distributed vertically (down columns, then across) via
// pkg/format.Columns. Columns are separated by two spaces.
// Per prd008-ls R1.1, prd003-format R1.1.
func printMultiColumn(names []string, w *bufio.Writer) {
	width := int(currentTermWidth.Load())
	if width <= 0 {
		width = 80
	}

	grid := format.Columns(names, width)
	if grid == nil {
		for _, name := range names {
			fmt.Fprintln(w, name)
		}
		return
	}

	// Compute the maximum display width for each column position.
	numCols := len(grid[0])
	colWidths := make([]int, numCols)
	for _, row := range grid {
		for ci, entry := range row {
			if dw := utf8.RuneCountInString(entry); dw > colWidths[ci] {
				colWidths[ci] = dw
			}
		}
	}

	// Print each row: entries padded to their column width, separated by two
	// spaces. The last entry in each row receives no trailing padding.
	for _, row := range grid {
		var sb strings.Builder
		for ci, entry := range row {
			if ci > 0 {
				sb.WriteString("  ")
			}
			if ci < len(row)-1 {
				sb.WriteString(format.PadRight(entry, colWidths[ci]))
			} else {
				sb.WriteString(entry)
			}
		}
		sb.WriteByte('\n')
		fmt.Fprint(w, sb.String())
	}
}

// printAccessError writes a "cannot access" diagnostic to stderr.
// Per prd008-ls R6.2.
func printAccessError(path string, err error) {
	fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", path, sysErrMsg(err))
}

// sysErrMsg extracts the underlying system error message from an error,
// unwrapping *os.PathError when present.
func sysErrMsg(err error) string {
	if pathErr, ok := err.(*os.PathError); ok {
		return pathErr.Err.Error()
	}
	return err.Error()
}
