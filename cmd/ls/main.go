// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/ls binary.
// Lists directory contents with support for filtering, sorting, and
// multi-column or single-column output formatting.
//
// Implements: prd008-ls R1-R7 (core flags)
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error diagnostics (R6).
const progName = "ls"

// defaultTermWidth is the terminal width used when stdout is not a TTY
// or when terminal width cannot be determined (D1).
const defaultTermWidth = 80

// filterMode determines which entries are included in the listing.
type filterMode int

const (
	filterDefault    filterMode = iota // hide entries starting with '.' (R1.4)
	filterAll                         // -a: show all including '.' and '..' (R3.1)
	filterAlmostAll                   // -A: show all except '.' and '..' (R3.2)
)

// sortMode determines the primary sort key.
type sortMode int

const (
	sortByName sortMode = iota // default: byte order (R4, D3)
	sortByTime                 // -t: newest first (R4)
	sortBySize                 // -S: largest first (R4)
)

// outputMode determines the output format.
type outputMode int

const (
	outputAuto    outputMode = iota // multi-column on TTY, single-column otherwise (R5)
	outputSingle                   // -1: force single-column (R5)
	outputColumns                  // -C: force multi-column (R5)
)

// lsConfig holds the resolved options for a single ls invocation.
type lsConfig struct {
	filter  filterMode
	sortBy  sortMode
	reverse bool // -r: reverse sort order (R4)
	output  outputMode
}

// fileEntry holds the name and optional metadata for a listed entry.
type fileEntry struct {
	name string
	info *sys.FileInfo
}

func main() {
	// R7.1: SIGPIPE handler so piping to head exits cleanly.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGPIPE)
	go func() {
		<-sigCh
		os.Exit(0)
	}()

	cfg, operands, parseErr := parseFlags()
	if parseErr != nil {
		// R6: exit 2 for invalid options.
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, parseErr)
		os.Exit(2)
	}

	// R1: default to current directory when no operands.
	if len(operands) == 0 {
		operands = []string{"."}
	}

	exitCode := 0
	var fileOps []fileEntry
	var dirOps []fileEntry

	// R2: classify operands into files and directories.
	for _, op := range operands {
		fi, err := sys.Lstat(op)
		if err != nil {
			// R6: print error, set exit code, continue.
			fmt.Fprintf(os.Stderr, "%s: cannot access '%s': %v\n", progName, op, sysErr(err))
			exitCode = 2
			continue
		}
		entry := fileEntry{name: op, info: fi}
		if fi.Mode.IsDir() {
			dirOps = append(dirOps, entry)
		} else {
			fileOps = append(fileOps, entry)
		}
	}

	// R2: directory header when more than one operand is given.
	showHeader := len(operands) > 1
	needNewline := false

	// R2: list file operands first, sorted.
	if len(fileOps) > 0 {
		sortEntries(fileOps, cfg)
		printEntries(entryNames(fileOps), cfg)
		needNewline = true
	}

	// R2: list directory operands, sorted.
	sortEntries(dirOps, cfg)
	for _, de := range dirOps {
		if needNewline {
			fmt.Println()
		}
		if showHeader {
			fmt.Printf("%s:\n", de.name)
		}
		entries, err := readDir(de.name, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot open directory '%s': %v\n",
				progName, de.name, sysErr(err))
			exitCode = 2
			needNewline = true
			continue
		}
		sortEntries(entries, cfg)
		printEntries(entryNames(entries), cfg)
		needNewline = true
	}

	os.Exit(exitCode)
}

// parseFlags parses command-line arguments, supporting combined short flags
// (e.g., -aSr). Returns the resolved config, operand paths, and any parse
// error. Follows the manual parsing pattern established in cmd/wc.
func parseFlags() (lsConfig, []string, error) {
	cfg := lsConfig{}
	var operands []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// "--" terminates flag parsing.
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}

		// Long flags.
		if strings.HasPrefix(arg, "--") {
			return lsConfig{}, nil, fmt.Errorf("unrecognized option '%s'", arg)
		}

		// Short flags (may be combined: -aSr).
		if len(arg) > 1 && arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					// R3.1, R3.4: last of -a/-A wins.
					cfg.filter = filterAll
				case 'A':
					// R3.2, R3.4: last of -a/-A wins.
					cfg.filter = filterAlmostAll
				case 'r':
					cfg.reverse = true
				case 't':
					cfg.sortBy = sortByTime
				case 'S':
					cfg.sortBy = sortBySize
				case '1':
					cfg.output = outputSingle
				case 'C':
					cfg.output = outputColumns
				default:
					return lsConfig{}, nil, fmt.Errorf("invalid option -- '%c'", ch)
				}
			}
			continue
		}

		operands = append(operands, arg)
	}

	return cfg, operands, nil
}

// readDir reads directory entries and applies the active filter.
// For filterAll, "." and ".." are prepended to the listing since
// os.ReadDir omits them.
func readDir(dirPath string, cfg lsConfig) ([]fileEntry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	needStat := cfg.sortBy != sortByName
	var entries []fileEntry

	// R3.1: -a includes "." and ".." which os.ReadDir omits.
	if cfg.filter == filterAll {
		entries = append(entries, makeEntry(dirPath, ".", needStat))
		entries = append(entries, makeEntry(dirPath, "..", needStat))
	}

	for _, de := range dirEntries {
		name := de.Name()

		// R1.4: default filter hides entries starting with '.'.
		if cfg.filter == filterDefault && strings.HasPrefix(name, ".") {
			continue
		}

		entries = append(entries, makeEntry(dirPath, name, needStat))
	}

	return entries, nil
}

// makeEntry creates a fileEntry, stat'ing the file when needStat is true
// to obtain metadata for time or size sorting.
func makeEntry(dirPath, name string, needStat bool) fileEntry {
	entry := fileEntry{name: name}
	if needStat {
		path := joinPath(dirPath, name)
		fi, err := sys.Lstat(path)
		if err == nil {
			entry.info = fi
		}
	}
	return entry
}

// joinPath constructs a child path preserving the parent directory's prefix
// form. Avoids filepath.Join which cleans ".." components, breaking stat of
// "dir/.." entries needed for -a.
func joinPath(dir, name string) string {
	if len(dir) > 0 && dir[len(dir)-1] == '/' {
		return dir + name
	}
	return dir + "/" + name
}

// sortEntries sorts entries in place according to the active sort mode.
// R4: default is byte order; -t sorts by mtime newest first; -S sorts by
// size largest first. Ties are broken by name. -r reverses any sort.
func sortEntries(entries []fileEntry, cfg lsConfig) {
	if len(entries) < 2 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		less := compareEntries(entries[i], entries[j], cfg.sortBy)
		if cfg.reverse {
			return !less
		}
		return less
	})
}

// compareEntries returns true when a should appear before b under the
// given sort mode. Falls back to name comparison when metadata is missing
// or values are equal (tie-breaking by name per R4).
func compareEntries(a, b fileEntry, sm sortMode) bool {
	switch sm {
	case sortByTime:
		if a.info == nil || b.info == nil {
			return a.name < b.name
		}
		if !a.info.ModTime.Equal(b.info.ModTime) {
			return a.info.ModTime.After(b.info.ModTime)
		}
		return a.name < b.name
	case sortBySize:
		if a.info == nil || b.info == nil {
			return a.name < b.name
		}
		if a.info.Size != b.info.Size {
			return a.info.Size > b.info.Size
		}
		return a.name < b.name
	default:
		return a.name < b.name
	}
}

// entryNames extracts the names from a slice of file entries.
func entryNames(entries []fileEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

// printEntries outputs names in single-column or multi-column format
// depending on the active output mode and whether stdout is a TTY.
func printEntries(names []string, cfg lsConfig) {
	if len(names) == 0 {
		return
	}

	if useSingleColumn(cfg) {
		for _, name := range names {
			fmt.Println(name)
		}
		return
	}

	// R5: multi-column output using pkg/format.
	width := termWidth()
	grid := format.Columns(names, width)
	for _, row := range grid {
		line := strings.Join(row, "  ")
		fmt.Println(strings.TrimRight(line, " "))
	}
}

// useSingleColumn returns true when output should be one entry per line.
func useSingleColumn(cfg lsConfig) bool {
	switch cfg.output {
	case outputSingle:
		return true
	case outputColumns:
		return false
	default:
		// R5: auto-detect based on stdout.
		return !isStdoutTTY()
	}
}

// isStdoutTTY reports whether stdout is connected to a terminal (D4).
func isStdoutTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// termWidth returns the current terminal column count, falling back to
// defaultTermWidth when the width cannot be determined (D1).
func termWidth() int {
	w, err := sys.TerminalWidth()
	if err != nil || w <= 0 {
		return defaultTermWidth
	}
	return w
}

// sysErr extracts the underlying system error from an os.PathError for
// cleaner diagnostic messages matching GNU ls format.
func sysErr(err error) error {
	if pe, ok := err.(*os.PathError); ok { //nolint:errorlint
		return pe.Err
	}
	return err
}
