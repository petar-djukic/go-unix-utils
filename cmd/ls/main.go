// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/ls lists directory contents and file arguments.
//
// Implements: prd008-ls R1.1, R1.2, R1.3, R1.4
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// defaultTermWidth is used for multi-column output when the terminal width
// cannot be determined.
const defaultTermWidth = 80

func main() {
	// R1.4 / D1: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	// R1.4: exit code 2 for serious trouble (unrecognized flags).
	// Minimal flag handling: reject any argument starting with "-".
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && arg != "-" && arg != "--" {
			fmt.Fprintf(os.Stderr, "ls: unrecognized option '%s'\n", arg)
			os.Exit(2)
		}
		if arg == "--" {
			break
		}
	}

	// Strip "--" separator if present.
	filtered := make([]string, 0, len(args))
	pastDash := false
	for _, arg := range args {
		if arg == "--" && !pastDash {
			pastDash = true
			continue
		}
		filtered = append(filtered, arg)
	}
	args = filtered

	// R1.2: default to current directory when no arguments given.
	if len(args) == 0 {
		args = []string{"."}
	}

	// D4: detect whether stdout is a terminal for output mode selection.
	isTTY := sys.IsTerminal(os.Stdout.Fd())

	// Separate file and directory arguments.
	var files []string
	var dirs []string
	exitCode := 0

	for _, arg := range args {
		fi, err := sys.Lstat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ls: cannot access '%s': %s\n", arg, unwrapMsg(err))
			// R1.4: exit 2 for cannot access a command-line argument.
			exitCode = 2
			continue
		}
		if fi.Mode.IsDir() {
			dirs = append(dirs, arg)
		} else {
			files = append(files, arg)
		}
	}

	// R1.2: list file arguments first, then directory arguments.
	// R1.3 / D5: sort file names in C locale byte order.
	sort.Strings(files)
	sort.Strings(dirs)

	needBlank := false

	// Print file arguments.
	if len(files) > 0 {
		printEntries(files, isTTY)
		needBlank = true
	}

	// Print directory arguments.
	multipleTargets := len(files) > 0 || len(dirs) > 1
	for _, dir := range dirs {
		if needBlank {
			fmt.Println()
		}
		if multipleTargets {
			fmt.Printf("%s:\n", dir)
		}
		if err := listDir(dir, isTTY); err != nil {
			fmt.Fprintf(os.Stderr, "ls: reading directory '%s': %s\n", dir, unwrapMsg(err))
			if exitCode < 1 {
				exitCode = 1
			}
		}
		needBlank = true
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// listDir reads and prints the contents of a single directory.
// R1.3: entries sorted lexicographically in C locale byte order.
// R1.4: dotfiles hidden by default.
func listDir(dir string, isTTY bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		// R1.4 / R1.3: hide dotfiles by default.
		if strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}

	// R1.3 / D5: sort in C locale byte order (Go sort.Strings is byte-order).
	sort.Strings(names)

	printEntries(names, isTTY)
	return nil
}

// printEntries outputs names in multi-column format (terminal) or one-per-line (pipe).
// R1.1: multi-column when stdout is a TTY.
// R1.2: single-column when stdout is not a TTY.
func printEntries(names []string, isTTY bool) {
	if len(names) == 0 {
		return
	}

	if !isTTY {
		// R1.2: one entry per line for non-terminal output.
		for _, name := range names {
			fmt.Println(name)
		}
		return
	}

	// R1.1 / D3: multi-column output using pkg/format.Columns.
	termWidth, err := sys.TerminalWidth()
	if err != nil {
		termWidth = defaultTermWidth
	}

	rows := format.Columns(names, termWidth)
	for _, row := range rows {
		fmt.Println(strings.Join(row, " "))
	}
}

// unwrapMsg extracts a user-friendly error message, stripping Go error prefixes.
func unwrapMsg(err error) string {
	msg := err.Error()
	// Strip common prefixes like "lstat /path: " added by os/sys wrappers.
	if idx := strings.LastIndex(msg, ": "); idx >= 0 {
		return msg[idx+2:]
	}
	return msg
}
