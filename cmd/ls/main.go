// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd008-ls R1.1-R1.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const helpText = `Usage: ls [OPTION]... [FILE]...
List information about the FILEs (the current directory by default).
Sort entries alphabetically if no flags given.

      --help     display this help and exit
      --version  output version information and exit
`

const versionText = `ls (go-unix-utils) dev
`

func main() {
	sys.InstallSIGPIPEHandler()
	paths := parseArgs(os.Args[1:])
	if len(paths) == 0 {
		paths = []string{"."}
	}
	os.Exit(run(paths))
}

func parseArgs(args []string) []string {
	var paths []string
	for len(args) > 0 {
		switch args[0] {
		case "--help":
			fmt.Fprint(os.Stdout, helpText)
			os.Exit(0)
		case "--version":
			fmt.Fprint(os.Stdout, versionText)
			os.Exit(0)
		case "--":
			return append(paths, args[1:]...)
		default:
			if strings.HasPrefix(args[0], "-") && len(args[0]) > 1 {
				fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", args[0])
				fmt.Fprintln(os.Stderr, "Try 'ls --help' for more information.")
				os.Exit(2)
			}
			paths = append(paths, args[0])
		}
		args = args[1:]
	}
	return paths
}

func run(paths []string) int {
	exitCode := 0
	var files, dirs []string
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", p, sysError(err))
			exitCode = 2
			continue
		}
		if info.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
	}
	needSep := false
	if len(files) > 0 {
		sort.Strings(files)
		writeEntries(files)
		needSep = true
	}
	showHeader := len(dirs) > 1 || len(files) > 0
	for _, dir := range dirs {
		if needSep {
			fmt.Fprintln(os.Stdout)
		}
		if showHeader {
			fmt.Fprintf(os.Stdout, "%s:\n", dir)
		}
		if err := listDir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot open directory '%s': %s\n",
				dir, sysError(err))
			exitCode = 2
		}
		needSep = true
	}
	return exitCode
}

func listDir(path string) error {
	names, err := readNames(path)
	if err != nil {
		return err
	}
	writeEntries(names)
	return nil
}

func readNames(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func writeEntries(names []string) {
	if len(names) == 0 {
		return
	}
	if sys.IsTerminal(os.Stdout.Fd()) {
		writeColumns(names)
	} else {
		writeLines(names)
	}
}

func writeColumns(names []string) {
	termWidth, err := sys.TerminalWidth()
	if err != nil {
		termWidth = 80
	}
	grid := format.Columns(names, termWidth)
	widths := gridColumnWidths(grid)
	for _, row := range grid {
		writeGridRow(row, widths)
	}
}

func writeLines(names []string) {
	for _, name := range names {
		writeLine(name)
	}
}

func gridColumnWidths(grid [][]string) []int {
	if len(grid) == 0 {
		return nil
	}
	widths := make([]int, len(grid[0]))
	for _, row := range grid {
		for c, cell := range row {
			if w := utf8.RuneCountInString(cell); w > widths[c] {
				widths[c] = w
			}
		}
	}
	return widths
}

func writeGridRow(row []string, widths []int) {
	var b strings.Builder
	for i, cell := range row {
		if i < len(row)-1 {
			b.WriteString(format.PadRight(cell, widths[i]))
			b.WriteString("  ")
		} else {
			b.WriteString(cell)
		}
	}
	writeLine(b.String())
}

func writeLine(s string) {
	if _, err := fmt.Fprintln(os.Stdout, s); err != nil {
		os.Exit(1)
	}
}

func sysError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}
