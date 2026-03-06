// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the cmd/ls binary.
// Lists directory contents with support for filtering, sorting, and
// multi-column or single-column output formatting.
//
// Implements: prd008-ls R1-R7 (core flags), prd010-ls-extended R2 (sort), R3 (time selection), R4 (time-style)
// Architecture: docs/ARCHITECTURE.yaml § cmd/
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

// timeStyle controls the timestamp format in long listing mode.
type timeStyle int

const (
	timeStyleDefault timeStyle = iota // default ls format (Jan _2 15:04 or Jan _2  2006)
	timeStyleFullISO                  // --time-style=full-iso
	timeStyleLongISO                  // --time-style=long-iso
	timeStyleISO                      // --time-style=iso
	timeStyleLocale                   // --time-style=locale (same as default under LC_ALL=C, DD6)
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
	filter    filterMode
	sortBy    sortMode
	reverse   bool // -r/--reverse: reverse sort order (R2)
	output    outputMode
	timeField timeField // -c, -u, --time=WORD: timestamp selection (R3)
	timeStyle timeStyle // --time-style=STYLE, --full-time: timestamp format (R4)
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
// (e.g., -aSr) and long flags (--reverse, --sort=none, --time=WORD,
// --time-style=STYLE, --full-time). Returns the resolved config, operand
// paths, and any parse error. Follows the manual parsing pattern established
// in cmd/wc.
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
			switch {
			case arg == "--reverse":
				cfg.reverse = true
			case arg == "--full-time":
				// R4: --full-time is an alias for --time-style=full-iso.
				cfg.timeStyle = timeStyleFullISO
			case arg == "--sort=none":
				// R1: --sort=none is an alias for -U.
				cfg.sortBy = sortNone
			case strings.HasPrefix(arg, "--sort="):
				return lsConfig{}, nil, fmt.Errorf("invalid argument '%s' for '--sort'", arg[len("--sort="):])
			case strings.HasPrefix(arg, "--time-style="):
				ts, err := parseTimeStyle(arg[len("--time-style="):])
				if err != nil {
					return lsConfig{}, nil, err
				}
				cfg.timeStyle = ts
			case strings.HasPrefix(arg, "--time="):
				tf, err := parseTimeWord(arg[len("--time="):])
				if err != nil {
					return lsConfig{}, nil, err
				}
				cfg.timeField = tf
			default:
				return lsConfig{}, nil, fmt.Errorf("unrecognized option '%s'", arg)
			}
			continue
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
					// R2.6: last sort flag wins.
					cfg.sortBy = sortByTime
				case 'S':
					cfg.sortBy = sortBySize
				case 'X':
					cfg.sortBy = sortByExtension
				case 'U':
					// R2.4: directory order, no sorting.
					cfg.sortBy = sortNone
				case 'c':
					// R3: select status change time (ctime).
					cfg.timeField = timeChanged
				case 'u':
					// R3: select access time (atime).
					cfg.timeField = timeAccessed
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

// parseTimeStyle maps a --time-style=STYLE argument to a timeStyle value.
func parseTimeStyle(style string) (timeStyle, error) {
	switch style {
	case "full-iso":
		return timeStyleFullISO, nil
	case "long-iso":
		return timeStyleLongISO, nil
	case "iso":
		return timeStyleISO, nil
	case "locale":
		return timeStyleLocale, nil
	default:
		return 0, fmt.Errorf("invalid argument '%s' for '--time-style'", style)
	}
}

// parseTimeWord maps a --time=WORD argument to a timeField value.
func parseTimeWord(word string) (timeField, error) {
	switch word {
	case "mtime", "modification":
		return timeModified, nil
	case "ctime", "status":
		return timeChanged, nil
	case "atime", "access", "use":
		return timeAccessed, nil
	default:
		return 0, fmt.Errorf("invalid argument '%s' for '--time'", word)
	}
}

// readDir reads directory entries and applies the active filter.
// For filterAll, "." and ".." are prepended to the listing since
// os.ReadDir omits them.
func readDir(dirPath string, cfg lsConfig) ([]fileEntry, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	needStat := cfg.sortBy == sortByTime || cfg.sortBy == sortBySize
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

// formatTime formats a timestamp according to the active time style.
// R4: --time-style controls the format in long listing mode. The default
// and locale styles produce the same output under LC_ALL=C (DD6).
func formatTime(t time.Time, style timeStyle, now time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	recent := !t.After(now) && now.Sub(t) < sixMonths

	switch style {
	case timeStyleFullISO:
		return t.Format("2006-01-02 15:04:05.000000000 -0700")
	case timeStyleLongISO:
		return t.Format("2006-01-02 15:04")
	case timeStyleISO:
		if recent {
			return t.Format("01-02 15:04")
		}
		return t.Format("2006-01-02 ")
	default:
		// timeStyleDefault and timeStyleLocale: same format under LC_ALL=C (DD6).
		if recent {
			return t.Format("Jan _2 15:04")
		}
		return t.Format("Jan _2  2006")
	}
}
